package main

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testPassword = "openbuzz"

func TestTokenRoundTrip(t *testing.T) {
	secret := []byte("test-secret-please-ignore")
	now := time.Unix(1_000_000, 0)
	exp := now.Add(time.Hour).Unix()
	tok := signToken(secret, exp)
	if !validToken(secret, tok, now) {
		t.Fatalf("freshly signed token should be valid")
	}
}

func TestTokenExpired(t *testing.T) {
	secret := []byte("test-secret-please-ignore")
	now := time.Unix(1_000_000, 0)
	exp := now.Add(-time.Second).Unix() // already expired
	tok := signToken(secret, exp)
	if validToken(secret, tok, now) {
		t.Fatalf("expired token must be rejected")
	}
}

func TestTokenTampered(t *testing.T) {
	secret := []byte("test-secret-please-ignore")
	now := time.Unix(1_000_000, 0)
	exp := now.Add(time.Hour).Unix()
	tok := signToken(secret, exp)
	// Extend the expiry without re-signing: HMAC must no longer match.
	forged := signTokenExp(now.Add(10*time.Hour).Unix(), tok)
	if validToken(secret, forged, now) {
		t.Fatalf("tampered expiry must be rejected")
	}
}

func TestTokenWrongSecret(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	exp := now.Add(time.Hour).Unix()
	tok := signToken([]byte("secret-a"), exp)
	if validToken([]byte("secret-b"), tok, now) {
		t.Fatalf("token signed with a different secret must be rejected")
	}
}

func TestLimiterBurstThenBlock(t *testing.T) {
	l := newLimiter(5, 30*time.Second)
	now := time.Unix(2_000_000, 0)
	for i := 0; i < 5; i++ {
		if !l.allow("1.2.3.4", now) {
			t.Fatalf("attempt %d within burst should be allowed", i+1)
		}
	}
	if l.allow("1.2.3.4", now) {
		t.Fatalf("6th attempt in the same instant must be blocked")
	}
}

func TestLimiterRefillsAfterWindow(t *testing.T) {
	l := newLimiter(5, 30*time.Second)
	now := time.Unix(2_000_000, 0)
	for i := 0; i < 5; i++ {
		l.allow("1.2.3.4", now)
	}
	if l.allow("1.2.3.4", now) {
		t.Fatalf("bucket should be empty")
	}

	later := now.Add(31 * time.Second) // full window elapsed -> fully refilled
	if !l.allow("1.2.3.4", later) {
		t.Fatalf("attempt after the window should be allowed again")
	}
}

func TestLimiterPerKey(t *testing.T) {
	l := newLimiter(1, 30*time.Second)
	now := time.Unix(2_000_000, 0)
	if !l.allow("a", now) || !l.allow("b", now) {
		t.Fatalf("distinct keys must not share a bucket")
	}
	if l.allow("a", now) {
		t.Fatalf("key a should now be exhausted")
	}
}

