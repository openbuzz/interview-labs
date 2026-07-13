package pack

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// dirNamePattern is the bundle/scenario directory charset — the pack name
// pattern from pack.schema.json reused for directories.
var dirNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func validateDirName(kind, name string) error {
	if !dirNamePattern.MatchString(name) {
		return fmt.Errorf("invalid %s directory name %q: must match %s",
			kind, name, dirNamePattern.String())
	}
	return nil
}

// validateKindPairing enforces "kind/manifests/ without cluster.yaml is an
// error" (spec §4.2).
func validateKindPairing(fsys fs.FS, bundleDir string, hasKind bool) error {
	manifests, err := fs.Glob(fsys, path.Join(bundleDir, "kind/manifests/*.yaml"))
	if err != nil {
		return fmt.Errorf("glob kind manifests: %w", err)
	}
	if len(manifests) > 0 && !hasKind {
		return fmt.Errorf("bundle %s: kind/manifests present without kind/cluster.yaml",
			bundleDir)
	}
	return nil
}

// LoadDir loads a pack from a real directory, first rejecting any symlink
// that resolves outside the pack root (dir packs only — embedded packs
// cannot contain symlinks).
func LoadDir(root string) (*Pack, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := rejectEscapingSymlinks(abs); err != nil {
		return nil, err
	}
	return Load(os.DirFS(abs))
}

func rejectEscapingSymlinks(root string) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	return filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return nil
		}
		resolved, err := filepath.EvalSymlinks(p)
		if err != nil {
			return fmt.Errorf("symlink %s: %w", p, err)
		}
		if resolved != resolvedRoot &&
			!strings.HasPrefix(resolved, resolvedRoot+string(os.PathSeparator)) {
			return fmt.Errorf("symlink %s escapes the pack root", p)
		}
		return nil
	})
}
