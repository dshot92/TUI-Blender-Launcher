package tui

import (
	"fmt"
	"github.com/dshot92/TUI-Blender-Launcher/model"
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
		content = strings.Join(m.detailBlock(*build, innerWidth).lines, "\n")
	} else {
		content = centerBlock(m.Style.Empty.Render("Select a build to see details"), innerWidth, avail)
	}

	return renderBox("", title, content, boxWidth, height, lp.Color(m.Style.Palette.Border))
}

// detailBlock holds the rendered lines for the detail pane together with the
// position of the download-URL row, so a mouse click can be mapped back to it.
type detailBlock struct {
	lines   []string
	urlLine int    // index of the first URL row within lines, or -1
	urlRows int    // number of wrapped URL rows
	url     string // the download URL, when present
}

// detailBlock builds the labelled lines shown in the detail pane.
func (m *Model) detailBlock(build model.BlenderBuild, w int) detailBlock {
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

	block := detailBlock{urlLine: -1}
	// The download link, shown for any build that can be fetched online. Long
	// URLs are hard-wrapped (and indented) so the whole link stays readable.
	if build.DownloadURL != "" {
		valueW := w - labelW
		if valueW < 1 {
			valueW = 1
		}
		chunks := hardWrap(build.DownloadURL, valueW)
		block.urlLine = len(lines)
		block.urlRows = len(chunks)
		block.url = build.DownloadURL
		for i, chunk := range chunks {
			if i == 0 {
				lines = append(lines, m.Style.Label.Width(labelW).Render("URL")+
					m.Style.Link.Render(chunk))
			} else {
				lines = append(lines, strings.Repeat(" ", labelW)+m.Style.Link.Render(chunk))
			}
		}
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

	block.lines = lines
	return block
}

// urlAt maps terminal coordinates to the download-URL row of the info panel.
// It returns the URL and true only when (x, y) lands on that row and the row is
// actually visible. It performs no I/O, so it is safe to call on every mouse
// event and straightforward to test.
func (m *Model) urlAt(x, y int) (string, bool) {
	if !m.layout.detailVisible {
		return "", false
	}
	build := m.List.GetSelectedBuild()
	if build == nil || build.DownloadURL == "" {
		return "", false
	}

	innerW := m.layout.detailBoxWidth - boxBorderFrameWidth
	if innerW < 1 {
		return "", false
	}
	block := m.detailBlock(*build, innerW)
	if block.urlLine < 0 {
		return "", false
	}

	// The URL row must not have been clipped by the box height.
	if block.urlLine+block.urlRows-1 >= m.layout.detailBoxHeight-boxBorderFrameHeight {
		return "", false
	}

	// Detail pane origin within the frame. Content starts one row below the
	// box's top border and one column right of its left border.
	top := headerBoxHeight
	left := 0
	if m.layout.detailStacked {
		top += m.layout.listBoxHeight
	} else {
		left = m.layout.listBoxWidth + paneGap
	}
	first := top + 1 + block.urlLine
	last := first + block.urlRows - 1

	if y < first || y > last {
		return "", false
	}
	if x < left+1 || x >= left+1+innerW {
		return "", false
	}
	return block.url, true
}

// buildIndexAt maps terminal coordinates to an index in List.Builds. It returns
// -1 when (x, y) does not land on a rendered build row.
func (m *Model) buildIndexAt(x, y int) int {
	if len(m.List.Builds) == 0 {
		return -1
	}
	if x < 0 || x >= m.layout.listBoxWidth {
		return -1
	}
	if m.layout.listBoxWidth <= 0 || m.layout.listRows <= 0 {
		return -1
	}

	firstDataY := headerBoxHeight + 2
	if y < firstDataY {
		return -1
	}
	offset := y - firstDataY
	if offset >= m.layout.listRows {
		return -1
	}
	idx := m.List.StartIndex + offset
	if idx < 0 || idx >= len(m.List.Builds) {
		return -1
	}
	return idx
}

// buildAt is the bool-returning form of buildIndexAt, mirroring urlAt.
func (m *Model) buildAt(x, y int) (int, bool) {
	idx := m.buildIndexAt(x, y)
	if idx < 0 {
		return -1, false
	}
	return idx, true
}

// columnIndexAt maps a click on the builds table header row to its column index.
// It returns -1 when (x, y) is not on the header row or does not land on any
// column.
func (m *Model) columnIndexAt(x, y int) int {
	if y != headerBoxHeight+1 {
		return -1
	}
	cols := m.columnsLayout()
	if len(cols) == 0 {
		return -1
	}
	// The builds pane has a border, pane padding, and the selection gutter
	// before the first column: 1 + panePadX + gutterWidth.
	const columnStart = 1 + panePadX + gutterWidth
	pos := x - columnStart
	if pos < 0 {
		return -1
	}
	running := 0
	for i, c := range cols {
		running += c.Width
		if pos < running {
			return i
		}
	}
	return -1
}

// columnAt is the bool-returning form of columnIndexAt, mirroring buildAt.
func (m *Model) columnAt(x, y int) (int, bool) {
	idx := m.columnIndexAt(x, y)
	if idx < 0 {
		return -1, false
	}
	return idx, true
}

// orDash returns s, or a muted placeholder when empty.
func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
