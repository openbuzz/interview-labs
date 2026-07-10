package terraform

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFake writes an executable fake tf binary that answers `version -json`.
func writeFake(t *testing.T, dir, name, version string) {
	t.Helper()
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"version\" ]; then\n" +
		"  echo '{\"terraform_version\":\"" + version + "\"}'\n" +
		"fi\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestFindPrefersTerraform(t *testing.T) {
	dir := t.TempDir()
	writeFake(t, dir, "terraform", "1.9.5")
	writeFake(t, dir, "tofu", "1.8.1")
	t.Setenv("PATH", dir)

	b, err := Find()
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "terraform" || b.Version != "1.9.5" {
		t.Fatalf("Find() = %+v", b)
	}
}

func TestFindFallsBackToTofu(t *testing.T) {
	dir := t.TempDir()
	writeFake(t, dir, "tofu", "1.8.1")
	t.Setenv("PATH", dir)

	b, err := Find()
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "tofu" || b.Version != "1.8.1" {
		t.Fatalf("Find() = %+v", b)
	}
}

func TestFindNeitherErrors(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := Find(); err == nil {
		t.Fatal("Find() succeeded with empty PATH")
	}
}
