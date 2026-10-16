package tui

import (
	"TUI-Blender-Launcher/model"
)

// ProgressModel holds the snapshot of download states used for rendering.
// The actual download bookkeeping lives in DownloadManager; this model is a
// read-only cache refreshed on each tick.
type ProgressModel struct {
	ActiveDownloadID string
	DownloadStates   map[string]*model.DownloadState
}

// NewProgressModel creates a new ProgressModel.
func NewProgressModel() ProgressModel {
	return ProgressModel{
		DownloadStates: make(map[string]*model.DownloadState),
	}
}

// SyncDownloadStates replaces the local download state snapshot with the
// source of truth. Replacing (instead of merging) prevents states for
// finished/removed downloads from accumulating indefinitely.
func (m *ProgressModel) SyncDownloadStates(states map[string]*model.DownloadState) {
	if states == nil {
		states = make(map[string]*model.DownloadState)
	}
	m.DownloadStates = states
}