func TestLimiterSweep(t *testing.T) {
	l := newLimiter(5, 30*time.Second)
	now := time.Unix(2_000_000, 0)
	l.allow("stale", now)
	l.sweep(now.Add(61 * time.Second)) // > 2*window
	if got := l.size(); got != 0 {
		t.Fatalf("stale bucket should have been swept, size=%d", got)
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.9:5555"
	if got := clientIP(r, false); got != "10.0.0.9" {
		t.Fatalf("RemoteAddr host expected, got %q", got)
	}

	r.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	if got := clientIP(r, false); got != "10.0.0.9" {
		t.Fatalf("XFF must be ignored when trustProxy=false, got %q", got)
	}

	if got := clientIP(r, true); got != "203.0.113.7" {
		t.Fatalf("leftmost XFF expected when trustProxy=true, got %q", got)
	}
}

// signTokenExp swaps in a new exp prefix while keeping the original HMAC suffix,
// simulating an attacker editing the expiry of a stolen cookie.
func signTokenExp(newExp int64, original string) string {
	for i := len(original) - 1; i >= 0; i-- {
		if original[i] == '.' {
			return itoa(newExp) + original[i:]
		}
	}
	return original
}

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("GATEWAY_PASSWORD", "pw")
	// Ensure optional vars are unset for this test.
	for _, k := range []string{
		"GATEWAY_ADDR", "GATEWAY_SECRET", "GATEWAY_TTL_MINUTES",
		"GATEWAY_IDE_UPSTREAM", "GATEWAY_TERM_UPSTREAM", "GATEWAY_SECURE_COOKIE",
		"GATEWAY_LOGIN_BURST", "GATEWAY_LOGIN_WINDOW_SECONDS", "GATEWAY_TRUST_PROXY",
	} {
		t.Setenv(k, "")
	}

	c, err := loadConfig(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []struct {
		name string
		ok   bool
	}{
		{"addr", c.addr == ":8080"},
		{"ttl", c.ttl == 120*time.Minute},
		{"loginBurst", c.loginBurst == 5},
		{"loginWindow", c.loginWindow == 30*time.Second},
		{"secureCookie", !c.secureCookie},
		{"trustProxy", !c.trustProxy},
		{"ideUpstream", c.ideUpstream.Host == "vscode:8080"},
		{"termUpstream", c.termUpstream.Host == "vscode:7681"},
		{"secretLen", len(c.secret) == 32},
		{"secretGenerated", c.secretGenerated},
	}
	for _, ch := range checks {
		if !ch.ok {
			t.Errorf("default %s wrong: %+v", ch.name, c)
		}
	}
}

func TestLoadConfigRequiresPassword(t *testing.T) {
	t.Setenv("GATEWAY_PASSWORD", "")
	if _, err := loadConfig(nil); err == nil {
		t.Fatalf("empty GATEWAY_PASSWORD must be an error")
	}
}

func TestLoadConfigPinnedSecret(t *testing.T) {
	t.Setenv("GATEWAY_PASSWORD", "pw")
	t.Setenv("GATEWAY_SECRET", "pinned-secret")

	c, err := loadConfig(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.secretGenerated || string(c.secret) != "pinned-secret" {
		t.Fatalf("pinned secret should be used verbatim, generated=%v", c.secretGenerated)
	}
}

func TestSafeNext(t *testing.T) {
	cases := map[string]string{
		"/ide/foo":         "/ide/foo",
		"/term/":           "/term/",
		"":                 "/",
		"//evil.com":       "/",
		"https://evil.com": "/",
		"/\\evil.com":      "/",
		"javascript:alert": "/",
	}

	for in, want := range cases {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", in, got, want)
		}
	}
}

func testServer(t *testing.T, upstreamBody string) (*server, *httptest.Server) {
	t.Helper()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo the path the upstream actually received, to prove prefix stripping.
		w.Write([]byte(upstreamBody + " path=" + r.URL.Path))
	}))

	t.Cleanup(up.Close)
	u, _ := url.Parse(up.URL)
	c := &config{
		addr:         ":0",
		password:     testPassword,
		secret:       []byte("unit-test-secret"),
		ttl:          time.Hour,
		ideUpstream:  u,
		termUpstream: u,
		loginBurst:   5,
		loginWindow:  30 * time.Second,
		logLevel:     "error",
		logFormat:    "text",
	}
	return newServer(c, slog.New(slog.NewTextHandler(io.Discard, nil))), up
}

func TestHealthzPublic(t *testing.T) {
	s, _ := testServer(t, "ide")
	rr := httptest.NewRecorder()
	s.handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("healthz = %d, want 200", rr.Code)
	}
}

func TestGuardedRouteRedirectsWhenUnauthenticated(t *testing.T) {
	s, _ := testServer(t, "ide")
	rr := httptest.NewRecorder()
	s.handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/ide/foo", nil))
	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rr.Code)
	}
	if loc := rr.Header().Get("Location"); !strings.HasPrefix(loc, "/login") {
		t.Fatalf("redirect = %q, want /login...", loc)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	s, _ := testServer(t, "ide")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("password=nope"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if len(rr.Result().Cookies()) != 0 {
		t.Fatalf("no cookie should be set on failure")
	}
}

func TestLoginSuccessSetsCookie(t *testing.T) {
	s, _ := testServer(t, "ide")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("password="+testPassword))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rr.Code)
	}

	var found bool
	for _, ck := range rr.Result().Cookies() {
		if ck.Name == cookieName && ck.HttpOnly {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected HttpOnly %s cookie", cookieName)
	}
}

