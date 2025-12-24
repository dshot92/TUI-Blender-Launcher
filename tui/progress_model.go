package tui

import (
	"TUI-Blender-Launcher/model"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

// ProgressModel handles the state and logic for download progress.
type ProgressModel struct {
	ProgressBar      progress.Model
	ActiveDownloadID string
	DownloadStates   map[string]*model.DownloadState
}

// NewProgressModel creates a new ProgressModel.
func NewProgressModel() ProgressModel {
	// Configure the progress bar
	progModel := progress.New(
		progress.WithGradient(highlightColor, "255"),
		progress.WithoutPercentage(),
		progress.WithWidth(30),
		progress.WithSolidFill(highlightColor),
	)

	return ProgressModel{
		ProgressBar:    progModel,
		DownloadStates: make(map[string]*model.DownloadState),
	}
}

// Init initializes the model.
func (m ProgressModel) Init() tea.Cmd {
	return nil
}

// View returns the string representation of the model.
func (m ProgressModel) View() string {
	return ""
}

// Update handles update messages for the progress model.
func (m *ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case progress.FrameMsg:
		progressModel, cmd := m.ProgressBar.Update(msg)
		m.ProgressBar = progressModel.(progress.Model)
		return m, cmd

	case tickMsg:
		// Logic currently in update.go's handleDownloadProgress
		// It returns commands, but also modifies model state.
		// Since handleDownloadProgress is quite complex and integrated with the logic of what to do
		// (sorting builds etc), we might keep the heavy logic in the main model's Update or move it here.
		// For now, let's keep the model simple and let the main `Update` drive specific progress updates.
		// But the `ProgressBar` update MUST happen here for the animation to work if we delegate.
		return m, nil
	}
	return m, nil
}

// SyncDownloadStates updates the local download states from the source
func (m *ProgressModel) SyncDownloadStates(states map[string]*model.DownloadState) {
	for id, state := range states {
		m.DownloadStates[id] = state
	}
}

// GetActiveDownloadProgress returns the progress of the active download or 0
func (m *ProgressModel) GetActiveDownloadProgress() float64 {
	if m.ActiveDownloadID != "" {
		if state, ok := m.DownloadStates[m.ActiveDownloadID]; ok {
			return state.Progress
		}
	}
	return 0.0
}
