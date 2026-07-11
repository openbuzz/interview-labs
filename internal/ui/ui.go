// Package ui is the single home of styling: ANSI-16 tokens, glyphs, blocks.
package ui

import (
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/openbuzz/interview-labs/internal/provider"
	"golang.org/x/term"
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
	GlyphTodo = "○"
)

// Width is the standard render width: 68-col wordmark + 2-space margins.
const Width = 72

const logoArt = `██╗███╗   ██╗████████╗███████╗██████╗ ██╗   ██╗██╗███████╗██╗    ██╗
██║████╗  ██║╚══██╔══╝██╔════╝██╔══██╗██║   ██║██║██╔════╝██║    ██║
██║██╔██╗ ██║   ██║   █████╗  ██████╔╝██║   ██║██║█████╗  ██║ █╗ ██║
██║██║╚██╗██║   ██║   ██╔══╝  ██╔══██╗╚██╗ ██╔╝██║██╔══╝  ██║███╗██║
██║██║ ╚████║   ██║   ███████╗██║  ██║ ╚████╔╝ ██║███████╗╚███╔███╔╝
╚═╝╚═╝  ╚═══╝   ╚═╝   ╚══════╝╚═╝  ╚═╝  ╚═══╝  ╚═╝╚══════╝ ╚══╝╚══╝

                  ██╗      █████╗ ██████╗ ███████╗
                  ██║     ██╔══██╗██╔══██╗██╔════╝
                  ██║     ███████║██████╔╝███████╗
                  ██║     ██╔══██║██╔══██╗╚════██║
                  ███████╗██║  ██║██████╔╝███████║
                  ╚══════╝╚═╝  ╚═╝╚═════╝ ╚══════╝`

const logoTagline = "             Stop testing answers. Start testing work."

// Logo renders the wordmark: art in bold accent, faint centered tagline.
func Logo() string {
	lines := strings.Split(logoArt, "\n")
	for i, l := range lines {
		lines[i] = "  " + Accent.Bold(true).Render(l)
	}
	return strings.Join(lines, "\n") + "\n\n" + "  " + Faint.Render(logoTagline)
}

var logoShown bool

// ResetLogoOnce re-arms the once-per-process logo print for tests.
func ResetLogoOnce() { logoShown = false }

// LogoOnce returns the logo block on the first call and "" afterwards, so
// menu-dispatched subcommands never repeat the wordmark. The help template
// keeps using Logo() directly: template construction runs on every
// newRootCmd call and must not consume this guard.
func LogoOnce() string {
	if logoShown {
		return ""
	}

	logoShown = true
	return Logo()
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

// Box renders a rounded-border block: bold title, blank line, body lines.
func Box(title string, style lipgloss.Style, lines ...string) string {
	body := style.Bold(true).Render(title) + "\n\n" + strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(style.GetForeground()).
		Padding(1, 2).
		Width(Width - 2).
		Render(body)
}

// Badge renders a provider's configured state glyph.
func Badge(configured bool) string {
	if configured {
		return OK.Render(GlyphOK)
	}
	return Faint.Render(GlyphTodo)
}

// SizeLabel renders one size row: category, specs, ceil'd hourly price
// (never understates), trailing slug.
func SizeLabel(s provider.SizeInfo) string {
	price := math.Ceil(s.Hourly*100) / 100
	return fmt.Sprintf("%-16s%2d vCPU, %3d GB memory, %3d GB disk   ~%s%.2f/h   %s",
		s.Category, s.VCPUs, s.MemGB, s.DiskGB, s.Currency, price, s.Slug)
}

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

// FormKeyMap returns huh's default keymap with "esc" added to Quit, so ESC
// aborts a form exactly like Ctrl+C (huh v1.0.0 binds Quit to ctrl+c only).
func FormKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Quit.SetKeys(append(km.Quit.Keys(), "esc")...)
	return km
}

// receiptLine formats the transcript line a completed form leaves behind.
func receiptLine(title, choice string) string {
	return Faint.Render("┃ " + title + " → " + choice)
}

// SelectForm runs one single-select with the house theme, keymap, title
// and faint description; a completed pick leaves a one-line receipt in
// the transcript. desc may be empty.
func SelectForm[T comparable](title, desc string,
	opts []huh.Option[T], value *T) error {
	err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[T]().Title(title).Description(desc).
			Options(opts...).Value(value),
	)).WithTheme(Theme()).WithKeyMap(FormKeyMap()).Run()
	if err != nil {
		return err
	}

	fmt.Println(receiptLine(title, fmt.Sprint(*value)))
	return nil
}

// ConfirmForm runs one confirm with the house theme and keymap; the
// initial *value picks the focused button (true focuses Yes). A completed
// confirm leaves a Yes/No receipt in the transcript.
func ConfirmForm(title, desc string, value *bool) error {
	err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title(title).Description(desc).Value(value),
	)).WithTheme(Theme()).WithKeyMap(FormKeyMap()).Run()
	if err != nil {
		return err
	}

	choice := "No"
	if *value {
		choice = "Yes"
	}
	fmt.Println(receiptLine(title, choice))
	return nil
}

// Interactive is a seam: whether stdout can host live redraw (spinners).
var Interactive = func() bool { return term.IsTerminal(int(os.Stdout.Fd())) }

// termWidth is a seam: stdout's column count, 0 when unknown.
var termWidth = func() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 0
	}
	return w
}

var warnedNarrow bool

// ResetNarrowWarning re-arms the once-per-process narrow-terminal warning.
func ResetNarrowWarning() { warnedNarrow = false }

// NarrowWarning returns one warn row when the terminal is narrower than
// Width; empty on wide terminals, non-TTYs, and after the first call.
func NarrowWarning() string {
	if warnedNarrow || !Interactive() {
		return ""
	}
	w := termWidth()
	if w == 0 || w >= Width {
		return ""
	}

	warnedNarrow = true
	return Warn.Render(GlyphWarn) + fmt.Sprintf(
		" terminal is %d columns; interview renders at %d — expect wrapped output", w, Width)
}
