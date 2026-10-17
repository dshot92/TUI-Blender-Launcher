package tui

import (
	"strings"
	"testing"

	"github.com/dshot92/TUI-Blender-Launcher/config"
	"github.com/dshot92/TUI-Blender-Launcher/model"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSettingsReleaseTypesToggle(t *testing.T) {
	cfg := config.Config{DownloadDir: "/tmp/x", BuildType: "daily", Theme: "dracula"}
	s := NewSettingsModel(cfg, NewStyle())
	s.FocusIndex = settingReleaseFilter

	// Enter opens the multi-select toggle mode.
	s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !s.releaseEditing {
		t.Fatalf("Enter on the release row should start toggling")
	}

	// Space toggles the first chip (alpha).
	s.Update(tea.KeyMsg{Type: tea.KeySpace})
	if got := strings.Join(s.ReleaseTypes, ","); got != "alpha" {
		t.Fatalf("after toggling alpha, selection = %q", got)
	}

	// Move to beta and toggle it too.
	s.Update(tea.KeyMsg{Type: tea.KeyRight})
	s.Update(tea.KeyMsg{Type: tea.KeySpace})
	if got := strings.Join(s.ReleaseTypes, ","); got != "alpha,beta" {
		t.Fatalf("after toggling beta, selection = %q", got)
	}

	// Toggling alpha back off leaves only beta.
	s.Update(tea.KeyMsg{Type: tea.KeyLeft})
	s.Update(tea.KeyMsg{Type: tea.KeySpace})
	if got := strings.Join(s.ReleaseTypes, ","); got != "beta" {
		t.Fatalf("after untoggling alpha, selection = %q", got)
	}

	// Enter saves the selection and leaves toggle mode.
	s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.releaseEditing {
		t.Fatalf("Enter should save and leave the release toggle mode")
	}
	if got := strings.Join(s.ReleaseTypes, ","); got != "beta" {
		t.Fatalf("Enter should keep the selection, got %q", got)
	}
	if s.FocusIndex != settingReleaseFilter {
		t.Fatalf("focus moved unexpectedly to %d", s.FocusIndex)
	}

	// Esc cancels: changes made since entering are discarded.
	s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	s.Update(tea.KeyMsg{Type: tea.KeySpace}) // toggle alpha back on
	if got := strings.Join(s.ReleaseTypes, ","); got != "alpha,beta" {
		t.Fatalf("after re-toggling alpha, selection = %q", got)
	}
	s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if s.releaseEditing {
		t.Fatalf("Esc should leave the release toggle mode")
	}
	if got := strings.Join(s.ReleaseTypes, ","); got != "beta" {
		t.Fatalf("Esc should discard changes, got %q", got)
	}
}

func TestApplyFiltersRelease(t *testing.T) {
	m := &Model{config: config.Config{ReleaseTypes: []string{"stable"}}}
	builds := []model.BlenderBuild{
		{Version: "5.3.0", ReleaseCycle: "alpha", Status: model.StateOnline},
		{Version: "5.1.2", ReleaseCycle: "stable", Status: model.StateOnline},
		{Version: "4.2.0", ReleaseCycle: "alpha", Status: model.StateLocal},
	}
	got := m.applyFilters(builds)
	if len(got) != 2 {
		t.Fatalf("applyFilters kept %d builds, want 2 (stable online + local)", len(got))
	}
}
