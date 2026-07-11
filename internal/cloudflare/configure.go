package cloudflare

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

const guidanceTitle = "How to create a Cloudflare API token"

const guidance = `If you don't have an account yet, create one at:
  https://dash.cloudflare.com/sign-up

1. Open https://dash.cloudflare.com
2. Go to "My Profile" → "API Tokens" → "Create Token"
3. Select "Create Custom Token", click "Get started"
4. Set a token name (e.g. "interview-labs")
5. Grant "Zone.DNS:Edit" and "Zone.Zone:Read" permissions
6. Restrict "Zone Resources" to the zone you will use
7. Click "Continue to summary" → "Create Token"
8. Copy the token and paste it into the prompt below`

// Seams for tests.
var (
	out io.Writer = os.Stdout

	promptToken = func(validate func(string) error) (string, error) {
		var token string
		form := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("Cloudflare API token").
				EchoMode(huh.EchoModePassword).
				Validate(validate).
				Value(&token),
		)).WithTheme(ui.Theme()).WithKeyMap(ui.FormKeyMap())
		if err := form.Run(); err != nil {
			return "", err
		}
		return token, nil
	}

	validateCreds = func(ctx context.Context, token string) error {
		return validateToken(ctx, token)
	}

	listZones = func(ctx context.Context, token string) ([]Zone, error) {
		return zones(ctx, token)
	}

	pickZone = func(zs []Zone) (Zone, error) {
		opts := make([]huh.Option[string], 0, len(zs))
		for _, z := range zs {
			opts = append(opts, huh.NewOption(z.Name, z.ID))
		}
		var id string
		if err := ui.SelectForm("Select a Cloudflare zone",
			"Sessions get a <slug>.<zone> DNS record in this zone.",
			opts, &id); err != nil {
			return Zone{}, err
		}
		for _, z := range zs {
			if z.ID == id {
				return z, nil
			}
		}
		return zs[0], nil
	}

	promptZoneManual = func() (Zone, error) {
		var z Zone
		form := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("Cloudflare zone ID").
				Validate(notEmpty("zone ID")).
				Value(&z.ID),
			huh.NewInput().
				Title("Zone domain (e.g. example.com)").
				Validate(notEmpty("domain")).
				Value(&z.Name),
		)).WithTheme(ui.Theme()).WithKeyMap(ui.FormKeyMap())
		if err := form.Run(); err != nil {
			return Zone{}, err
		}
		return z, nil
	}
)

func notEmpty(what string) func(string) error {
	return func(s string) error {
		if s == "" {
			return fmt.Errorf("%s is empty", what)
		}
		return nil
	}
}

type cf struct{}

// New returns the Cloudflare provider.
func New() provider.Provider { return cf{} }

func (cf) Name() string  { return "cloudflare" }
func (cf) Label() string { return "Cloudflare" }

func (cf) Roles() []provider.Role { return []provider.Role{provider.RoleAccess} }

func (cf) Configured(cfg config.Config) bool {
	return token(cfg) != "" && cfg.Providers.Cloudflare.ZoneID != ""
}

// Configure shows the token guidance, prompts, validates with retries, then
// resolves the zone: live picker when the list call works, manual entry
// otherwise. Stores token + zone id + domain.
func (cf) Configure(ctx context.Context, cfg *config.Config) error {
	fmt.Fprintln(out, ui.Box(guidanceTitle, ui.Accent, strings.Split(guidance, "\n")...))
	fmt.Fprintln(out, ui.Faint.Render("The token is validated before it is stored (0600)."))

	tok, err := promptToken(notEmpty("token"))
	if err != nil {
		return err
	}

	if err := provider.TestCredentials(ctx, out, ui.Step, func(ctx context.Context) error {
		return validateCreds(ctx, tok)
	}); err != nil {
		fmt.Fprintln(out, ui.RowFail("credentials", "token rejected — nothing stored"))
		return nil
	}

	zone, err := resolveZone(ctx, tok)
	if err != nil {
		return err
	}

	cfg.Providers.Cloudflare = config.Cloudflare{
		APIToken: tok, ZoneID: zone.ID, Domain: zone.Name,
	}
	fmt.Fprintln(out, ui.RowOK("credentials", "valid — token and zone stored"))
	return nil
}

// resolveZone offers the live zone picker, falling back to manual entry
// when the list call fails or returns nothing.
func resolveZone(ctx context.Context, tok string) (Zone, error) {
	zs, err := listZones(ctx, tok)
	if err != nil || len(zs) == 0 {
		fmt.Fprintln(out, ui.RowWarn("zones",
			"could not list zones — enter the zone manually"))
		return promptZoneManual()
	}
	return pickZone(zs)
}
