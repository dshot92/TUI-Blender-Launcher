package tui

import (
	lp "github.com/charmbracelet/lipgloss"
)

// Style aggregates all lipgloss styles used across the TUI.
// Centralizing styles keeps rendering code clean and consistent.
type Style struct {
	HeaderCell         lp.Style
	SelectedHeaderCell lp.Style
	RegularRow         lp.Style
	SelectedRow        lp.Style
	Key                lp.Style
	Separator          lp.Style
	Newline            lp.Style
	Footer             lp.Style
}

// NewStyle constructs the default UI style palette.
func NewStyle() Style {
	baseText := lp.Color(textColor)
	bg := lp.Color(backgroundColor)
	hl := lp.Color(highlightColor)

	return Style{
		HeaderCell: lp.NewStyle().
			Foreground(baseText).
			Background(bg).
			Bold(true),

		SelectedHeaderCell: lp.NewStyle().
			Foreground(baseText).
			Background(hl).
			Bold(true).
			BorderForeground(hl),

		RegularRow: lp.NewStyle().
			Foreground(baseText),

		SelectedRow: lp.NewStyle().
			Foreground(lp.Color(textColor)).
			Background(hl).
			BorderForeground(hl),

		Key: lp.NewStyle().
			Foreground(hl).
			Bold(true),

		Separator: lp.NewStyle().
			Foreground(lp.Color("241")),

		Newline: lp.NewStyle(),

		Footer: lp.NewStyle().
			Foreground(baseText),
	}
}
