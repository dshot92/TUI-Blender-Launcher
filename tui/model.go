package tui

import (
	"TUI-Blender-Launcher/config"
)

// Model represents the state of the TUI application.
type Model struct {
	config   config.Config
	commands *Commands
	err      error

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
}

// InitialModel creates the initial state of the TUI model.
func InitialModel(cfg config.Config, needsSetup bool) *Model {
	style := NewStyle()

	m := &Model{
		config:   cfg,
		commands: NewCommands(cfg),
		List:     NewListModel(style),
		Settings: NewSettingsModel(cfg, style),
		Progress: NewProgressModel(),
		Style:    style,
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
	downloadDir, versionFilter, buildType := m.Settings.GetValues()

	m.config.DownloadDir = downloadDir
	m.config.VersionFilter = versionFilter
	m.config.BuildType = buildType

	// Save the config
	return config.SaveConfig(m.config)
}

func (m *Model) View() string {
	// Sync download states before rendering
	m.SyncDownloadStates()

	// Render the page using the custom render function.
	return m.renderPageForView()
}

// Proxy methods for compatibility/convenience if needed,
// strictly speaking we should access via m.List... etc.
