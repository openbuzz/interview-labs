package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/openrouter"
	"github.com/openbuzz/interview-labs/internal/session"
)

// destroySetup creates one session and a fake tf recording its invocations.
func destroySetup(t *testing.T) (*session.Session, string) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")

	dir := t.TempDir()
	record := filepath.Join(dir, "record")
	script := "#!/bin/sh\n" +
		"[ \"$1\" = version ] && echo '{\"terraform_version\":\"1.9.5\"}'\n" +
		"echo \"$@\" >> " + record + "\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "terraform"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/bin")

	s, err := session.New("fra1", "s-1vcpu-1gb", "ubuntu-26-04-x64", "root",
		map[string]string{"vm": "digitalocean"},
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

// TestDestroyMultipleSessionsUsesPickerWithTitle pins the title/desc thread:
// with 2+ sessions on a TTY, destroy must route through pickSession with its
// own copy, not ssh's.
func TestDestroyMultipleSessionsUsesPickerWithTitle(t *testing.T) {
	destroySetup(t)
	s2, err := session.New("fra1", "s-1vcpu-1gb", "ubuntu-26-04-x64", "root",
		map[string]string{"vm": "digitalocean"},
		session.TerraformInfo{Binary: "terraform", Version: "1.9.5"})
	if err != nil {
		t.Fatal(err)
	}
	swapTTY(t, true)

	var sawTitle, sawDesc string
	oldPick := pickSession
	pickSession = func(all []*session.Session, title, desc string) (*session.Session, error) {
		sawTitle, sawDesc = title, desc
		return s2, nil
	}
	t.Cleanup(func() { pickSession = oldPick })

	out, code := runCmd(t, "destroy", "--yes")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	if sawTitle != "Select a session to destroy" {
		t.Fatalf("picker title = %q", sawTitle)
	}
	if sawDesc != "Tears down the cloud resources and archives logs. Stops billing." {
		t.Fatalf("picker desc = %q", sawDesc)
	}
}

func TestDestroyNoSessionsErrors(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
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

// aiSessionSetup creates one session carrying ai+access roles and a minted
// key hash, plus a fake tf that records args and env.
func aiSessionSetup(t *testing.T) (*session.Session, string) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("DIGITALOCEAN_TOKEN", "tok")
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "mk")
	t.Setenv("CLOUDFLARE_API_TOKEN", "cf-tok")

	cfg := config.Config{}
	cfg.Providers.Cloudflare.ZoneID = "z1"
	cfg.Providers.Cloudflare.Domain = "example.test"
	if err := cfg.Write(); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	record := filepath.Join(dir, "record")
	script := "#!/bin/sh\n" +
		"[ \"$1\" = version ] && echo '{\"terraform_version\":\"1.9.5\"}'\n" +
		"echo \"$@\" >> " + record + "\n" +
		"env >> " + record + ".env\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "terraform"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// /usr/bin stays on PATH: the fake script shells out to env(1).
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/bin"+
		string(os.PathListSeparator)+"/usr/bin")

	s, err := session.New("fra1", "s-1vcpu-1gb", "ubuntu-26-04-x64", "root",
		map[string]string{"vm": "digitalocean", "ai": "openrouter",
			"access": "cloudflare"},
		session.TerraformInfo{Binary: "terraform", Version: "1.9.5"})
	if err != nil {
		t.Fatal(err)
	}
	s.Meta.AIKeyHash, s.Meta.AICapUSD = "hash-1", 10
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	return s, record
}

// revokeServer fakes DELETE /keys/{hash} with a fixed status.
func revokeServer(t *testing.T, status int) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete || r.URL.Path != "/keys/hash-1" {
				t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			}
			calls.Add(1)
			w.WriteHeader(status)
			if status >= 400 {
				w.Write([]byte(`{"error":{"message":"boom"}}`))
			}
		}))
	t.Cleanup(srv.Close)
	old := openrouter.BaseURL
	openrouter.BaseURL = srv.URL
	t.Cleanup(func() { openrouter.BaseURL = old })
	return &calls
}

func TestDestroyRevokesAIKeyAndMergesCFToken(t *testing.T) {
	s, record := aiSessionSetup(t)
	calls := revokeServer(t, http.StatusOK)

	out, code := runCmd(t, "destroy", "--yes")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}

	if calls.Load() != 1 {
		t.Fatalf("revoke calls = %d, want 1", calls.Load())
	}
	if _, err := session.Get(s.Meta.Slug); err == nil {
		t.Fatal("session still present after destroy")
	}
	envDump, err := os.ReadFile(record + ".env")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(envDump), "CLOUDFLARE_API_TOKEN=cf-tok") {
		t.Fatalf("cf token missing from destroy env:\n%s", envDump)
	}
}

func TestDestroyRevoke404IsClean(t *testing.T) {
	s, _ := aiSessionSetup(t)
	calls := revokeServer(t, http.StatusNotFound)

	out, code := runCmd(t, "destroy", "--yes")
	if code != 0 {
		t.Fatalf("404 revoke must not fail destroy: exit %d\n%s", code, out)
	}
	if calls.Load() != 1 {
		t.Fatalf("revoke calls = %d", calls.Load())
	}
	if _, err := session.Get(s.Meta.Slug); err == nil {
		t.Fatal("session not archived")
	}
}

func TestDestroyDeclineCancelsQuietly(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	s, err := session.New("fra1", "s-1vcpu-1gb", "ubuntu-26-04-x64", "root",
		map[string]string{"vm": "digitalocean"},
		session.TerraformInfo{Binary: "terraform", Version: "1.9.5"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	swapTTY(t, true)
	old := confirmDestroy
	confirmDestroy = func(*session.Session) (bool, error) { return false, nil }
	t.Cleanup(func() { confirmDestroy = old })

	out, code := runCmd(t, "destroy")
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "cancelled") {
		t.Fatalf("missing cancelled line:\n%s", out)
	}
	if strings.Contains(out, "error:") {
		t.Fatalf("error echo survived:\n%s", out)
	}
}

func TestDestroyRevokeFailureKeepsSession(t *testing.T) {
	s, _ := aiSessionSetup(t)
	revokeServer(t, http.StatusInternalServerError)

	out, code := runCmd(t, "destroy", "--yes")
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, out)
	}

	got, err := session.Get(s.Meta.Slug)
	if err != nil {
		t.Fatalf("session gone after failed revoke: %v", err)
	}
	if got.Meta.Status != session.StatusFailedDestroy || got.Meta.AIKeyHash != "hash-1" {
		t.Fatalf("meta = %+v", got.Meta)
	}
	if !strings.Contains(out, "interview destroy "+s.Meta.Slug) {
		t.Fatalf("missing retry hint:\n%s", out)
	}
}
