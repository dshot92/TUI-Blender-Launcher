package tui

import (
	"TUI-Blender-Launcher/api" // Import the api package
	"TUI-Blender-Launcher/config"
	"TUI-Blender-Launcher/download" // Import download package
	"TUI-Blender-Launcher/local"    // Import local package
	"TUI-Blender-Launcher/model"    // Import the model package
	"TUI-Blender-Launcher/util"     // Import util package
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings" // Import strings
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput" // Import textinput
	tea "github.com/charmbracelet/bubbletea"
	lp "github.com/charmbracelet/lipgloss" // Import lipgloss
	"github.com/mattn/go-runewidth"        // Import runewidth
)

// View states
type viewState int

const (
	viewList viewState = iota
	viewInitialSetup
	viewSettings
	viewDeleteConfirm  // New state for delete confirmation
	viewCleanupConfirm // Confirmation for cleaning up old builds
)

// Define messages for communication between components
type buildsFetchedMsg struct { // Online builds fetched
	builds []model.BlenderBuild
}
type localBuildsScannedMsg struct { // Initial local scan complete
	builds []model.BlenderBuild
	err    error // Include error from scanning
}
type buildsUpdatedMsg struct { // Builds list updated (e.g., status change)
	builds []model.BlenderBuild
}
type startDownloadMsg struct { // Request to start download for a build
	build model.BlenderBuild
}
type downloadCompleteMsg struct { // Download & extraction finished
	buildVersion  string // Version of the build that finished
	extractedPath string
	err           error
}
type oldBuildsInfo struct { // Information about old builds
	count int
	size  int64
	err   error
}
type cleanupOldBuildsMsg struct { // Result of cleaning up old builds
	err error
}
type errMsg struct{ err error }
type downloadProgressMsg struct { // Reports download progress
	BuildVersion string // Identifier for the build being downloaded
	CurrentBytes int64
	TotalBytes   int64
	Percent      float64 // Calculated percentage 0.0 to 1.0
	Speed        float64 // Bytes per second
}

// tickMsg tells the TUI to check for download progress updates
type tickMsg time.Time

// Implement the error interface for errMsg
func (e errMsg) Error() string { return e.err.Error() }

// Model represents the state of the TUI application.
type Model struct {
	// Core data
	builds []model.BlenderBuild
	config config.Config
	// programRef *tea.Program // Ensure this is removed or commented out

	// UI state
	cursor         int
	isLoading      bool
	downloadStates map[string]*DownloadState // Map version to download state
	downloadMutex  sync.Mutex                // Mutex for downloadStates
	cancelDownloads chan struct{}            // Channel to signal download cancellation
	err            error
	currentView    viewState
	progressBar    progress.Model // Progress bar component
	buildToDelete  string         // Store version of build to delete for confirmation
	blenderRunning string         // Version of Blender currently running, empty if none
	
	// Old builds information
	oldBuildsCount int  // Number of old builds
	oldBuildsSize  int64 // Size of old builds in bytes
	
	// Sorting state
	sortColumn    int // Which column index is being sorted
	sortReversed  bool // Whether sorting is reversed
	
	// Settings/Setup specific state
	settingsInputs []textinput.Model
	focusIndex     int
	editMode       bool // Whether we're in edit mode in settings
	terminalWidth  int // Store terminal width
}

// DownloadState holds progress info for an active download
type DownloadState struct {
	Progress float64 // 0.0 to 1.0
	Current  int64
	Total    int64
	Speed    float64 // Bytes per second
	Message  string  // e.g., "Preparing...", "Downloading...", "Extracting...", "Local", "Failed: ..."
}

// Styles using lipgloss
var (
	// Using default terminal colors
	headerStyle = lp.NewStyle().Bold(true).Padding(0, 1)
	// Style for the selected row
	selectedRowStyle = lp.NewStyle().Background(lp.Color("240")).Foreground(lp.Color("255"))
	// Style for regular rows (use default)
	regularRowStyle = lp.NewStyle()
	// Footer style
	footerStyle = lp.NewStyle().MarginTop(1).Faint(true)
	// Separator style (using box characters)
	separator = lp.NewStyle().SetString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━").Faint(true).String()

	// Column Widths (adjust as needed)
	colWidthSelect  = 0 // Removed selection column
	colWidthVersion = 18
	colWidthStatus  = 18
	colWidthBranch  = 12
	colWidthType    = 18 // Release Cycle
	colWidthHash    = 15
	colWidthSize    = 12
	colWidthDate    = 20 // YYYY-MM-DD HH:MM

	// Define base styles for columns (can be customized further)
	cellStyleCenter = lp.NewStyle().Align(lp.Center)
	cellStyleRight  = lp.NewStyle().Align(lp.Right)
	cellStyleLeft   = lp.NewStyle() // Default
)

// InitialModel creates the initial state of the TUI model.
func InitialModel(cfg config.Config, needsSetup bool) Model {
	// Use a green gradient for the progress bar
	progModel := progress.New(
		progress.WithDefaultGradient(),
		progress.WithGradient("#00FF00", "#008800"), // Green gradient
	)
	m := Model{
		config:         cfg,
		isLoading:      !needsSetup,
		downloadStates: make(map[string]*DownloadState),
		progressBar:    progModel,
		cancelDownloads: make(chan struct{}),
		sortColumn:     0, // Default sort by Version
		sortReversed:   true, // Default descending sort (newest versions first)
		blenderRunning: "", // No Blender running initially
		editMode:       false, // Start in navigation mode, not edit mode
	}

	if needsSetup {
		m.currentView = viewInitialSetup
		m.settingsInputs = make([]textinput.Model, 2)

		var t textinput.Model
		// Download Dir input
		t = textinput.New()
		t.Placeholder = cfg.DownloadDir // Show default as placeholder
		t.SetValue(cfg.DownloadDir)     // Set initial value
		t.Focus()
		t.CharLimit = 256
		t.Width = 50
		m.settingsInputs[0] = t

		// Version Filter input (renamed from Cutoff)
		t = textinput.New()
		t.Placeholder = "e.g., 4.0, 3.6 (leave empty for none)"
		t.SetValue(cfg.VersionFilter)
		t.CharLimit = 10
		t.Width = 50
		m.settingsInputs[1] = t

		m.focusIndex = 0 // Start focus on the first input
	} else {
		m.currentView = viewList
		// Start loading local builds immediately
		m.isLoading = true
		// Trigger initial local scan via Init command
	}

	return m
}

