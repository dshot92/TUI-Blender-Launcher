package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

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

// wrapText breaks s into lines of at most max display columns, splitting on
// spaces. A word longer than max is left on its own line rather than split. It
// keeps settings help text visible on narrow panes instead of letting the box
// truncate it.
func wrapText(s string, max int) []string {
	if max <= 0 {
		return []string{s}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	lines := make([]string, 0, 1)
	cur := words[0]
	for _, word := range words[1:] {
		if width(cur)+1+width(word) <= max {
			cur += " " + word
			continue
		}
		lines = append(lines, cur)
		cur = word
	}
	return append(lines, cur)
}
