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
	"github.com/openbuzz/interview-labs/internal/digitalocean"
	"github.com/openbuzz/interview-labs/internal/hetzner"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/ui"
)

// providers is the registry, in menu order.
var providers = []provider.Provider{digitalocean.New(), hetzner.New(), aws.New()}

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

const actionExit = "exit"

// pickMainAction is a seam; production shows the main menu.
var pickMainAction = func() (string, error) {
	opts := []huh.Option[string]{
		huh.NewOption("Doctor   — check tools and credentials", "doctor"),
		huh.NewOption("Init     — configure cloud providers", "init"),
		huh.NewOption("Launch   — deploy a session VM", "launch"),
		huh.NewOption("List     — show sessions", "list"),
		huh.NewOption("SSH      — open a shell on a session VM", "ssh"),
		huh.NewOption("Destroy  — tear a session down", "destroy"),
		huh.NewOption("Exit", actionExit),
	}

	var sel string
	err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("What do you want to do?").
			Options(opts...).Value(&sel),
	)).WithTheme(ui.Theme()).Run()
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
		Use:   "interview",
		Short: "disposable interview lab VMs",
		Long: `interview runs one disposable cloud VM per interview session.

Commands:
  doctor   Check required tools, directories and credentials.
  init     Configure cloud provider credentials (guided, re-runnable).
  launch   Deploy a session VM: pick provider, region and instance size.
  list     Show sessions with provider, age and status.
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
			fmt.Fprintln(cmd.OutOrStdout(), ui.Logo())

			for {
				action, err := pickMainAction()
				if err != nil {
					return err
				}
				if action == actionExit {
					return nil
				}
				out, errW := cmd.OutOrStdout(), cmd.ErrOrStderr()
				if err := runSubcommand(cmd.Context(), action, out, errW); err != nil {
					fmt.Fprintf(errW, "error: %v\n", err)
				}
			}
		},
	}
	root.PersistentFlags().BoolVar(&verbose, "verbose", false,
		"stream terraform output instead of progress rows")
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newLaunchCmd())
	root.AddCommand(newListCmd(), newSSHCmd(), newDestroyCmd())
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
