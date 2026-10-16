package tui

import (
	"fmt"
	"github.com/dshot92/TUI-Blender-Launcher/launch"
	"github.com/dshot92/TUI-Blender-Launcher/local"
	"github.com/dshot92/TUI-Blender-Launcher/model"
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
			version := selectedBuild.Version
			buildInfo, dirPath, err := local.FindBuild(m.config.DownloadDir, func(b model.BlenderBuild) bool {
				return b.Version == version
			})
			if err != nil {
				return errMsg{fmt.Errorf("failed to read download directory %s: %w", m.config.DownloadDir, err)}
			}
			if buildInfo == nil {
				return errMsg{fmt.Errorf("build directory for Blender version %s not found", version)}
			}
			if err := local.OpenFileExplorer(dirPath); err != nil {
				return errMsg{fmt.Errorf("failed to open directory: %w", err)}
			}
			return nil // Success
		}
	}
	return m, nil
}

// handleOpenURL opens the selected build's download URL in the default browser.
func (m *Model) handleOpenURL() (tea.Model, tea.Cmd) {
	selectedBuild := m.List.GetSelectedBuild()
	if selectedBuild == nil || selectedBuild.DownloadURL == "" {
		return m, nil
	}
	return m, openURLCmd(selectedBuild.DownloadURL)
}

