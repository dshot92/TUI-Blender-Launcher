package tui

import (
	"TUI-Blender-Launcher/model"
	"fmt"
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

const (
	// gutterWidth is the space reserved at the left of every table row for the
	// selection bar (1 bar + 1 space).
	gutterWidth = 2
	// accentBar marks the currently selected row.
	accentBar = "▌"
)

// Row represents a single row in the builds table.
type Row struct {
	Build      model.BlenderBuild
	IsSelected bool
	Status     *model.DownloadState
}

// NewRow creates a new row instance from a build.
func NewRow(build model.BlenderBuild, isSelected bool, status *model.DownloadState) Row {
	return Row{
		Build:      build,
		IsSelected: isSelected,
		Status:     status,
	}
}

// columnConfig describes how a column participates in width distribution.
// Lower priority numbers are conceptually more important; the field documents
// the intended ordering should we ever need to hide columns on narrow screens.
type columnConfig struct {
	priority int
	flex     float64
}

var columnConfigs = map[string]columnConfig{
	"Version":    {priority: 1, flex: 1.0},
	"Status":     {priority: 2, flex: 1.0},
	"Build Date": {priority: 3, flex: 1.0},
	"Type":       {priority: 4, flex: 1.0},
	"Branch":     {priority: 5, flex: 1.0},
	"Hash":       {priority: 6, flex: 1.0},
	"Size":       {priority: 7, flex: 1.0},
}

// ColumnConfig represents the configuration for a table column.
type ColumnConfig struct {
	Name  string
	Key   string
	Width int
	Index int
	Style func(string) string
}

// GetBuildColumns builds the column layout for a given column area width
// (excluding the selection gutter and pane padding). Widths always sum to
// exactly the supplied width.
func GetBuildColumns(colWidth int) []ColumnConfig {
	columns := []ColumnConfig{
		{Name: "Version", Key: "Version", Index: 0},
		{Name: "Status", Key: "Status", Index: 1},
		{Name: "Branch", Key: "Branch", Index: 2},
		{Name: "Type", Key: "Type", Index: 3},
		{Name: "Hash", Key: "Hash", Index: 4},
		{Name: "Size", Key: "Size", Index: 5},
		{Name: "Build Date", Key: "Build Date", Index: 6},
	}

	totalFlex := 0.0
	for i := range columns {
		totalFlex += columnConfigs[columns[i].Key].flex
	}

	used := 0
	for i := range columns {
		colW := int(float64(colWidth) * columnConfigs[columns[i].Key].flex / totalFlex)
		if i == len(columns)-1 {
			colW = colWidth - used
		}
		if colW < 1 {
			colW = 1
		}
		columns[i].Width = colW
		used += colW
		columns[i].Style = func(w int) func(string) string {
			return func(s string) string {
				return lp.NewStyle().Align(lp.Center).Width(w).Render(truncate(s, w))
			}
		}(colW)
	}

	return columns
}

func sumColumnWidths(columns []ColumnConfig) int {
	sum := 0
	for _, col := range columns {
		sum += col.Width
	}
	return sum
}

// Render renders a single row to exactly sum(column widths) display cells. The
// caller is responsible for the selection gutter and pane padding.
func (r Row) Render(columns []ColumnConfig, style Style) string {
	isDownloading := r.Build.Status == model.StateDownloading && r.Status != nil
	isExtracting := r.Build.Status == model.StateExtracting && r.Status != nil
	isFailed := r.Build.Status == model.StateFailed
	isCancelled := r.Build.Status == model.StateCancelled

	badge := func(s model.BuildState) string {
		label := stateGlyph(s) + " " + s.String()
		return lp.NewStyle().Foreground(lp.Color(style.Palette.StateColor(s))).Render(label)
	}

	var rowString string

	if isDownloading || isExtracting {
		// Render the leading cells (Version, Status, Branch) and append a
		// progress bar spanning the remaining columns.
		var prefixCells []string
		barWidth := 0
		for _, col := range columns {
			var cellContent string
			switch col.Key {
			case "Version":
				cellContent = r.Build.Version
			case "Status":
				cellContent = badge(r.Build.Status)
			case "Branch":
				if isDownloading && r.Status.Speed > 0 {
					mbps := r.Status.Speed / 1024 / 1024
					if mbps < 100 {
						cellContent = fmt.Sprintf("%6.1f MB/s", mbps)
					} else {
						cellContent = fmt.Sprintf("%6.0f MB/s", mbps)
					}
				} else if isExtracting {
					cellContent = fmt.Sprintf("%6.1f%%", r.Status.Progress*100)
				} else {
					cellContent = ""
				}
				cellContent = lp.NewStyle().Foreground(lp.Color(style.Palette.Downloading)).Render(cellContent)
			default:
				barWidth += col.Width
				continue
			}
			prefixCells = append(prefixCells, col.Style(cellContent))
		}
		rowString = lp.JoinHorizontal(lp.Left, prefixCells...) + renderProgressBar(r.Status.Progress, barWidth, style.Palette)
	} else {
		cells := make([]string, 0, len(columns))
		for _, col := range columns {
			var cellContent string
			switch col.Key {
			case "Version":
				cellContent = r.Build.Version
			case "Status":
				cellContent = badge(r.Build.Status)
			case "Branch":
				cellContent = r.Build.Branch
			case "Type":
				cellContent = r.Build.ReleaseCycle
			case "Hash":
				cellContent = r.Build.Hash
			case "Size":
				cellContent = model.FormatByteSize(r.Build.Size)
			case "Build Date":
				cellContent = model.FormatBuildDate(r.Build.BuildDate)
			}
			cells = append(cells, col.Style(cellContent))
		}
		rowString = lp.JoinHorizontal(lp.Left, cells...)
	}

	totalWidth := sumColumnWidths(columns)
	switch {
	case r.IsSelected:
		return style.SelectedRow.Width(totalWidth).Render(rowString)
	case isFailed || isCancelled:
		return lp.NewStyle().Foreground(lp.Color(style.Palette.Error)).Width(totalWidth).Render(rowString)
	default:
		return style.RegularRow.Width(totalWidth).Render(rowString)
	}
}

// renderProgressBar renders a progress bar occupying exactly width cells, using
// a horizontal gradient for the completed portion.
func renderProgressBar(progress float64, barWidth int, p Palette) string {
	if barWidth <= 0 {
		return ""
	}
	if progress < 0 {
		progress = 0
	} else if progress > 1 {
		progress = 1
	}

	filled := int(float64(barWidth) * progress)
	if filled > barWidth {
		filled = barWidth
	}

	emptyStyle := lp.NewStyle().Background(lp.Color(p.Subtle))
	var b strings.Builder
	for i := 0; i < barWidth; i++ {
		if i < filled {
			t := 0.0
			if filled > 1 {
				t = float64(i) / float64(filled-1)
			}
			b.WriteString(lp.NewStyle().Background(lerpColor(p.GradientStart, p.GradientEnd, t)).Render(" "))
		} else {
			b.WriteString(emptyStyle.Render(" "))
		}
	}
	return b.String()
}

// padRow wraps a rendered row with pane padding and the selection gutter so the
// line is exactly listInnerWidth wide.
func (m *Model) padRow(content string, selected bool) string {
	gutter := "  "
	if selected {
		gutter = lp.NewStyle().Foreground(lp.Color(m.Style.Palette.Accent)).Render(accentBar) + " "
	}
	return strings.Repeat(" ", panePadX) + gutter + content + strings.Repeat(" ", panePadX)
}

// renderBuildsPane renders the bordered builds pane (title + table).
func (m *Model) renderBuildsPane(boxWidth, height int) string {
	p := m.Style.Palette
	innerWidth := boxWidth - boxBorderFrameWidth
	availRows := height - boxBorderFrameHeight - 1 // minus the header row
	if availRows < 1 {
		availRows = 1
	}

	title := m.Style.ActivePaneTitle.Render(" Builds ")

	if len(m.List.Builds) == 0 {
		msg := "No Blender builds yet — press " + PrimaryKey(CmdFetchBuilds) + " to fetch"
		content := centerBlock(m.Style.Empty.Render(msg), innerWidth, height-boxBorderFrameHeight)
		return renderBox("", title, content, boxWidth, height, lp.Color(p.Accent))
	}

	columns := m.columnsLayout()

	// Header row.
	headerCells := make([]string, 0, len(columns))
	for _, col := range columns {
		text := col.Name
		if col.Index == m.List.SortColumn {
			if m.List.SortReversed {
				text += " ↓"
			} else {
				text += " ↑"
			}
		}
		cellStyle := m.Style.HeaderCell
		if col.Index == m.List.SortColumn {
			cellStyle = m.Style.SelectedHeaderCell
		}
		headerCells = append(headerCells, cellStyle.Width(col.Width).Render(truncate(text, col.Width)))
	}
	headerLine := m.padRow(lp.JoinHorizontal(lp.Left, headerCells...), false)

	lines := []string{headerLine}

	endIndex := m.List.StartIndex + availRows
	if endIndex > len(m.List.Builds) {
		endIndex = len(m.List.Builds)
	}
	for i := m.List.StartIndex; i < endIndex; i++ {
		build := m.List.Builds[i]

		var downloadState *model.DownloadState
		if build.Status == model.StateDownloading || build.Status == model.StateExtracting {
			if state, ok := m.Progress.DownloadStates[build.ID()]; ok {
				downloadState = state
			}
		} else if m.commands != nil && m.commands.downloads != nil {
			downloadState = m.commands.downloads.GetState(build.ID())
		}

		row := NewRow(build, i == m.List.Cursor, downloadState)
		lines = append(lines, m.padRow(row.Render(columns, m.Style), i == m.List.Cursor))
	}

	return renderBox("", title, strings.Join(lines, "\n"), boxWidth, height, lp.Color(p.Accent))
}

// centerBlock horizontally centers each line of s and adds leading blank lines
// so the content sits roughly in the middle of a height-tall block.
func centerBlock(s string, w, h int) string {
	centered := lp.NewStyle().Width(w).Align(lp.Center).Render(s)
	lines := strings.Split(centered, "\n")
	pad := (h - len(lines)) / 2
	if pad <= 0 {
		return centered
	}
	blank := strings.Repeat(" ", w)
	prefix := make([]string, 0, pad+len(lines))
	for i := 0; i < pad; i++ {
		prefix = append(prefix, blank)
	}
	return strings.Join(append(prefix, lines...), "\n")
}
