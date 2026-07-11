package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"
)

func main() {
	cfg, err := loadConfig(os.Args[1:])
	if err != nil {
		slog.Error("startup failed", "err", err.Error())
		os.Exit(1)
	}

	if cfg.healthcheck {
		os.Exit(runHealthcheck(cfg.addr, cfg.tlsEnabled()))
	}

	log := newLogger(cfg.logLevel, cfg.logFormat)
	srv := newServer(cfg, log)

	// Bound the limiter map: sweep stale buckets on an interval.
	go func() {
		t := time.NewTicker(cfg.loginWindow)
		defer t.Stop()
		for range t.C {
			srv.lim.sweep(time.Now())
		}
	}()

	log.Info("gateway starting",
		"addr", cfg.addr,
		"ttl", cfg.ttl.String(),
		"ide_upstream", cfg.ideUpstream.String(),
		"term_upstream", cfg.termUpstream.String(),
		"trust_proxy", cfg.trustProxy,
		"secure_cookie", cfg.secureCookie,
		"login_burst", cfg.loginBurst,
		"login_window", cfg.loginWindow.String(),
		"secret_generated", cfg.secretGenerated,
		"tls", cfg.tlsEnabled(),
	)

	var serveErr error
	if cfg.tlsEnabled() {
		serveErr = http.ListenAndServeTLS(cfg.addr, cfg.tlsCert, cfg.tlsKey, srv.handler())
	} else {
		serveErr = http.ListenAndServe(cfg.addr, srv.handler())
	}

	if serveErr != nil {
		log.Error("server exited", "err", serveErr.Error())
		os.Exit(1)
	}
}
