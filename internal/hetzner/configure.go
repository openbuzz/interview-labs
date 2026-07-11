package hetzner

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/ui"
)

const guidanceTitle = "How to create a Hetzner Cloud API token"

const guidance = `If you don't have an account yet, create one at:
  https://accounts.hetzner.com/signUp

1. Open https://console.hetzner.cloud
2. Select your project, go to "Security" → "API Tokens"
3. Click "Generate API Token"
4. Name it (e.g. "interview-labs"), select "Read & Write"
5. Click "Generate API Token"
6. Copy the token (it is shown only once)
7. Paste the token into the prompt below`

// Seams for tests.
var (
	out io.Writer = os.Stdout

	promptToken = func(validate func(string) error) (string, error) {
		var token string
		form := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("Hetzner Cloud API token").
				EchoMode(huh.EchoModePassword).
				Validate(validate).
				Value(&token),
		)).WithTheme(ui.Theme()).WithKeyMap(ui.FormKeyMap())
		if err := form.Run(); err != nil {
			return "", err
		}
		return token, nil
	}

	validateToken = func(ctx context.Context, token string) error {
		return ValidateToken(ctx, NewClient(token))
	}
)

type hz struct{}

// New returns the Hetzner Cloud provider.
func New() provider.Provider { return hz{} }

func (hz) Name() string  { return "hetzner" }
func (hz) Label() string { return "Hetzner" }

func (hz) Roles() []provider.Role { return []provider.Role{provider.RoleVM} }

func (hz) Configured(cfg config.Config) bool { return token(cfg) != "" }

// Configure shows the token guidance, prompts, validates with retries, stores.
func (hz) Configure(ctx context.Context, cfg *config.Config) error {
	fmt.Fprintln(out, ui.Box(guidanceTitle, ui.Accent, strings.Split(guidance, "\n")...))
	fmt.Fprintln(out, ui.Faint.Render("The token is validated before it is stored (0600)."))

	tok, err := promptToken(func(t string) error {
		if t == "" {
			return fmt.Errorf("token is empty")
		}
		return nil
	})
	if err != nil {
		return err
	}

	if err := provider.TestCredentials(ctx, out, func(ctx context.Context) error {
		return validateToken(ctx, tok)
	}); err != nil {
		fmt.Fprintln(out, ui.RowFail("credentials", "token rejected — nothing stored"))
		return nil
	}

	cfg.Providers.Hetzner.Token = tok
	fmt.Fprintln(out, ui.RowOK("credentials", "valid — token stored"))
	return nil
}
