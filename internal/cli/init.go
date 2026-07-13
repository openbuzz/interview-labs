package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/ui"
)

// isTTY is a seam for tests.
var isTTY = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

// roleLabels name roles in menu rows.
var roleLabels = map[provider.Role]string{
	provider.RoleVM:     "Cloud VM",
	provider.RoleAI:     "AI Tooling",
	provider.RoleAccess: "DNS / access",
}

// menuRows renders one aligned row per provider: badge, label, role.
func menuRows(all []provider.Provider, cfg config.Config) []string {
	rows := make([][]string, len(all))
	for i, p := range all {
		rows[i] = []string{ui.Badge(p.Configured(cfg)), p.Label(),
			roleLabels[p.Roles()[0]]}
	}
	return ui.Columns(rows)
}

// pickInitAction is a seam; production shows the provider menu.
// A nil provider with nil error means Exit.
var pickInitAction = func(all []provider.Provider,
	cfg config.Config) (provider.Provider, error) {
	const exit = "exit"
	rows := menuRows(all, cfg)
	opts := make([]huh.Option[string], 0, len(all)+1)
	for i, p := range all {
		opts = append(opts, huh.NewOption(rows[i], p.Name()))
	}
	opts = append(opts, huh.NewOption("exit", exit))

	var sel string
	err := ui.SelectForm("Select a provider to configure",
		"Credentials are validated live and stored in the config file (0600). "+
			"Re-run any time.",
		opts, &sel)
	if errors.Is(err, huh.ErrUserAborted) {
		return nil, nil // ESC or Ctrl-C at the menu behaves like Exit
	}
	if err != nil {
		return nil, err
	}
	for _, p := range all {
		if p.Name() == sel {
			return p, nil
		}
	}
	return nil, nil
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "configure cloud providers",
		Long: `Configure provider credentials for launching interview VMs.

Shows one entry per supported provider with its configured state; each
entry walks through creating and validating the credentials it needs.
Credentials are stored with 0600 permissions in the config file. Re-run
init any time to add or update providers.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if !isTTY() {
				p, _ := config.Path()
				return usageError(fmt.Sprintf(
					"interview init needs a terminal; write %s yourself or set "+
						"provider env vars (DIGITALOCEAN_TOKEN, HCLOUD_TOKEN, or "+
						"AWS_ACCESS_KEY_ID + AWS_SECRET_ACCESS_KEY)", p))
			}
			printLogoOnce(out)
			printNarrowWarning(out)

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if err := runInitLoop(cmd.Context(), out, &cfg); err != nil {
				return err
			}

			printInitSummary(out, cfg)
			return nil
		},
	}
}

// runInitLoop shows the provider menu until Exit, persisting after each configure.
func runInitLoop(ctx context.Context, out io.Writer, cfg *config.Config) error {
	for first := true; ; first = false {
		if !first {
			fmt.Fprintln(out)
		}

		p, err := pickInitAction(providers, *cfg)
		if err != nil {
			return err
		}
		if p == nil {
			return nil
		}

		err = p.Configure(ctx, cfg)
		if errors.Is(err, huh.ErrUserAborted) {
			continue
		}
		if err != nil {
			return err
		}
		if err := cfg.Write(); err != nil {
			return err
		}
	}
}

// printInitSummary renders the final per-provider state section and next step.
func printInitSummary(out io.Writer, cfg config.Config) {
	rows := menuRows(providers, cfg)
	anyConfigured := false
	for _, p := range providers {
		anyConfigured = anyConfigured || p.Configured(cfg)
	}

	style, next := ui.Warn, "interview init"
	if anyConfigured {
		style, next = ui.OK, "interview launch"
	}
	fmt.Fprintln(out, ui.Section(style.Render("SETUP"), rows...))
	fmt.Fprintln(out)
	fmt.Fprintln(out, ui.Next(next))
}
