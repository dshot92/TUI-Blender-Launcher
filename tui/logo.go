package tui

import (
	"fmt"
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// hexToRGB parses a "#rrggbb" color string into its RGB components. Invalid
// input yields a neutral gray so callers never have to handle errors.
func hexToRGB(hex string) (int, int, int) {
	h := strings.TrimPrefix(hex, "#")
	if len(h) != 6 {
		return 0x88, 0x88, 0x88
	}
	var r, g, b int
	if _, err := fmt.Sscanf(h, "%02x%02x%02x", &r, &g, &b); err != nil {
		return 0x88, 0x88, 0x88
	}
	return r, g, b
}

// lerp8 linearly interpolates between two 8-bit color components.
func lerp8(a, b int, t float64) int {
	v := float64(a) + (float64(b)-float64(a))*t
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return int(v + 0.5)
}

// lerpColor interpolates between two hex colors, returning a lipgloss color.
func lerpColor(startHex, endHex string, t float64) lp.Color {
	sr, sg, sb := hexToRGB(startHex)
	er, eg, eb := hexToRGB(endHex)
	return lp.Color(fmt.Sprintf("#%02x%02x%02x", lerp8(sr, er, t), lerp8(sg, eg, t), lerp8(sb, eb, t)))
}

// gradientText renders text with a horizontal gradient from start to end. Each
// rune is colored individually and kept bold.
func gradientText(text, startHex, endHex string) string {
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return ""
	}
	var b strings.Builder
	for i, r := range runes {
		t := 0.0
		if n > 1 {
			t = float64(i) / float64(n-1)
		}
		b.WriteString(lp.NewStyle().
			Foreground(lerpColor(startHex, endHex, t)).
			Bold(true).
			Render(string(r)))
	}
	return b.String()
}
