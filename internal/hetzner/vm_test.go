package hetzner

import (
	"testing"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
)

var _ provider.VM = hz{}

func TestEnvCredsEnvBeatsFile(t *testing.T) {
	var cfg config.Config
	cfg.Providers.Hetzner.Token = "file-tok"

	t.Setenv("HCLOUD_TOKEN", "env-tok")
	if got := (hz{}).EnvCreds(cfg)["HCLOUD_TOKEN"]; got != "env-tok" {
		t.Fatalf("env creds = %q, want env-tok", got)
	}

	t.Setenv("HCLOUD_TOKEN", "")
	if got := (hz{}).EnvCreds(cfg)["HCLOUD_TOKEN"]; got != "file-tok" {
		t.Fatalf("file creds = %q, want file-tok", got)
	}
}

func TestIdentityAndStatics(t *testing.T) {
	p := New()
	if p.Name() != "hetzner" || p.Label() != "Hetzner" {
		t.Fatalf("identity = %s/%s", p.Name(), p.Label())
	}
	if got := (hz{}).Image(); got != "ubuntu-26.04" {
		t.Fatalf("image = %q", got)
	}
	if got := (hz{}).SSHUser(); got != "root" {
		t.Fatalf("ssh user = %q", got)
	}
}

func TestDefaultsRoundTrip(t *testing.T) {
	var cfg config.Config
	(hz{}).SetDefaults(&cfg, "fsn1", "cx22")

	r, s := (hz{}).Defaults(cfg)
	if r != "fsn1" || s != "cx22" {
		t.Fatalf("defaults = %q/%q", r, s)
	}
}
