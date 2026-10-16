package tui

import (
	"fmt"
	"github.com/dshot92/TUI-Blender-Launcher/local"
	"github.com/dshot92/TUI-Blender-Launcher/model"
	"slices"
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
			m.autoSaveSettings()
			return mdl, c
		}
		m.syncVersionFilterFeedback()
		m.autoSaveSettings()
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

// autoSaveSettings persists the settings whenever the user changes a value.
// While a text field or the release-type multi-select is mid-edit it waits, so
// a partially typed value (or a cancelable toggle selection) is never written;
// the change is saved once the edit is committed. An unchanged snapshot is a
// no-op, so plain navigation never touches the disk.
func (m *Model) autoSaveSettings() {
	if m.Settings.EditMode || m.Settings.releaseEditing {
		return
	}
	cur := m.Settings.GetValues()
	if settingsEqual(cur, m.settingsSaved) {
		return
	}
	if err := m.SaveSettings(); err != nil {
		m.err = err
		return
	}
	if m.commands != nil {
		m.commands.SetConfig(m.config)
	}
	m.settingsSaved = cur
}

// settingsEqual reports whether two settings snapshots are identical.
func settingsEqual(a, b SettingsValues) bool {
	return a.DownloadDir == b.DownloadDir &&
		a.VersionFilter == b.VersionFilter &&
		a.BuildType == b.BuildType &&
		a.Theme == b.Theme &&
		slices.Equal(a.ReleaseTypes, b.ReleaseTypes)
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
					wasInitial := m.currentView == viewInitialSetup
					m.currentView = viewList
					mdl, cmd := m.SaveSettingsAndReturn()
					if wasInitial {
						statusCmd := func() tea.Msg {
							return statusMsg{text: "Press f to fetch builds"}
						}
						return mdl, tea.Batch(cmd, statusCmd)
					}
					return mdl, cmd
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

	case tea.MouseMsg:
		// A left click on the download-URL row of the info panel opens the link.
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if url, ok := m.urlAt(msg.X, msg.Y); ok {
				return m, openURLCmd(url)
			}
		}
		return m, nil

	case tea.KeyMsg:
		// A pending delete/cancel confirmation captures all keys until it is
		// resolved, so the destructive action is never performed accidentally.
		if m.deleteConfirm {
			switch msg.Type {
			case tea.KeyEsc:
				m.deleteConfirm = false
				return m, nil
			case tea.KeyLeft:
				m.deleteConfirmChoice = 0
				return m, nil
			case tea.KeyRight:
				m.deleteConfirmChoice = 1
				return m, nil
			case tea.KeyEnter:
				m.deleteConfirm = false
				if m.deleteConfirmChoice == 0 {
					return m.doDeleteBuild()
				}
				return m, nil
			}
			if msg.Type == tea.KeyRunes {
				switch string(msg.Runes) {
				case "h":
					m.deleteConfirmChoice = 0
					return m, nil
				case "l":
					m.deleteConfirmChoice = 1
					return m, nil
				case "x", "X", "y", "Y":
					m.deleteConfirm = false
					return m.doDeleteBuild()
				case "n", "N", "q", "Q":
					m.deleteConfirm = false
					return m, nil
				}
			}
			return m, nil
		}

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
					// Baseline for auto-save: nothing is written until an edit.
					m.settingsSaved = m.Settings.GetValues()
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
					build := m.List.GetSelectedBuild()
					if build == nil {
						return m, nil
					}
					downloading := build.Status == model.StateDownloading || build.Status == model.StateExtracting
					if !downloading && m.commands != nil && m.commands.downloads != nil {
						if state := m.commands.downloads.GetState(build.ID()); state != nil &&
							(state.BuildState == model.StateDownloading || state.BuildState == model.StateExtracting) {
							downloading = true
						}
					}
					if !downloading && build.Status != model.StateLocal && build.Status != model.StateUpdate {
						return m, nil
					}
					m.deleteConfirm = true
					m.deleteConfirmBuild = *build
					m.deleteConfirmCancel = downloading
					m.deleteConfirmChoice = 0
					return m, nil
				case CmdCleanOldBuilds:
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
				case CmdToggleDetail:
					m.detailShown = !m.detailShown
					m.computeLayout()
					if m.detailShown && !m.layout.detailVisible {
						return m, func() tea.Msg {
							return statusMsg{text: "not enough space for the info panel"}
						}
					}
					return m, nil
				case CmdOpenURL:
					return m.handleOpenURL()
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
