package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	interviewlabs "github.com/openbuzz/interview-labs"
	"github.com/openbuzz/interview-labs/internal/pack"
	"github.com/openbuzz/interview-labs/internal/ui"
)

func newPackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pack",
		Short: "inspect and scaffold content packs",
		Long: `Work with content packs.

A pack is one organization's interview content: one bundle per position,
each bundle holding the scenarios a candidate works on. The embedded
"default" pack ships ready-made bundles; "template" is the scaffold that
"interview pack init" copies for authoring your own.`,
	}
	cmd.AddCommand(newPackValidateCmd(), newPackInitCmd())
	return cmd
}

func newPackValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <ref>",
		Short: "load a pack and report its contents",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := pack.Resolve(args[0])
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, ui.Section(
				ui.SectionTitle("pack")+" "+ui.Accent.Bold(true).Render(p.Name),
				packRows(p)...))
			return nil
		},
	}
}

// packRows renders one row per bundle: image, scenario count, kind marker.
func packRows(p *pack.Pack) []string {
	rows := []string{fmt.Sprintf("%-10s%s", "version", p.Version)}
	for _, b := range p.Bundles {
		detail := fmt.Sprintf("%s, %d scenarios", b.Image, len(b.Scenarios))
		if b.HasKind {
			detail += ", kind cluster"
		}
		if b.HasSetup {
			detail += ", setup.sh"
		}
		rows = append(rows, fmt.Sprintf("%-10s%s", b.Name, detail))
	}
	return rows
}

func newPackInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init <dir>",
		Short: "scaffold a new pack from the embedded template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dst := args[0]
			if entries, err := os.ReadDir(dst); err == nil && len(entries) > 0 {
				return fmt.Errorf("%s is not empty", dst)
			}

			if err := copyTemplate(dst); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), ui.RowOK("pack init", dst))
			fmt.Fprintln(cmd.OutOrStdout(),
				ui.Next("interview pack validate "+dst,
					"interview launch --pack "+dst+" --bundle demo"))
			return nil
		},
	}
}

// copyTemplate materializes the embedded template pack: *.sh 0755, rest 0644.
func copyTemplate(dst string) error {
	sub, err := fs.Sub(interviewlabs.PacksFS, "packs/template")
	if err != nil {
		return err
	}
	return fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := fs.ReadFile(sub, p)
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if filepath.Ext(p) == ".sh" {
			mode = 0o755
		}
		return os.WriteFile(target, data, mode)
	})
}
