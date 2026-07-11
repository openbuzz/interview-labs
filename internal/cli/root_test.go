package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestExecuteUnknownCommandExitsUsage(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	code := run([]string{"definitely-not-a-command"})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestRootHelpListsCommandsWithBanner(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	out, code := runCmd(t)
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	for _, want := range []string{
		"██╗███╗", "one disposable VM per interview",
		"doctor", "init", "launch", "list", "ssh", "destroy",
		"interview doctor",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("root help missing %q", want)
		}
	}
}

func TestUsageError(t *testing.T) {
	err := usageError("bad flags")
	if !IsUsage(err) {
		t.Fatal("IsUsage(usageError(...)) = false")
	}
	if IsUsage(errors.New("other")) {
		t.Fatal("IsUsage(plain error) = true")
	}
}

func TestBareRootNonTTYShowsHelp(t *testing.T) {
	swapTTY(t, false)

	out, code := runCmd(t)

	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	// Brief checks for the Short string "disposable interview lab VMs", but
	// cobra's defaultHelpFunc only falls back to Short when Long is empty
	// (command.go: usage := c.Long; if usage == "" { usage = c.Short }) —
	// root.Long is set, so Short never renders. Matching the substring
	// TestRootHelpListsCommandsWithBanner already verifies for this same
	// non-TTY help path instead.
	if !strings.Contains(out, "one disposable VM per interview") {
		t.Fatalf("help not shown:\n%s", out)
	}
}

func TestBareRootTTYRunsMenuLoop(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	swapTTY(t, true)

	picks := []string{"list", actionExit}
	oldPick := pickMainAction
	pickMainAction = func() (string, error) {
		p := picks[0]
		picks = picks[1:]
		return p, nil
	}
	t.Cleanup(func() { pickMainAction = oldPick })

	out, code := runCmd(t)

	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, out)
	}
	if !strings.Contains(out, "no sessions") {
		t.Fatalf("list not dispatched:\n%s", out)
	}
	if len(picks) != 0 {
		t.Fatal("menu loop did not continue after the action")
	}
}

func TestBareRootTTYSubcommandErrorKeepsLooping(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	swapTTY(t, true)

	picks := []string{"ssh", actionExit} // no sessions → ssh errors
	oldPick := pickMainAction
	pickMainAction = func() (string, error) {
		p := picks[0]
		picks = picks[1:]
		return p, nil
	}
	t.Cleanup(func() { pickMainAction = oldPick })

	_, code := runCmd(t)

	if code != 0 {
		t.Fatalf("exit = %d — a failed action must not kill the menu", code)
	}
	if len(picks) != 0 {
		t.Fatal("menu loop stopped after the failed action")
	}
}
