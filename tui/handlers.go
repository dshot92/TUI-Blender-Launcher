package tui

import (
	"TUI-Blender-Launcher/download"
	"TUI-Blender-Launcher/launch"
	"TUI-Blender-Launcher/local"
	"TUI-Blender-Launcher/model"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Helper to check key matches
func MatchKey(msg tea.KeyMsg, cmdType CommandType) bool {
	return key.Matches(msg, GetKeyBinding(cmdType))
}

// Helper functions for handling specific actions in list view
func (m *Model) handleLaunchBlender() (tea.Model, tea.Cmd) {
	selectedBuild := m.List.GetSelectedBuild()
	if selectedBuild == nil {
		return m, nil
	}

	// Only attempt to launch if it's a local build or has an update available
	if selectedBuild.Status == model.StateLocal || selectedBuild.Status == model.StateUpdate {
		cmd := local.LaunchBlenderCmd(m.config.DownloadDir, selectedBuild.Version)
		return m, cmd
	}
	return m, nil
}

// handleOpenBuildDir opens the build directory for a specific version
func (m *Model) handleOpenBuildDir() (tea.Model, tea.Cmd) {
	selectedBuild := m.List.GetSelectedBuild()
	if selectedBuild == nil {
		return m, nil
	}

	// Only open dir if it's a local build or has an update available
	if selectedBuild.Status == model.StateLocal || selectedBuild.Status == model.StateUpdate {
		// Create a command that locates the correct build directory by version
		return m, func() tea.Msg {
			entries, err := os.ReadDir(m.config.DownloadDir)
			if err != nil {
				return errMsg{fmt.Errorf("failed to read download directory %s: %w", m.config.DownloadDir, err)}
			}

			version := selectedBuild.Version
			for _, entry := range entries {
				if entry.IsDir() && entry.Name() != download.DownloadingDir && entry.Name() != download.OldBuildsDir {
					dirPath := filepath.Join(m.config.DownloadDir, entry.Name())
					buildInfo, err := local.ReadBuildInfo(dirPath)
					if err != nil {
						// Error reading build info, but continue checking other directories
						continue
					}

					// Check if this is the build we want to open
					if buildInfo != nil && buildInfo.Version == version {
						// Open this directory
						if err := local.OpenFileExplorer(dirPath); err != nil {
							return errMsg{fmt.Errorf("failed to open directory: %w", err)}
						}
						return nil // Success
					}
				}
			}

			return errMsg{fmt.Errorf("build directory for Blender version %s not found", version)}
		}
	}
	return m, nil
}

// handleStartDownload initiates a download for the selected build (from key press)
func (m *Model) handleStartDownload() (tea.Model, tea.Cmd) {
	selectedBuild := m.List.GetSelectedBuild()
	if selectedBuild == nil {
		return m, nil
	}

	// Allow downloading Online, Update, Failed, and Cancelled builds
	if selectedBuild.Status == model.StateOnline ||
		selectedBuild.Status == model.StateUpdate ||
		selectedBuild.Status == model.StateFailed ||
		selectedBuild.Status == model.StateCancelled { // StateNone == Cancelled

		return m, func() tea.Msg {
			return startDownloadMsg{build: *selectedBuild}
		}
	}
	return m, nil
}

// handleStartDownloadMsg handles the actual start message
func (m *Model) handleStartDownloadMsg(msg startDownloadMsg) (tea.Model, tea.Cmd) {
	m.Progress.ActiveDownloadID = msg.buildID

	// Update the build status immediately to show downloading
	for i := range m.List.Builds {
		if m.List.Builds[i].Version == msg.build.Version {
			m.List.Builds[i].Status = model.StateDownloading
			break
		}
	}

	var cmds []tea.Cmd
	// Create a Commands instance and call DoDownload directly
	cmds = append(cmds, m.commands.DoDownload(msg.build))

	// Make sure the ticker is running with a faster initial tick for responsiveness
	cmds = append(cmds, tea.Tick(time.Millisecond*10, func(t time.Time) tea.Msg {
		return tickMsg(t)
	}))

	return m, tea.Batch(cmds...)
}

// handleCancelDownload cancels an active download
func (m *Model) handleCancelDownload() (tea.Model, tea.Cmd) {
	selectedBuild := m.List.GetSelectedBuild()
	if selectedBuild == nil {
		return m, nil
	}

	selectedBuildID := selectedBuild.Version
	if selectedBuild.Hash != "" {
		selectedBuildID = selectedBuild.Version + "-" + selectedBuild.Hash[:8]
	}

	// Use activeDownloadID if set; otherwise, use the selected build ID
	buildID := m.Progress.ActiveDownloadID
	if buildID == "" {
		buildID = selectedBuildID
	}

	// Cancel the download using the download manager
	m.commands.downloads.CancelDownload(buildID)

	// Update the build status to Cancelled (StateNone) after cancellation
	for i, build := range m.List.Builds {
		bID := build.Version
		if build.Hash != "" {
			bID = build.Version + "-" + build.Hash[:8]
		}

		// Update the status of both the selected build and any build matching the active download
		if bID == m.Progress.ActiveDownloadID || bID == selectedBuildID {
			// Only update if it's in a downloading or extracting state
			if m.List.Builds[i].Status == model.StateDownloading ||
				m.List.Builds[i].Status == model.StateExtracting {
				m.List.Builds[i].Status = model.StateCancelled // Set to Cancelled
			}
		}
	}

	// Clear the active download ID
	m.Progress.ActiveDownloadID = ""

	return m, nil
}

// handleDeleteBuild prepares to delete a build
func (m *Model) handleDeleteBuild() (tea.Model, tea.Cmd) {
	selectedBuild := m.List.GetSelectedBuild()
	if selectedBuild == nil {
		return m, nil
	}

	if selectedBuild.Status == model.StateDownloading || selectedBuild.Status == model.StateExtracting {
		return m.handleCancelDownload()
	}
	// Only allow deleting local builds or builds that can be updated
	if selectedBuild.Status == model.StateLocal || selectedBuild.Status == model.StateUpdate {
		return m, func() tea.Msg {
			success, err := local.DeleteBuild(m.config.DownloadDir, selectedBuild.Version)
			if err != nil {
				return errMsg{err}
			}
			if !success {
				return errMsg{fmt.Errorf("failed to delete build %s", selectedBuild.Version)}
			}
			// Remove the deleted build from the list
			// ... (requires manipulating ListModel, maybe better to just rescan)
			return m.commands.ScanLocalBuilds()()
		}
	}
	return m, nil
}

// handleLocalBuildsScanned processes the result of scanning local builds
func (m *Model) handleLocalBuildsScanned(msg localBuildsScannedMsg) (tea.Model, tea.Cmd) {
	// If there was an error scanning builds, store it but continue with empty list
	if msg.err != nil {
		m.err = msg.err
		m.List.Builds = []model.BlenderBuild{}
		return m, nil
	}

	// Set builds to local builds only
	m.List.Builds = msg.builds

	// Apply version filter if set
	if m.config.VersionFilter != "" {
		m.List.Builds = m.applyVersionFilter(m.List.Builds)
	}

	// Sort builds immediately
	m.List.SortBuilds()

	// Reset cursor and startIndex
	if len(m.List.Builds) > 0 {
		m.List.Cursor = 0
		m.List.StartIndex = 0
	}

	return m, nil
}

// handleBuildsFetched processes the result of fetching builds from the API
func (m *Model) handleBuildsFetched(msg buildsFetchedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		return m, nil
	}

	// Preserve only local builds from the current list.
	var localBuilds []model.BlenderBuild
	for _, build := range m.List.Builds {
		if build.Status == model.StateLocal {
			localBuilds = append(localBuilds, build)
		}
	}

	// Start with local builds + newly fetched builds.
	m.List.Builds = localBuilds
	m.List.Builds = append(m.List.Builds, msg.builds...)

	// Apply version filter if set *before* updating status
	if m.config.VersionFilter != "" {
		m.List.Builds = m.applyVersionFilter(m.List.Builds)
	}

	// Reset cursor and startIndex
	m.List.Cursor = 0
	m.List.StartIndex = 0

	// Update the status based on what's available locally vs online.
	return m, m.commands.UpdateBuildStatus(m.List.Builds)
}

