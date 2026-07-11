package cli

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/openbuzz/interview-labs/internal/session"
	"github.com/openbuzz/interview-labs/internal/ssh"
	"github.com/openbuzz/interview-labs/internal/ui"
)

// resolveSession picks a session by ref, sole-session default, or picker;
// title and desc caption the picker for the calling command.
func resolveSession(ref, title, desc string) (*session.Session, error) {
	if ref != "" {
		return session.Get(ref)
	}
	all, err := session.List()
	if err != nil {
		return nil, err
	}
	switch len(all) {
	case 0:
		return nil, fmt.Errorf("no sessions — run interview launch")
	case 1:
		return all[0], nil
	}
	if !isTTY() {
		return nil, usageError("several sessions — name one: interview <cmd> <slug>")
	}
	return pickSession(all, title, desc)
}

// pickSession is a seam; production shows a huh select.
var pickSession = func(all []*session.Session,
	title, desc string) (*session.Session, error) {
	return huhPickSession(all, title, desc)
}

// execProgram is a seam; production replaces the process with argv's
// program (host ssh for cloud sessions, docker exec for local ones).
var execProgram = func(argv []string) error {
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return fmt.Errorf("%s not found on PATH", argv[0])
	}
	return syscall.Exec(path, argv, os.Environ())
}

func newSSHCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ssh [slug]",
		Short: "open a shell on a session VM",
		Long: `Open an interactive shell on a session VM.

Cloud sessions hand the terminal to the host ssh binary with the
session's key and its pinned host key (per-session known_hosts). Local
sessions exec into the vscode container instead. With several sessions,
pass a slug or pick one from the menu.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			printNarrowWarning(cmd.OutOrStdout())
			ref := ""
			if len(args) == 1 {
				ref = args[0]
			}
			s, err := resolveSession(ref, "Select a session",
				"Opens an interactive shell on the session VM.")
			if err != nil {
				return err
			}
			if isLocalSession(s) {
				return execProgram([]string{"docker", "exec", "-it",
					"interview-" + s.Meta.Slug + "-vscode", "bash"})
			}
			if s.Meta.IP == "" {
				return fmt.Errorf("session %s has no IP (status %s)",
					s.Meta.Slug, s.Meta.Status)
			}
			return execProgram(ssh.Argv(
				s.KeyPath(), s.KnownHostsPath(), s.Meta.SSHUser, s.Meta.IP))
		},
	}
}

func huhPickSession(all []*session.Session,
	title, desc string) (*session.Session, error) {
	opts := make([]huh.Option[string], 0, len(all))
	for _, s := range all {
		opts = append(opts, huh.NewOption(
			fmt.Sprintf("%s  %s  %s", s.Meta.Slug, s.Meta.Region, s.Meta.Status),
			s.Meta.Slug))
	}
	var slug string
	if err := ui.SelectForm(title, desc, opts, &slug); err != nil {
		return nil, err
	}
	return session.Get(slug)
}
