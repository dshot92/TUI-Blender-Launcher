package tui

import "github.com/charmbracelet/x/ansi"

// truncate shortens s so that its display width is at most max columns,
// appending an ellipsis when content is cut. ANSI escape sequences are handled
// correctly, and the result never exceeds max columns.
func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= max {
		return s
	}
	return ansi.Truncate(s, max, "…")
}

// width returns the display width of a (possibly styled) string.
func width(s string) int {
	return ansi.StringWidth(s)
}

// capitalize upper-cases the first rune of s (ASCII only), leaving the rest
// untouched. Used for presenting palette names.
func capitalize(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] -= 'a' - 'A'
	}
	return string(r)
}
