package stack

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// Stage writes exactly the compose file: the VM builds nothing, so no build
// context, no bake file, no gateway sources ever ride along.
func TestStageWritesComposeOnly(t *testing.T) {
	dst := t.TempDir()
	if err := Stage(dst); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(dst, "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("compose.yaml mode = %o, want 644", info.Mode().Perm())
	}

	entries, err := os.ReadDir(dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "compose.yaml" {
		t.Errorf("staged entries = %v, want compose.yaml only", entries)
	}
}

func TestGeneratePassword(t *testing.T) {
	a, err := GeneratePassword()
	if err != nil {
		t.Fatal(err)
	}
	b, err := GeneratePassword()
	if err != nil {
		t.Fatal(err)
	}

	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(a) {
		t.Errorf("password %q not 16 hex chars", a)
	}
	if a == b {
		t.Error("two passwords identical — rand not wired")
	}
}
