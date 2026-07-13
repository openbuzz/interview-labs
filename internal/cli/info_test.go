package cli

import (
	"strings"
	"testing"

	"github.com/charmbracelet/huh"

	"github.com/openbuzz/interview-labs/internal/session"
)

func TestInfoWithSlug(t *testing.T) {
	s := newFactsSession(t, session.StatusReady, "203.0.113.9")

	out, code := runCmd(t, "info", s.Meta.Slug)

	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	for _, want := range []string{
		"SESSION", s.Meta.Slug, "203.0.113.9", "ubuntu-26-04-x64",
		"interview ssh " + s.Meta.Slug,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("info missing %q:\n%s", want, out)
		}
	}
}

func TestInfoUnknownSlug(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	out, code := runCmd(t, "info", "no-such-lab")

	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "no-such-lab") {
		t.Fatalf("error does not name the slug:\n%s", out)
	}
}

func TestInfoNonTTYSeveralSessionsNeedsSlug(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	swapTTY(t, false)
	for range 2 {
		if _, err := session.New("fra1", "s-1vcpu-1gb", "img", "root",
			map[string]string{"vm": "digitalocean"},
			session.TerraformInfo{Binary: "terraform", Version: "1.14.8"}); err != nil {
			t.Fatal(err)
		}
	}

	out, code := runCmd(t, "info")

	if code != 2 {
		t.Fatalf("exit = %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "several sessions") {
		t.Fatalf("unexpected error:\n%s", out)
	}
}

func TestInfoPickerAbortCancelsQuietly(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	for range 2 { // two sessions force the picker; petname slugs are unique
		s, err := session.New("fra1", "s", "img", "root",
			map[string]string{"vm": "digitalocean"},
			session.TerraformInfo{Binary: "terraform", Version: "1"})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Save(); err != nil {
			t.Fatal(err)
		}
	}
	swapTTY(t, true)
	old := pickSession
	pickSession = func([]*session.Session, string, string) (*session.Session, error) {
		return nil, huh.ErrUserAborted
	}
	t.Cleanup(func() { pickSession = old })

	out, code := runCmd(t, "info")
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "cancelled") || strings.Contains(out, "error:") {
		t.Fatalf("quiet-cancel shape wrong:\n%s", out)
	}
}
