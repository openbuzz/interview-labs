package main

import (
	"crypto/subtle"
	_ "embed"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type server struct {
	cfg *config
	log *slog.Logger
	lim *limiter
	tpl *template.Template
}

//go:embed login.html
var loginHTML string

//go:embed styles.css
var stylesCSS string

//go:embed landing.html
var landingShellHTML string

//go:embed landing-content.html
var landingDefault string

var loginTemplate = template.Must(template.New("login").Parse(loginHTML))

var landingTemplate = template.Must(template.New("landing").Parse(landingShellHTML))

func newServer(c *config, log *slog.Logger) *server {
	return &server{
		cfg: c,
		log: log,
		lim: newLimiter(c.loginBurst, c.loginWindow),
		tpl: loginTemplate,
	}
}

func (s *server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /login", s.loginForm)
	mux.HandleFunc("POST /login", s.loginSubmit)
	mux.HandleFunc("GET /styles.css", s.styles)
	mux.Handle("/ide/", s.requireAuth(s.proxy(s.cfg.ideUpstream, "/ide")))
	mux.Handle("/term/", s.requireAuth(s.proxy(s.cfg.termUpstream, "/term")))
	mux.Handle("/", s.requireAuth(http.HandlerFunc(s.root)))

	return s.accessLog(mux)
}

func (s *server) root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/ide/", http.StatusFound)
		return
	}

	frag := landingDefault
	if s.cfg.landingPage != "" {
		b, err := os.ReadFile(s.cfg.landingPage)
		if err != nil {
			// Boot already validated the path; a read error here means the file
			// vanished mid-life. Keep the page up with the default stub.
			s.log.Error("read landing fragment", "path", s.cfg.landingPage, "err", err.Error())
		} else {
			frag = string(b)
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = landingTemplate.Execute(w, struct{ Content template.HTML }{template.HTML(frag)})
}

func (s *server) renderLogin(w http.ResponseWriter, status int, errMsg, next string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = s.tpl.Execute(w, struct{ Error, Next string }{errMsg, next})
}

func (s *server) styles(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(stylesCSS))
}

func (s *server) loginForm(w http.ResponseWriter, r *http.Request) {
	s.renderLogin(w, http.StatusOK, "", safeNext(r.URL.Query().Get("next")))
}

func (s *server) loginSubmit(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r, s.cfg.trustProxy)
	if !s.lim.allow(ip, time.Now()) {
		w.Header().Set("Retry-After", itoa(int64(s.cfg.loginWindow.Seconds())))
		s.log.Warn("login rate limited", "ip", ip)
		http.Error(w, "too many attempts", http.StatusTooManyRequests)
		return
	}

	next := safeNext(r.PostFormValue("next"))
	pw := r.PostFormValue("password")
	if subtle.ConstantTimeCompare([]byte(pw), []byte(s.cfg.password)) != 1 {
		s.log.Warn("login failed", "ip", ip)
		s.renderLogin(w, http.StatusUnauthorized, "Incorrect password.", next)
		return
	}

	exp := time.Now().Add(s.cfg.ttl).Unix()
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    signToken(s.cfg.secret, exp),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.secureCookie,
		MaxAge:   int(s.cfg.ttl.Seconds()),
	})

	s.log.Info("login success", "ip", ip)
	http.Redirect(w, r, next, http.StatusFound)
}

func (s *server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil || !validToken(s.cfg.secret, c.Value, time.Now()) {
			s.log.Debug("auth rejected", "path", r.URL.Path)
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.Path), http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// safeNext restricts post-login redirects to a local absolute path, blocking open redirects.
func safeNext(next string) string {
	if next == "" || next[0] != '/' {
		return "/"
	}
	if strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		return "/"
	}

	return next
}
