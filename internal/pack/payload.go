package pack

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// StagePayload materializes the bundle's candidate-facing content under
// dst/payload: every scenario's task/ tree as workspace/<scenario>/, the
// lab/ tree verbatim, and the kind config when present. solution/ and
// scenario.yaml never leave the pack FS — unshippable by construction.
func StagePayload(dst string, fsys fs.FS, b *Bundle) error {
	for _, sc := range b.Scenarios {
		src := path.Join("bundles", b.Name, "scenarios", sc.Name, "task")
		if err := copyTree(fsys, src,
			filepath.Join(dst, "payload", "workspace", sc.Name)); err != nil {
			return err
		}
	}
	if b.HasLab {
		if err := copyTree(fsys, path.Join("bundles", b.Name, "lab"),
			filepath.Join(dst, "payload", "lab")); err != nil {
			return err
		}
	}
	if b.HasKind {
		if err := copyTree(fsys, path.Join("bundles", b.Name, "kind"),
			filepath.Join(dst, "payload", "kind")); err != nil {
			return err
		}
	}
	return nil
}

// copyTree copies one subtree out of the pack FS: dirs 0755, files 0644,
// *.sh 0755. A file named _go.mod is written back as go.mod — go:embed
// silently drops any subtree containing a nested go.mod (see embed.go), so
// exercise Go modules are checked in under that placeholder name and
// restored here, uniformly for both lab/ and workspace/ trees.
func copyTree(fsys fs.FS, src, dst string) error {
	return fs.WalkDir(fsys, src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if filepath.Base(target) == "_go.mod" {
			target = filepath.Join(filepath.Dir(target), "go.mod")
		}

		data, err := fs.ReadFile(fsys, p)
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
