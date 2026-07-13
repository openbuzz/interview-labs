package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackValidateEmbedded(t *testing.T) {
	out, code := runCmd(t, "pack", "validate", "default")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	for _, want := range []string{"default", "backend", "devops", "kind", "scenarios"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
}

func TestPackValidateBadDirFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"),
		[]byte("contract: 2\nname: x\nversion: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := runCmd(t, "pack", "validate", dir)
	if code == 0 || !strings.Contains(out, "contract") {
		t.Fatalf("exit = %d\n%s", code, out)
	}
}

func TestPackInitScaffolds(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "mypack")

	out, code := runCmd(t, "pack", "init", dst)
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}

	setup := filepath.Join(dst, "bundles/demo/lab/setup.sh")
	info, err := os.Stat(setup)
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("setup.sh: %v, mode %v", err, info.Mode())
	}
	if _, code := runCmd(t, "pack", "validate", dst); code != 0 {
		t.Fatal("scaffolded pack does not validate")
	}
}

func TestPackInitRefusesNonEmpty(t *testing.T) {
	dst := t.TempDir()
	if err := os.WriteFile(filepath.Join(dst, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := runCmd(t, "pack", "init", dst)
	if code == 0 || !strings.Contains(out, "not empty") {
		t.Fatalf("exit = %d\n%s", code, out)
	}
}
