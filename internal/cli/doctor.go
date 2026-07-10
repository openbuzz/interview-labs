package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/digitalocean"
	"github.com/openbuzz/interview-labs/internal/session"
	"github.com/openbuzz/interview-labs/internal/terraform"
	"github.com/openbuzz/interview-labs/internal/ui"
)

// validateDOToken is a seam for tests; production hits the DigitalOcean API.
var validateDOToken = func(ctx context.Context, token string) error {
	c, err := digitalocean.NewClient(token)
	if err != nil {
		return err
	}
	return digitalocean.ValidateToken(ctx, c)
}

// lookupSSH is a seam for tests; production checks PATH.
var lookupSSH = func() error {
	_, err := exec.LookPath("ssh")
	return err
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "check the tools and credentials interview needs",
		Long: `Check the local environment: terraform or opentofu on PATH, the ssh
client, XDG config/state/cache directories, and stored credentials.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			failed := false
			p := func(s string) { fmt.Fprintln(cmd.OutOrStdout(), s) }

			// tf binary
			if bin, err := terraform.Find(); err != nil {
				failed = true
				p(ui.RowFail("terraform", "not found"))
				p("  " + ui.Faint.Render(
					"install terraform or opentofu, then rerun interview doctor"))
			} else {
				p(ui.RowOK(bin.Name, bin.Version+" ("+bin.Path+")"))
			}

			// ssh client — a note, never a failure
			if err := lookupSSH(); err != nil {
				p(ui.RowWarn("ssh client", "not found — interview ssh unavailable; "+
					"launch still works"))
			} else {
				p(ui.RowOK("ssh client", "found"))
			}

			// xdg dirs
			xdgOK := true
			if p, err := config.Path(); err != nil || os.MkdirAll(filepath.Dir(p), 0o755) != nil {
				xdgOK = false
			}
			if r, err := session.Root(); err != nil || os.MkdirAll(r, 0o755) != nil {
				xdgOK = false
			}
			if _, err := terraform.PluginCacheDir(); err != nil {
				xdgOK = false
			}
			if xdgOK {
				p(ui.RowOK("state dirs", "writable"))
			} else {
				failed = true
				p(ui.RowFail("state dirs", "cannot create XDG directories"))
			}

			// credentials
			cfg, err := config.Load()
			if err != nil {
				failed = true
				p(ui.RowFail("credentials", "config unreadable: "+err.Error()))
			} else if token := cfg.Token(); token == "" {
				p(ui.RowWarn("credentials", "no DigitalOcean token configured"))
				p("  " + ui.Faint.Render("run interview init to configure one"))
			} else if err := validateDOToken(cmd.Context(), token); err != nil {
				failed = true
				p(ui.RowFail("credentials", err.Error()))
				p("  " + ui.Faint.Render("run interview init to replace the token"))
			} else {
				p(ui.RowOK("credentials", "DigitalOcean token valid"))
			}

			if failed {
				return fmt.Errorf("doctor found problems")
			}
			return nil
		},
	}
}
