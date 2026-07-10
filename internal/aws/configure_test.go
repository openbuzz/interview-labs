package aws

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/openbuzz/interview-labs/internal/config"
)

func TestConfigureStoresValidatedPair(t *testing.T) {
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

func TestConfigureRetriesOnRejectedPair(t *testing.T) {
	var buf bytes.Buffer
	oldOut, oldPrompt, oldValidate := out, promptCreds, validatePair
	t.Cleanup(func() { out, promptCreds, validatePair = oldOut, oldPrompt, oldValidate })
	out = &buf

	calls := 0
	promptCreds = func() (string, string, error) { calls++; return "AKIA1", "sec", nil }
	validatePair = func(ctx context.Context, keyID, secret string) error {
		if calls == 1 {
			return context.DeadlineExceeded
		}
		return nil
	}

	var cfg config.Config
	if err := (aw{}).Configure(context.Background(), &cfg); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("prompt calls = %d, want 2", calls)
	}
}