// command to fetch builds
// Now accepts the model to access config
func fetchBuildsCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		// Pass config (specifically VersionFilter) to FetchBuilds
		builds, err := api.FetchBuilds(cfg.VersionFilter)
		if err != nil {
			return errMsg{err}
		}
		return buildsFetchedMsg{builds}
	}
}

// Command to scan for LOCAL builds
func scanLocalBuildsCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		builds, err := local.ScanLocalBuilds(cfg.DownloadDir)
		// Return specific message for local scan results
		return localBuildsScannedMsg{builds: builds, err: err}
	}
}

// Command to re-scan local builds and update status of the provided (online) list
func updateStatusFromLocalScanCmd(onlineBuilds []model.BlenderBuild, cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		// Get all local builds - use full scan to compare hash values 
		localBuilds, err := local.ScanLocalBuilds(cfg.DownloadDir)
		if err != nil {
			// Propagate error if scanning fails
			return errMsg{fmt.Errorf("failed local scan during status update: %w", err)}
		}

		// Create a map of local builds by version for easy lookup
		localBuildMap := make(map[string]model.BlenderBuild)
		for _, build := range localBuilds {
			localBuildMap[build.Version] = build
		}

		updatedBuilds := make([]model.BlenderBuild, len(onlineBuilds))
		copy(updatedBuilds, onlineBuilds) // Work on a copy

		for i := range updatedBuilds {
			if localBuild, found := localBuildMap[updatedBuilds[i].Version]; found {
				// We found a matching version locally
				if local.CheckUpdateAvailable(localBuild, updatedBuilds[i]) {
					// Using our new function to check if update is available based on build date
					updatedBuilds[i].Status = "Update"
				} else {
					updatedBuilds[i].Status = "Local"
				}
			} else {
				updatedBuilds[i].Status = "Online" // Not installed
			}
		}
		return buildsUpdatedMsg{builds: updatedBuilds}
	}
}

// tickCmd sends a tickMsg after a short delay.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// doDownloadCmd starts the download in a goroutine which updates shared state.
func doDownloadCmd(build model.BlenderBuild, cfg config.Config, downloadMap map[string]*DownloadState, mutex *sync.Mutex, cancelCh <-chan struct{}) tea.Cmd {
	mutex.Lock()
	if _, exists := downloadMap[build.Version]; !exists {
		downloadMap[build.Version] = &DownloadState{Message: "Preparing..."}
	} else {
		mutex.Unlock()
		return nil
	}
	mutex.Unlock()

	// Create a done channel for this download
	done := make(chan struct{})

	go func() {
		// log.Printf("[Goroutine %s] Starting download...", build.Version)

		// Variables to track progress for speed calculation (persist across calls)
		var lastUpdateTime time.Time
		var lastUpdateBytes int64
		var currentSpeed float64 // Store speed between short intervals

		// Set up a cancellation handler
		go func() {
			select {
			case <-cancelCh:
				// Cancellation requested
				mutex.Lock()
				if state, ok := downloadMap[build.Version]; ok {
					state.Message = "Cancelled"
				}
				mutex.Unlock()
				// Signal this goroutine is done
				close(done)
			case <-done:
				// Normal completion, do nothing
				return
			}
		}()

		progressCallback := func(downloaded, total int64) {
			// Check for cancellation
			select {
			case <-done:
				return // Early exit if cancelled
			default:
				// Continue with progress update
			}
			
			currentTime := time.Now()
			percent := 0.0
			if total > 0 {
				percent = float64(downloaded) / float64(total)
			}

			// Calculate speed
			if !lastUpdateTime.IsZero() { // Don't calculate on the very first call
				elapsed := currentTime.Sub(lastUpdateTime).Seconds()
				// Update speed only if enough time has passed to get a meaningful value
				if elapsed > 0.2 {
					bytesSinceLast := downloaded - lastUpdateBytes
					if elapsed > 0 { // Avoid division by zero
						currentSpeed = float64(bytesSinceLast) / elapsed
					}
					lastUpdateBytes = downloaded
					lastUpdateTime = currentTime
				}
			} else {
				// First call, initialize time/bytes
				lastUpdateBytes = downloaded
				lastUpdateTime = currentTime
			}

			mutex.Lock()
			if state, ok := downloadMap[build.Version]; ok {
				// Use a virtual size threshold to detect extraction phase
				// Virtual size is 100MB for extraction as set in download.go
				const extractionVirtualSize int64 = 100 * 1024 * 1024
				
				// Check if we're getting extraction progress updates
				if total == extractionVirtualSize {
					// If we detect extraction progress based on the virtual size,
					// ensure the message is updated to "Extracting..."
					state.Message = "Extracting..."
					state.Progress = percent
					state.Speed = 0 // No download speed during extraction
				} else if state.Message == "Extracting..." {
					// During extraction phase, update progress but keep the "Extracting..." message
					state.Progress = percent
					// Don't update speed during extraction
				} else if state.Message == "Downloading..." || state.Message == "Preparing..." {
					// During download phase
					state.Progress = percent
					state.Current = downloaded
					state.Total = total
					state.Speed = currentSpeed
					state.Message = "Downloading..."
				}
			}
			mutex.Unlock()
		}

		// Call the download function with our progress callback
		_, err := download.DownloadAndExtractBuild(build, cfg.DownloadDir, progressCallback)

		// Update state to Local/Failed
		mutex.Lock()
		if state, ok := downloadMap[build.Version]; ok {
			if err != nil {
				state.Message = fmt.Sprintf("Failed: %v", err)
			} else {
				state.Message = "Local"
			}
		} // else: state might have been removed if cancelled?
		mutex.Unlock()
		
		// Signal completion
		close(done)
	}()

	return tickCmd()
}

