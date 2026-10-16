package tui

import (
	"TUI-Blender-Launcher/model"
	"fmt"
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// Row represents a single row in the builds table
type Row struct {
	Build      model.BlenderBuild
	IsSelected bool
	Status     *model.DownloadState
}

// NewRow creates a new row instance from a build
func NewRow(build model.BlenderBuild, isSelected bool, status *model.DownloadState) Row {
	return Row{
		Build:      build,
		IsSelected: isSelected,
		Status:     status,
	}
}

// Column configuration
type columnConfig struct {
	width    int
	priority int     // Lower number = higher priority (will be shown first)
	flex     float64 // Flex ratio for dynamic width calculation
}

// Column configurations
var (
	// Column configurations with priorities and flex values
	columnConfigs = map[string]columnConfig{
		"Version":    {width: 0, priority: 1, flex: 1.0}, // Version gets more space
		"Status":     {width: 0, priority: 2, flex: 1.0}, // Status needs room for different states
		"Branch":     {width: 0, priority: 5, flex: 1.0},
		"Type":       {width: 0, priority: 4, flex: 1.0},
		"Hash":       {width: 0, priority: 6, flex: 1.0},
		"Size":       {width: 0, priority: 7, flex: 1.0},
		"Build Date": {width: 0, priority: 3, flex: 1.0},
	}
)

// Render renders a single row with the given column configuration.
// It takes a Style parameter for row styling.
func (r Row) Render(columns []ColumnConfig, style Style) string {
	// Special handling for downloads and extractions
	isDownloading := r.Build.Status == model.StateDownloading && r.Status != nil
	isExtracting := r.Build.Status == model.StateExtracting && r.Status != nil
	isOnline := r.Build.Status == model.StateOnline
	isUpdate := r.Build.Status == model.StateUpdate
	isFailed := r.Build.Status == model.StateFailed
	isCancelled := r.Build.Status == model.StateCancelled // StateNone is "Cancelled"

	var rowString string

	if isDownloading || isExtracting {
		// Render the leading cells (Version, Status, Branch) individually and
		// append a progress bar spanning the remaining columns. Building the
		// bar cell-wise (rather than slicing rendered output by byte offset)
		// avoids cutting through ANSI escape sequences.
		var prefixCells []string
		barWidth := 0
		for _, col := range columns {
			var cellContent string
			switch col.Key {
			case "Version":
				cellContent = r.Build.Version
			case "Status":
				if isDownloading {
					cellContent = model.StateDownloading.String()
				} else {
					cellContent = model.StateExtracting.String()
				}
			case "Branch":
				// Show download speed / extraction percentage in the Branch column.
				if isDownloading && r.Status.Speed > 0 {
					speedMBps := r.Status.Speed / 1024 / 1024
					if speedMBps < 100 {
						cellContent = fmt.Sprintf("%6.1f MB/s", speedMBps)
					} else {
						cellContent = fmt.Sprintf("%6.0f MB/s", speedMBps)
					}
				} else if isExtracting {
					cellContent = fmt.Sprintf("%6.1f%%", r.Status.Progress*100)
				}
			default:
				// Type, Hash, Size and Build Date are replaced by the bar.
				barWidth += col.Width
				continue
			}
			prefixCells = append(prefixCells, col.Style(cellContent))
		}
		rowString = lp.JoinHorizontal(lp.Left, prefixCells...) + renderProgressBar(r.Status.Progress, barWidth)
	} else {
		// Normal rendering for non-downloading builds
		cells := make([]string, 0, len(columns))
		for _, col := range columns {
			var cellContent string
			switch col.Key {
			case "Version":
				cellContent = r.Build.Version
			case "Status":
				cellContent = r.Build.Status.String()
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

	// Apply a consistent style across the entire row
	totalWidth := sumColumnWidths(columns)
	switch {
	case r.IsSelected:
		return style.SelectedRow.Width(totalWidth).Render(rowString)
	case isFailed || isCancelled:
		return lp.NewStyle().
			Foreground(lp.Color(redColor)).
			Width(totalWidth).
			Render(rowString)
	case isOnline:
		return lp.NewStyle().
			Foreground(lp.Color(orangeColor)).
			Width(totalWidth).
			Render(rowString)
	case isUpdate:
		return lp.NewStyle().
			Foreground(lp.Color(greenColor)).
			Width(totalWidth).
			Render(rowString)
	default:
		return style.RegularRow.Width(totalWidth).Render(rowString)
	}
}

// renderProgressBar renders a progress bar occupying the given display width.
func renderProgressBar(progress float64, width int) string {
	if width <= 0 {
		return ""
	}
	if progress < 0 {
		progress = 0
	} else if progress > 1 {
		progress = 1
	}

	completedWidth := int(float64(width) * progress)
	if completedWidth > width {
		completedWidth = width
	}
	remainingWidth := width - completedWidth

	bar := ""
	if completedWidth > 0 {
		bar += lp.NewStyle().
			Background(lp.Color(highlightColor)).
			Foreground(lp.Color(textColor)).
			Width(completedWidth).
			Render("")
	}
	if remainingWidth > 0 {
		bar += lp.NewStyle().
			Background(lp.Color(backgroundColor)).
			Width(remainingWidth).
			Render("")
	}
	return bar
}

// Helper function to calculate the sum of all column widths
func sumColumnWidths(columns []ColumnConfig) int {
	sum := 0
	for _, col := range columns {
		sum += col.Width
	}
	return sum
}

// ColumnConfig represents the configuration for a table column
type ColumnConfig struct {
	Name  string
	Key   string
	Width int
	Index int
	Style func(string) string
}

// GetBuildColumns builds the column layout for the given terminal width.
func GetBuildColumns(terminalWidth int) []ColumnConfig {
	var cellStyleCenter = lp.NewStyle().Align(lp.Center)
	columns := []ColumnConfig{
		{Name: "Version", Key: "Version", Index: 0},
		{Name: "Status", Key: "Status", Index: 1},
		{Name: "Branch", Key: "Branch", Index: 2},
		{Name: "Type", Key: "Type", Index: 3},
		{Name: "Hash", Key: "Hash", Index: 4},
		{Name: "Size", Key: "Size", Index: 5},
		{Name: "Build Date", Key: "Build Date", Index: 6},
	}
	// Compute total flex for all columns
	totalFlex := 0.0
	for i := range columns {
		totalFlex += columnConfigs[columns[i].Key].flex
	}
	// Assign each column a width proportional to its flex value
	for i := range columns {
		flex := columnConfigs[columns[i].Key].flex
		colWidth := int((float64(terminalWidth) * flex) / totalFlex)
		columns[i].Width = colWidth
		columns[i].Style = func(width int) func(string) string {
			return func(s string) string {
				return cellStyleCenter.Width(width).Render(s)
			}
		}(colWidth)
	}
	return columns
}

// RenderRows builds the visible rows for the current model state.
func RenderRows(m *Model, visibleRowsCount int) string {
	var output strings.Builder
	newlineStyle := lp.NewStyle().Render("\n")

	// Get column configuration (cached)
	columns := m.columnsLayout()

	// Calculate visible range
	endIndex := m.List.StartIndex + visibleRowsCount
	if endIndex > len(m.List.Builds) {
		endIndex = len(m.List.Builds)
	}

	// Map to track which build IDs we've processed in this render pass
	processedBuilds := make(map[string]bool)

	// Only render rows in the visible range
	for i := m.List.StartIndex; i < endIndex; i++ {
		build := m.List.Builds[i]

		// Create a buildID to check for download state
		buildID := build.ID()

		// Track that we're processing this build
		processedBuilds[buildID] = true

		// Get download state if exists
		var downloadState *model.DownloadState = nil

		// Check if this is a downloading or extracting build
		if build.Status == model.StateDownloading || build.Status == model.StateExtracting {
			// Check in current model's download states using ListModel or ProgressModel
			// ProgressModel has DownloadStates
			if state, exists := m.Progress.DownloadStates[buildID]; exists {
				downloadState = state

				// Update last render state for downloads
				m.List.LastRenderState[buildID] = state.Progress
			}
		} else {
			// Fallback to checking in commands downloads manager
			if m.commands != nil && m.commands.downloads != nil {
				downloadState = m.commands.downloads.GetState(buildID)
			}
		}

		// Always render downloading/extracting rows, never skip them
		row := NewRow(build, i == m.List.Cursor, downloadState)
		rowText := row.Render(columns, m.Style)

		output.WriteString(rowText)
		if i < endIndex-1 {
			output.WriteString(newlineStyle)
		}
	}

	// Clean up lastRenderState for builds that are no longer visible/processing
	for buildID := range m.List.LastRenderState {
		if !processedBuilds[buildID] {
			delete(m.List.LastRenderState, buildID)
		}
	}

	return output.String()
}

// renderBuildContent renders the builds table (header + rows) for the list view.
func (m *Model) renderBuildContent(availableHeight int) string {
	var output strings.Builder
	newlineStyle := lp.NewStyle().Render("\n")

	if len(m.List.Builds) == 0 {
		// No builds to display
		var msg string = "No Blender builds found locally or online."

		return lp.Place(
			m.terminalWidth,
			availableHeight,
			lp.Center,
			lp.Top,
			lp.NewStyle().Foreground(lp.Color(highlightColor)).Render(msg),
		)
	}

	// Get column configuration (cached)
	columns := m.columnsLayout()

	// Build table header row first (without styling yet)
	var headerCells []string
	for _, col := range columns {
		headerText := col.Name
		if col.Index == m.List.SortColumn {
			if m.List.SortReversed {
				headerText += " ↓"
			} else {
				headerText += " ↑"
			}
		}
		if col.Index == m.List.SortColumn {
			headerCells = append(headerCells, m.Style.SelectedHeaderCell.Width(col.Width).Render(headerText))
		} else {
			headerCells = append(headerCells, m.Style.HeaderCell.Width(col.Width).Render(headerText))
		}
	}

	// Join header cells horizontally
	headerRow := lp.JoinHorizontal(lp.Left, headerCells...)

	// Add a newline if needed
	if !strings.HasSuffix(headerRow, "\n") {
		headerRow += newlineStyle
	}

	// Add the styled header to output
	output.WriteString(headerRow)

	// Calculate how many rows can be displayed in the available height
	// Subtract 1 for the header row
	visibleRowsCount := availableHeight - 1
	if visibleRowsCount < 1 {
		visibleRowsCount = 1
	}

	// Render visible rows with scrolling
	rowsContent := RenderRows(m, visibleRowsCount)
	output.WriteString(rowsContent)

	// Create the final styled table with proper width
	finalOutput := lp.NewStyle().Width(m.terminalWidth).Render(output.String())

	return finalOutput
}
