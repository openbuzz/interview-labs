package hetzner

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/ui"
)

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
	if err := (hz{}).Configure(context.Background(), &cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.Providers.Hetzner.Token != "tok-123" || calls != 1 {
		t.Fatalf("token=%q calls=%d", cfg.Providers.Hetzner.Token, calls)
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
	if err := (hz{}).Configure(context.Background(), &cfg); err != nil {
		t.Fatal(err) // failure is reported, not returned
	}

	if cfg.Providers.Hetzner.Token != "" {
		t.Fatalf("rejected token stored: %q", cfg.Providers.Hetzner.Token)
	}
	if !strings.Contains(buf.String(), "401 unauthorized") {
		t.Fatalf("missing fail row:\n%s", buf.String())
	}
}