// openURLCmd opens url in the user's default browser, reporting the outcome as
// a transient toast.
func openURLCmd(url string) tea.Cmd {
	return func() tea.Msg {
		if err := local.OpenURL(url); err != nil {
			return errMsg{fmt.Errorf("failed to open download link: %w", err)}
		}
		return statusMsg{text: "opened download link"}
	}
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
		selectedBuild.Status == model.StateCancelled {

		return m, func() tea.Msg {
			return startDownloadMsg{build: *selectedBuild, buildID: selectedBuild.ID()}
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
	cmds = append(cmds, tea.Tick(initialTickDelay, func(t time.Time) tea.Msg {
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

	selectedBuildID := selectedBuild.ID()

	// Use activeDownloadID if set; otherwise, use the selected build ID
	buildID := m.Progress.ActiveDownloadID
	if buildID == "" {
		buildID = selectedBuildID
	}

	// Cancel the download using the download manager
	m.commands.downloads.CancelDownload(buildID)

	// Update the build status to Cancelled after cancellation
	for i, build := range m.List.Builds {
		bID := build.ID()

		// Update the status of both the selected build and any build matching the active download
		if bID == m.Progress.ActiveDownloadID || bID == selectedBuildID {
			// Only update if it's in a downloading or extracting state
			if m.List.Builds[i].Status == model.StateDownloading ||
				m.List.Builds[i].Status == model.StateExtracting {
				m.List.Builds[i].Status = model.StateCancelled
			}
		}
	}

	// Clear the active download ID
	m.Progress.ActiveDownloadID = ""

	return m, nil
}

// doDeleteBuild performs the (possibly cancel) action for a build.
func (m *Model) doDeleteBuild() (tea.Model, tea.Cmd) {
	selectedBuild := m.List.GetSelectedBuild()
	if selectedBuild == nil {
		return m, nil
	}

	if selectedBuild.Status == model.StateDownloading || selectedBuild.Status == model.StateExtracting {
		return m.handleCancelDownload()
	}
	// Only allow deleting local builds or builds that can be updated
	if selectedBuild.Status == model.StateLocal || selectedBuild.Status == model.StateUpdate {
		deletedHash := selectedBuild.Hash
		return m, func() tea.Msg {
			success, err := local.DeleteBuild(m.config.DownloadDir, deletedHash)
			if err != nil {
				return errMsg{err}
			}
			if !success {
				return errMsg{fmt.Errorf("failed to delete build")}
			}
			return buildDeletedMsg{hash: deletedHash}
		}
	}
	return m, nil
}

// handleBuildDeleted processes a successful build deletion
func (m *Model) handleBuildDeleted(msg buildDeletedMsg) (tea.Model, tea.Cmd) {
	m.List.RemoveBuildByHash(msg.hash)
	return m, m.commands.ScanLocalBuilds()
}

// mergeBuilds preserves local and in-progress builds from the current list and
// merges the incoming builds into them (de-duplicating by version+hash while
// preserving downloading/extracting status), then applies the version filter.
// Shared by the local-scan and online-fetch handlers.
func (m *Model) mergeBuilds(incoming []model.BlenderBuild) []model.BlenderBuild {
	// Start with preserved builds (local + in-progress).
	merged := make([]model.BlenderBuild, 0, len(m.List.Builds)+len(incoming))
	for _, build := range m.List.Builds {
		if build.Status == model.StateLocal || build.Status == model.StateDownloading || build.Status == model.StateExtracting {
			merged = append(merged, build)
		}
	}

	// Merge incoming builds, avoiding duplicates.
	for _, newBuild := range incoming {
		isDuplicate := false
		for i, existingBuild := range merged {
			if existingBuild.Version == newBuild.Version &&
				(existingBuild.Hash == newBuild.Hash || (existingBuild.Hash == "" && newBuild.Hash == "")) {
				isDuplicate = true
				// Preserve downloading/extracting status across the merge.
				if existingBuild.Status == model.StateDownloading || existingBuild.Status == model.StateExtracting {
					newBuild.Status = existingBuild.Status
				}
				merged[i] = newBuild
				break
			}
		}
		if !isDuplicate {
			merged = append(merged, newBuild)
		}
	}

	// Apply version and release-type filters if set.
	return m.applyFilters(merged)
}

// handleLocalBuildsScanned processes the result of scanning local builds
func (m *Model) handleLocalBuildsScanned(msg localBuildsScannedMsg) (tea.Model, tea.Cmd) {
	// If there was an error scanning builds, store it but continue with empty list
	if msg.err != nil {
		m.err = msg.err
		m.List.Builds = []model.BlenderBuild{}
		return m, nil
	}

	m.List.Builds = m.mergeBuilds(msg.builds)

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

	m.List.Builds = m.mergeBuilds(msg.builds)

	// Reset cursor and startIndex
	m.List.Cursor = 0
	m.List.StartIndex = 0

	// Update the status based on what's available locally vs online.
	return m, m.commands.UpdateBuildStatus(m.List.Builds)
}

// applyVersionFilter filters builds by version using the shared semantic
// version filter (so e.g. "4.10" is correctly treated as newer than "4.9").
// Local and in-progress builds are always kept. An empty or invalid filter is a
// no-op so a bad value never hides every build.
func (m *Model) applyVersionFilter(builds []model.BlenderBuild) []model.BlenderBuild {
	vf, err := model.ParseVersionFilter(m.config.VersionFilter)
	if err != nil {
		return builds
	}
	return vf.Apply(builds)
}

// applyFilters applies the configured version and release-type filters to a
// build list. Local and in-progress builds are always kept.
func (m *Model) applyFilters(builds []model.BlenderBuild) []model.BlenderBuild {
	if m.config.VersionFilter != "" {
		builds = m.applyVersionFilter(builds)
	}
	if f := model.ParseReleaseTypes(m.config.ReleaseTypes); len(f) > 0 {
		builds = f.Apply(builds)
	}
	return builds
}

// handleBuildsUpdated finalizes the build list after determining local/online status
func (m *Model) handleBuildsUpdated(msg buildsUpdatedMsg) (tea.Model, tea.Cmd) {
	// Create a map of downloading/extracting builds from the current list to restore later
	downloadStates := make(map[string]model.BuildState)
	for _, build := range m.List.Builds {
		if build.Status == model.StateDownloading || build.Status == model.StateExtracting {
			downloadStates[build.ID()] = build.Status
		}
	}

	// Use msg.builds as the authoritative source (they have correct Local/Online/Update status)
	m.List.Builds = msg.builds

	// Restore downloading/extracting statuses by matching version/hash
	for i := range m.List.Builds {
		if status, exists := downloadStates[m.List.Builds[i].ID()]; exists {
			m.List.Builds[i].Status = status
		}
	}

	// Apply the configured filters
	m.List.Builds = m.applyFilters(m.List.Builds)

	m.List.SortBuilds()
	m.List.EnsureCursorVisible()

	return m, nil
}

// handleBlenderExec handles launching Blender
func (m *Model) handleBlenderExec(msg model.BlenderExecMsg) (tea.Model, tea.Cmd) {
	return m, func() tea.Msg {
		err := launch.BlenderInNewTerminal(msg.Executable)
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

	// Update the existing commands in place so in-flight download state is
	// preserved. Recreating Commands would build a new DownloadManager and
	// discard any active download progress.
	if m.commands != nil {
		m.commands.SetConfig(m.config)
	} else {
		m.commands = NewCommands(m.config)
	}
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

// executeFooterCommand runs the action bound to a clicked/detected footer hint.
func (m *Model) executeFooterCommand(cmd CommandType) (tea.Model, tea.Cmd) {
	switch cmd {
	case CmdQuit:
		return m, tea.Quit
	case CmdShowSettings:
		m.currentView = viewSettings
		m.Settings.SetValues(SettingsValuesFromConfig(m.config))
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
	case CmdOpenURL:
		return m.handleOpenURL()
	case CmdToggleDetail:
		m.detailShown = !m.detailShown
		m.computeLayout()
		if m.detailShown && !m.layout.detailVisible {
			return m, func() tea.Msg {
				return statusMsg{text: "not enough space for the info panel"}
			}
		}
		return m, nil
	case CmdToggleSortOrder:
		m.List.SortReversed = !m.List.SortReversed
		m.List.SortBuilds()
		m.List.EnsureCursorVisible()
		return m, nil
	case CmdCleanOldBuilds:
		return m, m.cleanOldBuildsCmd()
	case CmdDeleteBuild:
		build := m.List.GetSelectedBuild()
		if build == nil {
			return m, nil
		}
		if !m.isDownloading(*build) && build.Status != model.StateLocal && build.Status != model.StateUpdate {
			return m, nil
		}
		m.beginDeleteConfirm(*build)
		return m, nil
	default:
		return m, nil
	}
}

// cleanOldBuildsCmd removes archived builds and reports the outcome as a toast.
func (m *Model) cleanOldBuildsCmd() tea.Cmd {
	return func() tea.Msg {
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

// beginDeleteConfirm opens the delete/cancel confirmation popup for build,
// defaulting to "No". The caller is responsible for checking that the build may
// be acted on.
func (m *Model) beginDeleteConfirm(build model.BlenderBuild) {
	m.deleteConfirm = true
	m.deleteConfirmBuild = build
	m.deleteConfirmCancel = m.isDownloading(build)
	m.deleteConfirmChoice = confirmChoiceNo
}

// isDownloading reports whether build is mid-transfer. The build's own status is
// authoritative, but a live transfer owned by the download manager also counts
// even before the next status sync.
func (m *Model) isDownloading(build model.BlenderBuild) bool {
	if build.Status == model.StateDownloading || build.Status == model.StateExtracting {
		return true
	}
	if state := m.stateFor(build.ID()); state != nil &&
		(state.BuildState == model.StateDownloading || state.BuildState == model.StateExtracting) {
		return true
	}
	return false
}

// executeSettingsCommand runs the action bound to a clicked settings hint.
func (m *Model) executeSettingsCommand(cmd CommandType) (tea.Model, tea.Cmd) {
	switch cmd {
	case CmdQuit:
		if m.Settings.EditMode || m.Settings.releaseEditing {
			return m, nil
		}
		return m, tea.Quit
	case CmdSaveSettings:
		if m.Settings.EditMode || m.Settings.releaseEditing {
			return m, nil
		}
		wasInitial := m.currentView == viewInitialSetup
		m.currentView = viewList
		mdl, c := m.SaveSettingsAndReturn()
		if wasInitial {
			statusCmd := func() tea.Msg {
				return statusMsg{text: "Press f to fetch builds"}
			}
			return mdl, tea.Batch(c, statusCmd)
		}
		return mdl, c
	case CmdToggleEditMode:
		_, c := m.Settings.handleToggleEdit()
		return m, c
	case CmdMoveLeft:
		if !m.Settings.EditMode {
			m.Settings.cycleOption(-1)
		}
		return m, nil
	case CmdCleanOldBuilds:
		if m.Settings.EditMode || m.Settings.releaseEditing {
			return m, nil
		}
		return m, m.cleanOldBuildsCmd()
	default:
		return m, nil
	}
}

// contextMenuItemsFor returns the action list applicable to build.
func (m *Model) contextMenuItemsFor(build model.BlenderBuild) []contextMenuItem {
	items := []contextMenuItem{}
	status := build.Status
	switch {
	case m.isDownloading(build):
		items = append(items, contextMenuItem{label: "Cancel download", cmd: CmdDeleteBuild})
	case status == model.StateLocal:
		items = append(items, contextMenuItem{label: "Launch", cmd: CmdLaunchBuild})
		items = append(items, contextMenuItem{label: "Open directory", cmd: CmdOpenBuildDir})
		items = append(items, contextMenuItem{label: "Delete", cmd: CmdDeleteBuild})
	case status == model.StateUpdate:
		items = append(items, contextMenuItem{label: "Download", cmd: CmdDownloadBuild})
		items = append(items, contextMenuItem{label: "Launch", cmd: CmdLaunchBuild})
		items = append(items, contextMenuItem{label: "Open directory", cmd: CmdOpenBuildDir})
		items = append(items, contextMenuItem{label: "Delete", cmd: CmdDeleteBuild})
	default:
		items = append(items, contextMenuItem{label: "Download", cmd: CmdDownloadBuild})
	}
	if build.DownloadURL != "" && status != model.StateLocal {
		items = append(items, contextMenuItem{label: "Open URL", cmd: CmdOpenURL})
	}
	if len(items) == 0 {
		items = append(items, contextMenuItem{label: "No actions", cmd: CmdNone})
	}
	return items
}

func (m *Model) openContextMenu(buildIdx, x, y int) {
	if buildIdx < 0 || buildIdx >= len(m.List.Builds) {
		return
	}
	m.contextMenuVisible = true
	m.contextMenuBuildIdx = buildIdx
	m.contextMenuX = x
	m.contextMenuY = y
	m.contextMenuChoice = 0
	m.contextMenuItems = m.contextMenuItemsFor(m.List.Builds[buildIdx])
}

func (m *Model) closeContextMenu() {
	m.contextMenuVisible = false
	m.contextMenuItems = nil
}

func (m *Model) contextMenuSize() (int, int) {
	maxLabel := 0
	for _, it := range m.contextMenuItems {
		l := len([]rune(it.label))
		if l > maxLabel {
			maxLabel = l
		}
	}
	w := maxLabel + contextMenuPadding
	if w < minContextMenuWidth {
		w = minContextMenuWidth
	}
	h := len(m.contextMenuItems) + boxBorderFrameHeight
	if h < boxBorderFrameHeight+1 {
		h = boxBorderFrameHeight + 1
	}
	return w, h
}

func (m *Model) contextMenuOrigin() (int, int) {
	w, h := m.contextMenuSize()
	x := m.contextMenuX
	y := m.contextMenuY
	if x+w > m.terminalWidth {
		x = m.terminalWidth - w
	}
	if x < 0 {
		x = 0
	}
	if y+h > m.terminalHeight {
		y = m.terminalHeight - h
	}
	if y < 0 {
		y = 0
	}
	return x, y
}

func (m *Model) contextMenuItemAt(x, y int) int {
	if !m.contextMenuVisible {
		return -1
	}
	w, h := m.contextMenuSize()
	ox, oy := m.contextMenuOrigin()
	if x < ox+1 || x >= ox+w-1 {
		return -1
	}
	if y < oy+1 || y >= oy+h-1 {
		return -1
	}
	idx := y - (oy + 1)
	if idx < 0 || idx >= len(m.contextMenuItems) {
		return -1
	}
	return idx
}

func (m *Model) executeContextMenuItem() (tea.Model, tea.Cmd) {
	if !m.contextMenuVisible || len(m.contextMenuItems) == 0 {
		m.closeContextMenu()
		return m, nil
	}
	if m.contextMenuChoice < 0 || m.contextMenuChoice >= len(m.contextMenuItems) {
		m.closeContextMenu()
		return m, nil
	}
	item := m.contextMenuItems[m.contextMenuChoice]
	// select the underlying build first
	if m.contextMenuBuildIdx >= 0 && m.contextMenuBuildIdx < len(m.List.Builds) {
		m.List.Cursor = m.contextMenuBuildIdx
		m.List.EnsureCursorVisible()
	}
	m.closeContextMenu()

	switch item.cmd {
	case CmdDownloadBuild:
		return m.handleStartDownload()
	case CmdLaunchBuild:
		return m.handleLaunchBlender()
	case CmdOpenBuildDir:
		return m.handleOpenBuildDir()
	case CmdDeleteBuild:
		if build := m.List.GetSelectedBuild(); build != nil {
			m.beginDeleteConfirm(*build)
		}
		return m, nil
	case CmdOpenURL:
		return m.handleOpenURL()
	default:
		return m, nil
	}
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

	var nextTickTime time.Duration = tickIntervalIdle
	if activeDownloads > 0 {
		nextTickTime = tickIntervalActive
	}

	cmd := tea.Tick(nextTickTime, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})

	m.updateBuildsStatusFromProgress()

	return m, cmd
}

func (m *Model) updateBuildsStatusFromProgress() {
	for i := range m.List.Builds {
		buildID := m.List.Builds[i].ID()

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
