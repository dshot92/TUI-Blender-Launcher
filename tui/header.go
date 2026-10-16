package tui

import (
	"TUI-Blender-Launcher/model"
	"fmt"
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// renderHeader renders the top chrome: a rounded box holding the gradient
// wordmark on the left and live status metadata on the right. It always
// occupies exactly headerBoxHeight lines.
func renderHeader(m *Model) string {
	p := m.Style.Palette
	innerWidth := m.terminalWidth - boxBorderFrameWidth
	if innerWidth < 1 {
		innerWidth = 1
	}

	mark := gradientText("◈ Blender", p.GradientStart, p.GradientEnd)
	left := mark + m.Style.Muted.Render(" Launcher")

	active := 0
	for _, st := range m.Progress.DownloadStates {
		if st != nil && (st.BuildState == model.StateDownloading || st.BuildState == model.StateExtracting) {
			active++
		}
	}

	meta := ""
	if innerWidth >= 46 {
		parts := []string{fmt.Sprintf("%d builds", len(m.List.Builds))}
		if active > 0 {
			parts = append(parts, fmt.Sprintf("%d active", active))
		}
		parts = append(parts, capitalize(p.Name))
		meta = m.Style.Muted.Render(strings.Join(parts, "  ·  "))
	}

	content := left
	if meta != "" {
		if gap := innerWidth - width(left) - width(meta); gap >= 1 {
			content = left + strings.Repeat(" ", gap) + meta
		}
	}

	return renderBox("", "", content, m.terminalWidth, headerBoxHeight, lp.Color(p.Border))
}
