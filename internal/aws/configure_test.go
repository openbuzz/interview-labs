package aws

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

func TestConfigureStoresValidatedPair(t *testing.T) {
	old := provider.RetryDelays
	provider.RetryDelays = nil
	t.Cleanup(func() { provider.RetryDelays = old })
	oldUI := ui.Interactive
	ui.Interactive = func() bool { return false }
	t.Cleanup(func() { ui.Interactive = oldUI })

	var buf bytes.Buffer
	oldOut, oldPrompt, oldValidate := out, promptCreds, validatePair
	t.Cleanup(func() { out, promptCreds, validatePair = oldOut, oldPrompt, oldValidate })
	out = &buf
	promptCreds = func() (string, string, error) { return "AKIA1", "sec", nil }
	validatePair = func(ctx context.Context, keyID, secret string) error { return nil }

	var cfg config.Config
	if err := (aw{}).Configure(context.Background(), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Providers.AWS.AccessKeyID != "AKIA1" ||
		cfg.Providers.AWS.SecretAccessKey != "sec" {
		t.Fatalf("stored = %+v", cfg.Providers.AWS)
	}
	if !strings.Contains(buf.String(), "How to create AWS credentials") {
		t.Fatalf("guidance missing:\n%s", buf.String())
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
	oldOut, oldPrompt, oldValidate := out, promptCreds, validatePair
	t.Cleanup(func() { out, promptCreds, validatePair = oldOut, oldPrompt, oldValidate })
	out = &buf
	promptCreds = func() (string, string, error) { return "AKIA1", "sec", nil }
	validatePair = func(context.Context, string, string) error {
		return errors.New("403 forbidden")
	}

	var cfg config.Config
	if err := (aw{}).Configure(context.Background(), &cfg); err != nil {
		t.Fatal(err) // failure is reported, not returned
	}

	if cfg.Providers.AWS.AccessKeyID != "" || cfg.Providers.AWS.SecretAccessKey != "" {
		t.Fatalf("rejected pair stored: %+v", cfg.Providers.AWS)
	}
	if !strings.Contains(buf.String(), "403 forbidden") {
		t.Fatalf("missing fail row:\n%s", buf.String())
	}
}
