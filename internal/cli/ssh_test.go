package cli

import (
	"strings"
	"testing"

	"github.com/charmbracelet/huh"

	"github.com/openbuzz/interview-labs/internal/session"
)

func TestSSHPickerAbortCancelsQuietly(t *testing.T) {
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

	out, code := runCmd(t, "ssh")
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "cancelled") || strings.Contains(out, "error:") {
		t.Fatalf("quiet-cancel shape wrong:\n%s", out)
	}
}
