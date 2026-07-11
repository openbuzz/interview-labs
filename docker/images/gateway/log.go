package main

import (
	"log/slog"
	"os"
	"strings"
)

func newLogger(level, format string) *slog.Logger {
	var lvl slog.Level

	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler = slog.NewTextHandler(os.Stderr, opts)
	if strings.ToLower(format) == "json" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	}

	return slog.New(h)
}
