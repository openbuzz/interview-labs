package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestExecuteUnknownCommandExitsUsage(t *testing.T) {
	code := run([]string{"definitely-not-a-command"})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestRootHelpListsCommandsWithBanner(t *testing.T) {
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
