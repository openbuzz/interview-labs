package cli

import (
	"context"
	"fmt"
	"io"

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
		Long: `Tear a session down.

Runs terraform destroy with the binary that applied the session, then
archives metadata and logs under the XDG state directory and removes the
session dir. Pass --yes to skip the confirmation.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDestroyCmd(cmd, args, yes)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

// runDestroyCmd resolves the session, confirms, and drives the destroy.
func runDestroyCmd(cmd *cobra.Command, args []string, yes bool) error {
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

	runner, err := destroyRunner(out, s)
	if err != nil {
		return err
	}
	return runDestroy(cmd.Context(), out, runner, s)
}

// destroyRunner rebuilds the runner for a session, preferring the binary that
// applied it and warning when falling back.
func destroyRunner(out io.Writer, s *session.Session) (*terraform.Runner, error) {
	bin, err := terraform.FindNamed(s.Meta.Terraform.Binary)
	if err != nil {
		bin, err = terraform.Find()
		if err != nil {
			return nil, err
		}
		fmt.Fprintln(out, ui.RowWarn("terraform",
			fmt.Sprintf("%s %s applied this session; using %s %s",
				s.Meta.Terraform.Binary, s.Meta.Terraform.Version,
				bin.Name, bin.Version)))
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	vmName := s.Meta.Roles["vm"]
	vm, ok := vmByName(vmName)
	if !ok {
		return nil, fmt.Errorf("session %s uses unknown provider %q",
			s.Meta.Slug, vmName)
	}
	cache, err := terraform.PluginCacheDir()
	if err != nil {
		return nil, err
	}

	return &terraform.Runner{
		Bin: bin, Dir: s.TerraformDir(),
		Env:     terraform.RunEnv(vm.EnvCreds(cfg), cache),
		LogsDir: s.LogsDir(), Out: out,
	}, nil
}

// runDestroy inits, destroys, and archives; failures keep the session with a
// failed-destroy status and point at the logs.
func runDestroy(ctx context.Context, out io.Writer,
	r *terraform.Runner, s *session.Session) error {
	s.SetStatus(session.StatusDestroying)
	if err := r.Init(ctx); err != nil {
		return failDestroy(out, s, err)
	}
	if err := r.Destroy(ctx); err != nil {
		return failDestroy(out, s, err)
	}

	if err := s.Archive(); err != nil {
		return err
	}
	fmt.Fprintln(out, ui.RowOK(s.Meta.Slug, "destroyed"))
	fmt.Fprintln(out, ui.Next("interview launch"))
	return nil
}

// failDestroy records the failure and prints the recovery hints.
func failDestroy(out io.Writer, s *session.Session, err error) error {
	s.SetStatus(session.StatusFailedDestroy)
	fmt.Fprintln(out, ui.Faint.Render("logs: "+s.LogsDir()))
	fmt.Fprintln(out, ui.Next("interview destroy "+s.Meta.Slug))
	return err
}