// Init initializes the TUI model.
func (m Model) Init() tea.Cmd {
	// Store the program reference when Init is called by Bubble Tea runtime
	// This is a bit of a hack, relies on Init being called once with the Program.
	// A dedicated message might be cleaner if issues arise.
	// NOTE: This won't work as Program is not passed here. Alternative needed.
	// We'll set it in Update on the first FrameMsg instead.
	var cmds []tea.Cmd
	
	if m.currentView == viewList {
		cmds = append(cmds, scanLocalBuildsCmd(m.config))
		// Get info about old builds
		cmds = append(cmds, getOldBuildsInfoCmd(m.config))
	}
	if m.currentView == viewInitialSetup && len(m.settingsInputs) > 0 {
		cmds = append(cmds, textinput.Blink)
	}
	
	if len(cmds) > 0 {
		return tea.Batch(cmds...)
	}
	return nil
}

// Helper to update focused input
func (m *Model) updateInputs(msg tea.Msg) tea.Cmd {
	// Make sure we have inputs to update
	if len(m.settingsInputs) == 0 {
		return nil
	}
	
	var cmds []tea.Cmd
	for i := range m.settingsInputs {
		m.settingsInputs[i], cmds[i] = m.settingsInputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

// Update handles messages and updates the model state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Handle global events first for better responsiveness
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global key handler for exit (works regardless of view)
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			// Signal all downloads to cancel before quitting
			close(m.cancelDownloads)
			// Create a new channel in case we continue (this handles the case
			// where the user pressed q but we're not actually quitting yet)
			m.cancelDownloads = make(chan struct{})
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		// Handle window size globally (avoid duplicate handlers)
		m.terminalWidth = msg.Width
		m.progressBar.Width = m.terminalWidth - 4
		return m, nil
	case tea.MouseMsg:
		// Process mouse events which can help maintain focus
		return m, nil
	}

	// Now handle view-specific events
	switch m.currentView {
	case viewInitialSetup, viewSettings:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			s := msg.String()
			if m.editMode {
				// In edit mode - handle exiting edit mode and input-specific keys
				switch s {
				case "enter":
					// Toggle out of edit mode
					m.editMode = false
					// Blur the current input
					if m.focusIndex >= 0 && m.focusIndex < len(m.settingsInputs) {
						m.settingsInputs[m.focusIndex].Blur()
					}
					return m, nil
				case "esc", "escape":
					// Also exit edit mode with Escape
					m.editMode = false
					// Blur the current input
					if m.focusIndex >= 0 && m.focusIndex < len(m.settingsInputs) {
						m.settingsInputs[m.focusIndex].Blur()
					}
					return m, nil
				default:
					// Pass other keys to the focused input
					if m.focusIndex >= 0 && m.focusIndex < len(m.settingsInputs) {
						m.settingsInputs[m.focusIndex], cmd = m.settingsInputs[m.focusIndex].Update(msg)
					}
					return m, cmd
				}
			} else {
				// In navigation mode - handle navigation and entering edit mode
				switch s {
				case "h", "left":
					// Exit settings and go back to list view
					m.currentView = viewList
					return m, nil
				case "j", "down":
					// Move focus down
					oldFocus := m.focusIndex
					m.focusIndex++
					if m.focusIndex >= len(m.settingsInputs) {
						m.focusIndex = 0
					}
					updateFocusStyles(&m, oldFocus)
					return m, nil
				case "k", "up":
					// Move focus up
					oldFocus := m.focusIndex
					m.focusIndex--
					if m.focusIndex < 0 {
						m.focusIndex = len(m.settingsInputs) - 1
					}
					updateFocusStyles(&m, oldFocus)
					return m, nil
				case "tab":
					// Tab navigates between inputs
					oldFocus := m.focusIndex
					m.focusIndex++
					if m.focusIndex >= len(m.settingsInputs) {
						m.focusIndex = 0
					}
					updateFocusStyles(&m, oldFocus)
					return m, nil
				case "shift+tab":
					// Shift+Tab navigates backwards
					oldFocus := m.focusIndex
					m.focusIndex--
					if m.focusIndex < 0 {
						m.focusIndex = len(m.settingsInputs) - 1
					}
					updateFocusStyles(&m, oldFocus)
					return m, nil
				case "enter":
					// Enter edit mode
					m.editMode = true
					if m.focusIndex >= 0 && m.focusIndex < len(m.settingsInputs) {
						m.settingsInputs[m.focusIndex].Focus()
					}
					return m, textinput.Blink
				case "s", "S":
					// Save settings
					return saveSettings(m)
				}
				return m, nil
			}
		}
		
		// Only pass message to the focused input if in edit mode
		if m.editMode {
			currentFocus := m.focusIndex
			if len(m.settingsInputs) > 0 && currentFocus >= 0 && currentFocus < len(m.settingsInputs) {
				m.settingsInputs[currentFocus], cmd = m.settingsInputs[currentFocus].Update(msg)
			}
		}
		return m, cmd

	case viewList:
		switch msg := msg.(type) {
		// Handle key presses
		case tea.KeyMsg:
			switch msg.String() {
			case "up", "k":
				// Move cursor up
				if len(m.builds) > 0 {
					m.cursor--
					if m.cursor < 0 {
						m.cursor = len(m.builds) - 1
					}
				}
				return m, nil
			case "down", "j":
				// Move cursor down
				if len(m.builds) > 0 {
					m.cursor++
					if m.cursor >= len(m.builds) {
						m.cursor = 0
					}
				}
				return m, nil
			case "left", "h":
				// Move to previous column for sorting
				if m.sortColumn > 0 {
					m.sortColumn--
				} else {
					m.sortColumn = 6 // Wrap to the last column
				}
				// Re-sort the list
				m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)
				return m, nil
			case "right", "l":
				// Move to next column for sorting
				m.sortColumn++
				if m.sortColumn > 6 { // Assuming 7 columns (0 to 6)
					m.sortColumn = 0
				}
				// Re-sort the list
				m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)
				return m, nil
			case "r":
				// Toggle sort order
				m.sortReversed = !m.sortReversed
				// Re-sort the list
				m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)
				return m, nil
			case "enter":
				// Handle enter key for launching Blender
				if len(m.builds) > 0 && m.cursor < len(m.builds) {
					selectedBuild := m.builds[m.cursor]
					// Only attempt to launch if it's a local build
					if selectedBuild.Status == "Local" {
						// Add launch logic here
						log.Printf("Launching Blender %s", selectedBuild.Version)
						cmd := local.LaunchBlenderCmd(m.config.DownloadDir, selectedBuild.Version)
						return m, cmd
					}
				}
				return m, nil
			case "d", "D":
				// Start download of the selected build
				if len(m.builds) > 0 && m.cursor < len(m.builds) {
					selectedBuild := m.builds[m.cursor]
					// Allow downloading both Online builds and Updates
					if selectedBuild.Status == "Online" || selectedBuild.Status == "Update" {
						// Update status to avoid duplicate downloads
						selectedBuild.Status = "Preparing..."
						m.builds[m.cursor] = selectedBuild
						// Send message to start download
						return m, func() tea.Msg {
							return startDownloadMsg{build: selectedBuild}
						}
					}
				}
				return m, nil
			case "o", "O":
				// Open download directory
				cmd := local.OpenDownloadDirCmd(m.config.DownloadDir)
				return m, cmd
			case "s", "S":
				// Show settings
				m.currentView = viewSettings
				m.editMode = false // Ensure we start in navigation mode
				
				// Initialize settings inputs if not already done
				if len(m.settingsInputs) == 0 {
					m.settingsInputs = make([]textinput.Model, 2)
					
					// Download Dir input
					var t textinput.Model
					t = textinput.New()
					t.Placeholder = m.config.DownloadDir
					t.CharLimit = 256
					t.Width = 50
					m.settingsInputs[0] = t
					
					// Version Filter input
					t = textinput.New()
					t.Placeholder = "e.g., 4.0, 3.6 (leave empty for none)"
					t.CharLimit = 10
					t.Width = 50
					m.settingsInputs[1] = t
				}
				
				// Copy current config values
				m.settingsInputs[0].SetValue(m.config.DownloadDir)
				m.settingsInputs[1].SetValue(m.config.VersionFilter)
				
				// Focus first input (but don't focus for editing yet)
				m.focusIndex = 0
				updateFocusStyles(&m, -1)
				
				return m, nil
			case "f", "F":
				// Fetch from Builder API
				m.isLoading = true
				return m, fetchBuildsCmd(m.config)
			case "x", "X":
				// Delete a build
				if len(m.builds) > 0 && m.cursor < len(m.builds) {
					selectedBuild := m.builds[m.cursor]
					// Only allow deleting local builds
					if selectedBuild.Status == "Local" {
						m.buildToDelete = selectedBuild.Version
						m.currentView = viewDeleteConfirm
						return m, nil
					}
				}
				return m, nil
			case "c", "C":
				// Clean up old builds
				if m.oldBuildsCount > 0 {
					// Prompt for confirmation
					m.currentView = viewCleanupConfirm
					return m, nil
				}
				return m, nil
			}
		// Handle initial local scan results
		case localBuildsScannedMsg:
			m.isLoading = false
			if msg.err != nil {
				m.err = msg.err
			} else {
				m.builds = msg.builds
				// Sort the builds based on current sort settings
				m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)
				m.err = nil
			}
			// Adjust cursor if necessary
			if m.cursor >= len(m.builds) {
				m.cursor = 0
				if len(m.builds) > 0 {
					m.cursor = len(m.builds) - 1
				}
			}
			return m, nil

		// Handle online builds fetched
		case buildsFetchedMsg:
			// Don't stop loading yet, need to merge with local status
			m.builds = msg.builds // Temporarily store fetched builds
			m.err = nil
			// Now trigger the local scan for status update
			cmd = updateStatusFromLocalScanCmd(m.builds, m.config)
			return m, cmd

		// Handle builds list after status update
		case buildsUpdatedMsg:
			m.isLoading = false // Now loading is complete
			m.builds = msg.builds
			// Sort the builds based on current sort settings
			m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)
			m.err = nil
			// Adjust cursor
			if m.cursor >= len(m.builds) {
				m.cursor = 0
				if len(m.builds) > 0 {
					m.cursor = len(m.builds) - 1
				}
			}
			return m, nil

		case model.BlenderLaunchedMsg:
			// Record that Blender is running
			m.blenderRunning = msg.Version
			// Update the footer message
			m.err = nil
			return m, nil

		case model.BlenderExecMsg:
			// Store Blender info
			execInfo := msg
			
			// Write a command file that the main.go program will execute after the TUI exits
			// This ensures Blender runs in the same terminal session after the TUI is fully terminated
			launcherPath := filepath.Join(os.TempDir(), "blender_launch_command.txt")
			
			// First try to save the command
			err := os.WriteFile(launcherPath, []byte(execInfo.Executable), 0644)
			if err != nil {
				return m, func() tea.Msg {
					return errMsg{fmt.Errorf("failed to save launch info: %w", err)}
				}
			}
			
			// Set an environment variable to tell the main program to run Blender on exit
			os.Setenv("TUI_BLENDER_LAUNCH", launcherPath)
			
			// Display exit message with info about Blender launch
			m.err = nil
			m.blenderRunning = execInfo.Version
			
			// Simply quit - the main program will handle launching Blender
			return m, tea.Quit

		case errMsg:
			m.isLoading = false
			m.err = msg.err
			return m, nil

		// Handle Download Start Request
		case startDownloadMsg:
			cmd = doDownloadCmd(msg.build, m.config, m.downloadStates, &m.downloadMutex, m.cancelDownloads)
			return m, cmd

		case tickMsg:
			m.downloadMutex.Lock()
			activeDownloads := 0
			var progressCmds []tea.Cmd
			completedDownloads := []string{}
			
			// Add a safety counter to prevent infinite ticks
			maxTicks := 1000 // Adjust this number based on your needs
			tickCounter, ok := m.downloadStates["_tickCounter"]
			if !ok {
				tickCounter = &DownloadState{Current: 0}
				m.downloadStates["_tickCounter"] = tickCounter
			}
			
			tickCounter.Current++
			if tickCounter.Current > int64(maxTicks) {
				// Too many ticks, clear all downloads to prevent freeze
				log.Printf("WARNING: Too many ticks (%d) detected, clearing download states to prevent freeze", maxTicks)
				m.downloadStates = make(map[string]*DownloadState)
				m.downloadStates["_tickCounter"] = &DownloadState{Current: 0}
				m.downloadMutex.Unlock()
				return m, nil
			}

			for version, state := range m.downloadStates {
				// Skip the counter
				if version == "_tickCounter" {
					continue
				}
				
				if state.Message == "Local" || strings.HasPrefix(state.Message, "Failed") || state.Message == "Cancelled" {
					completedDownloads = append(completedDownloads, version)
					// Update main build list status
					foundInBuilds := false
					for i := range m.builds {
						if m.builds[i].Version == version {
							m.builds[i].Status = state.Message
							foundInBuilds = true
							break
						}
					}
					if !foundInBuilds {
						log.Printf("[Update tick] Completed download %s not found in m.builds list!", version)
					}
				} else if strings.HasPrefix(state.Message, "Downloading") || state.Message == "Preparing..." || state.Message == "Extracting..." {
					// Still active (includes Extracting now)
					activeDownloads++
					// Update progress bar for both downloading and extracting
					progressCmds = append(progressCmds, m.progressBar.SetPercent(state.Progress))
				}
			}

			// Clean up completed downloads from the state map
			if len(completedDownloads) > 0 {
				for _, version := range completedDownloads {
					delete(m.downloadStates, version)
				}
				// Reset the tick counter when downloads complete
				tickCounter.Current = 0
			}

			m.downloadMutex.Unlock()

			if activeDownloads > 0 {
				cmds = append(cmds, tickCmd())
			} else {
				// No active downloads, reset the tick counter
				m.downloadMutex.Lock()
				if counter, exists := m.downloadStates["_tickCounter"]; exists {
					counter.Current = 0
				}
				m.downloadMutex.Unlock()
			}
			cmds = append(cmds, progressCmds...)
			if len(cmds) > 0 {
				cmd = tea.Batch(cmds...)
			}
			return m, cmd

		case downloadCompleteMsg:
			// Just trigger a refresh of local files
			cmd = scanLocalBuildsCmd(m.config)
			// Also refresh old builds info after download completes
			return m, tea.Batch(cmd, getOldBuildsInfoCmd(m.config))
			
		case oldBuildsInfo:
			m.oldBuildsCount = msg.count
			m.oldBuildsSize = msg.size
			if msg.err != nil {
				m.err = msg.err
			}
			return m, nil
			
		case cleanupOldBuildsMsg:
			if msg.err != nil {
				m.err = msg.err
			} else {
				m.oldBuildsCount = 0
				m.oldBuildsSize = 0
			}
			m.currentView = viewList
			return m, nil
		}
	case viewDeleteConfirm:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "y", "Y":
				// User confirmed deletion
				// Implement actual deletion logic using the DeleteBuild function
				success, err := local.DeleteBuild(m.config.DownloadDir, m.buildToDelete)
				if err != nil {
					log.Printf("Error deleting build %s: %v", m.buildToDelete, err)
					m.err = fmt.Errorf("Failed to delete build: %w", err)
				} else if !success {
					log.Printf("Build %s not found for deletion", m.buildToDelete)
					m.err = fmt.Errorf("Build %s not found", m.buildToDelete)
				} else {
					log.Printf("Successfully deleted build: %s", m.buildToDelete)
					// Clear any previous error
					m.err = nil
				}
				
				// Return to builds view and refresh the builds list
				m.buildToDelete = ""
				m.currentView = viewList
				m.isLoading = true
				return m, scanLocalBuildsCmd(m.config)
				
			case "n", "N", "esc", "escape":
				// User cancelled deletion
				m.buildToDelete = ""
				m.currentView = viewList
				return m, nil
			}
		}
	case viewCleanupConfirm:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "y", "Y":
				// User confirmed cleanup
				m.currentView = viewList
				return m, cleanupOldBuildsCmd(m.config)
				
			case "n", "N", "esc", "escape":
				// User cancelled cleanup
				m.currentView = viewList
				return m, nil
			}
		}
	}
	// Pass message to inputs if in settings view and in edit mode
	if (m.currentView == viewInitialSetup || m.currentView == viewSettings) && m.editMode {
		currentFocus := m.focusIndex
		if len(m.settingsInputs) > 0 && currentFocus >= 0 && currentFocus < len(m.settingsInputs) {
			m.settingsInputs[currentFocus], cmd = m.settingsInputs[currentFocus].Update(msg)
		}
		return m, cmd
	}
	return m, cmd
}

