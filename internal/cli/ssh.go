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

// resolveSession picks a session by ref, sole-session default, or picker.
func resolveSession(ref string) (*session.Session, error) {
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
	return pickSession(all)
}

// pickSession is a seam; production shows a huh select.
var pickSession = func(all []*session.Session) (*session.Session, error) {
	// implemented with huh in this file; tests replace the var
	return huhPickSession(all)
}

// execSSH is a seam; production replaces the process with host ssh.
var execSSH = func(argv []string) error {
	path, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh client not found — install openssh-client to use " +
			"interview ssh")
	}
	return syscall.Exec(path, argv, os.Environ())
}

func newSSHCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ssh [slug]",
		Short: "open a shell on a session VM",
		Long: `Open an interactive shell on a session VM.

Hands the terminal to the host ssh binary with the session's key and its
pinned host key (per-session known_hosts). With several sessions, pass a
slug or pick one from the menu.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := ""
			if len(args) == 1 {
				ref = args[0]
			}
			s, err := resolveSession(ref)
			if err != nil {
				return err
			}
			if s.Meta.IP == "" {
				return fmt.Errorf("session %s has no IP (status %s)",
					s.Meta.Slug, s.Meta.Status)
			}
			return execSSH(ssh.Argv(
				s.KeyPath(), s.KnownHostsPath(), "root", s.Meta.IP))
		},
	}
}

func huhPickSession(all []*session.Session) (*session.Session, error) {
	opts := make([]huh.Option[string], 0, len(all))
	for _, s := range all {
		opts = append(opts, huh.NewOption(
			fmt.Sprintf("%s  %s  %s", s.Meta.Slug, s.Meta.Region, s.Meta.Status),
			s.Meta.Slug))
	}
	var slug string
	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Session").Options(opts...).Value(&slug),
	)).WithTheme(ui.Theme()).Run(); err != nil {
		return nil, err
	}
	return session.Get(slug)
}
