package main

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

type config struct {
	addr            string
	password        string
	secret          []byte
	secretGenerated bool
	ttl             time.Duration
	ideUpstream     *url.URL
	termUpstream    *url.URL
	secureCookie    bool
	loginBurst      int
	loginWindow     time.Duration
	trustProxy      bool
	logLevel        string
	logFormat       string
	tlsCert         string
	tlsKey          string
	landingPage     string
	healthcheck     bool
}

func (c *config) tlsEnabled() bool { return c.tlsCert != "" && c.tlsKey != "" }

// resolveSecret picks the cookie-signing key: an explicit value wins; else a
// persisted file is read (and created with a fresh key on first use); else an
// ephemeral random key. A read/write error on a configured file is fatal — it
// never silently falls back to an ephemeral key.
func resolveSecret(explicit, file string) (secret []byte, generated bool, err error) {
	if explicit != "" {
		return []byte(explicit), false, nil
	}

	if file != "" {
		data, readErr := os.ReadFile(file)
		if readErr == nil && len(data) > 0 {
			return data, false, nil
		}
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return nil, false, fmt.Errorf("read secret file: %w", readErr)
		}

		fresh, genErr := randomSecret()
		if genErr != nil {
			return nil, false, genErr
		}
		if writeErr := os.WriteFile(file, fresh, 0o600); writeErr != nil {
			return nil, false, fmt.Errorf("write secret file: %w", writeErr)
		}

		return fresh, true, nil
	}

	fresh, genErr := randomSecret()
	if genErr != nil {
		return nil, false, genErr
	}

	return fresh, true, nil
}

func randomSecret() ([]byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}

	return b, nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

type rawConfig struct {
	addr               *string
	password           *string
	secret             *string
	secretFile         *string
	ttlMinutes         *int
	ideUpstream        *string
	termUpstream       *string
	secureCookie       *bool
	loginBurst         *int
	loginWindowSeconds *int
	trustProxy         *bool
	logLevel           *string
	logFormat          *string
	tlsCert            *string
	tlsKey             *string
	landingPage        *string
	healthcheck        *bool
}

func registerFlags(fs *flag.FlagSet) *rawConfig {
	return &rawConfig{
		addr: fs.String("addr", env("GATEWAY_ADDR", ":8080"), "listen address"),
		password: fs.String("password", os.Getenv("GATEWAY_PASSWORD"),
			"shared login password (required)"),
		secret: fs.String("secret", os.Getenv("GATEWAY_SECRET"), "cookie-signing key; random if empty"),
		secretFile: fs.String("secret-file", env("GATEWAY_SECRET_FILE", ""),
			"persist the auto-generated secret here (survives restart)"),
		ttlMinutes: fs.Int("ttl-minutes", envInt("GATEWAY_TTL_MINUTES", 120),
			"cookie lifetime in minutes"),
		ideUpstream: fs.String("ide-upstream", env("GATEWAY_IDE_UPSTREAM", "http://vscode:8080"),
			"code-server upstream URL"),
		termUpstream: fs.String("term-upstream", env("GATEWAY_TERM_UPSTREAM", "http://vscode:7681"),
			"ttyd upstream URL"),
		secureCookie: fs.Bool("secure-cookie", envBool("GATEWAY_SECURE_COOKIE", false),
			"set Secure on the auth cookie"),
		loginBurst: fs.Int("login-burst", envInt("GATEWAY_LOGIN_BURST", 5),
			"allowed POST /login per IP per window"),
		loginWindowSeconds: fs.Int("login-window-seconds", envInt("GATEWAY_LOGIN_WINDOW_SECONDS", 30),
			"rate-limit window in seconds"),
		trustProxy: fs.Bool("trust-proxy", envBool("GATEWAY_TRUST_PROXY", false),
			"trust X-Forwarded-For (only behind a trusted proxy)"),
		logLevel:  fs.String("log-level", env("GATEWAY_LOG_LEVEL", "info"), "debug|info|warn|error"),
		logFormat: fs.String("log-format", env("GATEWAY_LOG_FORMAT", "text"), "text|json"),
		tlsCert: fs.String("tls-cert", os.Getenv("GATEWAY_TLS_CERT"),
			"path to TLS certificate (set with -tls-key)"),
		tlsKey: fs.String("tls-key", os.Getenv("GATEWAY_TLS_KEY"),
			"path to TLS private key (set with -tls-cert)"),
		landingPage: fs.String("landing-page", env("GATEWAY_LANDING_PAGE", ""),
			"content fragment for the landing shell; unset → default stub"),
		healthcheck: fs.Bool("healthcheck", false, "probe /healthz on the local listener and exit"),
	}
}

func loadConfig(args []string) (*config, error) {
	fs := flag.NewFlagSet("gateway", flag.ContinueOnError)
	r := registerFlags(fs)

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if (*r.tlsCert == "") != (*r.tlsKey == "") {
		return nil, errors.New("GATEWAY_TLS_CERT and GATEWAY_TLS_KEY must be set together")
	}

	if *r.healthcheck {
		return &config{addr: *r.addr, tlsCert: *r.tlsCert, tlsKey: *r.tlsKey, healthcheck: true}, nil
	}

	if *r.password == "" {
		return nil, errors.New("GATEWAY_PASSWORD is required")
	}

	ide, err := url.Parse(*r.ideUpstream)
	if err != nil {
		return nil, fmt.Errorf("GATEWAY_IDE_UPSTREAM: %w", err)
	}

	term, err := url.Parse(*r.termUpstream)
	if err != nil {
		return nil, fmt.Errorf("GATEWAY_TERM_UPSTREAM: %w", err)
	}

	if *r.landingPage != "" {
		f, openErr := os.Open(*r.landingPage)
		if openErr != nil {
			return nil, fmt.Errorf("landing page: %w", openErr)
		}
		f.Close()
	}

	secretBytes, generated, err := resolveSecret(*r.secret, *r.secretFile)
	if err != nil {
		return nil, err
	}

	return &config{
		addr:            *r.addr,
		password:        *r.password,
		secret:          secretBytes,
		secretGenerated: generated,
		ttl:             time.Duration(*r.ttlMinutes) * time.Minute,
		ideUpstream:     ide,
		termUpstream:    term,
		secureCookie:    *r.secureCookie,
		loginBurst:      *r.loginBurst,
		loginWindow:     time.Duration(*r.loginWindowSeconds) * time.Second,
		trustProxy:      *r.trustProxy,
		logLevel:        *r.logLevel,
		logFormat:       *r.logFormat,
		tlsCert:         *r.tlsCert,
		tlsKey:          *r.tlsKey,
		landingPage:     *r.landingPage,
	}, nil
}
