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

// sessionTitle renders the facts header — SESSION <slug> — <state> — with
// the state word colored by lifecycle.
func sessionTitle(s *session.Session) string {
	style := ui.Accent
	switch s.Meta.Status {
	case session.StatusReady:
		style = ui.OK
	case session.StatusFailed, session.StatusFailedDestroy:
		style = ui.Fail
	}
	return ui.SectionTitle("session") + " " +
		ui.Accent.Bold(true).Render(s.Meta.Slug) +
		ui.Faint.Render(" — ") + style.Render(s.Meta.Status)
}

// sessionSection renders the session facts; the header owns the status and
// the copy zones own url/password, so none of those appear as rows. Rows
// without a value (local sessions have no IP/region/size) are omitted.
func sessionSection(s *session.Session) string {
	rows := []string{fmt.Sprintf("%-10s%s", "provider", s.Meta.Roles["vm"])}
	rows = appendRow(rows, "profile", s.Meta.Profile)
	if s.Meta.AIKeyHash != "" {
		cap := strconv.FormatFloat(s.Meta.AICapUSD, 'f', -1, 64)
		rows = append(rows, fmt.Sprintf("%-10s%s (cap $%s)", "ai",
			s.Meta.Roles["ai"], cap))
	}
	rows = appendRow(rows, "ip", s.Meta.IP)
	rows = appendRow(rows, "dns", s.Meta.FQDN)
	rows = appendRow(rows, "os", s.Meta.Image)
	rows = appendRow(rows, "ssh user", s.Meta.SSHUser)
	rows = appendRow(rows, "region", s.Meta.Region)
	rows = appendRow(rows, "size", s.Meta.Size)
	rows = append(rows, fmt.Sprintf("%-10s%s", "created",
		s.Meta.CreatedAt.Local().Format("2006-01-02 15:04 -07:00")))
	return ui.Section(sessionTitle(s), rows...)
}

// appendRow adds one labeled row when the value is non-empty.
func appendRow(rows []string, label, value string) []string {
	if value == "" {
		return rows
	}
	return append(rows, fmt.Sprintf("%-10s%s", label, value))
}

// printHandover renders the facts section and, for ready sessions, the
// candidate and ssh copy zones, then the NEXT block.
func printHandover(out io.Writer, s *session.Session) {
	fmt.Fprintln(out, sessionSection(s))

	next := []string{"interview destroy " + s.Meta.Slug}
	if s.Meta.Status == session.StatusReady {
		if s.Meta.URL != "" {
			lines := []string{s.Meta.URL}
			if s.Meta.GatewayPassword != "" {
				lines = append(lines, "password: "+s.Meta.GatewayPassword)
			}
			fmt.Fprintln(out)
			fmt.Fprintln(out, ui.CopyZone("send to candidate", lines...))
		}
		if s.Meta.IP != "" {
			fmt.Fprintln(out)
			fmt.Fprintln(out, ui.CopyZone("ssh", strings.Join(ssh.Argv(
				s.KeyPath(), s.KnownHostsPath(), s.Meta.SSHUser, s.Meta.IP), " ")))
		}
		next = append([]string{"interview ssh " + s.Meta.Slug}, next...)
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, ui.Next(next...))
}
