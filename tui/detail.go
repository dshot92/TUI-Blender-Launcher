package tui

import (
	"TUI-Blender-Launcher/model"
	"fmt"
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// renderDetailPane renders the right-hand pane describing the selected build,
// plus live download information when a transfer is in progress.
func (m *Model) renderDetailPane(boxWidth, height int) string {
	innerWidth := boxWidth - boxBorderFrameWidth
	avail := height - boxBorderFrameHeight
	if avail < 1 {
		avail = 1
	}

	title := m.Style.PaneTitle.Render(" Details ")

	var content string
	if build := m.List.GetSelectedBuild(); build != nil {
		content = strings.Join(m.detailLines(*build, innerWidth), "\n")
	} else {
		content = centerBlock(m.Style.Empty.Render("Select a build to see details"), innerWidth, avail)
	}

	return renderBox("", title, content, boxWidth, height, lp.Color(m.Style.Palette.Border))
}

// detailLines builds the labelled lines shown in the detail pane.
func (m *Model) detailLines(build model.BlenderBuild, w int) []string {
	p := m.Style.Palette
	const labelW = 11

	value := func(s string) string {
		return m.Style.Value.Render(truncate(s, w-labelW))
	}
	row := func(label, val string) string {
		return m.Style.Label.Width(labelW).Render(label) + value(val)
	}

	lines := []string{
		row("Version", build.Version),
		m.Style.Label.Width(labelW).Render("Status") +
			lp.NewStyle().Foreground(lp.Color(p.StateColor(build.Status))).Bold(true).
				Render(stateGlyph(build.Status)+" "+build.Status.String()),
		row("Branch", orDash(build.Branch)),
		row("Type", orDash(build.ReleaseCycle)),
		row("Hash", orDash(build.Hash)),
		row("Size", model.FormatByteSize(build.Size)),
		row("Built", model.FormatBuildDate(build.BuildDate)),
	}
	if build.FileName != "" {
		lines = append(lines, row("File", build.FileName))
	}

	// Live download information.
	var state *model.DownloadState
	if s, ok := m.Progress.DownloadStates[build.ID()]; ok {
		state = s
	} else if m.commands != nil && m.commands.downloads != nil {
		state = m.commands.downloads.GetState(build.ID())
	}
	if state != nil && (state.BuildState == model.StateDownloading || state.BuildState == model.StateExtracting) {
		lines = append(lines, "")
		lines = append(lines, m.Style.Title.Render(strings.ToUpper(state.BuildState.String())))
		barWidth := w - 2
		if barWidth < 1 {
			barWidth = 1
		}
		lines = append(lines, renderProgressBar(state.Progress, barWidth, p))
		lines = append(lines, row("Progress", fmt.Sprintf("%.0f%%", state.Progress*100)))
		if state.Total > 0 {
			lines = append(lines, row("Downloaded", fmt.Sprintf("%s / %s",
				model.FormatByteSize(state.Current), model.FormatByteSize(state.Total))))
		}
		if state.BuildState == model.StateDownloading && state.Speed > 0 {
			lines = append(lines, row("Speed", fmt.Sprintf("%.1f MB/s", state.Speed/1024/1024)))
		}
	}

	return lines
}

// orDash returns s, or a muted placeholder when empty.
func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
