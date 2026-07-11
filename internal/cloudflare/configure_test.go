package cloudflare

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

func stubConfigure(t *testing.T, token string, listErr error, zs []Zone,
	picked Zone) *bytes.Buffer {
	t.Helper()
	buf := muteConfigureUI(t)
	oldPrompt, oldValidate, oldList, oldPick := promptToken, validateCreds, listZones, pickZone
	t.Cleanup(func() {
		promptToken, validateCreds, listZones, pickZone = oldPrompt, oldValidate, oldList, oldPick
	})
	promptToken = func(func(string) error) (string, error) { return token, nil }
	validateCreds = func(context.Context, string) error { return nil }
	listZones = func(context.Context, string) ([]Zone, error) { return zs, listErr }
	pickZone = func(zones []Zone) (Zone, error) { return picked, nil }
	return buf
}

func TestConfigureStoresTokenAndPickedZone(t *testing.T) {
	stubConfigure(t, "cf-tok", nil,
		[]Zone{{ID: "z1", Name: "example.test"}, {ID: "z2", Name: "other.example"}},
		Zone{ID: "z1", Name: "example.test"})

	var cfg config.Config
	if err := (cf{}).Configure(context.Background(), &cfg); err != nil {
		t.Fatal(err)
	}

	want := config.Cloudflare{APIToken: "cf-tok", ZoneID: "z1", Domain: "example.test"}
	if cfg.Providers.Cloudflare != want {
		t.Fatalf("stored = %+v", cfg.Providers.Cloudflare)
	}
}

func TestConfigureZoneListFailureFallsBackToManual(t *testing.T) {
	buf := stubConfigure(t, "cf-tok", errors.New("api down"), nil, Zone{})
	oldManual := promptZoneManual
	t.Cleanup(func() { promptZoneManual = oldManual })
	promptZoneManual = func() (Zone, error) {
		return Zone{ID: "manual-z", Name: "manual.test"}, nil
	}

	var cfg config.Config
	if err := (cf{}).Configure(context.Background(), &cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.Providers.Cloudflare.ZoneID != "manual-z" ||
		cfg.Providers.Cloudflare.Domain != "manual.test" {
		t.Fatalf("stored = %+v", cfg.Providers.Cloudflare)
	}
	if !strings.Contains(buf.String(), "could not list zones") {
		t.Fatalf("missing fallback warning:\n%s", buf.String())
	}
}

func TestConfigureRejectedTokenStoresNothing(t *testing.T) {
	buf := muteConfigureUI(t)
	oldPrompt, oldValidate := promptToken, validateCreds
	t.Cleanup(func() { promptToken, validateCreds = oldPrompt, oldValidate })
	promptToken = func(func(string) error) (string, error) { return "bad", nil }
	validateCreds = func(context.Context, string) error {
		return errors.New("token status: disabled")
	}

	var cfg config.Config
	if err := (cf{}).Configure(context.Background(), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Providers.Cloudflare.APIToken != "" {
		t.Fatalf("rejected token stored: %+v", cfg.Providers.Cloudflare)
	}
	if !strings.Contains(buf.String(), "token status: disabled") {
		t.Fatalf("missing fail row:\n%s", buf.String())
	}
}

func TestConfigureCancelAtZonePickerAborts(t *testing.T) {
	stubConfigure(t, "cf-tok", nil, []Zone{{ID: "z1", Name: "example.test"}}, Zone{})
	oldPick := pickZone
	t.Cleanup(func() { pickZone = oldPick })
	pickZone = func([]Zone) (Zone, error) { return Zone{}, huh.ErrUserAborted }

	var cfg config.Config
	err := (cf{}).Configure(context.Background(), &cfg)
	if !errors.Is(err, huh.ErrUserAborted) {
		t.Fatalf("err = %v, want ErrUserAborted", err)
	}
	if cfg.Providers.Cloudflare.APIToken != "" {
		t.Fatalf("aborted configure stored token: %+v", cfg.Providers.Cloudflare)
	}
}
