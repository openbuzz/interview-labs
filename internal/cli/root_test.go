package cli

import (
	"errors"
	"testing"
)

func TestExecuteUnknownCommandExitsUsage(t *testing.T) {
	code := run([]string{"definitely-not-a-command"})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
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
