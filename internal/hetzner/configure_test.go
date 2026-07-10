package hetzner

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/openbuzz/interview-labs/internal/config"
)

func TestConfigureStoresValidatedToken(t *testing.T) {
	var buf bytes.Buffer
	oldOut, oldPrompt, oldValidate := out, promptToken, validateToken
	t.Cleanup(func() { out, promptToken, validateToken = oldOut, oldPrompt, oldValidate })
	out = &buf
	promptToken = func(validate func(string) error) (string, error) {
		if err := validate("hz-tok"); err != nil {
			return "", err
		}
		return "hz-tok", nil
	}
	validateToken = func(ctx context.Context, token string) error { return nil }

	var cfg config.Config
	if err := (hz{}).Configure(context.Background(), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Providers.Hetzner.Token != "hz-tok" {
		t.Fatalf("stored token = %q", cfg.Providers.Hetzner.Token)
	}
	if !strings.Contains(buf.String(), "How to create a Hetzner Cloud API token") {
		t.Fatalf("guidance missing:\n%s", buf.String())
	}
}
