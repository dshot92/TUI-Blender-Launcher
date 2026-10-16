package tui

import (
	"TUI-Blender-Launcher/local"
	"TUI-Blender-Launcher/model"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// statusLifetime is how long a transient status message stays on screen.
const statusLifetime = 5 * time.Second

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	var cmds []tea.Cmd

	// Start with local build scan to get builds already on disk
	cmds = append(cmds, m.commands.ScanLocalBuilds())

	// Add a program message listener to receive messages from background goroutines
	cmds = append(cmds, m.commands.ProgramMsgListener())

	// Start a ticker for continuous UI updates to show download progress
	cmds = append(cmds, m.commands.StartTicker())

	return tea.Batch(cmds...)
}

// Update updates the model based on messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle global messages
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.UpdateWindowSize(msg.Width, msg.Height)
		return m, nil

	case errMsg:
		m.err = msg.err
		m.status = ""
		return m, nil

	case error:
		// Raw errors (e.g. from local.LaunchBlenderCmd) would otherwise fall
		// through and be silently dropped.
		m.err = msg
		m.status = ""
		return m, nil

	case statusMsg:
		m.status = msg.text
		m.err = nil
		// Schedule the status line to expire. Bump the token so any earlier
		// timer becomes a no-op.
		m.statusToken++
		token := m.statusToken
		return m, tea.Tick(statusLifetime, func(time.Time) tea.Msg {
			return clearStatusMsg{token: token}
		})

	case clearStatusMsg:
		// Only clear if no newer status message has replaced this one.
		if msg.token == m.statusToken {
			m.status = ""
		}
		return m, nil
	}

	// Route based on view
	var cmd tea.Cmd
	switch m.currentView {
	case viewSettings, viewInitialSetup:
		// Handle tick messages at top level to maintain update chain even in settings
		if _, ok := msg.(tickMsg); ok {
			return m.handleTickMsg(msg.(tickMsg))
		}

		var newSettings tea.Model
		newSettings, cmd = m.Settings.Update(msg) // settings_model Update might perform specific actions
		m.Settings = *newSettings.(*SettingsModel)

		// Handle specific signals from settings model if any?
		// For now, SettingsModel Update handles simple inputs.
		// We still need to intercept logic like "SaveSettings" which was in updateSettingsView

		// Check if we need to handle specific keys here that perform "Controllers" logic
		// that affects the whole app (like changing view)
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			mdl, c := m.updateSettingsViewController(keyMsg, cmd)
			// Validate the version filter live as the user types or navigates,
			// so a malformed value surfaces in the toast instead of being
			// silently ignored.
			m.syncVersionFilterFeedback()
			return mdl, c
		}
		m.syncVersionFilterFeedback()
		return m, cmd

	default: // viewList
		// Handle list logic
		return m.updateListViewController(msg)
	}
}

// syncVersionFilterFeedback validates the Version Filter while its settings row
// is focused, surfacing a malformed value as a live error toast. The message is
// cleared once the row loses focus (or the value becomes valid), so it never
// lingers in the list view.
func (m *Model) syncVersionFilterFeedback() {
	inSettings := m.currentView == viewSettings || m.currentView == viewInitialSetup
	if inSettings && m.Settings.FocusIndex == settingVersionFilter {
		v := strings.TrimSpace(m.Settings.Inputs[settingVersionFilter].Value())
		if _, err := model.ParseVersionFilter(v); err != nil {
			m.filterErr = fmt.Errorf("invalid version filter format '%s': %w", v, err)
			return
		}
	}
	m.filterErr = nil
}

// updateSettingsViewController handles app-level logic for settings view
func (m *Model) updateSettingsViewController(msg tea.KeyMsg, innerCmd tea.Cmd) (tea.Model, tea.Cmd) {
	// Live-preview the selected palette as the user cycles options.
	if m.Settings.Theme != m.Style.Palette.Name {
		m.setTheme(ResolvePalette(m.Settings.Theme))
	}

	// We check for specific commands that trigger state changes in the main model
	for _, cmd := range GetCommandsForView(m.currentView) {
		if MatchKey(msg, cmd.Type) {
			switch cmd.Type {
			case CmdQuit:
				// While editing a text field, "q" is a literal character.
				if !m.Settings.EditMode {
					return m, tea.Quit
				}
			case CmdSaveSettings:
				if !m.Settings.EditMode {
					m.currentView = viewList
					return m.SaveSettingsAndReturn()
				}
			case CmdCleanOldBuilds:
				if !m.Settings.EditMode {
					return m, func() tea.Msg {
						count, err := local.CleanOldBuilds(m.config.DownloadDir)
						if err != nil {
							return errMsg{err}
						}
						if count == 0 {
							return statusMsg{text: "no old builds to clean"}
						}
						return statusMsg{text: fmt.Sprintf("successfully cleaned %d old build(s)", count)}
					}
				}
			}
		}
	}
	return m, innerCmd
}

// updateListViewController handles logic for list view (controller layer)
func (m *Model) updateListViewController(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case localBuildsScannedMsg:
		return m.handleLocalBuildsScanned(msg)

	case buildsFetchedMsg:
		return m.handleBuildsFetched(msg)

	case buildsUpdatedMsg:
		return m.handleBuildsUpdated(msg)

	case model.BlenderExecMsg:
		return m.handleBlenderExec(msg)

	case startDownloadMsg:
		return m.handleStartDownloadMsg(msg)

	case downloadCompleteMsg:
		return m.handleDownloadCompleteMsg(msg)

	case buildDeletedMsg:
		return m.handleBuildDeleted(msg)

	case tickMsg:
		return m.handleTickMsg(msg)

	case tea.KeyMsg:
		// Check for app-level commands first
		for _, command := range GetCommandsForView(viewList) {
			if MatchKey(msg, command.Type) {
				switch command.Type {
				case CmdQuit:
					return m, tea.Quit
				case CmdShowSettings:
					m.currentView = viewSettings
					m.Settings.SetValues(SettingsValues{
						DownloadDir:   m.config.DownloadDir,
						VersionFilter: m.config.VersionFilter,
						BuildType:     m.config.BuildType,
						ReleaseTypes:  m.config.ReleaseTypes,
						Theme:         m.config.Theme,
					})
					return m, nil
				case CmdFetchBuilds:
					return m, m.commands.FetchBuilds()
				case CmdDownloadBuild:
					return m.handleStartDownload()
				case CmdLaunchBuild:
					return m.handleLaunchBlender()
				case CmdOpenBuildDir:
					return m.handleOpenBuildDir()
				case CmdDeleteBuild:
					return m.handleDeleteBuild()
				case CmdToggleDetail:
					m.detailShown = !m.detailShown
					m.computeLayout()
					if m.detailShown && !m.layout.detailVisible {
						return m, func() tea.Msg {
							return statusMsg{text: "not enough space for the info panel"}
						}
					}
					return m, nil
				}
			}
		}
	}

	// Pass to ListModel
	var newList tea.Model
	newList, cmd = m.List.Update(msg)
	m.List = *newList.(*ListModel)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}
