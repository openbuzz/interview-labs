package cli

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/ui"
)

// isTTY and promptToken are seams for tests.
var isTTY = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

var promptToken = func(validate func(string) error) (string, error) {
	var token string
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("DigitalOcean API token").
			EchoMode(huh.EchoModePassword).
			Validate(validate).
			Value(&token),
	)).WithTheme(ui.Theme())
	if err := form.Run(); err != nil {
		return "", err
	}
	return token, nil
}

const tokenHowTo = `Create a DigitalOcean API token:

  1. Open https://cloud.digitalocean.com/account/api/tokens
  2. Generate New Token → Full Access
     (or a custom scope set covering droplet, ssh_key, firewall, tag)
  3. Short expiry recommended — DigitalOcean shows the token once.

The token is validated before it is stored (0600).`

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "store and validate the DigitalOcean token",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !isTTY() {
				p, _ := config.Path()
				return usageError(fmt.Sprintf(
					"interview init needs a terminal; write %s yourself or set "+
						"DIGITALOCEAN_TOKEN (see config.yaml docs in the README)", p))
			}
			fmt.Fprintln(cmd.OutOrStdout(), tokenHowTo)

			token, err := promptToken(func(t string) error {
				if t == "" {
					return fmt.Errorf("token is empty")
				}
				return validateDOToken(cmd.Context(), t)
			})
			if err != nil {
				return err
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.DigitalOceanToken = token
			if err := cfg.Write(); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), ui.RowOK("credentials", "token stored"))
			fmt.Fprintln(cmd.OutOrStdout(), ui.Next("interview launch"))
			return nil
		},
	}
}
