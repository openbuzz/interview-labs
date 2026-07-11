package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// clientIP resolves the caller's address. Behind a trusted proxy, the leftmost
// X-Forwarded-For entry is used; otherwise the spoofable header is ignored.
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

func (s *server) proxy(target *url.URL, prefix string) http.Handler {
	rp := httputil.NewSingleHostReverseProxy(target)
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		s.log.Error("proxy upstream error",
			"upstream", target.String(), "path", r.URL.Path, "err", err.Error())
		w.WriteHeader(http.StatusBadGateway)
	}

	return http.StripPrefix(prefix, rp)
}

// statusRecorder captures the status code and preserves Hijacker so WebSocket
// upgrades through the reverse proxy keep working under the access-log wrapper.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijack unsupported")
	}

	return hj.Hijack()
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		level := slog.LevelInfo
		if r.URL.Path == "/healthz" || r.URL.Path == "/styles.css" {
			level = slog.LevelDebug
		}

		s.log.Log(r.Context(), level, "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"ip", clientIP(r, s.cfg.trustProxy),
			"dur", time.Since(start).String(),
		)
	})
}
