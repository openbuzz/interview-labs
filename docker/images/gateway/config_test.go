package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigTLSBothSet(t *testing.T) {
	t.Setenv("GATEWAY_PASSWORD", "pw")
	t.Setenv("GATEWAY_TLS_CERT", "/c.pem")
	t.Setenv("GATEWAY_TLS_KEY", "/k.pem")

	c, err := loadConfig(nil)
	if err != nil {
		t.Fatalf("both set should load: %v", err)
	}

	if !c.tlsEnabled() {
		t.Fatalf("tlsEnabled should be true when both set")
	}
}

func TestLoadConfigTLSNeitherSet(t *testing.T) {
	t.Setenv("GATEWAY_PASSWORD", "pw")

	c, err := loadConfig(nil)
	if err != nil {
		t.Fatalf("neither set should load: %v", err)
	}

	if c.tlsEnabled() {
		t.Fatalf("tlsEnabled should be false when neither set")
	}
}

func TestLoadConfigTLSOnlyCert(t *testing.T) {
	t.Setenv("GATEWAY_PASSWORD", "pw")
	t.Setenv("GATEWAY_TLS_CERT", "/c.pem")

	if _, err := loadConfig(nil); err == nil {
		t.Fatalf("only cert set must error")
	}
}

func TestLoadConfigTLSOnlyKey(t *testing.T) {
	t.Setenv("GATEWAY_PASSWORD", "pw")
	t.Setenv("GATEWAY_TLS_KEY", "/k.pem")

	if _, err := loadConfig(nil); err == nil {
		t.Fatalf("only key set must error")
	}
}

func TestLoadConfigFlagOverridesEnv(t *testing.T) {
	t.Setenv("GATEWAY_PASSWORD", "pw")
	t.Setenv("GATEWAY_ADDR", ":9999")

	c, err := loadConfig([]string{"-addr=:7777"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.addr != ":7777" {
		t.Fatalf("flag should beat env: addr = %q, want :7777", c.addr)
	}
}

func TestResolveSecretPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")

	s1, gen1, err := resolveSecret("", path)
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	if !gen1 || len(s1) != 32 {
		t.Fatalf("first resolve should generate 32 bytes, gen=%v len=%d", gen1, len(s1))
	}

	s2, gen2, err := resolveSecret("", path)
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if gen2 {
		t.Fatalf("second resolve should load from file, not generate")
	}
	if !bytes.Equal(s1, s2) {
		t.Fatalf("persisted secret should be stable across restarts")
	}
}

func TestResolveSecretExplicitWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")

	s, gen, err := resolveSecret("pinned", path)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if gen || string(s) != "pinned" {
		t.Fatalf("explicit secret should win, gen=%v secret=%q", gen, s)
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Fatalf("explicit secret must not create the file")
	}
}

func TestLoadConfigLandingPage(t *testing.T) {
	t.Setenv("GATEWAY_PASSWORD", "pw")
	page := filepath.Join(t.TempDir(), "landing.html")
	if err := os.WriteFile(page, []byte("<h1>hi</h1>"), 0o600); err != nil {
		t.Fatal(err)
	}

	c, err := loadConfig([]string{"-landing-page", page})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.landingPage != page {
		t.Fatalf("landingPage = %q, want %q", c.landingPage, page)
	}
}

func TestLoadConfigLandingPageMissing(t *testing.T) {
	t.Setenv("GATEWAY_PASSWORD", "pw")
	if _, err := loadConfig([]string{"-landing-page", "/no/such/file.html"}); err == nil {
		t.Fatalf("missing landing page must be a boot error")
	}
}

func TestResolveSecretWriteError(t *testing.T) {
	notDir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notDir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(notDir, "secret") // parent is a file, not a dir

	if _, _, err := resolveSecret("", bad); err == nil {
		t.Fatalf("write to an impossible path must error")
	}
}
