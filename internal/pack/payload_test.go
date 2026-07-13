package pack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStagePayloadShape(t *testing.T) {
	p := goodFS(t)
	b, _ := BundleByName(p, "devops")
	dst := t.TempDir()

	if err := StagePayload(dst, p.FS, b); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"payload/workspace/broken-pods/README.md",
		"payload/lab/setup.sh",
		"payload/kind/cluster.yaml",
		"payload/kind/manifests/00-ns.yaml",
	} {
		if _, err := os.Stat(filepath.Join(dst, want)); err != nil {
			t.Fatalf("missing %s: %v", want, err)
		}
	}

	info, _ := os.Stat(filepath.Join(dst, "payload/lab/setup.sh"))
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("setup.sh mode = %v", info.Mode())
	}
	if _, err := os.Stat(filepath.Join(dst, "payload/workspace/broken-pods/solution")); err == nil {
		t.Fatal("solution/ leaked into the workspace")
	}
}

func TestStagePayloadBackendOmitsKindAndLab(t *testing.T) {
	p := goodFS(t)
	b, _ := BundleByName(p, "backend")
	dst := t.TempDir()

	if err := StagePayload(dst, p.FS, b); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dst, "payload/kind")); err == nil {
		t.Fatal("kind/ staged for a kindless bundle")
	}
	if _, err := os.Stat(filepath.Join(dst, "payload/lab")); err == nil {
		t.Fatal("lab/ staged for a labless bundle")
	}
	if _, err := os.Stat(filepath.Join(dst,
		"payload/workspace/fix-tests/README.md")); err != nil {
		t.Fatalf("task content missing: %v", err)
	}
}

// TestStagePayloadRenamesGoMod covers the controller addition: go:embed
// silently drops any subtree containing a nested go.mod (see embed.go), so
// exercise task/ Go modules are checked in as _go.mod and must come back as
// go.mod in the staged payload.
func TestStagePayloadRenamesGoMod(t *testing.T) {
	p := goodFS(t)
	b, _ := BundleByName(p, "backend")
	dst := t.TempDir()

	if err := StagePayload(dst, p.FS, b); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dst,
		"payload/workspace/fix-tests/go.mod")); err != nil {
		t.Fatalf("go.mod not staged: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst,
		"payload/workspace/fix-tests/_go.mod")); err == nil {
		t.Fatal("_go.mod leaked into the workspace unrenamed")
	}
}
