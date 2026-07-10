package ui

import (
	"strings"
	"testing"
)

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

func TestLogoArt(t *testing.T) {
	logo := Logo()
	if !strings.Contains(logo, "██╗███╗   ██╗████████╗") {
		t.Fatalf("logo missing INTERVIEW block: %q", logo)
	}
	if !strings.Contains(logo, "one disposable VM per interview") {
		t.Fatalf("logo missing tagline: %q", logo)
	}
	for _, line := range strings.Split(logo, "\n") {
		if n := len([]rune(line)); n > 100 {
			t.Fatalf("logo line %d runes wide: %q", n, line)
		}
	}
}

func TestBox(t *testing.T) {
	box := Box("Title Here", Accent, "line one", "", "line two")
	for _, want := range []string{"╭", "╰", "Title Here", "line one", "line two"} {
		if !strings.Contains(box, want) {
			t.Fatalf("box missing %q:\n%s", want, box)
		}
	}
}

func TestBadge(t *testing.T) {
	if got := Badge(true); !strings.Contains(got, GlyphOK) {
		t.Fatalf("configured badge = %q, want %q", got, GlyphOK)
	}
	if got := Badge(false); !strings.Contains(got, GlyphTodo) {
		t.Fatalf("unconfigured badge = %q, want %q", got, GlyphTodo)
	}
}
