package cli

import (
	"fmt"
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

			fmt.Fprintln(out, ui.Faint.Render(fmt.Sprintf(
				"%-24s %-13s %-8s %-16s %-8s %s",
				"SLUG", "PROVIDER", "REGION", "IP", "AGE", "STATUS")))
			now := time.Now().UTC()
			for _, s := range all {
				fmt.Fprintf(out, "%-24s %-13s %-8s %-16s %-8s %s\n",
					s.Meta.Slug, s.Meta.Roles["vm"], s.Meta.Region, s.Meta.IP,
					formatAge(s.Age(now)), s.Meta.Status)
			}
			return nil
		},
	}
}
