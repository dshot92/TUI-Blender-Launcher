package tui

import (
	"fmt"
	"github.com/dshot92/TUI-Blender-Launcher/model"
	"math"
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

// buildColumnNames lists the table columns in display order. A column's
// position in this slice is its Index and sort key.
var buildColumnNames = []string{"Version", "Status", "Branch", "Type", "Hash", "Size", "Build Date"}

// branchColumn is the index of the Branch column, whose cell is reused for the
// live download speed / extract percentage while a transfer is in progress.
const branchColumn = 2

// downloadCellWidth is the width reserved for that live cell, e.g.
// " 999.9 MB/s", so an in-progress row never clips its speed.
const downloadCellWidth = 11

// ColumnConfig represents the configuration for a table column.
type ColumnConfig struct {
	Name  string
	Key   string
	Width int
	Index int
	Style func(string) string
}

// columnDesiredWidths returns, for each column in display order, the width
// needed to show its header and every value in builds without truncation. The
// sorted column's header reserves two extra cells for the ↑/↓ marker.
func columnDesiredWidths(builds []model.BlenderBuild, sortColumn int) []int {
	desired := make([]int, len(buildColumnNames))
	for i, name := range buildColumnNames {
		desired[i] = width(name)
		if i == sortColumn {
			desired[i] += 2 // " ↑" / " ↓"
		}
	}

	consider := func(i int, s string) {
		if w := width(s); w > desired[i] {
			desired[i] = w
		}
	}

	for _, b := range builds {
		consider(0, b.Version)
		consider(1, stateGlyph(b.Status)+" "+b.Status.String())
		consider(branchColumn, b.Branch)
		consider(3, b.ReleaseCycle)
		consider(4, b.Hash)
		consider(5, model.FormatByteSize(b.Size))
		consider(6, model.FormatBuildDate(b.BuildDate))
	}

	// The Branch cell doubles as the live download speed / extract percentage
	// while a transfer is running, so reserve enough room for it.
	if desired[branchColumn] < downloadCellWidth {
		desired[branchColumn] = downloadCellWidth
	}

	return desired
}

// columnsFitWidth is the minimum column area width that shows every header and
// cell in full given the per-column desired widths.
func columnsFitWidth(desired []int) int {
	sum := 0
	for _, d := range desired {
		if d < 1 {
			d = 1
		}
		sum += d
	}
	return sum
}

// GetBuildColumns builds the column layout for a given column area width
// (excluding the selection gutter and pane padding) and the per-column desired
// widths. Widths always sum to exactly colWidth and, whenever colWidth is at
// least the sum of desired, no column is narrower than it needs to be.
func GetBuildColumns(colWidth int, desired []int) []ColumnConfig {
	widths := allocateWidths(colWidth, desired)

	columns := make([]ColumnConfig, len(buildColumnNames))
	for i, name := range buildColumnNames {
		w := widths[i]
		columns[i] = ColumnConfig{
			Name:  name,
			Key:   name,
			Index: i,
			Width: w,
			Style: func(w int) func(string) string {
				return func(s string) string {
					return lp.NewStyle().Align(lp.Center).Width(w).Render(truncate(s, w))
				}
			}(w),
		}
	}

	return columns
}

// allocateWidths distributes colWidth across columns proportionally to desired,
// guaranteeing the result sums to exactly colWidth. When there is enough room,
// each column receives at least its desired width and the surplus is shared.
func allocateWidths(colWidth int, desired []int) []int {
	n := len(desired)
	if n == 0 {
		return nil
	}
	if colWidth < n {
		colWidth = n
	}

	total := 0
	for _, d := range desired {
		if d < 1 {
			d = 1
		}
		total += d
	}

	widths := make([]int, n)
	remainders := make([]float64, n)
	assigned := 0
	for i, d := range desired {
		if d < 1 {
			d = 1
		}
		exact := float64(colWidth) * float64(d) / float64(total)
		base := int(math.Floor(exact))
		if base < 1 {
			base = 1
		}
		widths[i] = base
		assigned += base
		remainders[i] = exact - math.Floor(exact)
	}

	// Hand out any leftover cells to the largest fractional parts first.
	for diff := colWidth - assigned; diff > 0; diff-- {
		best := 0
		for i := 1; i < n; i++ {
			if remainders[i] > remainders[best] {
				best = i
			}
		}
		widths[best]++
		remainders[best] = -1
	}
	// Reclaim cells if the per-column minimum pushed us over budget.
	for diff := colWidth - assigned; diff < 0; diff++ {
		best := -1
		for i := 0; i < n; i++ {
			if widths[i] > 1 && (best == -1 || widths[i] > widths[best]) {
				best = i
			}
		}
		if best == -1 {
			break
		}
		widths[best]--
	}

	return widths
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