// applyVersionFilter filters builds by version
func (m *Model) applyVersionFilter(builds []model.BlenderBuild) []model.BlenderBuild {
	if m.config.VersionFilter == "" {
		return builds
	}

	filtered := make([]model.BlenderBuild, 0)
	for _, build := range builds {
		// Always keep local builds regardless of version filter
		if build.Status == model.StateLocal {
			filtered = append(filtered, build)
			continue
		}

		// Compare versions
		if build.Version >= m.config.VersionFilter {
			filtered = append(filtered, build)
		}
	}
	return filtered
}

// handleBuildsUpdated finalizes the build list after determining local/online status
func (m *Model) handleBuildsUpdated(msg buildsUpdatedMsg) (tea.Model, tea.Cmd) {
	// Replace builds with updated ones that have correct status
	m.List.Builds = msg.builds

	// Sync logic... (skipped simple sync logic for brevity as it's handled in ProgressModel essentially)
	// But we need to cleanup downloadStates

	// Apply version filter if set
	if m.config.VersionFilter != "" {
		m.List.Builds = m.applyVersionFilter(m.List.Builds)
	}

	m.List.SortBuilds()
	m.List.EnsureCursorVisible()

	return m, nil
}

// handleBlenderExec handles launching Blender
func (m *Model) handleBlenderExec(msg model.BlenderExecMsg) (tea.Model, tea.Cmd) {
	execInfo := msg
	return m, func() tea.Msg {
		blenderExe := execInfo.Executable
		err := launch.BlenderInNewTerminal(blenderExe)
		if err != nil {
			return errMsg{fmt.Errorf("failed to launch Blender: %w", err)}
		}
		return nil
	}
}

