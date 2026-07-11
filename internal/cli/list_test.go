package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openbuzz/interview-labs/internal/session"
)

func TestListEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	out, code := runCmd(t, "list")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "interview launch") {
		t.Fatalf("empty list must hint launch:\n%s", out)
	}
}

func TestListShowsSessions(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	s, err := session.New("fra1", "s-1vcpu-1gb", "ubuntu-26-04-x64", "root",
		map[string]string{"vm": "digitalocean"},
		session.TerraformInfo{Binary: "terraform", Version: "1.9.5"})
	if err != nil {
		t.Fatal(err)
	}
	s.SetIP("203.0.113.9")
	s.SetStatus(session.StatusReady)

	out, code := runCmd(t, "list")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	for _, want := range []string{s.Meta.Slug, "fra1", "203.0.113.9", "ready"} {
		if !strings.Contains(out, want) {
			t.Fatalf("list missing %q:\n%s", want, out)
		}
	}
}

func TestListShowsVMProvider(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	s, err := session.New("fra1", "s-1vcpu-1gb", "img", "root",
		map[string]string{"vm": "digitalocean"}, session.TerraformInfo{})
	if err != nil {
		t.Fatal(err)
	}
	_ = s

	out, code := runCmd(t, "list")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}

	if !strings.Contains(out, "PROVIDER") || !strings.Contains(out, "digitalocean") {
		t.Fatalf("list missing provider column:\n%s", out)
	}
}

func writeListSession(t *testing.T, slug, ip, status string, age time.Duration) {
	t.Helper()
	root, err := session.Root()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "sessions", slug)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}

	meta := fmt.Sprintf(`{"schema":2,"slug":"%s","created_at":"%s",
"region":"fra1","size":"s-2vcpu-2gb","image":"ubuntu-26-04-x64",
"roles":{"vm":"digitalocean"},"ssh_user":"root",
"terraform":{"binary":"terraform","version":"1.9.0"},
"ip":"%s","status":"%s","phase":"summary"}`,
		slug, time.Now().UTC().Add(-age).Format(time.RFC3339), ip, status)
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"),
		[]byte(meta), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestListAlignsLongSlugsAndDashesEmptyIP(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	writeListSession(t, "extraordinarily-photogenic-mongoose-x9k2",
		"203.0.113.7", "ready", 90*time.Minute)
	writeListSession(t, "tiny-ant-1a2b", "", "failed", 5*time.Minute)

	out, code := runCmd(t, "list")

	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want header+2 rows, got %d lines:\n%s", len(lines), out)
	}

	head := lines[0]
	for _, l := range lines[1:] {
		if strings.Index(head, "PROVIDER") != strings.Index(l, "digitalocean") {
			t.Fatalf("PROVIDER column misaligned:\n%s", out)
		}
		if strings.Index(head, "STATUS") == -1 {
			t.Fatalf("missing STATUS header:\n%s", out)
		}
	}
	if !strings.Contains(lines[2], " -") && !strings.Contains(lines[1], " -") {
		t.Fatalf("empty IP not dashed:\n%s", out)
	}
}

func TestListShowsLocalSession(t *testing.T) {
	blankCloudEnv(t)
	_ = fakeDocker(t)
	if out, code := runCmd(t, "launch"); code != 0 {
		t.Fatalf("launch: %d\n%s", code, out)
	}

	out, code := runCmd(t, "list")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	if !strings.Contains(out, "local") || !strings.Contains(out, "ready") {
		t.Fatalf("list output:\n%s", out)
	}
}
