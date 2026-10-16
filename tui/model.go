package tui

import (
	"time"

	"github.com/dshot92/TUI-Blender-Launcher/config"
	"github.com/dshot92/TUI-Blender-Launcher/model"
)

// contextMenuItem is a single build-context-menu entry.
type contextMenuItem struct {
	label string
	cmd   CommandType
}

// Model represents the state of the TUI application.
type Model struct {
	config   config.Config
	commands *Commands
	err      error
	status   string

	// filterErr is transient validation feedback for the Version Filter while
	// its settings row is focused. It is surfaced as an error toast so a
	// malformed value is shown instead of being silently ignored.
	filterErr error

	// settingsSaved is the last snapshot written to disk. The settings view
	// persists the config automatically whenever the live values differ, so an
	// edit survives without an explicit save.
	settingsSaved SettingsValues

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

	// detailShown controls whether the build info panel is requested. It starts
	// hidden on launch and is toggled on demand; when space is tight a shown
	// panel docks below the list rather than disappearing.
	detailShown bool

	// deleteConfirm tracks a pending delete/cancel confirmation popup for a
	// build, so a destructive action is never performed immediately.
	deleteConfirm       bool
	deleteConfirmBuild  model.BlenderBuild
	deleteConfirmCancel bool
	deleteConfirmChoice int // 0 = confirm (yes), 1 = abort (no)

	// Layout, recomputed whenever the terminal or view changes.
	layout layoutInfo

	// Cached column layout, rebuilt when the column area width or the desired
	// column widths change.
	columns        []ColumnConfig
	columnsWidth   int
	columnsDesired []int

	// Context menu opened by a right click on a build row.
	contextMenuVisible  bool
	contextMenuBuildIdx int
	contextMenuX        int
	contextMenuY        int
	contextMenuChoice   int
	contextMenuItems    []contextMenuItem

	// Double-click detection for launching a local build. lastClickBuild is the
	// row of the previous left click, used to recognise a repeat click on the
	// same row within doubleClickWindow.
	lastClickTime  time.Time
	lastClickBuild int
	lastClickX     int
	lastClickY     int
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
		detailShown: false,
	}
	// Treat the loaded config as the baseline so nothing is written until the
	// user actually changes a setting.
	m.settingsSaved = m.Settings.GetValues()

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

	// columnDesired caches the per-column widths needed to avoid truncation for
	// the current data, so the column build below can reuse them.
	columnDesired []int

	settingsWidth int
}

// computeLayout derives pane sizes from the terminal dimensions. It keeps the
// row count known to the ListModel in sync with what is actually rendered so
// scrolling never desyncs from the viewport.
func (m *Model) computeLayout() {
	w, h := m.terminalWidth, m.terminalHeight
	li := layoutInfo{width: w, height: h}

	// The transient status/error toast is layered over the bottom of the frame
	// rather than reserving a row, so the body always gets the full height.
	li.bodyHeight = h - headerBoxHeight - footerHeight
	if li.bodyHeight < boxBorderFrameHeight+2 {
		li.bodyHeight = boxBorderFrameHeight + 2
	}

	// Minimum heights needed to render a usable builds pane and detail panel.
	const minDetailHeight = 5
	minBuildsHeight := boxBorderFrameHeight + 1 + 2 // borders + header + 2 data rows

	// Widths each column needs to show its values without clipping. The builds
	// pane is only placed beside the detail pane while the table still fits in
	// full, so nothing truncates just to make room for the info panel.
	desired := columnDesiredWidths(m.List.Builds, m.List.SortColumn)
	li.columnDesired = desired
	fitColWidth := columnsFitWidth(desired)

	li.listBoxWidth = w
	li.listBoxHeight = li.bodyHeight

	if m.detailShown {
		// Prefer a side-by-side split while the table still fits in full.
		if w >= minDetailWidth {
			left := int(float64(w) * (1 - detailRatio))
			right := w - left - paneGap
			if right >= 30 && colWidthForBox(left) >= fitColWidth {
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

	li.colWidth = colWidthForBox(li.listBoxWidth)
	li.tableWidth = li.colWidth + gutterWidth
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
	// Keep the modal inside the terminal with a small margin on each side.
	if maxW := w - 2; sw > maxW {
		sw = maxW
	}
	if sw < 20 {
		sw = 20
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
	v := m.Settings.GetValues()

	m.config.DownloadDir = v.DownloadDir
	m.config.VersionFilter = v.VersionFilter
	m.config.BuildType = v.BuildType
	m.config.ReleaseTypes = v.ReleaseTypes
	m.config.Theme = v.Theme
	m.setTheme(ResolvePalette(v.Theme))

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

// columnsLayout returns the cached table columns, rebuilding them only when the
// column area width or the desired column widths change. Avoids rebuilding
// lipgloss styles every render.
func (m *Model) columnsLayout() []ColumnConfig {
	desired := m.layout.columnDesired
	if desired == nil {
		desired = columnDesiredWidths(m.List.Builds, m.List.SortColumn)
	}
	if m.columns == nil || m.columnsWidth != m.layout.colWidth || !equalInts(m.columnsDesired, desired) {
		m.columns = GetBuildColumns(m.layout.colWidth, desired)
		m.columnsWidth = m.layout.colWidth
		m.columnsDesired = desired
	}
	return m.columns
}

// colWidthForBox returns the column area width available inside a builds pane of
// the given box width (excluding borders, pane padding, and the selection
// gutter).
func colWidthForBox(boxWidth int) int {
	inner := boxWidth - boxBorderFrameWidth
	if inner < 3 {
		inner = 3
	}
	tw := inner - 2*panePadX
	if tw < gutterWidth+1 {
		tw = gutterWidth + 1
	}
	return tw - gutterWidth
}

// equalInts reports whether two int slices have identical contents.
func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Proxy methods for compatibility/convenience if needed,
// strictly speaking we should access via m.List... etc.
