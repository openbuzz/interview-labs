package openrouter

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/ui"
)

const guidanceTitle = "How to create an OpenRouter management key"

const guidance = `This is a management key, not a regular API key.
It can create and revoke scoped API keys for each lab session.

If you don't have an account yet, create one at:
  https://openrouter.ai/sign-up

1. Open https://openrouter.ai/settings/provisioning-keys
   (OpenRouter's console calls these "provisioning keys")
2. Click "Create"
3. Name it (e.g. "interview-labs"), click "Create"
4. Copy the key (it is shown only once)
5. Paste the key into the prompts below`

// Seams for tests.
var (
	out io.Writer = os.Stdout

	promptKeyCap = func(capPrefill string) (key, capStr string, err error) {
		capStr = capPrefill
		form := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("OpenRouter management key").
				EchoMode(huh.EchoModePassword).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("key is empty")
					}
					return nil
				}).
				Value(&key),
			huh.NewInput().
				Title("Per-session spend cap (USD)").
				Validate(validateCap).
				Value(&capStr),
		)).WithTheme(ui.Theme()).WithKeyMap(ui.FormKeyMap())
		if err := form.Run(); err != nil {
			return "", "", err
		}
		return key, capStr, nil
	}

	validateKey = func(ctx context.Context, key string) error {
		return validate(ctx, key)
	}
)

// validateCap accepts an empty string (the default applies at use) or a
// positive number.
func validateCap(s string) error {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return fmt.Errorf("enter a positive number")
	}
	return nil
}

type or struct{}

// New returns the OpenRouter provider.
func New() provider.Provider { return or{} }

func (or) Name() string  { return "openrouter" }
func (or) Label() string { return "OpenRouter" }

func (or) Roles() []provider.Role { return []provider.Role{provider.RoleAI} }

func (or) Configured(cfg config.Config) bool { return managementKey(cfg) != "" }

// Configure shows the management-key guidance, prompts for key and cap,
// validates the key with retries, stores both.
func (or) Configure(ctx context.Context, cfg *config.Config) error {
	fmt.Fprintln(out, ui.Box(guidanceTitle, ui.Accent, strings.Split(guidance, "\n")...))
	fmt.Fprintln(out, ui.Faint.Render(
		"The management key is validated before it is stored (0600)."))

	capPrefill := strconv.FormatFloat(DefaultCapUSD, 'f', -1, 64)
	if c := cfg.Providers.OpenRouter.CapUSD; c > 0 {
		capPrefill = strconv.FormatFloat(c, 'f', -1, 64)
	}
	key, capStr, err := promptKeyCap(capPrefill)
	if err != nil {
		return err
	}

	if err := provider.TestCredentials(ctx, out, ui.Step, func(ctx context.Context) error {
		return validateKey(ctx, key)
	}); err != nil {
		fmt.Fprintln(out, ui.RowFail("credentials", "key rejected — nothing stored"))
		return nil
	}

	cfg.Providers.OpenRouter.ManagementKey = key
	cfg.Providers.OpenRouter.CapUSD = 0
	if capStr != "" {
		cfg.Providers.OpenRouter.CapUSD, _ = strconv.ParseFloat(capStr, 64)
	}
	fmt.Fprintln(out, ui.RowOK("credentials", "valid — management key stored"))
	return nil
}
