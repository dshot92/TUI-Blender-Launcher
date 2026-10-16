package tui

import (
	"os"
	"reflect"
	"testing"

	"TUI-Blender-Launcher/config"

	tea "github.com/charmbracelet/bubbletea"
)

// openSettingsForAutoSave mirrors the list-view "open settings" flow: it loads
// the values from the config and establishes the auto-save baseline.
func openSettingsForAutoSave(m *Model) {
	m.currentView = viewSettings
	m.Settings.SetValues(SettingsValues{
		DownloadDir:   m.config.DownloadDir,
		VersionFilter: m.config.VersionFilter,
		BuildType:     m.config.BuildType,
		ReleaseTypes:  m.config.ReleaseTypes,
		Theme:         m.config.Theme,
	})
	m.settingsSaved = m.Settings.GetValues()
}

func autoSaveConfigPath(t *testing.T) string {
	t.Helper()
	p, err := config.GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath: %v", err)
	}
	return p
}

func TestAutoSaveWritesOnReleaseToggle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := InitialModel(config.DefaultConfig(), false)
	openSettingsForAutoSave(m)

	if _, err := os.Stat(autoSaveConfigPath(t)); !os.IsNotExist(err) {
		t.Fatalf("config should not exist before any edit, stat err = %v", err)
	}

	// Enter toggle mode on the Release Types row, turn the focused (alpha) chip
	// off, then commit with enter.
	m.Settings.FocusIndex = settingReleaseFilter
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	loaded, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	want := []string{"beta", "candidate", "stable"}
	if !reflect.DeepEqual(loaded.ReleaseTypes, want) {
		t.Fatalf("saved release_types = %v, want %v", loaded.ReleaseTypes, want)
	}
}

func TestAutoSaveIgnoresNavigation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := InitialModel(config.DefaultConfig(), false)
	openSettingsForAutoSave(m)

	// Moving the focus must not write the config: nothing changed.
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.KeyMsg{Type: tea.KeyDown})

	if _, err := os.Stat(autoSaveConfigPath(t)); !os.IsNotExist(err) {
		t.Fatalf("navigation should not create a config file, stat err = %v", err)
	}
}