// calculateSplitIndex finds the rune index to split a string for a given visual width.
func calculateSplitIndex(s string, targetWidth int) int {
	currentWidth := 0
	for i, r := range s {
		runeWidth := runewidth.RuneWidth(r)
		if currentWidth+runeWidth > targetWidth {
			return i // Split before this rune
		}
		currentWidth += runeWidth
	}
	return len(s) // Target width is >= string width
}

// View renders the UI based on the model state.
func (m Model) View() string {
	var viewBuilder strings.Builder

	switch m.currentView {
	case viewInitialSetup, viewSettings:
		title := "Initial Setup"
		if m.currentView == viewSettings {
			title = "Settings"
		}
		viewBuilder.WriteString(fmt.Sprintf("%s\n\n", title))
		viewBuilder.WriteString("Download Directory:\n")
		
		// Only render inputs if they exist
		if len(m.settingsInputs) >= 2 {
			viewBuilder.WriteString(m.settingsInputs[0].View() + "\n\n")
			viewBuilder.WriteString("Minimum Blender Version Filter (e.g., 4.0, 3.6 - empty for none):\n")
			viewBuilder.WriteString(m.settingsInputs[1].View() + "\n\n")
		} else {
			// Fallback if inputs aren't initialized
			viewBuilder.WriteString(m.config.DownloadDir + "\n\n")
			viewBuilder.WriteString("Minimum Blender Version Filter (e.g., 4.0, 3.6 - empty for none):\n")
			viewBuilder.WriteString(m.config.VersionFilter + "\n\n")
		}
		
		if m.err != nil {
			viewBuilder.WriteString(lp.NewStyle().Foreground(lp.Color("9")).Render(fmt.Sprintf("Error: %v\n\n", m.err)))
		}
		
		// Show different help text based on current mode
		var helpText string
		if m.editMode {
			helpText = "Enter: Save Edits | Esc: Cancel Edit | Tab: Next Field"
			// Add a visual indicator that edit mode is active
			modeIndicator := lp.NewStyle().
				Background(lp.Color("11")).
				Foreground(lp.Color("0")).
				Padding(0, 1).
				Render(" EDIT MODE ")
			viewBuilder.WriteString(modeIndicator + "\n\n")
		} else {
			helpText = "Enter: Edit Field | h: Back | j/k: Navigate | s: Save Settings"
			// Add a visual indicator that navigation mode is active
			modeIndicator := lp.NewStyle().
				Background(lp.Color("12")).
				Foreground(lp.Color("0")).
				Padding(0, 1).
				Render(" NAVIGATION MODE ")
			viewBuilder.WriteString(modeIndicator + "\n\n")
		}
		viewBuilder.WriteString(footerStyle.Render(helpText))

	case viewList:
		loadingMsg := ""
		if m.isLoading {
			if len(m.builds) == 0 {
				loadingMsg = "Scanning local builds..."
			} else {
				loadingMsg = "Fetching online builds..."
			}
		}

		if loadingMsg != "" {
			// Simple full-screen loading message for now
			return loadingMsg
		}

		if m.err != nil {
			return fmt.Sprintf(`Error: %v

Press f to try fetching online builds, s for settings, q to quit.`, m.err)
		}
		if len(m.builds) == 0 {
			return `No Blender builds found (local or online matching criteria).

Press f to fetch online builds, s for settings, q to quit.`
		}

		// --- Render Table ---
		var tableBuilder strings.Builder
		// --- Header rendering (Remove selection column from header) ---
		headerCols := []string{
			cellStyleCenter.Copy().Width(colWidthVersion).Render(getSortIndicator(m, 0, "Version")),
			cellStyleCenter.Copy().Width(colWidthStatus).Render(getSortIndicator(m, 1, "Status")),
			cellStyleCenter.Copy().Width(colWidthBranch).Render(getSortIndicator(m, 2, "Branch")),
			cellStyleCenter.Copy().Width(colWidthType).Render(getSortIndicator(m, 3, "Type")),
			cellStyleCenter.Copy().Width(colWidthHash).Render(getSortIndicator(m, 4, "Hash")),
			cellStyleCenter.Copy().Width(colWidthSize).Render(getSortIndicator(m, 5, "Size")),
			cellStyleCenter.Copy().Width(colWidthDate).Render(getSortIndicator(m, 6, "Build Date")),
		}
		tableBuilder.WriteString(headerStyle.Render(lp.JoinHorizontal(lp.Left, headerCols...)))
		tableBuilder.WriteString("\n")
		tableBuilder.WriteString(separator)
		tableBuilder.WriteString("\n")

		// --- Rows --- (Remove selection marker from rows)
		for i, build := range m.builds {
			downloadState, isDownloadingThis := m.downloadStates[build.Version]

			// --- Default row cell values (Apply alignment) ---
			versionCell := cellStyleCenter.Copy().Width(colWidthVersion).Render(util.TruncateString("Blender "+build.Version, colWidthVersion))
			statusTextStyle := regularRowStyle

			// --- Adjust cells based on status (Apply alignment within style) ---
			if build.Status == "Local" {
				statusTextStyle = lp.NewStyle().Foreground(lp.Color("10"))
			} else if build.Status == "Update" {
				statusTextStyle = lp.NewStyle().Foreground(lp.Color("12")) // Light blue for updates
			} else if strings.HasPrefix(build.Status, "Failed") {
				statusTextStyle = lp.NewStyle().Foreground(lp.Color("9"))
			}

			// --- Override cells if downloading ---
			if isDownloadingThis {
				statusTextStyle = lp.NewStyle().Foreground(lp.Color("11")) // Keep text style separate from alignment
				statusCell := cellStyleCenter.Copy().Width(colWidthStatus).Render(downloadState.Message)
				
				// Calculate the combined width for a true spanning cell
				combinedWidth := colWidthSize + colWidthDate
				
				// Create a wider progress bar
				m.progressBar.Width = combinedWidth
				progressBarOutput := m.progressBar.ViewAs(downloadState.Progress)
				
				// Create a wider cell that spans both size and date columns
				combinedCell := lp.NewStyle().Width(combinedWidth).Render(progressBarOutput)
				
				// Display different content based on download state
				hashText := util.FormatSpeed(downloadState.Speed)
				if downloadState.Message == "Extracting..." {
					// For extraction, show "Extracting" instead of download speed
					hashText = "Extracting..."
				}
				hashCell := cellStyleCenter.Copy().Width(colWidthHash).Render(hashText)
				
				// First render the individual cells
				specialRowCols := []string{
					versionCell,
					statusCell,
					cellStyleCenter.Copy().Width(colWidthBranch).Render(util.TruncateString(build.Branch, colWidthBranch)),
					cellStyleCenter.Copy().Width(colWidthType).Render(util.TruncateString(build.ReleaseCycle, colWidthType)),
					hashCell,
					combinedCell, // This cell spans both size and date columns
				}
				
				// Join cells into a single row
				rowContent := lp.JoinHorizontal(lp.Left, specialRowCols...)
				
				// Then apply selection style to the entire row
				if m.cursor == i {
					tableBuilder.WriteString(selectedRowStyle.Render(rowContent))
				} else {
					tableBuilder.WriteString(rowContent)
				}
				tableBuilder.WriteString("\n")
				
				// Skip the regular row assembly
				continue
			}

			// First create cell content with their individual styles
			statusCell := statusTextStyle.Copy().Inherit(cellStyleCenter).Width(colWidthStatus).Render(util.TruncateString(build.Status, colWidthStatus))
			
			// Render each cell individually with appropriate styles
			rowCols := []string{
				versionCell,
				statusCell,
				cellStyleCenter.Copy().Width(colWidthBranch).Render(util.TruncateString(build.Branch, colWidthBranch)),
				cellStyleCenter.Copy().Width(colWidthType).Render(util.TruncateString(build.ReleaseCycle, colWidthType)),
				cellStyleCenter.Copy().Width(colWidthHash).Render(util.TruncateString(build.Hash, colWidthHash)),
				cellStyleCenter.Copy().Width(colWidthSize).Render(util.FormatSize(build.Size)),
				cellStyleCenter.Copy().Width(colWidthDate).Render(build.BuildDate.Time().Format("2006-01-02 15:04")),
			}
			
			// Join the content horizontally into a single string
			rowContent := lp.JoinHorizontal(lp.Left, rowCols...)

			// Apply selection style to the entire row if it's selected
			if m.cursor == i {
				tableBuilder.WriteString(selectedRowStyle.Render(rowContent))
			} else {
				tableBuilder.WriteString(rowContent)
			}
			tableBuilder.WriteString("\n")
		}

		// --- Combine table and footer ---
		viewBuilder.WriteString(tableBuilder.String())

		// Display running Blender notice if applicable
		if m.blenderRunning != "" {
			runningNotice := lp.NewStyle().
				Foreground(lp.Color("10")). // Green text
				Bold(true).
				Render(fmt.Sprintf("⚠ Blender %s is running - this terminal will display its console output", m.blenderRunning))
			viewBuilder.WriteString("\n" + runningNotice + "\n")
		}

		// ... Footer rendering ...
		footerKeybinds1 := "Enter:Launch  D:Download  O:Open Dir  X:Delete"
		footerKeybinds2 := "F:Fetch  R:Reverse  S:Settings  Q:Quit"
		if m.oldBuildsCount > 0 {
			footerKeybinds2 = fmt.Sprintf("F:Fetch  C:Cleanup(%d)  S:Settings  Q:Quit", m.oldBuildsCount)
		}
		footerKeybinds3 := "←→:Column  R:Reverse"
		keybindSeparator := "│"
		footerKeys := fmt.Sprintf("%s  %s  %s  %s  %s", footerKeybinds1, keybindSeparator, footerKeybinds2, keybindSeparator, footerKeybinds3)
		
		// Create colored status indicators for the legend
		localStatus := lp.NewStyle().Foreground(lp.Color("10")).Render("■ Local")
		updateStatus := lp.NewStyle().Foreground(lp.Color("12")).Render("■ Update Available")
		onlineStatus := lp.NewStyle().Foreground(lp.Color("15")).Render("■ Online")
		
		footerLegend := fmt.Sprintf("%s   %s   %s   ↑↓ Sort Direction", localStatus, updateStatus, onlineStatus)
		viewBuilder.WriteString(footerStyle.Render(footerKeys))
		viewBuilder.WriteString("\n")
		viewBuilder.WriteString(footerStyle.Render(footerLegend))

	case viewDeleteConfirm:
		// Styled confirmation dialog with box border
		confirmWidth := 50
		
		// Create a styled border box
		boxStyle := lp.NewStyle().
			BorderStyle(lp.RoundedBorder()).
			BorderForeground(lp.Color("11")). // Yellow border
			Padding(1, 2)
		
		// Title with warning styling
		titleStyle := lp.NewStyle().
			Foreground(lp.Color("11")). // Yellow text
			Bold(true)
		
		// Build version styling
		buildStyle := lp.NewStyle().
			Foreground(lp.Color("15")). // White text
			Bold(true)
		
		// Create the content
		var contentBuilder strings.Builder
		contentBuilder.WriteString(titleStyle.Render("Confirm Deletion") + "\n\n")
		contentBuilder.WriteString("Are you sure you want to delete ")
		contentBuilder.WriteString(buildStyle.Render("Blender " + m.buildToDelete) + "?\n\n")
		contentBuilder.WriteString("This will permanently remove this build from your system.\n\n")
		
		// Button styling
		yesStyle := lp.NewStyle().
			Foreground(lp.Color("9")). // Red for delete
			Bold(true)
		noStyle := lp.NewStyle().
			Foreground(lp.Color("10")). // Green for cancel
			Bold(true)
			
		contentBuilder.WriteString(yesStyle.Render("[Y] Yes, delete it") + "    ")
		contentBuilder.WriteString(noStyle.Render("[N] No, cancel"))
		
		// Combine everything in the box
		confirmBox := boxStyle.Width(confirmWidth).Render(contentBuilder.String())
		
		// Center the box in the terminal
		viewBuilder.WriteString("\n\n") // Add some top spacing
		viewBuilder.WriteString(lp.Place(m.terminalWidth, 20,
			lp.Center, lp.Center,
			confirmBox))
		viewBuilder.WriteString("\n\n")
	case viewCleanupConfirm:
		// Styled confirmation dialog with box border
		confirmWidth := 60
		
		// Create a styled border box
		boxStyle := lp.NewStyle().
			BorderStyle(lp.RoundedBorder()).
			BorderForeground(lp.Color("11")). // Yellow border
			Padding(1, 2)
		
		// Title with warning styling
		titleStyle := lp.NewStyle().
			Foreground(lp.Color("11")). // Yellow text
			Bold(true)
		
		// Create the content
		var contentBuilder strings.Builder
		contentBuilder.WriteString(titleStyle.Render("Confirm Cleanup") + "\n\n")
		contentBuilder.WriteString(fmt.Sprintf("Are you sure you want to clean up %d old builds?\n", m.oldBuildsCount))
		contentBuilder.WriteString(fmt.Sprintf("This will free up %s of disk space.\n\n", util.FormatSize(m.oldBuildsSize)))
		contentBuilder.WriteString("All backed up builds in the .oldbuilds directory will be permanently deleted.\n\n")
		
		// Button styling
		yesStyle := lp.NewStyle().
			Foreground(lp.Color("9")). // Red for delete
			Bold(true)
		noStyle := lp.NewStyle().
			Foreground(lp.Color("10")). // Green for cancel
			Bold(true)
			
		contentBuilder.WriteString(yesStyle.Render("[Y] Yes, delete them") + "    ")
		contentBuilder.WriteString(noStyle.Render("[N] No, cancel"))
		
		// Combine everything in the box
		confirmBox := boxStyle.Width(confirmWidth).Render(contentBuilder.String())
		
		// Center the box in the terminal
		viewBuilder.WriteString("\n\n") // Add some top spacing
		viewBuilder.WriteString(lp.Place(m.terminalWidth, 20,
			lp.Center, lp.Center,
			confirmBox))
		viewBuilder.WriteString("\n\n")
	}

	return viewBuilder.String()
}

