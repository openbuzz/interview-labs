package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/huh"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
)

func TestInitNonTTYExits2(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	old := isTTY
	isTTY = func() bool { return false }
	t.Cleanup(func() { isTTY = old })

	out, code := runCmd(t, "init")
	if code != 2 {
		t.Fatalf("exit = %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "config.yaml") {
		t.Fatalf("non-TTY message must point at the config file:\n%s", out)
	}
}

type fakeProvider struct {
	name, label string
	configured  bool
	configure   func(ctx context.Context, cfg *config.Config) error
}

func (f *fakeProvider) Name() string           { return f.name }
func (f *fakeProvider) Label() string          { return f.label }
func (f *fakeProvider) Roles() []provider.Role { return []provider.Role{provider.RoleVM} }

func (f *fakeProvider) Configured(cfg config.Config) bool {
	return f.configured || cfg.Providers.DigitalOcean.Token != ""
}

func (f *fakeProvider) Configure(ctx context.Context, cfg *config.Config) error {
	return f.configure(ctx, cfg)
}

func TestInitConfiguresProviderThenExits(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	swapTTY(t, true)
	fp := &fakeProvider{name: "digitalocean", label: "DigitalOcean",
		configure: func(_ context.Context, cfg *config.Config) error {
			cfg.Providers.DigitalOcean.Token = "tok"
			return nil
		}}
	oldProviders, oldPick := providers, pickInitAction
	t.Cleanup(func() { providers, pickInitAction = oldProviders, oldPick })
	providers = []provider.Provider{fp}
	calls := 0
	pickInitAction = func(all []provider.Provider,
		cfg config.Config) (provider.Provider, error) {
		calls++
		if calls == 1 {
			return all[0], nil
		}
		return nil, nil // Exit
	}

	// runCmd is the package's existing test runner: (string, int) exit code, not
	// (string, error) — adapted from the brief's literal `out, err :=` snippet.
	out, code := runCmd(t, "init")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Providers.DigitalOcean.Token != "tok" {
		t.Fatal("configure result not persisted")
	}
	for _, want := range []string{"Setup", "DigitalOcean", "interview launch"} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary missing %q:\n%s", want, out)
		}
	}
}

func TestInitConfigureCancelReturnsToMenu(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	swapTTY(t, true)
	fp := &fakeProvider{name: "digitalocean", label: "DigitalOcean",
		configure: func(context.Context, *config.Config) error {
			return huh.ErrUserAborted
		}}
	oldProviders, oldPick := providers, pickInitAction
	t.Cleanup(func() { providers, pickInitAction = oldProviders, oldPick })
	providers = []provider.Provider{fp}
	calls := 0
	pickInitAction = func([]provider.Provider,
		config.Config) (provider.Provider, error) {
		calls++
		if calls == 1 {
			return fp, nil
		}
		return nil, nil
	}

	// runCmd returns (string, int); see the adaptation note above.
	_, code := runCmd(t, "init")

	if code != 0 {
		t.Fatalf("cancelled configure bubbled up: exit %d", code)
	}
	if calls != 2 {
		t.Fatalf("menu shown %d times, want 2", calls)
	}
}
