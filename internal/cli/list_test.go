package cli

import (
	"strings"
	"testing"

	"github.com/openbuzz/interview-labs/internal/session"
)

func TestListEmpty(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	out, code := runCmd(t, "list")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "interview launch") {
		t.Fatalf("empty list must hint launch:\n%s", out)
	}
}

func TestListShowsSessions(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	s, err := session.New("fra1", "s-1vcpu-1gb", "ubuntu-26-04-x64",
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
