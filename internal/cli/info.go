package cli

import (
	"github.com/spf13/cobra"
)

func newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info [slug]",
		Short: "show session details",
		Long: `Show one session's details.

Prints the session box — status, provider, IP, OS image, ssh user, region,
size, created — plus the raw ssh line and next steps for ready sessions.
Without a slug: the sole session, or an interactive picker on a terminal.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			printNarrowWarning(out)

			ref := ""
			if len(args) == 1 {
				ref = args[0]
			}
			s, err := resolveSession(ref, "Select a session",
				"Shows the session's connection and lifecycle details.")
			if err != nil {
				return err
			}

			printHandover(out, s)
			return nil
		},
	}
}
