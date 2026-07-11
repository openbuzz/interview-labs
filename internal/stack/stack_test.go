package stack

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	interviewlabs "github.com/openbuzz/interview-labs"
)

// The staged tree carries build/runtime files only: no tests, no task
// cache, no compiled gateway binary — the payload every VM receives stays
// minimal and deterministic (lesson of the 79 MB terraform/.terraform
// incident). Asserted post-Stage because the tree is the union of two
// embeds (DockerFS + the _payload gateway copy).
func TestStagedTreeHasSourcesOnly(t *testing.T) {
	dst := t.TempDir()
	if err := Stage(dst); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"compose.yaml",
		"docker-bake.hcl",
		"images/gateway/Dockerfile",
		"images/gateway/go.mod",
		"images/gateway/main.go",
		"images/vscode/layers/base/Dockerfile",
		"images/vscode/layers/base/files/entrypoint.sh",
		"images/vscode/layers/ai/Dockerfile",
	} {
		if _, err := os.Stat(filepath.Join(dst, want)); err != nil {
			t.Errorf("missing %s: %v", want, err)
		}
	}

	err := filepath.WalkDir(dst, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dst, p)
		if err != nil {
			return err
		}
		if rel = filepath.ToSlash(rel); nonSource(rel) {
			t.Errorf("staged non-source file: %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// nonSource reports staged paths the VM payload must never carry.
func nonSource(rel string) bool {
	return strings.Contains(rel, "/tests/") || strings.HasSuffix(rel, "_test.go") ||
		strings.Contains(rel, "/.task/") ||
		strings.HasSuffix(rel, "go.mod.payload") ||
		rel == "images/gateway/gateway"
}

func TestStageWritesTreeWithScriptModes(t *testing.T) {
	dst := t.TempDir()
	if err := Stage(dst); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"compose.yaml", "docker-bake.hcl",
		"images/gateway/Dockerfile", "images/vscode/layers/base/Dockerfile"} {
		if _, err := os.Stat(filepath.Join(dst, want)); err != nil {
			t.Errorf("missing %s: %v", want, err)
		}
	}

	// go:embed strips modes; Stage must restore the exec bit on scripts —
	// the devops layer runs setup.sh from its build, and tar preserves what
	// Stage wrote.
	for _, script := range []string{
		"images/vscode/layers/base/files/entrypoint.sh",
		"images/vscode/layers/devops/setup.sh",
		"images/vscode/layers/base/files/bin/cs-extension-install",
	} {
		info, err := os.Stat(filepath.Join(dst, script))
		if err != nil {
			t.Fatalf("%s: %v", script, err)
		}
		if info.Mode().Perm() != 0o755 {
			t.Errorf("%s mode = %o, want 755", script, info.Mode().Perm())
		}
	}
	info, err := os.Stat(filepath.Join(dst, "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("compose.yaml mode = %o, want 644", info.Mode().Perm())
	}
}

func TestTarGzRoundTrips(t *testing.T) {
	dst := t.TempDir()
	if err := Stage(dst); err != nil {
		t.Fatal(err)
	}
	data, err := TarGz(dst)
	if err != nil {
		t.Fatal(err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	found := map[string]int64{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		found[hdr.Name] = hdr.Mode
	}

	if _, ok := found["compose.yaml"]; !ok {
		t.Fatalf("tar misses compose.yaml (names must be dst-relative): %v", found)
	}
	if mode := found["images/vscode/layers/base/files/entrypoint.sh"]; mode&0o111 == 0 {
		t.Errorf("entrypoint.sh not executable in tar: %o", mode)
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

// The _payload gateway copy is derived state; docker/images/gateway is the
// single source of truth. Both directions checked: every build input must
// be embedded byte-equal, and the embed must hold nothing else.
func TestPayloadInSync(t *testing.T) {
	srcDir := filepath.Join("..", "..", "docker", "images", "gateway")
	want := payloadWant(t, srcDir)

	const root = "_payload/docker/images/gateway"
	for name, srcPath := range want {
		assertPayloadEqual(t, path.Join(root, name), srcPath)
	}
	assertNoStrays(t, root, want)
}

// payloadWant maps embedded payload names to their source files; go.mod
// travels renamed (go:embed skips any dir holding a go.mod, even under
// _payload).
func payloadWant(t *testing.T, srcDir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !payloadFile(e.Name()) {
			continue
		}
		name := e.Name()
		if name == "go.mod" {
			name = "go.mod.payload"
		}
		want[name] = filepath.Join(srcDir, e.Name())
	}
	return want
}

// assertPayloadEqual checks one embedded payload file byte-matches its source.
func assertPayloadEqual(t *testing.T, embedded, srcPath string) {
	t.Helper()
	got, err := fs.ReadFile(interviewlabs.GatewayFS, embedded)
	if err != nil {
		t.Errorf("payload misses %s — run: task docker:payload (%v)",
			path.Base(embedded), err)
		return
	}

	src, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, src) {
		t.Errorf("payload %s differs from source — run: task docker:payload",
			path.Base(embedded))
	}
}

// assertNoStrays walks the embedded payload rejecting files outside want.
func assertNoStrays(t *testing.T, root string, want map[string]string) {
	t.Helper()
	err := fs.WalkDir(interviewlabs.GatewayFS, root,
		func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if _, ok := want[path.Base(p)]; !ok && !d.IsDir() {
				t.Errorf("stray payload file %s — run: task docker:payload", p)
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
}

// payloadFile mirrors task docker:payload's filter: gateway build inputs
// only — the Dockerfile pair, module file, non-test go sources and the
// assets the binary serves.
// ponytail: flat filter — gateway build inputs are all top-level today;
// extend both this and the task target if subdirectories appear.
func payloadFile(name string) bool {
	if strings.HasSuffix(name, "_test.go") {
		return false
	}
	switch name {
	case "Dockerfile", ".dockerignore", "go.mod", "go.sum":
		return true
	}
	switch filepath.Ext(name) {
	case ".go", ".html", ".css":
		return true
	}
	return false
}
