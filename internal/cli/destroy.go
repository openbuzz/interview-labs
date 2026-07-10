package cli

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/session"
	"github.com/openbuzz/interview-labs/internal/terraform"
	"github.com/openbuzz/interview-labs/internal/ui"
)

// confirmDestroy is a seam; production asks via huh.
var confirmDestroy = func(s *session.Session) (bool, error) {
	ok := false
	err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("Destroy %s (%s, %s)?",
				s.Meta.Slug, s.Meta.IP, s.Meta.Region)).
			Value(&ok),
	)).WithTheme(ui.Theme()).Run()
	return ok, err
}

func newDestroyCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "destroy [slug]",
		Short: "destroy a session VM",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			ref := ""
			if len(args) == 1 {
				ref = args[0]
			}
			s, err := resolveSession(ref)
			if err != nil {
				return err
			}
			if !yes {
				if !isTTY() {
					return usageError("destroy needs --yes when not on a terminal")
				}
				ok, err := confirmDestroy(s)
				if err != nil || !ok {
					return err
				}
			}
			release, err := s.Lock()
			if err != nil {
				return err
			}
			defer release()

			// prefer the binary that applied; fall back with a warning
			bin, err := terraform.FindNamed(s.Meta.Terraform.Binary)
			if err != nil {
				bin, err = terraform.Find()
				if err != nil {
					return err
				}
				fmt.Fprintln(out, ui.RowWarn("terraform",
					fmt.Sprintf("%s %s applied this session; using %s %s",
						s.Meta.Terraform.Binary, s.Meta.Terraform.Version,
						bin.Name, bin.Version)))
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cache, err := terraform.PluginCacheDir()
			if err != nil {
				return err
			}
			runner := &terraform.Runner{
				Bin: bin, Dir: s.TerraformDir(),
				Env:     terraform.RunEnv(cfg.Token(), cache),
				LogsDir: s.LogsDir(), Out: out,
			}

			s.SetStatus(session.StatusDestroying)
			if err := runner.Init(cmd.Context()); err != nil {
				s.SetStatus(session.StatusFailedDestroy)
				fmt.Fprintln(out, ui.Faint.Render("logs: "+s.LogsDir()))
				fmt.Fprintln(out, ui.Next("interview destroy "+s.Meta.Slug))
				return err
			}
			if err := runner.Destroy(cmd.Context()); err != nil {
				s.SetStatus(session.StatusFailedDestroy)
				fmt.Fprintln(out, ui.Faint.Render("logs: "+s.LogsDir()))
				fmt.Fprintln(out, ui.Next("interview destroy "+s.Meta.Slug))
				return err
			}

			if err := s.Archive(); err != nil {
				return err
			}
			fmt.Fprintln(out, ui.RowOK(s.Meta.Slug, "destroyed"))
			fmt.Fprintln(out, ui.Next("interview launch"))
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}
