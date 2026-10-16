package tui

import (
	"TUI-Blender-Launcher/config"
)

// Model represents the state of the TUI application.
type Model struct {
	config   config.Config
	commands *Commands
	err      error
	status   string

	// statusToken identifies the most recent transient status message. Expiry
	// timers carry the token they were scheduled for so a stale timer cannot
	// clear a newer status.
	statusToken int64

	// Layout
	terminalWidth  int
	terminalHeight int

	// Application State
	currentView viewState

	// Sub-models
	List     ListModel
	Settings SettingsModel
	Progress ProgressModel

	Style Style

	// detailShown controls whether the build info panel is requested. The panel
	// is hidden only when the user toggles it off; when space is tight it docks
	// below the list rather than disappearing.
	detailShown bool

	// Layout, recomputed whenever the terminal or view changes.
	layout layoutInfo

	// Cached column layout, rebuilt only when the column area width changes.
	columns      []ColumnConfig
	columnsWidth int
}

// InitialModel creates the initial state of the TUI model.
func InitialModel(cfg config.Config, needsSetup bool) *Model {
	style := NewStyleFromPalette(ResolvePalette(cfg.Theme))

	m := &Model{
		config:      cfg,
		commands:    NewCommands(cfg),
		List:        NewListModel(style),
		Settings:    NewSettingsModel(cfg, style),
		Progress:    NewProgressModel(),
		Style:       style,
		detailShown: true,
	}

	if needsSetup {
		m.currentView = viewInitialSetup
		// Ensure focus is correct
		m.Settings.FocusIndex = 0
	} else {
		m.currentView = viewList
	}

	return m
}

// UpdateWindowSize updates the terminal dimensions and recalculates layout
func (m *Model) UpdateWindowSize(width, height int) {
	m.terminalWidth = width
	m.terminalHeight = height
	m.List.TerminalHeight = height
	m.computeLayout()
}

// layoutInfo holds all derived dimensions for the current frame.
type layoutInfo struct {
	width, height int

	// Detail panel placement.
	detailVisible bool // the panel is rendered
	detailStacked bool // rendered below the list instead of beside it

	listBoxWidth    int
	listBoxHeight   int
	detailBoxWidth  int
	detailBoxHeight int

	bodyHeight int

	// tableWidth is the width available to row content inside the builds pane,
	// excluding borders and pane padding. colWidth additionally excludes the
	// selection gutter.
	tableWidth int
	colWidth   int
	listRows   int

	settingsWidth int
}

// computeLayout derives pane sizes from the terminal dimensions. It keeps the
// row count known to the ListModel in sync with what is actually rendered so
// scrolling never desyncs from the viewport.
func (m *Model) computeLayout() {
	w, h := m.terminalWidth, m.terminalHeight
	li := layoutInfo{width: w, height: h}

	statusH := 0
	if m.err != nil || m.status != "" {
		statusH = 1
	}

	li.bodyHeight = h - headerBoxHeight - footerHeight - statusH
	if li.bodyHeight < boxBorderFrameHeight+2 {
		li.bodyHeight = boxBorderFrameHeight + 2
	}

	// Minimum heights needed to render a usable builds pane and detail panel.
	const minDetailHeight = 5
	minBuildsHeight := boxBorderFrameHeight + 1 + 2 // borders + header + 2 data rows

	li.listBoxWidth = w
	li.listBoxHeight = li.bodyHeight

	if m.detailShown {
		// Prefer a side-by-side split when there is enough lateral room.
		if w >= minDetailWidth {
			left := int(float64(w) * (1 - detailRatio))
			right := w - left - paneGap
			if right >= 30 {
				li.detailVisible = true
				li.detailStacked = false
				li.listBoxWidth = left
				li.detailBoxWidth = right
			}
		}
		// Otherwise dock the panel below the list, if there is vertical room.
		if !li.detailVisible && li.bodyHeight >= minBuildsHeight+minDetailHeight {
			detailH := li.bodyHeight * 2 / 5
			if detailH < minDetailHeight {
				detailH = minDetailHeight
			}
			if detailH > 12 {
				detailH = 12
			}
			if li.bodyHeight-detailH < minBuildsHeight {
				detailH = li.bodyHeight - minBuildsHeight
			}
			li.detailVisible = true
			li.detailStacked = true
			li.listBoxWidth = w
			li.detailBoxWidth = w
			li.detailBoxHeight = detailH
			li.listBoxHeight = li.bodyHeight - detailH
		}
	}

	if li.detailVisible && !li.detailStacked {
		li.listBoxHeight = li.bodyHeight
		li.detailBoxHeight = li.bodyHeight
	}

	innerWidth := li.listBoxWidth - boxBorderFrameWidth
	if innerWidth < 3 {
		innerWidth = 3
	}
	li.tableWidth = innerWidth - 2*panePadX
	if li.tableWidth < gutterWidth+1 {
		li.tableWidth = gutterWidth + 1
	}
	li.colWidth = li.tableWidth - gutterWidth
	li.listRows = li.listBoxHeight - boxBorderFrameHeight - 1
	if li.listRows < 1 {
		li.listRows = 1
	}

	sw := w
	if sw > settingsMaxWidth {
		sw = settingsMaxWidth
	}
	if sw < settingsMinWidth {
		sw = settingsMinWidth
	}
	li.settingsWidth = sw
	if inner := sw - boxBorderFrameWidth; inner > 0 {
		m.Settings.SetWidth(inner)
	}

	m.layout = li
	m.List.VisibleRows = li.listRows
}

// SyncDownloadStates ensures the model has the latest download states from the commands manager
func (m *Model) SyncDownloadStates() {
	if m.commands == nil || m.commands.downloads == nil {
		return
	}

	// Get all states from the download manager
	states := m.commands.downloads.GetAllStates()
	if states == nil {
		return
	}

	// Update our local copy of states
	m.Progress.SyncDownloadStates(states)
}

// SaveSettings saves the current settings to the configuration file
func (m *Model) SaveSettings() error {
	// Update config values from settings inputs
	downloadDir, versionFilter, buildType, theme := m.Settings.GetValues()

	m.config.DownloadDir = downloadDir
	m.config.VersionFilter = versionFilter
	m.config.BuildType = buildType
	m.config.Theme = theme
	m.setTheme(ResolvePalette(theme))

	// Save the config
	return config.SaveConfig(m.config)
}

// setTheme applies a palette to the live UI, propagating it to the sub-models
// so every component renders with the new colors on the next frame.
func (m *Model) setTheme(p Palette) {
	if m.Style.Palette.Name == p.Name {
		return
	}
	m.Style = NewStyleFromPalette(p)
	m.Settings.Style = m.Style
	m.List.Style = m.Style
}

func (m *Model) View() string {
	// Render the page using the custom render function.
	// Note: download states are synced on tick (handleTickMsg), not here, so
	// rendering stays side-effect free.
	return m.renderPageForView()
}

// columnsLayout returns the cached table columns, rebuilding them only when
// the column area width changes. Avoids rebuilding lipgloss styles every render.
func (m *Model) columnsLayout() []ColumnConfig {
	if m.columns == nil || m.columnsWidth != m.layout.colWidth {
		m.columns = GetBuildColumns(m.layout.colWidth)
		m.columnsWidth = m.layout.colWidth
	}
	return m.columns
}

// Proxy methods for compatibility/convenience if needed,
// strictly speaking we should access via m.List... etc.
