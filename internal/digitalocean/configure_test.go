package digitalocean

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/huh"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/ui"
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

func TestConfigureValidatesWithRetries(t *testing.T) {
	old := provider.RetryDelays
	provider.RetryDelays = nil // no sleeps in tests
	t.Cleanup(func() { provider.RetryDelays = old })
	oldUI := ui.Interactive
	ui.Interactive = func() bool { return false }
	t.Cleanup(func() { ui.Interactive = oldUI })

	var buf bytes.Buffer
	oldOut, oldPrompt, oldValidate := out, promptToken, validateToken
	out = &buf
	promptToken = func(validate func(string) error) (string, error) {
		if err := validate(""); err == nil {
			t.Fatal("empty token accepted")
		}
		return "tok-123", nil
	}
	calls := 0
	validateToken = func(ctx context.Context, token string) error {
		calls++
		return nil
	}
	t.Cleanup(func() { out, promptToken, validateToken = oldOut, oldPrompt, oldValidate })

	var cfg config.Config
	if err := (do{}).Configure(context.Background(), &cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.Providers.DigitalOcean.Token != "tok-123" || calls != 1 {
		t.Fatalf("token=%q calls=%d", cfg.Providers.DigitalOcean.Token, calls)
	}
	if !strings.Contains(buf.String(), "Testing credentials…") {
		t.Fatalf("missing banner:\n%s", buf.String())
	}
}

func TestConfigureRejectedStoresNothing(t *testing.T) {
	old := provider.RetryDelays
	provider.RetryDelays = nil
	t.Cleanup(func() { provider.RetryDelays = old })
	oldUI := ui.Interactive
	ui.Interactive = func() bool { return false }
	t.Cleanup(func() { ui.Interactive = oldUI })

	var buf bytes.Buffer
	oldOut, oldPrompt, oldValidate := out, promptToken, validateToken
	out = &buf
	promptToken = func(func(string) error) (string, error) { return "bad", nil }
	validateToken = func(context.Context, string) error {
		return errors.New("401 unauthorized")
	}
	t.Cleanup(func() { out, promptToken, validateToken = oldOut, oldPrompt, oldValidate })

	var cfg config.Config
	if err := (do{}).Configure(context.Background(), &cfg); err != nil {
		t.Fatal(err) // failure is reported, not returned
	}

	if cfg.Providers.DigitalOcean.Token != "" {
		t.Fatalf("rejected token stored: %q", cfg.Providers.DigitalOcean.Token)
	}
	if !strings.Contains(buf.String(), "401 unauthorized") {
		t.Fatalf("missing fail row:\n%s", buf.String())
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
