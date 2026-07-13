package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/kindx"
	"github.com/openbuzz/interview-labs/internal/session"
	"github.com/openbuzz/interview-labs/internal/terraform"
	"github.com/openbuzz/interview-labs/internal/ui"
)

// confirmDestroy is a seam; production asks via huh.
var confirmDestroy = func(s *session.Session) (bool, error) {
	ok := false
	target := fmt.Sprintf("(%s, %s)", s.Meta.IP, s.Meta.Region)
	desc := "Removes the VM and firewall; logs are archived locally."
	if isLocalSession(s) {
		target = "(local docker)"
		desc = "Stops the local stack and removes its volumes; logs are archived."
	}
	err := ui.ConfirmForm(
		fmt.Sprintf("Destroy %s %s?", s.Meta.Slug, target), desc, &ok)
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
	printNarrowWarning(out)
	ref := ""
	if len(args) == 1 {
		ref = args[0]
	}
	s, err := resolveSession(ref, "Select a session to destroy",
		"Tears down the cloud resources and archives logs. Stops billing.")
	if err != nil {
		return err
	}

	proceed, err := confirmDestroyGate(s, yes)
	if err != nil || !proceed {
		return err
	}
	release, err := s.Lock()
	if err != nil {
		return err
	}
	defer release()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if isLocalSession(s) {
		return runLocalDestroy(cmd.Context(), out, cfg, s)
	}
	runnerOut := io.Writer(out)
	if quietOutput() {
		runnerOut = io.Discard
	}
	runner, err := destroyRunner(out, runnerOut, cfg, s)
	if err != nil {
		return err
	}
	return runDestroy(cmd.Context(), out, cfg, runner, s)
}

// confirmDestroyGate blocks on the destroy confirmation unless yes is set;
// proceed is false on a clean decline (err is nil) or a hard failure.
func confirmDestroyGate(s *session.Session, yes bool) (proceed bool, err error) {
	if yes {
		return true, nil
	}
	if !isTTY() {
		return false, usageError("destroy needs --yes when not on a terminal")
	}
	ok, err := confirmDestroy(s)
	if err != nil || !ok {
		return false, err
	}
	return true, nil
}

// destroyRunner rebuilds the runner for a session, preferring the binary that
// applied it and warning when falling back. The fallback warning always goes
// to out; runnerOut feeds the Runner's own Out field (routed when quiet).
func destroyRunner(out, runnerOut io.Writer, cfg config.Config,
	s *session.Session) (*terraform.Runner, error) {
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

	vmName := s.Meta.Roles["vm"]
	vm, ok := vmByName(vmName)
	if !ok {
		return nil, fmt.Errorf("session %s uses unknown provider %q",
			s.Meta.Slug, vmName)
	}
	creds := vm.EnvCreds(cfg)
	if name := s.Meta.Roles["access"]; name != "" {
		acc, ok := accessByName(name)
		if !ok {
			return nil, fmt.Errorf("session %s uses unknown access provider %q",
				s.Meta.Slug, name)
		}
		creds = mergeCreds(creds, acc.EnvCreds(cfg))
	}
	cache, err := terraform.PluginCacheDir()
	if err != nil {
		return nil, err
	}

	return &terraform.Runner{
		Bin: bin, Dir: s.TerraformDir(),
		Env:     terraform.RunEnv(creds, cache),
		LogsDir: s.LogsDir(), Out: runnerOut,
	}, nil
}

// runDestroy inits, destroys, revokes the session's AI key, and archives;
// failures keep the session with a failed-destroy status and point at the
// logs. A rerun is safe: terraform destroy is idempotent and a 404 on
// revoke counts as success.
func runDestroy(ctx context.Context, out io.Writer, cfg config.Config,
	r *terraform.Runner, s *session.Session) error {
	quiet := quietOutput()
	s.SetStatus(session.StatusDestroying)
	if err := step(out, quiet, "terraform init", func() error {
		return r.Init(ctx)
	}); err != nil {
		return failDestroy(out, s, err)
	}
	if err := step(out, quiet, "terraform destroy", func() error {
		return r.Destroy(ctx)
	}); err != nil {
		return failDestroy(out, s, err)
	}
	if err := revokeAIKey(ctx, out, quiet, cfg, s); err != nil {
		return failDestroy(out, s, err)
	}

	if err := s.Archive(); err != nil {
		return err
	}
	fmt.Fprintln(out, ui.RowOK(s.Meta.Slug, "destroyed"))
	fmt.Fprintln(out, ui.Next("interview launch"))
	return nil
}

// runLocalDestroy tears the local stack down: compose down (volumes
// included), key revoke, archive. No terraform. Reruns are safe — down on
// an absent project succeeds.
func runLocalDestroy(ctx context.Context, out io.Writer, cfg config.Config,
	s *session.Session) error {
	quiet := quietOutput()
	s.SetStatus(session.StatusDestroying)
	if s.Meta.Kind {
		if err := step(out, quiet, "delete cluster", func() error {
			return kindx.DeleteLocal(ctx, s.Meta.Slug, io.Discard)
		}); err != nil {
			return failDestroy(out, s, err)
		}
	}
	if err := step(out, quiet, "compose down", func() error {
		return execDocker(ctx, out, quiet, s, "stack-down.log", "", nil,
			"compose", "-p", "interview-"+s.Meta.Slug, "down", "-v")
	}); err != nil {
		return failDestroy(out, s, err)
	}
	if err := revokeAIKey(ctx, out, quiet, cfg, s); err != nil {
		return failDestroy(out, s, err)
	}

	if err := s.Archive(); err != nil {
		return err
	}
	fmt.Fprintln(out, ui.RowOK(s.Meta.Slug, "destroyed"))
	fmt.Fprintln(out, ui.Next("interview launch"))
	return nil
}

// revokeAIKey revokes the session's minted key when one exists; sessions
// without a hash (never minted, or pre-AI) are a no-op.
func revokeAIKey(ctx context.Context, out io.Writer, quiet bool,
	cfg config.Config, s *session.Session) error {
	if s.Meta.AIKeyHash == "" {
		return nil
	}
	name := s.Meta.Roles["ai"]
	ai, ok := aiByName(name)
	if !ok {
		return fmt.Errorf("session %s uses unknown ai provider %q", s.Meta.Slug, name)
	}
	return step(out, quiet, "revoke ai key", func() error {
		return ai.Revoke(ctx, cfg, s.Meta.AIKeyHash)
	})
}

// failDestroy records the failure and prints the recovery hints.
func failDestroy(out io.Writer, s *session.Session, err error) error {
	s.SetStatus(session.StatusFailedDestroy)
	fmt.Fprintln(out, ui.Faint.Render("logs: "+s.LogsDir()))
	fmt.Fprintln(out, ui.Next("interview destroy "+s.Meta.Slug))
	return err
}
