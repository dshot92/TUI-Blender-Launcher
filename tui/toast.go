package tui

import (
	"strings"

	lp "github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// toastMinWidth is the narrowest popup panel we will render.
const toastMinWidth = 16

// toastHeight is the panel height: top border, one text line, bottom border.
const toastHeight = 3

// toastMessage returns the styled message text and the border color for the
// current status/error, or ok=false when there is nothing to show. Both styles
// come from the active palette, so the toast follows the live theme: statuses
// use the theme's accent color (not the shared green "success" color, which is
// the same in every theme) and errors use the theme's error color.
func (m *Model) toastMessage() (text string, border lp.Color, ok bool) {
	p := m.Style.Palette
	switch {
	case m.err != nil:
		return m.Style.Error.Render("✖ " + m.err.Error()), lp.Color(p.Error), true
	case m.filterErr != nil:
		return m.Style.Error.Render("✖ " + m.filterErr.Error()), lp.Color(p.Error), true
	case m.status != "":
		accent := lp.NewStyle().Foreground(lp.Color(p.Accent)).Bold(true)
		return accent.Render("✓ " + m.status), lp.Color(p.Accent), true
	default:
		return "", lp.Color(""), false
	}
}

// renderToast builds the bottom popup panel for the current status/error. The
// panel is at most maxWidth columns wide and is themed with the active palette's
// accent (status) or error color. The second return value reports whether there
// is anything to render.
func (m *Model) renderToast(maxWidth int) (string, bool) {
	text, border, ok := m.toastMessage()
	if !ok {
		return "", false
	}

	if maxWidth < toastMinWidth {
		maxWidth = toastMinWidth
	}

	// Inner width is the text plus one space of padding on each side.
	panelW := width(text) + 2 + boxBorderFrameWidth
	if panelW > maxWidth {
		panelW = maxWidth
	}
	if panelW < toastMinWidth {
		panelW = toastMinWidth
	}

	inner := panelW - boxBorderFrameWidth
	if inner < 1 {
		inner = 1
	}
	// Truncate the (styled) text so it never widens the panel.
	text = truncate(text, inner)
	line := lp.NewStyle().Width(inner).Align(lp.Center).Render(text)

	return renderBox("", "", line, panelW, toastHeight, border), true
}

// overlayToast composites the status/error popup over the bottom of the frame,
// keeping every resulting line exactly terminalWidth columns wide.
func (m *Model) overlayToast(frame string) string {
	if m.terminalWidth <= 0 || m.terminalHeight <= 0 {
		return frame
	}

	panel, ok := m.renderToast(m.terminalWidth - 2)
	if !ok {
		return frame
	}

	panelLines := strings.Split(panel, "\n")
	panelW := width(panelLines[0])
	x := (m.terminalWidth - panelW) / 2
	// Place the popup about three-quarters down the terminal, while keeping it
	// inside the body/table area so it still never overlaps the footer/commands.
	y := m.terminalHeight * 3 / 4
	if y+len(panelLines) > m.terminalHeight-footerHeight {
		y = m.terminalHeight - footerHeight - len(panelLines)
	}
	if y < 0 {
		y = 0
	}

	return overlayBlock(frame, panel, x, y, m.terminalWidth)
}

// overlayBlock composites block into base at column x and row y, where every
// line of base is w columns wide. The panel is stamped over the frame and the
// content behind it (e.g. footer hints) is preserved on either side, so the
// toast reads as a floating panel without erasing the page underneath. Every
// resulting line stays w columns wide.
func overlayBlock(base, block string, x, y, w int) string {
	if w <= 0 {
		return base
	}

	lines := strings.Split(base, "\n")
	blockLines := strings.Split(block, "\n")

	for i, bl := range blockLines {
		row := y + i
		if row < 0 || row >= len(lines) {
			continue
		}

		bw := width(bl)
		if bw > w {
			bl = truncate(bl, w)
			bw = width(bl)
		}

		cx := x
		if cx < 0 {
			cx = 0
		}
		if cx+bw > w {
			cx = w - bw
		}
		if cx < 0 {
			cx = 0
		}

		// Keep the base line's content on either side of the panel; only the
		// panel's own columns are replaced. ansi.Cut is escape-aware, so the
		// surviving text keeps its styling.
		line := lines[row]
		left := ansi.Cut(line, 0, cx)
		right := ansi.Cut(line, cx+bw, w)
		lines[row] = left + bl + right
	}

	return strings.Join(lines, "\n")
}
