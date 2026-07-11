package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/openbuzz/interview-labs/internal/session"
	"github.com/openbuzz/interview-labs/internal/ssh"
	"github.com/openbuzz/interview-labs/internal/ui"
)

// sessionBox renders the session facts box; the border tracks status.
func sessionBox(s *session.Session) string {
	style := ui.Accent
	switch s.Meta.Status {
	case session.StatusReady:
		style = ui.OK
	case session.StatusFailed, session.StatusFailedDestroy:
		style = ui.Fail
	}

	ip := s.Meta.IP
	if ip == "" {
		ip = "—"
	}
	rows := []string{
		fmt.Sprintf("%-10s%s", "status", s.Meta.Status),
		fmt.Sprintf("%-10s%s", "provider", s.Meta.Roles["vm"]),
	}
	if s.Meta.AIKeyHash != "" {
		cap := strconv.FormatFloat(s.Meta.AICapUSD, 'f', -1, 64)
		rows = append(rows, fmt.Sprintf("%-10s%s (cap $%s)", "ai",
			s.Meta.Roles["ai"], cap))
	}
	rows = append(rows, fmt.Sprintf("%-10s%s", "ip", ip))
	if s.Meta.FQDN != "" {
		rows = append(rows, fmt.Sprintf("%-10s%s", "dns", s.Meta.FQDN))
	}
	rows = append(rows,
		fmt.Sprintf("%-10s%s", "os", s.Meta.Image),
		fmt.Sprintf("%-10s%s", "ssh user", s.Meta.SSHUser),
		fmt.Sprintf("%-10s%s", "region", s.Meta.Region),
		fmt.Sprintf("%-10s%s", "size", s.Meta.Size),
		fmt.Sprintf("%-10s%s", "created",
			s.Meta.CreatedAt.Local().Format("2006-01-02 15:04 -07:00")),
	)
	return ui.Box("Session "+s.Meta.Slug, style, rows...)
}

// printHandover renders the box, the raw ssh line for ready sessions, and
// the NEXT block.
func printHandover(out io.Writer, s *session.Session) {
	fmt.Fprintln(out, sessionBox(s))

	next := []string{"interview destroy " + s.Meta.Slug}
	if s.Meta.Status == session.StatusReady && s.Meta.IP != "" {
		line := strings.Join(ssh.Argv(
			s.KeyPath(), s.KnownHostsPath(), s.Meta.SSHUser, s.Meta.IP), " ")
		fmt.Fprintln(out, ui.Faint.Render(line))
		next = append([]string{"interview ssh " + s.Meta.Slug}, next...)
	}
	fmt.Fprintln(out, ui.Next(next...))
}
