package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/openbuzz/interview-labs/internal/provider"
)

// Columns pads cells to per-column visible width (ANSI-aware), joins them
// with a two-space gutter, and leaves the last column ragged. Indices in
// right are right-aligned (the last column never is). Rows share one length;
// lines come back right-trimmed.
func Columns(rows [][]string, right ...int) []string {
	if len(rows) == 0 {
		return nil
	}
	widths := columnWidths(rows)
	rightSet := make(map[int]bool, len(right))
	for _, i := range right {
		rightSet[i] = true
	}

	out := make([]string, len(rows))
	for ri, r := range rows {
		out[ri] = renderRow(r, widths, rightSet)
	}
	return out
}

// columnWidths returns the visible width of the widest cell per column.
func columnWidths(rows [][]string) []int {
	widths := make([]int, len(rows[0]))
	for _, r := range rows {
		for i, cell := range r {
			if w := lipgloss.Width(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}
	return widths
}

// renderRow pads and joins one row's cells per widths/rightSet, right-trimmed.
func renderRow(r []string, widths []int, rightSet map[int]bool) string {
	var b strings.Builder
	for i, cell := range r {
		pad := strings.Repeat(" ", widths[i]-lipgloss.Width(cell))
		switch {
		case i == len(r)-1:
			b.WriteString(cell)
		case rightSet[i]:
			b.WriteString(pad + cell + "  ")
		default:
			b.WriteString(cell + pad + "  ")
		}
	}
	return strings.TrimRight(b.String(), " ")
}

// SizeRows renders one aligned row per size: category, specs, ceil'd
// hourly price (never understates), trailing slug. Columns size across
// the whole list; the numeric columns right-align so units line up.
func SizeRows(sizes []provider.SizeInfo) []string {
	rows := make([][]string, len(sizes))
	for i, s := range sizes {
		price := math.Ceil(s.Hourly*100) / 100
		rows[i] = []string{s.Category,
			fmt.Sprintf("%d vCPU", s.VCPUs),
			fmt.Sprintf("%d GB memory", s.MemGB),
			fmt.Sprintf("%d GB disk", s.DiskGB),
			fmt.Sprintf("~%s%.2f/h", s.Currency, price),
			s.Slug}
	}
	return Columns(rows, 1, 2, 3, 4)
}