func TestValidCookieProxiesStripped(t *testing.T) {
	s, _ := testServer(t, "ide-sim")

	// Stand up a distinct term upstream to prove /term reaches a different server.
	termUp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("term-sim path=" + r.URL.Path))
	}))
	t.Cleanup(termUp.Close)
	termURL, _ := url.Parse(termUp.URL)
	s.cfg.termUpstream = termURL

	exp := time.Now().Add(time.Hour).Unix()
	for path, want := range map[string]string{
		"/ide/foo":  "ide-sim path=/foo",
		"/term/bar": "term-sim path=/bar",
	} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(&http.Cookie{Name: cookieName, Value: signToken(s.cfg.secret, exp)})
		s.handler().ServeHTTP(rr, req)
		if body := rr.Body.String(); body != want {
			t.Fatalf("GET %s body = %q, want %q (proves prefix strip + distinct upstream)", path, body, want)
		}
	}
}

func TestForgedCookieRejected(t *testing.T) {
	s, _ := testServer(t, "ide")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ide/foo", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "9999999999.deadbeef"})
	s.handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("forged cookie status = %d, want 302", rr.Code)
	}
}

func TestHealthzLogLevel(t *testing.T) {
	u, _ := url.Parse("http://127.0.0.1:1")
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	s := newServer(&config{ideUpstream: u, termUpstream: u}, log)

	h := s.handler()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/login", nil))

	out := buf.String()
	if strings.Contains(out, "path=/healthz") {
		t.Fatalf("/healthz must not log at info level:\n%s", out)
	}
	if !strings.Contains(out, "path=/login") {
		t.Fatalf("/login should log at info level:\n%s", out)
	}
}

func TestLoginRateLimited(t *testing.T) {
	s, _ := testServer(t, "ide")
	var last int

	for i := 0; i < 7; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("password=nope"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = "5.5.5.5:1234"
		s.handler().ServeHTTP(rr, req)
		last = rr.Code
	}

	if last != http.StatusTooManyRequests {
		t.Fatalf("after burst, status = %d, want 429", last)
	}
}

func TestRootServesDefaultLanding(t *testing.T) {
	s, _ := testServer(t, "ide") // no landingPage configured -> default stub

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name: cookieName, Value: signToken(s.cfg.secret, time.Now().Add(time.Hour).Unix()),
	})
	s.handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type = %q, want text/html", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "interview-labs workspace") {
		t.Fatalf("default landing missing expected content")
	}
	if !strings.Contains(body, `href="/styles.css"`) {
		t.Fatalf("default landing not wrapped by shell (no stylesheet link)")
	}
}

func TestRootServesLandingPage(t *testing.T) {
	s, _ := testServer(t, "ide")
	page := filepath.Join(t.TempDir(), "fragment.html")
	if err := os.WriteFile(page, []byte("<h1>welcome</h1>"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.cfg.landingPage = page

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name: cookieName, Value: signToken(s.cfg.secret, time.Now().Add(time.Hour).Unix()),
	})
	s.handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type = %q, want text/html", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "welcome") {
		t.Fatalf("body = %q, want fragment content", body)
	}
	if !strings.Contains(body, `href="/styles.css"`) {
		t.Fatalf("fragment not wrapped by the landing shell")
	}
}

func TestStylesServedPublicly(t *testing.T) {
	s, _ := testServer(t, "ide")
	rr := httptest.NewRecorder()
	s.handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/styles.css", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (no cookie required)", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Fatalf("content-type = %q, want text/css", ct)
	}
	if !strings.Contains(rr.Body.String(), "--accent") {
		t.Fatalf("stylesheet body missing --accent token")
	}
}

func TestLoginPageLinksStylesheet(t *testing.T) {
	s, _ := testServer(t, "ide")
	rr := httptest.NewRecorder()
	s.handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/login", nil))

	body := rr.Body.String()
	if !strings.Contains(body, `href="/styles.css"`) {
		t.Fatalf("login page missing stylesheet link")
	}
	if strings.Contains(body, "<style") {
		t.Fatalf("login page must not carry inline <style>")
	}
	if !strings.Contains(body, "Sign in") {
		t.Fatalf("login page lost the 'Sign in' text")
	}
}

func TestRootRedirectsUnknownPath(t *testing.T) {
	s, _ := testServer(t, "ide")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whatever", nil)
	req.AddCookie(&http.Cookie{
		Name: cookieName, Value: signToken(s.cfg.secret, time.Now().Add(time.Hour).Unix()),
	})
	s.handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/ide/" {
		t.Fatalf("redirect = %q, want /ide/", loc)
	}
}
