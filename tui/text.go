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

// hardWrap breaks s into lines of at most max display columns, splitting
// mid-token rather than on spaces (unlike wrapText). It is intended for
// spaceless strings such as URLs, which wrapText would leave on a single
// over-long line. Input is expected to be plain text (no ANSI escapes).
func hardWrap(s string, max int) []string {
	if max <= 0 || s == "" {
		return []string{s}
	}
	var (
		lines []string
		cur   strings.Builder
		curW  int
	)
	for _, r := range s {
		rw := width(string(r))
		if curW+rw > max && curW > 0 {
			lines = append(lines, cur.String())
			cur.Reset()
			curW = 0
		}
		cur.WriteRune(r)
		curW += rw
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
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
