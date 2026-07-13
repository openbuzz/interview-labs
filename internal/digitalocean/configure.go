package digitalocean

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

const guidanceTitle = "How to create a DigitalOcean API token"

const guidance = `If you don't have an account yet, create one at:
  https://cloud.digitalocean.com/registrations/new

1. Open https://cloud.digitalocean.com/account/api/tokens
2. Click "Generate New Token"
3. Name it (e.g. "interview-labs"), select "Full Access"
4. Click "Generate Token"
5. Copy the token (it is shown only once)
6. Paste the token into the prompt below`

// Seams for tests.
var (
	out io.Writer = os.Stdout

	promptToken = func(validate func(string) error) (string, error) {
		var token string
		form := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("DigitalOcean API token").
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
		client, err := NewClient(token)
		if err != nil {
			return err
		}
		return ValidateToken(ctx, client)
	}
)

type do struct{}

// New returns the DigitalOcean provider.
func New() provider.Provider { return do{} }

func (do) Name() string  { return "digitalocean" }
func (do) Label() string { return "DigitalOcean" }

func (do) Roles() []provider.Role { return []provider.Role{provider.RoleVM} }

func (do) Configured(cfg config.Config) bool { return token(cfg) != "" }

// Configure shows the token guidance, prompts, validates with retries, stores.
func (do) Configure(ctx context.Context, cfg *config.Config) error {
	fmt.Fprintln(out, ui.Section(ui.SectionTitle(guidanceTitle),
		strings.Split(guidance, "\n")...))
	fmt.Fprintln(out, ui.Faint.Render("The token is validated before it is stored (0600)."))

	token, err := promptToken(func(t string) error {
		if t == "" {
			return fmt.Errorf("token is empty")
		}
		return nil
	})
	if err != nil {
		return err
	}

	if err := provider.TestCredentials(ctx, out, ui.Step, func(ctx context.Context) error {
		return validateToken(ctx, token)
	}); err != nil {
		fmt.Fprintln(out, ui.RowFail("credentials", "token rejected — nothing stored"))
		return nil
	}

	cfg.Providers.DigitalOcean.Token = token
	fmt.Fprintln(out, ui.RowOK("credentials", "valid — token stored"))
	return nil
}
