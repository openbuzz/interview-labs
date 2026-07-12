// Package cli assembles the interview command tree.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/openbuzz/interview-labs/internal/aws"
	"github.com/openbuzz/interview-labs/internal/cloudflare"
	"github.com/openbuzz/interview-labs/internal/digitalocean"
	"github.com/openbuzz/interview-labs/internal/hetzner"
	"github.com/openbuzz/interview-labs/internal/localvm"
	"github.com/openbuzz/interview-labs/internal/openrouter"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/ui"
	"github.com/openbuzz/interview-labs/internal/version"
)

// providers is the registry, in menu order.
var providers = []provider.Provider{
	digitalocean.New(), hetzner.New(), aws.New(), localvm.New(),
	openrouter.New(), cloudflare.New(),
}

type usageErr struct{ msg string }

func (e usageErr) Error() string { return e.msg }

func usageError(msg string) error { return usageErr{msg: msg} }

// IsUsage reports whether err should map to exit code 2.
func IsUsage(err error) bool {
	var u usageErr
	return errors.As(err, &u)
}

// verbose streams raw terraform output instead of quiet phase rows.
var verbose bool

// quietOutput reports whether phases render as spinner rows: interactive
// terminal, not overridden by --verbose. Pipes and CI always stream.
func quietOutput() bool { return isTTY() && !verbose }

// printNarrowWarning surfaces the once-per-process narrow-terminal warning.
func printNarrowWarning(out io.Writer) {
	if w := ui.NarrowWarning(); w != "" {
		fmt.Fprintln(out, w)
	}
}

// printLogoOnce surfaces the once-per-process logo block plus its trailing
// blank line, so menu-dispatched subcommands never repeat the wordmark.
func printLogoOnce(out io.Writer) {
	if l := ui.LogoOnce(); l != "" {
		fmt.Fprintln(out, l)
		fmt.Fprintln(out)
	}
}

const actionExit = "exit"

// pickMainAction is a seam; production shows the main menu.
var pickMainAction = func() (string, error) {
	opts := []huh.Option[string]{
		huh.NewOption("doctor   — check tools and credentials", "doctor"),
		huh.NewOption("init     — configure cloud providers", "init"),
		huh.NewOption("launch   — deploy a session VM", "launch"),
		huh.NewOption("list     — show sessions", "list"),
		huh.NewOption("info     — show session details", "info"),
		huh.NewOption("ssh      — open a shell on a session VM", "ssh"),
		huh.NewOption("destroy  — tear a session down", "destroy"),
		huh.NewOption("exit", actionExit),
	}

	var sel string
	err := ui.SelectForm("What do you want to do?",
		"Arrows move, Enter selects, ESC exits.",
		opts, &sel)
	if errors.Is(err, huh.ErrUserAborted) {
		return actionExit, nil // ESC behaves like Exit
	}
	if err != nil {
		return "", err
	}
	return sel, nil
}

// runSubcommand is a seam; production executes one subcommand on a fresh
// command tree so flag and arg state never bleeds between menu picks. out
// and errW are threaded from the caller so output lands on the same writers
// as the bare-root invocation instead of the process's real stdout/stderr.
// Assigned from init, not inline: its default calls newRootCmd, whose RunE
// calls back into runSubcommand — an inline initializer is a Go
// initialization cycle (runSubcommand -> newRootCmd -> runSubcommand).
var runSubcommand func(ctx context.Context, name string, out, errW io.Writer) error

func init() {
	runSubcommand = func(ctx context.Context, name string, out, errW io.Writer) error {
		c := newRootCmd()
		c.SetArgs([]string{name})
		c.SetOut(out)
		c.SetErr(errW)
		return c.ExecuteContext(ctx)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "interview",
		Short:   "disposable interview lab VMs",
		Version: func() string { info, _ := version.ResolvePins(); return info.Version }(),
		Long: `interview runs one disposable cloud VM per interview session.

Commands:
  doctor   Check required tools, directories and credentials.
  init     Configure cloud provider credentials (guided, re-runnable).
  launch   Deploy a session VM: pick provider, region and instance size.
  list     Show sessions with provider, age and status.
  info     Show one session's details.
  ssh      Open a shell on a session VM.
  destroy  Tear a session down and archive its logs.

Start with "interview doctor", then "interview init".`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !isTTY() {
				return cmd.Help()
			}
			out, errW := cmd.OutOrStdout(), cmd.ErrOrStderr()
			printLogoOnce(out)
			printNarrowWarning(out)

			for {
				action, err := pickMainAction()
				if err != nil {
					return err
				}
				if action == actionExit {
					return nil
				}
				if err := runSubcommand(cmd.Context(), action, out, errW); err != nil {
					fmt.Fprintf(errW, "error: %v\n", err)
				}
				fmt.Fprintln(out)
			}
		},
	}
	root.PersistentFlags().BoolVar(&verbose, "verbose", false,
		"stream terraform output instead of progress rows")
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newLaunchCmd())
	root.AddCommand(newListCmd(), newInfoCmd(), newSSHCmd(), newDestroyCmd())
	root.SetHelpTemplate(ui.Logo() + "\n\n" + root.HelpTemplate())
	return root
}

func isCobraUsage(err error) bool {
	s := err.Error()
	return strings.HasPrefix(s, "unknown command") || strings.HasPrefix(s, "unknown flag") ||
		strings.HasPrefix(s, "invalid argument")
}

func run(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	root := newRootCmd()
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		if IsUsage(err) || isCobraUsage(err) {
			return 2
		}
		return 1
	}
	return 0
}

// Execute runs the CLI and returns the process exit code.
func Execute() int { return run(os.Args[1:]) }
