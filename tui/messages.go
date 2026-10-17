package tui

import (
	"github.com/dshot92/TUI-Blender-Launcher/model"
	"time"
)

// Define messages for communication between components
// Group related message types together
type (
	// Data update messages
	buildsFetchedMsg struct { // Online builds fetched
		builds []model.BlenderBuild
		err    error // Add error field
	}
	localBuildsScannedMsg struct { // Initial local scan complete
		builds []model.BlenderBuild
		err    error // Include error from scanning
	}
	buildsUpdatedMsg struct { // Builds list updated (e.g., status change)
		builds []model.BlenderBuild
	}

	// Action messages
	startDownloadMsg struct { // Request to start download for a build
		build   model.BlenderBuild
		buildID string // Added unique build identifier
	}
	downloadCompleteMsg struct { // Download & extraction finished
		buildVersion string // Version of the build that finished
		err          error
	}
	buildDeletedMsg struct { // Build was deleted from disk
		hash string
	}
	// Error message
	errMsg struct{ err error }

	// Transient success/status message
	statusMsg struct{ text string }

	// clearStatusMsg clears the transient status line after it expires. The
	// token matches the statusMsg it was scheduled for, so a newer status
	// message is never cleared by an older timer.
	clearStatusMsg struct{ token int64 }

	// Timer message
	tickMsg time.Time
)
