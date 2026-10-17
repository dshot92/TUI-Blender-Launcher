package tui

import (
	"github.com/charmbracelet/bubbles/key"
)

// Layout constants for the TUI chrome and panes.
const (
	// Minimum terminal size we can render a meaningful layout for.
	minTermWidth  = 45
	minTermHeight = 12

	// Fixed vertical chrome heights (in lines).
	headerBoxHeight = 1 // single line with the logo/meta
	footerHeight    = 2 // two lines of key hints

	// Border frame sizes for a rounded box.
	boxBorderFrameWidth  = 2
	boxBorderFrameHeight = 2

	// Horizontal padding applied inside the builds pane (one space each side).
	panePadX = 1

	// Gap (in columns) between the builds pane and the detail pane.
	paneGap = 2

	// Below this terminal width the detail pane is hidden and the builds table
	// takes the full width.
	minDetailWidth = 104
	// Share of the terminal width given to the detail pane when visible.
	detailRatio = 0.40

	// Settings modal sizing.
	settingsMaxWidth = 110
	settingsMinWidth = 44

	// Settings list/detail layout. The panes sit side by side only when the
	// content is at least settingsMinTwoColumnWidth wide, otherwise they stack.
	settingsMinTwoColumnWidth = 68
	settingsListWidth         = 28
	settingsMinListWidth      = 20
	settingsMinDetailWidth    = 24
	settingsColumnGap         = 3 // gap plus the vertical divider between panes

	// Maximum completion candidates listed under the Download Directory row
	// before the rest are summarised as "… +N more".
	settingsMaxDirMatches = 6

	// Minimum width of the settings value area below which a value would be
	// unreadable, plus the slack subtracted when sizing a text input so it never
	// wraps the detail line.
	minValueWidth   = 8
	minInputWidth   = 10
	valueInputSlack = 5

	// Minimum width of the side-by-side detail pane, and of the confirm modal.
	minDetailPaneWidth = 30
	minConfirmWidth    = 30

	// Tallest the detail pane may grow when it is stacked below the list.
	maxStackedDetailHeight = 12

	// Build-completion dropdown sizing: chrome added around the widest
	// candidate, and the minimum dropdown width.
	dropdownChrome   = 4
	minDropdownWidth = 8

	// Context menu chrome: horizontal padding around the widest entry, and the
	// minimum menu width.
	contextMenuPadding  = 6
	minContextMenuWidth = 16

	// Below this width the header drops its build-count/version metadata.
	headerMetaMinWidth = minTermWidth + 1

	// Share of the terminal height at which the transient toast is anchored.
	toastPositionRatio = 0.75
)

// valueLabel is the "Value: " prefix shown before a settings value. Its display
// width is reused to size the value area and to anchor the completion dropdown.
const valueLabel = "Value: "

// View states
type viewState int

const (
	viewList viewState = iota
	viewInitialSetup
	viewSettings
)

// Command types for key bindings
type CommandType int

const (
	CmdQuit CommandType = iota
	CmdShowSettings
	CmdToggleSortOrder
	CmdFetchBuilds
	CmdDownloadBuild
	CmdLaunchBuild
	CmdOpenBuildDir
	CmdDeleteBuild
	CmdMoveUp
	CmdMoveDown
	CmdMoveLeft
	CmdMoveRight
	CmdSaveSettings
	CmdToggleEditMode
	CmdPageUp         // Add PageUp command
	CmdPageDown       // Add PageDown command
	CmdHome           // Add Home command
	CmdEnd            // Add End command
	CmdCleanOldBuilds // Add command for cleaning old builds
	CmdToggleDetail   // Show/hide the build info panel
	CmdOpenURL        // Open the selected build's download URL in a browser
)

// CmdNone marks a hint or menu entry with no bound action, so a click on it can
// be recognised as inert instead of running CmdQuit (the zero value).
const CmdNone CommandType = -1

// Delete/cancel confirmation choices. "No" is the default so that an accidental
// Enter never destroys anything.
const (
	confirmChoiceNo  = 0
	confirmChoiceYes = 1
)

// KeyCommand defines a keyboard command with its key binding and description
type KeyCommand struct {
	Type        CommandType
	Keys        []string
	Description string
}