// SaveSettingsAndReturn saves settings and returns to list view
func (m *Model) SaveSettingsAndReturn() (tea.Model, tea.Cmd) {
	if err := m.SaveSettings(); err != nil {
		m.err = err
		return m, nil
	}

	// Recreate commands with updated config
	m.commands = NewCommands(m.config)
	m.err = nil

	// Refresh list
	return m, m.commands.ScanLocalBuilds()
}

func (m *Model) handleDownloadCompleteMsg(msg downloadCompleteMsg) (tea.Model, tea.Cmd) {
	// Handle completion of download
	for i := range m.List.Builds {
		// Find the build by version and update its status
		if m.List.Builds[i].Version == msg.buildVersion {
			if msg.err != nil {
				// Handle download error
				m.List.Builds[i].Status = model.StateFailed
				m.err = msg.err
			} else {
				// Update to local state on success
				m.List.Builds[i].Status = model.StateLocal
				m.err = nil
			}
			break
		}
	}

	// Re-sort the builds
	m.List.SortBuilds()

	// Start listening for more program messages
	return m, m.commands.ProgramMsgListener()
}

func (m *Model) handleTickMsg(msg tickMsg) (tea.Model, tea.Cmd) {
	// Sync download states
	m.SyncDownloadStates()

	// Logic for finding next tick time
	activeDownloads := 0
	for _, state := range m.Progress.DownloadStates {
		if state.BuildState == model.StateDownloading || state.BuildState == model.StateExtracting {
			activeDownloads++
		}
	}

	var nextTickTime time.Duration = time.Millisecond * 500
	if activeDownloads > 0 {
		nextTickTime = time.Millisecond * 250
	}

	cmd := tea.Tick(nextTickTime, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})

	// Also perform the logic of handleDownloadProgress to update statuses in the List
	// We can extract that to a helper
	m.updateBuildsStatusFromProgress()

	return m, cmd
}

func (m *Model) updateBuildsStatusFromProgress() {
	// This logic mimics handleDownloadProgress from original code
	// updating m.List.Builds[i].Status based on m.Progress.DownloadStates

	for i := range m.List.Builds {
		buildID := m.List.Builds[i].Version
		if m.List.Builds[i].Hash != "" {
			buildID = m.List.Builds[i].Version + "-" + m.List.Builds[i].Hash[:8]
		}

		if state, ok := m.Progress.DownloadStates[buildID]; ok {
			if state.BuildState == model.StateDownloading || state.BuildState == model.StateExtracting {
				m.List.Builds[i].Status = state.BuildState
			} else if state.BuildState == model.StateLocal {
				m.List.Builds[i].Status = model.StateLocal
			} else if strings.HasPrefix(state.BuildState.String(), "Failed") {
				m.List.Builds[i].Status = model.StateFailed
			}
		}
	}
}