// sortBuilds sorts the builds based on the selected column and sort order
func sortBuilds(builds []model.BlenderBuild, column int, reverse bool) []model.BlenderBuild {
	// Create a copy of builds to avoid modifying the original
	sortedBuilds := make([]model.BlenderBuild, len(builds))
	copy(sortedBuilds, builds)

	// Sort based on the selected column
	switch column {
	case 0: // Version
		// Sort by version
		if reverse {
			sort.SliceStable(sortedBuilds, func(i, j int) bool {
				return sortedBuilds[i].Version > sortedBuilds[j].Version
			})
		} else {
			sort.SliceStable(sortedBuilds, func(i, j int) bool {
				return sortedBuilds[i].Version < sortedBuilds[j].Version
			})
		}
	case 1: // Status
		// Sort by status
		if reverse {
			sort.SliceStable(sortedBuilds, func(i, j int) bool {
				return sortedBuilds[i].Status > sortedBuilds[j].Status
			})
		} else {
			sort.SliceStable(sortedBuilds, func(i, j int) bool {
				return sortedBuilds[i].Status < sortedBuilds[j].Status
			})
		}
	case 2: // Branch
		// Sort by branch
		if reverse {
			sort.SliceStable(sortedBuilds, func(i, j int) bool {
				return sortedBuilds[i].Branch > sortedBuilds[j].Branch
			})
		} else {
			sort.SliceStable(sortedBuilds, func(i, j int) bool {
				return sortedBuilds[i].Branch < sortedBuilds[j].Branch
			})
		}
	case 3: // Type/ReleaseCycle
		// Sort by release cycle
		if reverse {
			sort.SliceStable(sortedBuilds, func(i, j int) bool {
				return sortedBuilds[i].ReleaseCycle > sortedBuilds[j].ReleaseCycle
			})
		} else {
			sort.SliceStable(sortedBuilds, func(i, j int) bool {
				return sortedBuilds[i].ReleaseCycle < sortedBuilds[j].ReleaseCycle
			})
		}
	case 4: // Hash
		// Sort by hash
		if reverse {
			sort.SliceStable(sortedBuilds, func(i, j int) bool {
				return sortedBuilds[i].Hash > sortedBuilds[j].Hash
			})
		} else {
			sort.SliceStable(sortedBuilds, func(i, j int) bool {
				return sortedBuilds[i].Hash < sortedBuilds[j].Hash
			})
		}
	case 5: // Size
		// Sort by size
		if reverse {
			sort.SliceStable(sortedBuilds, func(i, j int) bool {
				return sortedBuilds[i].Size > sortedBuilds[j].Size
			})
		} else {
			sort.SliceStable(sortedBuilds, func(i, j int) bool {
				return sortedBuilds[i].Size < sortedBuilds[j].Size
			})
		}
	case 6: // Date
		// Sort by build date
		if reverse {
			sort.SliceStable(sortedBuilds, func(i, j int) bool {
				return sortedBuilds[i].BuildDate.Time().After(sortedBuilds[j].BuildDate.Time())
			})
		} else {
			sort.SliceStable(sortedBuilds, func(i, j int) bool {
				return sortedBuilds[i].BuildDate.Time().Before(sortedBuilds[j].BuildDate.Time())
			})
		}
	}

	return sortedBuilds
}