// Commands mapping for different views
var (
	// Common commands for all views
	CommonCommands = []KeyCommand{
		{Type: CmdQuit, Keys: []string{"q", "Q", "ctrl+c"}, Description: "Quit application"},
	}

	// List view commands
	ListCommands = []KeyCommand{
		{Type: CmdShowSettings, Keys: []string{"s"}, Description: "Show settings"},
		{Type: CmdToggleSortOrder, Keys: []string{"r"}, Description: "Toggle sort order"},
		{Type: CmdFetchBuilds, Keys: []string{"f"}, Description: "Fetch online builds"},
		{Type: CmdDownloadBuild, Keys: []string{"d"}, Description: "Download selected build"},
		{Type: CmdLaunchBuild, Keys: []string{"enter"}, Description: "Launch selected build"},
		{Type: CmdOpenBuildDir, Keys: []string{"o"}, Description: "Open build directory"},
		{Type: CmdDeleteBuild, Keys: []string{"x"}, Description: "Delete build/Cancel download"},
		{Type: CmdMoveUp, Keys: []string{"up", "k"}, Description: "Move cursor up"},
		{Type: CmdMoveDown, Keys: []string{"down", "j"}, Description: "Move cursor down"},
		{Type: CmdMoveLeft, Keys: []string{"left", "h"}, Description: "Previous sort column"},
		{Type: CmdMoveRight, Keys: []string{"right", "l"}, Description: "Next sort column"},
		{Type: CmdPageUp, Keys: []string{"pgup"}, Description: "Page up"},
		{Type: CmdPageDown, Keys: []string{"pgdown"}, Description: "Page down"},
		{Type: CmdHome, Keys: []string{"home"}, Description: "Go to first item"},
		{Type: CmdEnd, Keys: []string{"end"}, Description: "Go to last item"},
		{Type: CmdToggleDetail, Keys: []string{"tab"}, Description: "Toggle build info panel"},
		{Type: CmdOpenURL, Keys: []string{"u"}, Description: "Open build download URL"},
		{Type: CmdCleanOldBuilds, Keys: []string{"c"}, Description: "Clean old builds"},
	}

	// Settings view commands
	SettingsCommands = []KeyCommand{
		{Type: CmdSaveSettings, Keys: []string{"s"}, Description: "Save settings and return"},
		{Type: CmdToggleEditMode, Keys: []string{"enter"}, Description: "Toggle edit mode"},
		{Type: CmdMoveUp, Keys: []string{"up", "k"}, Description: "Move cursor up"},
		{Type: CmdMoveDown, Keys: []string{"down", "j"}, Description: "Move cursor down"},
		{Type: CmdMoveLeft, Keys: []string{"left", "h"}, Description: "Select previous option"},
		{Type: CmdMoveRight, Keys: []string{"right", "l"}, Description: "Select next option"},
	}
)

// GetKeyBinding returns a tea key binding for the given command type
func GetKeyBinding(cmdType CommandType) key.Binding {
	var keys []string

	// Check in all command sets
	for _, cmd := range CommonCommands {
		if cmd.Type == cmdType {
			keys = cmd.Keys
			break
		}
	}

	if keys == nil {
		for _, cmd := range ListCommands {
			if cmd.Type == cmdType {
				keys = cmd.Keys
				break
			}
		}
	}

	if keys == nil {
		for _, cmd := range SettingsCommands {
			if cmd.Type == cmdType {
				keys = cmd.Keys
				break
			}
		}
	}

	return key.NewBinding(key.WithKeys(keys...))
}

// PrimaryKey returns the first configured key for a command type (used for
// rendering footer hints), or "" when the command has no binding.
func PrimaryKey(cmdType CommandType) string {
	if keys := GetKeyBinding(cmdType).Keys(); len(keys) > 0 {
		return keys[0]
	}
	return ""
}

// GetCommandsForView returns all commands available for a specific view
func GetCommandsForView(view viewState) []KeyCommand {
	result := make([]KeyCommand, len(CommonCommands))
	copy(result, CommonCommands)

	switch view {
	case viewList:
		result = append(result, ListCommands...)
	case viewSettings, viewInitialSetup:
		result = append(result, SettingsCommands...)
	}

	return result
}

// Styles using lipgloss are now in style.go
// To use styles, pass a Style struct (from style.go) to functions/components that need styling.
