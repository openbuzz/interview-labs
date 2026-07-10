package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileIsZero(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c, err := Load()
	if err != nil || c.DigitalOceanToken != "" {
		t.Fatalf("Load() = %+v, %v; want zero, nil", c, err)
	}
}

func TestWriteThenLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := (Config{DigitalOceanToken: "dop_v1_abc"}).Write(); err != nil {
		t.Fatal(err)
	}
	p, _ := Path()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %o, want 600", info.Mode().Perm())
	}
	c, err := Load()
	if err != nil || c.DigitalOceanToken != "dop_v1_abc" {
		t.Fatalf("round trip = %+v, %v", c, err)
	}
}

func TestTokenEnvBeatsFile(t *testing.T) {
	t.Setenv("DIGITALOCEAN_TOKEN", "env-token")
	c := Config{DigitalOceanToken: "file-token"}
	if c.Token() != "env-token" {
		t.Fatalf("Token() = %q, want env-token", c.Token())
	}
	t.Setenv("DIGITALOCEAN_TOKEN", "")
	os.Unsetenv("DIGITALOCEAN_TOKEN")
	if c.Token() != "file-token" {
		t.Fatalf("Token() = %q, want file-token", c.Token())
	}
}

func TestPathHonorsXDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if p != filepath.Join(dir, "interview", "config.yaml") {
		t.Fatalf("Path() = %s", p)
	}
}
