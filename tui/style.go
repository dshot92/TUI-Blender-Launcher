package tui

import (
	lp "github.com/charmbracelet/lipgloss"
)

// Style aggregates all lipgloss styles used across the TUI. Centralizing styles
// keeps rendering code clean and consistent, and deriving every style from a
// Palette means the whole UI can be re-themed from one place.
type Style struct {
	// The active palette, exposed for components that need raw colors
	// (borders, badges, gradients).
	Palette Palette

	// Table
	HeaderCell         lp.Style
	SelectedHeaderCell lp.Style
	RegularRow         lp.Style
	SelectedRow        lp.Style

	// Chrome
	Key             lp.Style
	Separator       lp.Style
	Newline         lp.Style
	Footer          lp.Style
	PaneTitle       lp.Style
	ActivePaneTitle lp.Style
	Title           lp.Style

	// Feedback
	Error    lp.Style
	Status   lp.Style
	Empty    lp.Style
	Muted    lp.Style
	Label    lp.Style
	Value    lp.Style
	Link     lp.Style
	HelpKey  lp.Style
	HelpDesc lp.Style
	HelpSep  lp.Style
}

// NewStyle constructs the default UI style palette.
func NewStyle() Style {
	return NewStyleFromPalette(DefaultPalette())
}

// NewStyleFromPalette constructs the UI styles for a given palette.
func NewStyleFromPalette(p Palette) Style {
	fg := lp.Color(p.Foreground)
	muted := lp.Color(p.Muted)
	subtle := lp.Color(p.Subtle)
	accent := lp.Color(p.Accent)

	return Style{
		Palette: p,

		HeaderCell: lp.NewStyle().
			Foreground(muted).
			Bold(true).
			Align(lp.Center),

		SelectedHeaderCell: lp.NewStyle().
			Foreground(accent).
			Bold(true).
			Underline(true).
			Align(lp.Center),

		RegularRow: lp.NewStyle().
			Foreground(fg),

		SelectedRow: lp.NewStyle().
			Foreground(fg).
			Background(subtle).
			Bold(true),

		Key: lp.NewStyle().
			Foreground(accent).
			Bold(true),

		Separator: lp.NewStyle().
			Foreground(subtle),

		Newline: lp.NewStyle(),

		Footer: lp.NewStyle().
			Foreground(muted),

		PaneTitle: lp.NewStyle().
			Foreground(fg).
			Bold(true),

		ActivePaneTitle: lp.NewStyle().
			Foreground(accent).
			Bold(true),

		Title: lp.NewStyle().
			Foreground(accent).
			Bold(true),

		Error: lp.NewStyle().
			Foreground(lp.Color(p.Error)).
			Bold(true),

		Status: lp.NewStyle().
			Foreground(lp.Color(p.Success)),

		Empty: lp.NewStyle().
			Foreground(muted).
			Italic(true),

		Muted: lp.NewStyle().
			Foreground(muted),

		Label: lp.NewStyle().
			Foreground(accent),

		Value: lp.NewStyle().
			Foreground(fg),

		// Link marks a clickable URL in the detail pane.
		Link: lp.NewStyle().
			Foreground(accent).
			Underline(true),

		HelpKey: lp.NewStyle().
			Foreground(accent).
			Bold(true),

		HelpDesc: lp.NewStyle().
			Foreground(muted),

		HelpSep: lp.NewStyle().
			Foreground(subtle),
	}
}
