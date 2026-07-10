// Package ui is the single home of styling: ANSI-16 tokens, glyphs, blocks.
package ui

import (
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Semantic styles — ANSI-16 indices only; hues come from the user's terminal theme.
var (
	Accent = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	OK     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	Warn   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	Fail   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	Faint  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// Status glyphs.
const (
	GlyphOK   = "●"
	GlyphWarn = "▲"
	GlyphFail = "✗"
)

const logoArt = `   __
  |  |     interview-labs
 /    \    one disposable VM per interview
/_◦__◦_\`

// Logo renders the wordmark for root help.
func Logo() string {
	lines := strings.Split(logoArt, "\n")
	for i, l := range lines {
		lines[i] = Accent.Render(l)
	}
	return strings.Join(lines, "\n")
}

// Next renders the NEXT block: full interview commands, one per line.
func Next(cmds ...string) string {
	var b strings.Builder
	b.WriteString(Faint.Render("NEXT"))
	for _, c := range cmds {
		b.WriteString("\n  " + Accent.Render(c))
	}
	return b.String()
}

func row(glyph string, style lipgloss.Style, name, detail string) string {
	out := style.Render(glyph) + " " + name
	if detail != "" {
		out += "  " + Faint.Render(detail)
	}
	return out
}

// RowOK / RowWarn / RowFail render aligned status rows.
func RowOK(name, detail string) string   { return row(GlyphOK, OK, name, detail) }
func RowWarn(name, detail string) string { return row(GlyphWarn, Warn, name, detail) }
func RowFail(name, detail string) string { return row(GlyphFail, Fail, name, detail) }

// Theme is the huh theme on the same ANSI palette.
func Theme() *huh.Theme {
	t := huh.ThemeBase()
	t.Focused.Title = t.Focused.Title.Foreground(lipgloss.Color("6")).Bold(true)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(lipgloss.Color("6"))
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(lipgloss.Color("6"))
	t.Focused.Description = t.Focused.Description.Foreground(lipgloss.Color("8"))
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(lipgloss.Color("1"))
	return t
}
