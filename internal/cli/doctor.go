package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/session"
	"github.com/openbuzz/interview-labs/internal/terraform"
	"github.com/openbuzz/interview-labs/internal/ui"
)

// validateCreds is a seam for tests; production hits the provider API.
var validateCreds = func(ctx context.Context, v provider.CredentialValidator,
	cfg config.Config) error {
	return v.ValidateCreds(ctx, cfg)
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
			p := func(s string) { fmt.Fprintln(cmd.OutOrStdout(), s) }

			ok := checkTerraform(p)
			checkSSHClient(p)
			ok = checkStateDirs(p) && ok
			ok = checkCredentials(cmd.Context(), p) && ok

			if !ok {
				return fmt.Errorf("doctor found problems")
			}
			return nil
		},
	}
}

// checkTerraform reports the tf binary row.
func checkTerraform(p func(string)) bool {
	bin, err := terraform.Find()
	if err != nil {
		p(ui.RowFail("terraform", "not found"))
		p("  " + ui.Faint.Render(
			"install terraform or opentofu, then rerun interview doctor"))
		return false
	}

	p(ui.RowOK(bin.Name, bin.Version+" ("+bin.Path+")"))
	return true
}

// checkSSHClient reports the ssh row — a note, never a failure.
func checkSSHClient(p func(string)) {
	if err := lookupSSH(); err != nil {
		p(ui.RowWarn("ssh client", "not found — interview ssh unavailable; "+
			"launch still works"))
		return
	}
	p(ui.RowOK("ssh client", "found"))
}

// checkStateDirs reports whether the XDG dirs are creatable.
func checkStateDirs(p func(string)) bool {
	ok := true
	if cp, err := config.Path(); err != nil ||
		os.MkdirAll(filepath.Dir(cp), 0o755) != nil {
		ok = false
	}
	if r, err := session.Root(); err != nil || os.MkdirAll(r, 0o755) != nil {
		ok = false
	}
	if _, err := terraform.PluginCacheDir(); err != nil {
		ok = false
	}

	if !ok {
		p(ui.RowFail("state dirs", "cannot create XDG directories"))
		return false
	}
	p(ui.RowOK("state dirs", "writable"))
	return true
}

// checkCredentials reports one row per credential-bearing provider —
// VM, AI and access alike; only the base-interface menu label differs.
func checkCredentials(ctx context.Context, p func(string)) bool {
	cfg, err := config.Load()
	if err != nil {
		p(ui.RowFail("credentials", "config unreadable: "+err.Error()))
		return false
	}

	ok, anyConfigured := true, false
	for _, pr := range providers {
		v, isValidator := pr.(provider.CredentialValidator)
		if !isValidator {
			continue
		}
		if !v.Configured(cfg) {
			p(ui.RowWarn(pr.Label(), "not configured"))
			continue
		}

		anyConfigured = true
		if err := validateCreds(ctx, v, cfg); err != nil {
			ok = false
			p(ui.RowFail(pr.Label(), err.Error()))
			p("  " + ui.Faint.Render("run interview init to replace the credentials"))
			continue
		}
		p(ui.RowOK(pr.Label(), "credentials valid"))
	}
	if !anyConfigured {
		p("  " + ui.Faint.Render("run interview init to configure a provider"))
	}
	return ok
}
