package pack

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

//nolint:cyclop // cyclop: dense field checks over one fixture — a split wouldn't lower it
func TestWriteTarShape(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "workspace/s1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "workspace/s1/README.md"),
		[]byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "setup.sh"),
		[]byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := WriteTar(&buf, dir); err != nil {
		t.Fatal(err)
	}

	seen := map[string]*tar.Header{}
	tr := tar.NewReader(&buf)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		seen[hdr.Name] = hdr
	}

	readme := seen["payload/workspace/s1/README.md"]
	if readme == nil || readme.Uid != 1000 || readme.Gid != 1000 {
		t.Fatalf("README header = %+v", readme)
	}
	setup := seen["payload/setup.sh"]
	if setup == nil || setup.Mode&0o755 != 0o755 {
		t.Fatalf("setup header = %+v", setup)
	}
	if d := seen["payload/workspace/"]; d == nil || d.Uid != 1000 {
		t.Fatalf("dir header = %+v", d)
	}
}
