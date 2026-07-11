// Package stack owns the container-stack machinery: staging the embedded
// docker tree, packing it for the ssh push, and session secrets.
package stack

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	interviewlabs "github.com/openbuzz/interview-labs"
)

// Stage materializes the docker tree into dst by merging the two embeds:
// DockerFS (compose, bake, vscode layers) and the _payload gateway copy,
// written back to images/gateway/ with its go.mod name restored. go:embed
// strips file modes, so scripts (*.sh, files/bin/*) get the exec bit back
// — the devops layer runs setup.sh from its build context, and TarGz ships
// whatever Stage wrote.
func Stage(dst string) error {
	for _, e := range []struct {
		fsys fs.FS
		root string
	}{
		{interviewlabs.DockerFS, "docker"},
		{interviewlabs.GatewayFS, "_payload/docker"},
	} {
		src, err := fs.Sub(e.fsys, e.root)
		if err != nil {
			return err
		}
		if err := writeTree(src, dst); err != nil {
			return err
		}
	}
	return nil
}

// writeTree copies an embedded tree into dst, restoring script modes and
// the payload's parked go.mod name.
func writeTree(src fs.FS, dst string) error {
	return fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dst, strings.TrimSuffix(path, ".payload"))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(src, path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, fileMode(path))
	})
}

// fileMode restores the exec bit go:embed strips from scripts.
func fileMode(path string) fs.FileMode {
	if strings.HasSuffix(path, ".sh") || strings.Contains(path, "files/bin/") {
		return 0o755
	}
	return 0o644
}

// TarGz packs dir's contents (dst-relative names, modes preserved) for the
// ssh push. The tree is small text — bytes in memory beat plumbing a pipe.
func TarGz(dir string) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return err
		}
		return addTarEntry(tw, dir, path, d)
	})
	if err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// addTarEntry writes one walked entry: the header, then the body for files.
func addTarEntry(tw *tar.Writer, dir, path string, d fs.DirEntry) error {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return err
	}
	info, err := d.Info()
	if err != nil {
		return err
	}
	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = filepath.ToSlash(rel)
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if d.IsDir() {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(tw, f)
	return err
}

// GeneratePassword mints the per-session gateway password: 16 hex chars,
// crypto/rand. Short-lived interview-gate material, not a durable secret.
func GeneratePassword() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
