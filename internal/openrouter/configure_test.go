package openrouter

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

func muteConfigureUI(t *testing.T) *bytes.Buffer {
	t.Helper()
	old := provider.RetryDelays
	provider.RetryDelays = nil
	t.Cleanup(func() { provider.RetryDelays = old })
	oldUI := ui.Interactive
	ui.Interactive = func() bool { return false }
	t.Cleanup(func() { ui.Interactive = oldUI })

	var buf bytes.Buffer
	oldOut := out
	out = &buf
	t.Cleanup(func() { out = oldOut })
	return &buf
}

func TestConfigureStoresKeyAndCap(t *testing.T) {
	buf := muteConfigureUI(t)
	oldPrompt, oldValidate := promptKeyCap, validateKey
	t.Cleanup(func() { promptKeyCap, validateKey = oldPrompt, oldValidate })
	promptKeyCap = func(capPrefill string) (string, string, error) {
		if capPrefill != "10" {
			t.Fatalf("cap prefill = %q, want 10", capPrefill)
		}
		return "mk-1", "12.5", nil
	}
	validateKey = func(context.Context, string) error { return nil }

	var cfg config.Config
	if err := (or{}).Configure(context.Background(), &cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.Providers.OpenRouter.ManagementKey != "mk-1" ||
		cfg.Providers.OpenRouter.CapUSD != 12.5 {
		t.Fatalf("stored = %+v", cfg.Providers.OpenRouter)
	}
	for _, want := range []string{"management key", "Testing credentials…"} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestConfigureEmptyCapDefaults(t *testing.T) {
	muteConfigureUI(t)
	oldPrompt, oldValidate := promptKeyCap, validateKey
	t.Cleanup(func() { promptKeyCap, validateKey = oldPrompt, oldValidate })
	promptKeyCap = func(string) (string, string, error) { return "mk-1", "", nil }
	validateKey = func(context.Context, string) error { return nil }

	var cfg config.Config
	if err := (or{}).Configure(context.Background(), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Providers.OpenRouter.CapUSD != 0 {
		t.Fatalf("empty cap must store 0 (default applies at use): %+v",
			cfg.Providers.OpenRouter)
	}
}

func TestConfigureRejectedStoresNothing(t *testing.T) {
	buf := muteConfigureUI(t)
	oldPrompt, oldValidate := promptKeyCap, validateKey
	t.Cleanup(func() { promptKeyCap, validateKey = oldPrompt, oldValidate })
	promptKeyCap = func(string) (string, string, error) { return "bad", "10", nil }
	validateKey = func(context.Context, string) error {
		return errors.New("401 unauthorized")
	}

	var cfg config.Config
	if err := (or{}).Configure(context.Background(), &cfg); err != nil {
		t.Fatal(err) // failure is reported, not returned
	}
	if cfg.Providers.OpenRouter.ManagementKey != "" {
		t.Fatalf("rejected key stored: %+v", cfg.Providers.OpenRouter)
	}
	if !strings.Contains(buf.String(), "401 unauthorized") {
		t.Fatalf("missing fail row:\n%s", buf.String())
	}
}

func TestConfigureCancelLeavesConfigUntouched(t *testing.T) {
	muteConfigureUI(t)
	oldPrompt := promptKeyCap
	t.Cleanup(func() { promptKeyCap = oldPrompt })
	promptKeyCap = func(string) (string, string, error) {
		return "", "", huh.ErrUserAborted
	}

	var cfg config.Config
	err := New().Configure(context.Background(), &cfg)
	if err == nil || cfg.Providers.OpenRouter.ManagementKey != "" {
		t.Fatalf("err = %v stored = %+v", err, cfg.Providers.OpenRouter)
	}
}
