package tui

import (
	"fmt"
	"github.com/dshot92/TUI-Blender-Launcher/model"
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// renderHeader renders the single-line top chrome: the gradient wordmark on the
// left and live status metadata (build count, active downloads, app version) on
// the right, padded to the terminal width. It always occupies exactly
// headerBoxHeight lines.
func renderHeader(m *Model) string {
	p := m.Style.Palette

	mark := gradientText("◈ Blender", p.GradientStart, p.GradientEnd)
	left := mark + m.Style.Muted.Render(" Launcher")

	active := 0
	for _, st := range m.Progress.DownloadStates {
		if st != nil && (st.BuildState == model.StateDownloading || st.BuildState == model.StateExtracting) {
			active++
		}
	}

	meta := ""
	if m.terminalWidth >= headerMetaMinWidth {
		parts := []string{fmt.Sprintf("%d builds", len(m.List.Builds))}
		if active > 0 {
			parts = append(parts, fmt.Sprintf("%d active", active))
		}
		parts = append(parts, versionLabel())
		meta = m.Style.Muted.Render(strings.Join(parts, "  ·  "))
	}

	content := left
	if meta != "" {
		if gap := m.terminalWidth - width(left) - width(meta); gap >= 1 {
			content = left + strings.Repeat(" ", gap) + meta
		}
	}

	// Keep the line exactly terminalWidth columns wide: truncate first so long
	// content can't wrap, then pad the remainder.
	content = truncate(content, m.terminalWidth)
	return lp.NewStyle().Width(m.terminalWidth).Render(content)
}
