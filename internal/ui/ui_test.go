package ui

import (
	"strings"
	"testing"
)

func TestLogoFitsWidth(t *testing.T) {
	for _, line := range strings.Split(Logo(), "\n") {
		if n := len([]rune(line)); n > 100 {
			t.Fatalf("logo line %d cols wide: %q", n, line)
		}
	}
	if !strings.Contains(Logo(), "interview-labs") {
		t.Fatal("logo lost the wordmark")
	}
}

func TestNextContainsCommands(t *testing.T) {
	out := Next("interview launch", "interview destroy calm-otter-7f3k")
	if !strings.Contains(out, "NEXT") || !strings.Contains(out, "interview launch") {
		t.Fatalf("Next() = %q", out)
	}
}

func TestRows(t *testing.T) {
	if !strings.Contains(RowOK("terraform", "1.9.5"), "terraform") {
		t.Fatal("RowOK lost its name")
	}
	if !strings.Contains(RowFail("credentials", "token rejected"), "token rejected") {
		t.Fatal("RowFail lost its detail")
	}
}
