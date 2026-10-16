package tui

import (
	"TUI-Blender-Launcher/api"
	"TUI-Blender-Launcher/config"
	"TUI-Blender-Launcher/download"
	"TUI-Blender-Launcher/local"
	"TUI-Blender-Launcher/model"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// DownloadManager handles all download operations with thread-safe state access
type DownloadManager struct {
	states map[string]*model.DownloadState
	cfg    config.Config
	mu     sync.Mutex
}

// NewDownloadManager creates a new download manager
func NewDownloadManager(cfg config.Config) *DownloadManager {
	return &DownloadManager{
		states: make(map[string]*model.DownloadState),
		cfg:    cfg,
	}
}

// GetState safely retrieves a snapshot of state for a build
func (dm *DownloadManager) GetState(buildID string) *model.DownloadState {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	return dm.states[buildID].Clone()
}

// GetAllStates returns a snapshot copy of all download states
func (dm *DownloadManager) GetAllStates() map[string]*model.DownloadState {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	result := make(map[string]*model.DownloadState, len(dm.states))
	for k, v := range dm.states {
		result[k] = v.Clone()
	}
	return result
}

// StartDownload begins a new download for a build
func (dm *DownloadManager) StartDownload(build model.BlenderBuild) tea.Msg {
	// Create a unique build ID
	buildID := build.ID()

	// Lock access to states map to prevent race conditions
	dm.mu.Lock()

	// Clean up previous state if it was Failed or Cancelled before starting anew
	if state, exists := dm.states[buildID]; exists {
		if state.BuildState == model.StateFailed || state.BuildState == model.StateCancelled {
			// Remove the old failed/cancelled state to allow restart
			delete(dm.states, buildID)
		} else if state.BuildState == model.StateDownloading || state.BuildState == model.StateExtracting {
			// If already downloading/extracting this exact build, don't start another one
			dm.mu.Unlock()
			return nil
		}
	}

	// Setup download state
	now := time.Now()
	cancelCh := make(chan struct{})
	dm.states[buildID] = &model.DownloadState{
		BuildID:     buildID,
		BuildState:  model.StateDownloading,
		StartTime:   now,
		LastUpdated: now,
		Progress:    0.0,
		CancelCh:    cancelCh,
	}

	// Create a temporary directory for downloads if it doesn't exist
	downloadTempDir := filepath.Join(dm.cfg.DownloadDir, download.DownloadingDir)
	if err := os.MkdirAll(downloadTempDir, 0750); err != nil {
		dm.states[buildID].BuildState = model.StateFailed
		dm.mu.Unlock()
		programCh <- downloadCompleteMsg{
			buildVersion: build.Version,
			err:          fmt.Errorf("failed to create download directory: %w", err),
		}
		return nil
	}
	dm.mu.Unlock()

	// Download and extract in a single background path.
	go func() {
		// updateSnapshot applies a state transition under the manager lock.
		updateSnapshot := func(fn func(*model.DownloadState)) {
			dm.mu.Lock()
			defer dm.mu.Unlock()
			state := dm.states[buildID]
			if state == nil {
				return
			}
			// Skip UI updates once cancellation has been requested.
			select {
			case <-cancelCh:
				return
			default:
			}
			fn(state)
		}

		downloadCb := func(downloaded, total int64, speed float64) {
			if total <= 0 {
				return
			}
			updateSnapshot(func(state *model.DownloadState) {
				state.BuildState = model.StateDownloading
				state.LastUpdated = time.Now()
				state.Progress = float64(downloaded) / float64(total)
				state.Current = downloaded
				state.Total = total
				state.Speed = speed
			})
		}

		extractCb := func(progress float64) {
			updateSnapshot(func(state *model.DownloadState) {
				state.BuildState = model.StateExtracting
				state.LastUpdated = time.Now()
				state.Progress = progress
				state.Speed = 0
			})
		}

		extractedPath, err := download.DownloadAndExtractBuild(build, dm.cfg.DownloadDir, downloadCb, extractCb, cancelCh)

		// Final state transition.
		dm.mu.Lock()
		state := dm.states[buildID]
		if state == nil {
			dm.mu.Unlock()
			return
		}
		state.Speed = 0
		if err != nil {
			if errors.Is(err, download.ErrCancelled) {
				state.BuildState = model.StateCancelled
			} else {
				state.BuildState = model.StateFailed
				state.Progress = 0.0
			}
		} else {
			state.BuildState = model.StateLocal
			state.Progress = 1.0
		}
		dm.mu.Unlock()

		programCh <- downloadCompleteMsg{
			buildVersion:  build.Version,
			extractedPath: extractedPath,
			err:           err,
		}
	}()

	return nil
}

// PruneInactiveStates removes terminal (failed/cancelled) download states,
// keeping only those that are actively downloading or extracting.
func (dm *DownloadManager) PruneInactiveStates() {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	newStates := make(map[string]*model.DownloadState, len(dm.states))
	for id, state := range dm.states {
		if state.BuildState == model.StateDownloading || state.BuildState == model.StateExtracting {
			newStates[id] = state
		}
	}
	dm.states = newStates
}

// CancelDownload stops an in-progress download. Safe to call multiple times.
func (dm *DownloadManager) CancelDownload(buildID string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	state := dm.states[buildID]
	if state == nil {
		return
	}
	// Guard against a double close of the cancel channel.
	if state.BuildState == model.StateCancelled {
		return
	}

	close(state.CancelCh)
	state.BuildState = model.StateCancelled
	state.Progress = 0.0 // Reset progress

	// Don't delete the state so we can track that it was cancelled
	// Keep it so it can be displayed with "Cancelled" status
}

// Commands generates tea commands for the TUI
type Commands struct {
	cfg       config.Config
	downloads *DownloadManager
}

// NewCommands creates a new Commands instance
func NewCommands(cfg config.Config) *Commands {
	return &Commands{
		cfg:       cfg,
		downloads: NewDownloadManager(cfg),
	}
}

// FetchBuilds fetches the list of builds from the API.
func (c *Commands) FetchBuilds() tea.Cmd {
	return func() tea.Msg {
		// Clean up download states, keeping only active ones.
		c.downloads.PruneInactiveStates()

		// Create API instance
		a := api.NewAPI()
		builds, err := a.FetchBuilds(c.cfg.VersionFilter, c.cfg.BuildType)
		return buildsFetchedMsg{builds, err}
	}
}

// ScanLocalBuilds creates a command to scan for local builds
func (c *Commands) ScanLocalBuilds() tea.Cmd {
	return func() tea.Msg {
		builds, err := local.ScanLocalBuilds(c.cfg.DownloadDir)
		return localBuildsScannedMsg{builds: builds, err: err}
	}
}

// CheckUpdateAvailable determines if an update is available for a local build by comparing build dates, branch, and release_cycle.
func CheckUpdateAvailable(localBuild, onlineBuild model.BlenderBuild) model.BuildState {
	// If online build hash is present and matches local build hash, treat as identical (no update)
	if onlineBuild.Hash != "" && onlineBuild.Hash == localBuild.Hash {
		return model.StateLocal
	}

	// Ensure version, branch, and release_cycle all match; if not, treat as no local match
	if localBuild.Version != onlineBuild.Version || localBuild.Branch != onlineBuild.Branch || localBuild.ReleaseCycle != onlineBuild.ReleaseCycle {
		return model.StateOnline
	}

	// If local build date is not set, assume update is available
	if localBuild.BuildDate.Time().IsZero() {
		return model.StateUpdate
	}
	if onlineBuild.BuildDate.Time().IsZero() {
		return model.StateOnline
	}

	if onlineBuild.BuildDate.Time().After(localBuild.BuildDate.Time()) {
		return model.StateUpdate
	}
	return model.StateLocal
}

// UpdateBuildStatus creates a command to update status of builds based on local scan
func (c *Commands) UpdateBuildStatus(onlineBuilds []model.BlenderBuild) tea.Cmd {
	return func() tea.Msg {
		localBuilds, err := local.ScanLocalBuilds(c.cfg.DownloadDir)
		if err != nil {
			return errMsg{fmt.Errorf("failed local scan during status update: %w", err)}
		}

		// Create maps for quick lookup by version and hash
		localBuildMap := make(map[string]model.BlenderBuild)
		localBuildHashMap := make(map[string]model.BlenderBuild)
		for _, build := range localBuilds {
			localBuildMap[build.Version] = build
			if build.Hash != "" {
				localBuildHashMap[build.Hash] = build
			}
		}

		// Group online builds by composite key: version|branch|releaseCycle
		grouped := make(map[string]model.BlenderBuild)
		for _, onlineBuild := range onlineBuilds {
			var localBuild *model.BlenderBuild
			status := model.StateOnline

			// First try to find exact match by hash
			if onlineBuild.Hash != "" {
				if lb, found := localBuildHashMap[onlineBuild.Hash]; found {
					localBuild = &lb
					status = model.StateLocal
				}
			}

			// If no exact hash match, check for version match and update status
			if localBuild == nil {
				if lb, found := localBuildMap[onlineBuild.Version]; found {
					localBuild = &lb
					status = CheckUpdateAvailable(*localBuild, onlineBuild)
				}
			}

			updated := onlineBuild
			updated.Status = status

			// Composite key: version|branch|releaseCycle
			key := onlineBuild.Version + "|" + onlineBuild.Branch + "|" + onlineBuild.ReleaseCycle

			// If an entry already exists, prefer the one with StateUpdate over StateLocal
			if existing, exists := grouped[key]; exists {
				if existing.Status == model.StateUpdate || status == model.StateUpdate {
					grouped[key] = updated
				}
			} else {
				grouped[key] = updated
			}
		}

		// Build final list
		finalBuilds := make([]model.BlenderBuild, 0, len(grouped))
		for _, b := range grouped {
			finalBuilds = append(finalBuilds, b)
		}

		return buildsUpdatedMsg{builds: finalBuilds}
	}
}

// DoDownload creates a command to download and extract a build
func (c *Commands) DoDownload(build model.BlenderBuild) tea.Cmd {
	return func() tea.Msg {
		return c.downloads.StartDownload(build)
	}
}

// StartTicker starts the initial tick to trigger UI updates
func (c *Commands) StartTicker() tea.Cmd {
	return func() tea.Msg {
		return tickMsg(time.Now())
	}
}

// Global channel for program messages - kept for compatibility
var programCh = make(chan tea.Msg)

// ProgramMsgListener returns a command that listens for program messages
func (c *Commands) ProgramMsgListener() tea.Cmd {
	return func() tea.Msg {
		return <-programCh
	}
}

// UIRefresh creates a command that forces a UI refresh
func (c *Commands) UIRefresh() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(time.Millisecond * 200)
		return forceRenderMsg{}
	}
}