// getSortIndicator returns a string indicating the sort direction for a given column
func getSortIndicator(m Model, column int, title string) string {
	if m.sortColumn == column {
		if m.sortReversed {
			return "↓ " + title
		} else {
			return "↑ " + title
		}
	}
	return title
}

// Command to get info about old builds
func getOldBuildsInfoCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		count, size, err := local.GetOldBuildsInfo(cfg.DownloadDir)
		return oldBuildsInfo{
			count: count,
			size:  size,
			err:   err,
		}
	}
}

// Command to clean up old builds
func cleanupOldBuildsCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		err := local.DeleteAllOldBuilds(cfg.DownloadDir)
		return cleanupOldBuildsMsg{err: err}
	}
}

// Helper function to update focus styling for settings inputs
func updateFocusStyles(m *Model, oldFocus int) {
	// Update the prompt style of all inputs
	for i := 0; i < len(m.settingsInputs); i++ {
		if i == m.focusIndex {
			// Just update the style, don't focus in navigation mode
			m.settingsInputs[i].PromptStyle = selectedRowStyle
		} else {
			m.settingsInputs[i].PromptStyle = regularRowStyle
		}
	}
}

// Helper function to save settings
func saveSettings(m Model) (tea.Model, tea.Cmd) {
	m.config.DownloadDir = m.settingsInputs[0].Value()
	m.config.VersionFilter = m.settingsInputs[1].Value()
	err := config.SaveConfig(m.config)
	if err != nil {
		m.err = fmt.Errorf("failed to save config: %w", err)
	} else {
		m.err = nil
		m.currentView = viewList
		// If list is empty, trigger initial local scan now
		if len(m.builds) == 0 {
			m.isLoading = true
			return m, scanLocalBuildsCmd(m.config)
		}
	}
	return m, nil
}
