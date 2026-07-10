// Package cli assembles the interview command tree.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/spf13/cobra"

	"github.com/openbuzz/interview-labs/internal/digitalocean"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/ui"
)

// providers is the registry, in menu order.
var providers = []provider.Provider{digitalocean.New()}

type usageErr struct{ msg string }

func (e usageErr) Error() string { return e.msg }

func usageError(msg string) error { return usageErr{msg: msg} }

// IsUsage reports whether err should map to exit code 2.
func IsUsage(err error) bool {
	var u usageErr
	return errors.As(err, &u)
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
		RunE:          func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
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
