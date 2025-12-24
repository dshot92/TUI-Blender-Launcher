package tui

import (
	"TUI-Blender-Launcher/local"
	"TUI-Blender-Launcher/model"
	"fmt"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

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
		return m, nil

	case progress.FrameMsg:
		// Pass to progress model
		newProgress, cmd := m.Progress.Update(msg)
		m.Progress = *newProgress.(*ProgressModel)
		return m, cmd
	}

	// Route based on view
	var cmd tea.Cmd
	switch m.currentView {
	case viewSettings, viewInitialSetup:
		var newSettings tea.Model
		newSettings, cmd = m.Settings.Update(msg) // settings_model Update might perform specific actions
		m.Settings = *newSettings.(*SettingsModel)

		// Handle specific signals from settings model if any?
		// For now, SettingsModel Update handles simple inputs.
		// We still need to intercept logic like "SaveSettings" which was in updateSettingsView

		// Check if we need to handle specific keys here that perform "Controllers" logic
		// that affects the whole app (like changing view)
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			return m.updateSettingsViewController(keyMsg, cmd)
		}
		return m, cmd

	default: // viewList
		// Handle list logic
		return m.updateListViewController(msg)
	}
}

// updateSettingsViewController handles app-level logic for settings view
func (m *Model) updateSettingsViewController(msg tea.KeyMsg, innerCmd tea.Cmd) (tea.Model, tea.Cmd) {
	// We check for specific commands that trigger state changes in the main model
	for _, cmd := range GetCommandsForView(m.currentView) {
		if MatchKey(msg, cmd.Type) {
			switch cmd.Type {
			case CmdQuit:
				return m, tea.Quit
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
							return errMsg{fmt.Errorf("no old builds to clean")}
						}
						return errMsg{fmt.Errorf("successfully cleaned %d old build(s)", count)}
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
					m.Settings.SetValues(m.config.DownloadDir, m.config.VersionFilter, m.config.BuildType)
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
