package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dshot92/TUI-Blender-Launcher/config"

	tea "github.com/charmbracelet/bubbletea"
)

// newDirSettings builds a SettingsModel focused on the Download Directory row,
// sized so the detail pane (and its completion popup) is rendered.
func newDirSettings(t *testing.T, dir string) *SettingsModel {
	t.Helper()
	cfg := config.Config{DownloadDir: dir, BuildType: "daily", Theme: "dracula"}
	s := NewSettingsModel(cfg, NewStyle())
	s.SetWidth(100)
	return &s
}

func TestSettingsTabIgnoredWhenNotEditing(t *testing.T) {
	s := newDirSettings(t, "/tmp/does-not-matter")
	before := s.Inputs[settingDownloadDir].Value()

	s.Update(tea.KeyMsg{Type: tea.KeyTab})

	if got := s.Inputs[settingDownloadDir].Value(); got != before {
		t.Fatalf("Tab outside edit mode changed value: %q -> %q", before, got)
	}
	if len(s.dirMatches) != 0 {
		t.Fatalf("Tab outside edit mode produced matches: %v", s.dirMatches)
	}
}

func TestSettingsTabCompletesUniqueMatch(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "fruits"), 0o755); err != nil {
		t.Fatal(err)
	}

	s := newDirSettings(t, filepath.Join(root, "fru"))
	s.Update(tea.KeyMsg{Type: tea.KeyEnter}) // enter edit mode
	s.Update(tea.KeyMsg{Type: tea.KeyTab})

	want := filepath.Join(root, "fruits") + string(os.PathSeparator)
	if got := s.Inputs[settingDownloadDir].Value(); got != want {
		t.Fatalf("value = %q, want %q", got, want)
	}
	if len(s.dirMatches) != 1 {
		t.Fatalf("matches = %v, want 1", s.dirMatches)
	}
}

func TestSettingsTabExtendsToCommonPrefix(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"apple", "apricot"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	s := newDirSettings(t, filepath.Join(root, "a"))
	s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	s.Update(tea.KeyMsg{Type: tea.KeyTab})

	want := filepath.Join(root, "ap")
	if got := s.Inputs[settingDownloadDir].Value(); got != want {
		t.Fatalf("value = %q, want common prefix %q", got, want)
	}
	if len(s.dirMatches) != 2 {
		t.Fatalf("matches = %v, want 2", s.dirMatches)
	}
}

func TestSettingsTabCyclesAmbiguousMatches(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"fruits/apple", "fruits/apricot"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	s := newDirSettings(t, filepath.Join(root, "fruits", "ap"))
	s.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sep := string(os.PathSeparator)
	apple := filepath.Join(root, "fruits", "apple") + sep
	apricot := filepath.Join(root, "fruits", "apricot") + sep

	for i, want := range []string{apple, apricot, apple} {
		s.Update(tea.KeyMsg{Type: tea.KeyTab})
		if got := s.Inputs[settingDownloadDir].Value(); got != want {
			t.Fatalf("Tab %d: value = %q, want %q", i+1, got, want)
		}
	}
	if len(s.dirMatches) != 2 {
		t.Fatalf("matches = %v, want 2", s.dirMatches)
	}
}

func TestSettingsTabSessionResetsOnTyping(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"fruits/apple", "fruits/apricot"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	s := newDirSettings(t, filepath.Join(root, "fruits", "ap"))
	s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	s.Update(tea.KeyMsg{Type: tea.KeyTab})
	if len(s.dirMatches) == 0 {
		t.Fatal("expected matches after Tab")
	}

	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if len(s.dirMatches) != 0 {
		t.Fatalf("typing should clear matches, got %v", s.dirMatches)
	}
}

func TestSettingsTabSessionResetsOnCommit(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"fruits/apple", "fruits/apricot"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	s := newDirSettings(t, filepath.Join(root, "fruits", "ap"))
	s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	s.Update(tea.KeyMsg{Type: tea.KeyTab})

	s.Update(tea.KeyMsg{Type: tea.KeyEnter}) // commit
	if s.EditMode {
		t.Fatal("Enter should leave edit mode")
	}
	if len(s.dirMatches) != 0 {
		t.Fatalf("commit should clear matches, got %v", s.dirMatches)
	}
}

// TestCompletionDropdownRendersInFrame checks that the floating completion
// candidates actually appear in the rendered settings frame via the IDE-style
// overlay, anchored near the Download Directory input.
func TestCompletionDropdownRendersInFrame(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"fruits/apple", "fruits/apricot"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	m := sampleModel()
	m.currentView = viewSettings
	m.UpdateWindowSize(120, 34)

	m.Settings.SetValues(SettingsValues{DownloadDir: filepath.Join(root, "fruits", "ap"), BuildType: "daily", Theme: "dracula"})
	m.UpdateWindowSize(120, 34)

	m.Settings.FocusIndex = settingDownloadDir
	m.Settings.EditMode = true
	m.Settings.updateFocusStyles()
	m.Settings.Update(tea.KeyMsg{Type: tea.KeyTab})

	if len(m.Settings.dirMatches) < 2 {
		t.Fatalf("expected >=2 candidate matches, got %v", m.Settings.dirMatches)
	}

	out := plain(m.View())
	for _, name := range []string{"apple", "apricot"} {
		if !strings.Contains(out, name) {
			t.Fatalf("completion dropdown should show %q in the frame:\n%s", name, out)
		}
	}
}
