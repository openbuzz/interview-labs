package digitalocean

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/huh"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
)

func TestProviderIdentity(t *testing.T) {
	p := New()
	if p.Name() != "digitalocean" || p.Label() != "DigitalOcean" {
		t.Fatalf("identity = %q/%q", p.Name(), p.Label())
	}
	if roles := p.Roles(); len(roles) != 1 || roles[0] != provider.RoleVM {
		t.Fatalf("roles = %v", roles)
	}
}

func TestConfigured(t *testing.T) {
	t.Setenv("DIGITALOCEAN_TOKEN", "")
	var cfg config.Config
	if New().Configured(cfg) {
		t.Fatal("empty config reported configured")
	}
	cfg.Providers.DigitalOcean.Token = "tok"
	if !New().Configured(cfg) {
		t.Fatal("token in file not reported configured")
	}
}

func TestConfigureStoresValidatedToken(t *testing.T) {
	var buf bytes.Buffer
	oldOut, oldPrompt, oldValidate := out, promptToken, validateToken
	t.Cleanup(func() { out, promptToken, validateToken = oldOut, oldPrompt, oldValidate })
	out = &buf
	validated := ""
	validateToken = func(_ context.Context, tok string) error { validated = tok; return nil }
	promptToken = func(validate func(string) error) (string, error) {
		if err := validate("dop_v1_x"); err != nil {
			return "", err
		}
		return "dop_v1_x", nil
	}

	var cfg config.Config
	if err := New().Configure(context.Background(), &cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.Providers.DigitalOcean.Token != "dop_v1_x" || validated != "dop_v1_x" {
		t.Fatalf("token = %q validated = %q", cfg.Providers.DigitalOcean.Token, validated)
	}
	for _, want := range []string{
		"How to create a DigitalOcean API token",
		"https://cloud.digitalocean.com/account/api/tokens",
		"token stored",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestConfigureCancelLeavesConfigUntouched(t *testing.T) {
	oldOut, oldPrompt := out, promptToken
	t.Cleanup(func() { out, promptToken = oldOut, oldPrompt })
	out = &bytes.Buffer{}
	promptToken = func(func(string) error) (string, error) {
		return "", huh.ErrUserAborted
	}

	var cfg config.Config
	err := New().Configure(context.Background(), &cfg)

	if err == nil || cfg.Providers.DigitalOcean.Token != "" {
		t.Fatalf("err = %v token = %q", err, cfg.Providers.DigitalOcean.Token)
	}
}
