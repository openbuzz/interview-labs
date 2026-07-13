package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/openbuzz/interview-labs/internal/session"
	"github.com/openbuzz/interview-labs/internal/ui"
)

func formatAge(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "list sessions",
		Long: `List sessions with provider, region, IP, age and status.

Reads session state from the XDG state directory; unreadable session
dirs are skipped rather than failing the listing.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			all, err := session.List()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(all) == 0 {
				fmt.Fprintln(out, "no sessions")
				fmt.Fprintln(out, ui.Next("interview launch"))
				return nil
			}

			renderList(out, all, time.Now().UTC())
			return nil
		},
	}
}

// renderList prints sessions in columns sized to their content.
func renderList(out io.Writer, all []*session.Session, now time.Time) {
	head := []string{"SLUG", "PROVIDER", "REGION", "IP", "AGE", "STATUS"}
	lines := ui.Columns(append([][]string{head}, listRows(all, now)...))

	fmt.Fprintln(out, ui.Faint.Render(lines[0]))
	for _, l := range lines[1:] {
		fmt.Fprintln(out, l)
	}
}

// listRows converts sessions into display cells; an empty IP renders as "-".
func listRows(all []*session.Session, now time.Time) [][]string {
	rows := make([][]string, 0, len(all))
	for _, s := range all {
		ip := s.Meta.IP
		if ip == "" {
			ip = "-"
		}
		rows = append(rows, []string{s.Meta.Slug, s.Meta.Roles["vm"], s.Meta.Region,
			ip, formatAge(s.Age(now)), s.Meta.Status})
	}
	return rows
}
