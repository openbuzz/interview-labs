package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/openbuzz/interview-labs/internal/provider"
)

func TestColumnsAlignsByWidestCell(t *testing.T) {
	lines := Columns([][]string{
		{"terraform", "1.9.5"},
		{"ssh client", "found"},
	})
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	if strings.Index(lines[0], "1.9.5") != strings.Index(lines[1], "found") {
		t.Fatalf("detail column misaligned:\n%q\n%q", lines[0], lines[1])
	}
	if want := "terraform   1.9.5"; lines[0] != want {
		t.Fatalf("line = %q, want %q", lines[0], want)
	}
}

func TestColumnsIsANSIAware(t *testing.T) {
	green := "\x1b[32m●\x1b[0m ok" // visible width 4
	lines := Columns([][]string{
		{green, "detail"},
		{"▲ warn", "detail"},
	})
	// Raw byte offsets diverge here on purpose: the green cell's escape codes
	// add invisible bytes, so alignment must be checked by rendered width.
	prefix0 := lines[0][:strings.Index(lines[0], "detail")]
	prefix1 := lines[1][:strings.Index(lines[1], "detail")]
	if lipgloss.Width(prefix0) != lipgloss.Width(prefix1) {
		t.Fatalf("ANSI cell broke alignment:\n%q\n%q", lines[0], lines[1])
	}
}

func TestColumnsRightAligns(t *testing.T) {
	lines := Columns([][]string{
		{"a", "2 vCPU", "x"},
		{"b", "16 vCPU", "y"},
	}, 1)
	if strings.Index(lines[0], " vCPU") != strings.Index(lines[1], " vCPU") {
		t.Fatalf("right-aligned units misaligned:\n%q\n%q", lines[0], lines[1])
	}
	if !strings.Contains(lines[0], " 2 vCPU") {
		t.Fatalf("short cell not left-padded: %q", lines[0])
	}
}

func TestColumnsLastColumnRaggedAndTrimmed(t *testing.T) {
	lines := Columns([][]string{
		{"short", "x"},
		{"a-much-longer-cell", ""},
	})
	if strings.HasSuffix(lines[0], " ") || strings.HasSuffix(lines[1], " ") {
		t.Fatalf("trailing whitespace survived: %q / %q", lines[0], lines[1])
	}
}

func TestColumnsEmptyInput(t *testing.T) {
	if got := Columns(nil); got != nil {
		t.Fatalf("Columns(nil) = %v, want nil", got)
	}
}

func TestSizeRowsAlignAndCeilPrices(t *testing.T) {
	rows := SizeRows([]provider.SizeInfo{
		{Category: "Shared CPU", VCPUs: 2, MemGB: 4, DiskGB: 80,
			Hourly: 0.0298, Currency: "$", Slug: "s-2vcpu-4gb"},
		{Category: "Dedicated CPU", VCPUs: 16, MemGB: 32, DiskGB: 160,
			Hourly: 0.1849, Currency: "€", Slug: "g-16vcpu-32gb"},
	})
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if !strings.Contains(rows[0], "~$0.03/h") { // 0.0298 ceils, never understates
		t.Fatalf("ceil price missing: %q", rows[0])
	}
	if !strings.Contains(rows[1], "~€0.19/h") {
		t.Fatalf("ceil price missing: %q", rows[1])
	}
	if strings.Contains(rows[0], ",") {
		t.Fatalf("comma separator survived: %q", rows[0])
	}
	if strings.Index(rows[0], " vCPU") != strings.Index(rows[1], " vCPU") {
		t.Fatalf("vCPU units misaligned:\n%q\n%q", rows[0], rows[1])
	}
	if !strings.HasSuffix(rows[0], "s-2vcpu-4gb") {
		t.Fatalf("slug not last: %q", rows[0])
	}
}

func TestSizeRowsSingleRow(t *testing.T) {
	rows := SizeRows([]provider.SizeInfo{
		{Category: "Shared CPU", VCPUs: 2, MemGB: 4, DiskGB: 80,
			Hourly: 0.03, Currency: "$", Slug: "s-2vcpu-4gb"},
	})
	if len(rows) != 1 || rows[0] == "" {
		t.Fatalf("rows = %q", rows)
	}
}
