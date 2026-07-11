package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testTF() TerraformInfo { return TerraformInfo{Binary: "terraform", Version: "1.9.0"} }

func newTestSession(t *testing.T) *Session {
	t.Helper()
	s, err := New("fra1", "s-1vcpu-1gb", "ubuntu-26-04-x64", "root",
		map[string]string{"vm": "digitalocean"}, testTF())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewCreatesLayout(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	s := newTestSession(t)

	for _, d := range []string{s.SSHDir(), s.TerraformDir(), s.LogsDir()} {
		fi, err := os.Stat(d)
		if err != nil || !fi.IsDir() {
			t.Fatalf("missing dir %s: %v", d, err)
		}
		if fi.Mode().Perm() != 0o700 {
			t.Fatalf("%s perm = %o, want 700", d, fi.Mode().Perm())
		}
	}
}

func TestNewInitialMetadata(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	s := newTestSession(t)

	fi, err := os.Stat(s.MetadataPath())
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("metadata perm = %o, want 600", fi.Mode().Perm())
	}
	if s.Meta.Status != StatusLaunching || s.Meta.Schema != 2 {
		t.Fatalf("meta = %+v", s.Meta)
	}
	if !strings.Contains(s.Meta.Slug, "-") || len(s.Meta.Slug) < 6 {
		t.Fatalf("slug = %q", s.Meta.Slug)
	}
}

func TestListGetRoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	a := newTestSession(t)
	b := newTestSession(t)

	all, err := List()
	if err != nil || len(all) != 2 {
		t.Fatalf("List() = %d sessions, %v; want 2", len(all), err)
	}
	got, err := Get(b.Meta.Slug)
	if err != nil || got.Meta.Slug != b.Meta.Slug {
		t.Fatalf("Get(%s) = %+v, %v", b.Meta.Slug, got, err)
	}
	if _, err := Get("nope"); err == nil {
		t.Fatal("Get(nope) succeeded")
	}
	_ = a
}

func TestSettersPersist(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	s := newTestSession(t)
	if err := s.SetIP("203.0.113.9"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(StatusReady); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPhase("summary"); err != nil {
		t.Fatal(err)
	}

	got, err := Get(s.Meta.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Meta.IP != "203.0.113.9" || got.Meta.Status != StatusReady ||
		got.Meta.Phase != "summary" {
		t.Fatalf("persisted meta = %+v", got.Meta)
	}
}

func TestLockIsExclusive(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	s := newTestSession(t)
	release, err := s.Lock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Lock(); err == nil || !strings.Contains(err.Error(), "session busy") {
		t.Fatalf("second Lock() err = %v, want session busy", err)
	}
	release()
	release2, err := s.Lock()
	if err != nil {
		t.Fatalf("Lock() after release: %v", err)
	}
	release2()
}

func TestArchiveMovesMetadataAndLogs(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", root)
	s := newTestSession(t)
	logFile := filepath.Join(s.LogsDir(), "terraform-apply.log")
	if err := os.WriteFile(logFile, []byte("log"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := s.Archive(); err != nil {
		t.Fatal(err)
	}

	arch := filepath.Join(root, "interview", "archive", s.Meta.Slug)
	if _, err := os.Stat(filepath.Join(arch, "metadata.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(arch, "logs", "terraform-apply.log")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Dir); !os.IsNotExist(err) {
		t.Fatalf("session dir still exists: %v", err)
	}
}

func TestAge(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	s := newTestSession(t)
	s.Meta.CreatedAt = time.Now().UTC().Add(-90 * time.Minute)
	if got := s.Age(time.Now().UTC()); got < 89*time.Minute || got > 91*time.Minute {
		t.Fatalf("Age() = %v", got)
	}
}

func TestNewRecordsRoles(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	s, err := New("fra1", "s-1vcpu-1gb", "img", "root",
		map[string]string{"vm": "digitalocean"}, TerraformInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if s.Meta.Schema != 2 {
		t.Fatalf("schema = %d, want 2", s.Meta.Schema)
	}
	if s.Meta.Roles["vm"] != "digitalocean" {
		t.Fatalf("roles = %v", s.Meta.Roles)
	}

	got, err := Get(s.Meta.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Meta.Roles["vm"] != "digitalocean" {
		t.Fatalf("reloaded roles = %v", got.Meta.Roles)
	}
}

func TestGetDefaultsSchema1Roles(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	s, err := New("fra1", "s-1vcpu-1gb", "img", "root",
		map[string]string{"vm": "digitalocean"}, TerraformInfo{})
	if err != nil {
		t.Fatal(err)
	}
	meta := `{"schema":1,"slug":"` + s.Meta.Slug + `","status":"ready","phase":"summary"}`
	if err := os.WriteFile(s.MetadataPath(), []byte(meta), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Get(s.Meta.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Meta.Roles["vm"] != "digitalocean" {
		t.Fatalf("schema-1 fallback roles = %v", got.Meta.Roles)
	}
}

func TestSSHUserPersistsAndBackfills(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	s, err := New("fra1", "s-1vcpu-1gb", "img", "ubuntu",
		map[string]string{"vm": "aws"}, TerraformInfo{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Get(s.Meta.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Meta.SSHUser != "ubuntu" {
		t.Fatalf("ssh user = %q, want ubuntu", got.Meta.SSHUser)
	}

	// legacy metadata without ssh_user reads back as root
	got.Meta.SSHUser = ""
	if err := got.Save(); err != nil {
		t.Fatal(err)
	}
	legacy, err := Get(s.Meta.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Meta.SSHUser != "root" {
		t.Fatalf("legacy ssh user = %q, want root", legacy.Meta.SSHUser)
	}
}

func TestAIAndFQDNFieldsRoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	s, err := New("fra1", "s-1vcpu-1gb", "img", "root",
		map[string]string{"vm": "digitalocean", "ai": "openrouter"},
		TerraformInfo{Binary: "terraform", Version: "1.9.5"})
	if err != nil {
		t.Fatal(err)
	}

	s.Meta.AIKeyHash, s.Meta.AICapUSD = "hash-1", 10
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFQDN("calm-otter.example.test"); err != nil {
		t.Fatal(err)
	}

	got, err := Get(s.Meta.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Meta.AIKeyHash != "hash-1" || got.Meta.AICapUSD != 10 ||
		got.Meta.FQDN != "calm-otter.example.test" {
		t.Fatalf("meta = %+v", got.Meta)
	}
	if got.Meta.Roles["ai"] != "openrouter" {
		t.Fatalf("roles = %v", got.Meta.Roles)
	}
}
