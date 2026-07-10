package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileIsZero(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c, err := Load()
	if err != nil || c != (Config{}) {
		t.Fatalf("Load() = %+v, %v; want zero, nil", c, err)
	}
}

func TestWriteThenLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c := Config{}
	c.Providers.DigitalOcean.Token = "dop_v1_abc"
	if err := c.Write(); err != nil {
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
	got, err := Load()
	if err != nil || got.Providers.DigitalOcean.Token != "dop_v1_abc" {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
}

func TestRoundTripProvidersTree(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	in := Config{}
	in.Providers.DigitalOcean = DigitalOcean{
		Token: "tok", Region: "fra1", Instance: "s-1vcpu-1gb",
	}
	in.Roles.VM = "digitalocean"
	if err := in.Write(); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != in {
		t.Fatalf("round trip = %+v, want %+v", got, in)
	}
}

func TestTokenEnvBeatsFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "env-tok")
	c := Config{}
	c.Providers.DigitalOcean.Token = "file-tok"
	if got := c.Token(); got != "env-tok" {
		t.Fatalf("Token() = %q, want env-tok", got)
	}
}

func TestLegacyFlatKeyIgnored(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	p := filepath.Join(dir, "interview", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("digitalocean_token: old\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DIGITALOCEAN_TOKEN", "")
	if got.Token() != "" {
		t.Fatalf("legacy key leaked into token: %q", got.Token())
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
