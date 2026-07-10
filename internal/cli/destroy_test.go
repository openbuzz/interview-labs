package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openbuzz/interview-labs/internal/session"
)

// destroySetup creates one session and a fake tf recording its invocations.
func destroySetup(t *testing.T) (*session.Session, string) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")

	dir := t.TempDir()
	record := filepath.Join(dir, "record")
	script := "#!/bin/sh\n" +
		"[ \"$1\" = version ] && echo '{\"terraform_version\":\"1.9.5\"}'\n" +
		"echo \"$@\" >> " + record + "\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "terraform"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/bin")

	s, err := session.New("fra1", "s-1vcpu-1gb", "ubuntu-26-04-x64",
		session.TerraformInfo{Binary: "terraform", Version: "1.9.5"})
	if err != nil {
		t.Fatal(err)
	}
	return s, record
}

func TestDestroySoleSessionWithYes(t *testing.T) {
	s, record := destroySetup(t)

	out, code := runCmd(t, "destroy", "--yes")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	rec, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rec), "init") || !strings.Contains(string(rec), "destroy") {
		t.Fatalf("tf calls: %s", rec)
	}

	if _, err := session.Get(s.Meta.Slug); err == nil {
		t.Fatal("session still present after destroy")
	}
	root, _ := session.Root()
	if _, err := os.Stat(filepath.Join(root, "archive", s.Meta.Slug,
		"metadata.json")); err != nil {
		t.Fatalf("archive missing: %v", err)
	}
}

func TestDestroyNoSessionsErrors(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	out, code := runCmd(t, "destroy", "--yes")
	if code != 1 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	if !strings.Contains(out, "interview launch") {
		t.Fatalf("missing launch hint:\n%s", out)
	}
}

func TestDestroyFailedTFKeepsSession(t *testing.T) {
	s, _ := destroySetup(t)
	// swap the fake tf for a failing one
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"[ \"$1\" = version ] && { echo '{\"terraform_version\":\"1.9.5\"}'; exit 0; }\n" +
		"exit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "terraform"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/bin")

	out, code := runCmd(t, "destroy", "--yes")
	if code != 1 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	got, err := session.Get(s.Meta.Slug)
	if err != nil {
		t.Fatalf("session gone after failed destroy: %v", err)
	}
	if got.Meta.Status != session.StatusFailedDestroy {
		t.Fatalf("status = %s, want failed-destroy", got.Meta.Status)
	}
	if !strings.Contains(out, "logs:") || !strings.Contains(out, "interview destroy") {
		t.Fatalf("failed destroy must print log path and retry hint:\n%s", out)
	}
}
