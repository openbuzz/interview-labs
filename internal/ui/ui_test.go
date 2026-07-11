package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/openbuzz/interview-labs/internal/provider"
)

func TestNextContainsCommands(t *testing.T) {
	out := Next("interview launch", "interview destroy calm-otter")
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
	if !strings.Contains(logo, "Stop testing answers. Start testing work.") {
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

func TestFormKeyMapQuitAcceptsEscAndCtrlC(t *testing.T) {
	keys := FormKeyMap().Quit.Keys()
	for _, want := range []string{"esc", "ctrl+c"} {
		found := false
		for _, k := range keys {
			if k == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Quit.Keys() = %v, want to contain %q", keys, want)
		}
	}
}

func TestLogoHasTwoSpaceMargin(t *testing.T) {
	for i, line := range strings.Split(Logo(), "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "  ") {
			t.Fatalf("logo line %d lacks 2-space margin: %q", i, line)
		}
	}
}

func TestBoxRendersAtStandardWidth(t *testing.T) {
	box := Box("Title", OK, "one line")
	for i, line := range strings.Split(box, "\n") {
		if w := lipgloss.Width(line); w != Width {
			t.Fatalf("box line %d width = %d, want %d", i, w, Width)
		}
	}
}

func TestNarrowWarningWarnsOncePerProcess(t *testing.T) {
	ResetNarrowWarning()
	oldI, oldW := Interactive, termWidth
	t.Cleanup(func() { Interactive, termWidth = oldI, oldW; ResetNarrowWarning() })
	Interactive = func() bool { return true }
	termWidth = func() int { return 65 }

	first := NarrowWarning()
	if !strings.Contains(first, "65 columns") || !strings.Contains(first, "72") {
		t.Fatalf("first warning = %q", first)
	}
	if second := NarrowWarning(); second != "" {
		t.Fatalf("second warning = %q, want empty", second)
	}
}

func TestNarrowWarningSilentWhenWideOrNonTTY(t *testing.T) {
	ResetNarrowWarning()
	oldI, oldW := Interactive, termWidth
	t.Cleanup(func() { Interactive, termWidth = oldI, oldW; ResetNarrowWarning() })

	Interactive = func() bool { return true }
	termWidth = func() int { return 120 }
	if w := NarrowWarning(); w != "" {
		t.Fatalf("wide terminal warned: %q", w)
	}

	Interactive = func() bool { return false }
	termWidth = func() int { return 40 }
	if w := NarrowWarning(); w != "" {
		t.Fatalf("non-TTY warned: %q", w)
	}
}

func TestLogoOnce(t *testing.T) {
	ResetLogoOnce()
	t.Cleanup(ResetLogoOnce)

	if first := LogoOnce(); !strings.Contains(first, "██╗███╗   ██╗████████╗") {
		t.Fatalf("first call missing art: %q", first)
	}
	if second := LogoOnce(); second != "" {
		t.Fatalf("second call = %q, want empty", second)
	}

	ResetLogoOnce()
	if again := LogoOnce(); again == "" {
		t.Fatal("reset did not re-arm the logo")
	}
}

func TestSizeLabelFormat(t *testing.T) {
	cases := []struct {
		in   provider.SizeInfo
		want string
	}{
		{
			provider.SizeInfo{Slug: "s-2vcpu-4gb", Category: "Basic", VCPUs: 2,
				MemGB: 4, DiskGB: 80, Hourly: 0.036, Currency: "$"},
			"Basic            2 vCPU,   4 GB memory,  80 GB disk   ~$0.04/h   s-2vcpu-4gb",
		},
		{
			// ceil: €0.0113 must render €0.02, never €0.01.
			provider.SizeInfo{Slug: "cx32", Category: "Shared", VCPUs: 4,
				MemGB: 8, DiskGB: 80, Hourly: 0.0113, Currency: "€"},
			"Shared           4 vCPU,   8 GB memory,  80 GB disk   ~€0.02/h   cx32",
		},
		{
			provider.SizeInfo{Slug: "g-2vcpu-8gb", Category: "General Purpose", VCPUs: 2,
				MemGB: 8, DiskGB: 25, Hourly: 0.0938, Currency: "$"},
			"General Purpose  2 vCPU,   8 GB memory,  25 GB disk   ~$0.10/h   g-2vcpu-8gb",
		},
		{
			// exact 2-decimal price stays exact (no ceil drift).
			provider.SizeInfo{Slug: "m5.large", Category: "General Purpose", VCPUs: 2,
				MemGB: 8, DiskGB: 40, Hourly: 0.096, Currency: "$"},
			"General Purpose  2 vCPU,   8 GB memory,  40 GB disk   ~$0.10/h   m5.large",
		},
	}
	for _, c := range cases {
		if got := SizeLabel(c.in); got != c.want {
			t.Fatalf("SizeLabel(%s):\n got %q\nwant %q", c.in.Slug, got, c.want)
		}
	}
}

func TestReceiptLine(t *testing.T) {
	line := receiptLine("What do you want to do?", "doctor")
	for _, want := range []string{"┃", "What do you want to do?", "→", "doctor"} {
		if !strings.Contains(line, want) {
			t.Fatalf("receipt missing %q: %q", want, line)
		}
	}
}
