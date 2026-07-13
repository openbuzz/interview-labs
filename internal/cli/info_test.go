package cli

import (
	"strings"
	"testing"

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
