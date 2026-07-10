package cli

import (
	"errors"
	"fmt"
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
	provider.RoleAccess: "Cloud VM Access",
}

func menuRow(p provider.Provider, cfg config.Config) string {
	return ui.Badge(p.Configured(cfg)) + " " + p.Label() + "  — " +
		roleLabels[p.Roles()[0]]
}

// pickInitAction is a seam; production shows the provider menu.
// A nil provider with nil error means Exit.
var pickInitAction = func(all []provider.Provider,
	cfg config.Config) (provider.Provider, error) {
	const exit = "exit"
	opts := make([]huh.Option[string], 0, len(all)+1)
	for _, p := range all {
		opts = append(opts, huh.NewOption(menuRow(p, cfg), p.Name()))
	}
	opts = append(opts, huh.NewOption("Exit", exit))

	var sel string
	err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Select a provider to configure:").
			Options(opts...).Value(&sel),
	)).WithTheme(ui.Theme()).Run()
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
			fmt.Fprintln(out, ui.Logo())

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			for {
				p, err := pickInitAction(providers, cfg)
				if err != nil {
					return err
				}
				if p == nil {
					break
				}

				err = p.Configure(cmd.Context(), &cfg)
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

			rows := make([]string, 0, len(providers))
			anyConfigured := false
			for _, p := range providers {
				rows = append(rows, menuRow(p, cfg))
				anyConfigured = anyConfigured || p.Configured(cfg)
			}
			style, next := ui.Warn, "interview init"
			if anyConfigured {
				style, next = ui.OK, "interview launch"
			}
			fmt.Fprintln(out, ui.Box("Setup", style, rows...))
			fmt.Fprintln(out, ui.Next(next))
			return nil
		},
	}
}
