package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"TUI-Blender-Launcher/config"
	"TUI-Blender-Launcher/model"

	tea "github.com/charmbracelet/bubbletea"
)

var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

// plain strips ANSI escape sequences for width assertions.
func plain(s string) string { return ansiPattern.ReplaceAllString(s, "") }

// sampleModel builds a model with one build in each interesting state plus an
// in-progress download.
func sampleModel() *Model {
	cfg := config.Config{DownloadDir: "/tmp/blender", BuildType: "daily", Theme: "dracula"}
	m := InitialModel(cfg, false)

	now := model.Timestamp(time.Now())
	mk := func(ver, branch, cycle, hash string, size int64, st model.BuildState) model.BlenderBuild {
		return model.BlenderBuild{
			Version: ver, Branch: branch, ReleaseCycle: cycle, Hash: hash,
			Size: size, BuildDate: now, Status: st,
			FileName: "blender-" + ver + "-linux-x64.tar.xz",
		}
	}
	m.List.Builds = []model.BlenderBuild{
		mk("4.5.1", "main", "daily", "a1b2c3d4e5f6", 350*1024*1024, model.StateOnline),
		mk("4.5.0", "main", "daily", "abcdef123456", 340*1024*1024, model.StateDownloading),
		mk("4.4.0", "main", "daily", "deadbeefcafe", 320*1024*1024, model.StateUpdate),
		mk("4.3.2", "stable", "stable", "1122334455aa", 300*1024*1024, model.StateLocal),
		mk("4.2.0", "main", "daily", "9999999999ff", 280*1024*1024, model.StateFailed),
	}
	m.List.Cursor = 1
	m.Progress.DownloadStates["4.5.0-abcdef12"] = &model.DownloadState{
		BuildID: "4.5.0-abcdef12", BuildState: model.StateDownloading,
		Progress: 0.42, Current: 140 * 1024 * 1024, Total: 340 * 1024 * 1024,
		Speed: 12 * 1024 * 1024,
	}
	return m
}

func assertExactWidth(t *testing.T, label, out string, w int) {
	t.Helper()
	for i, line := range strings.Split(out, "\n") {
		if got := utf8.RuneCountInString(line); got != w {
			t.Fatalf("%s: line %d width %d, want %d\n%q", label, i, got, w, line)
		}
	}
}

func TestViewLinesFitTerminalWidth(t *testing.T) {
	sizes := [][2]int{{150, 40}, {120, 30}, {104, 26}, {100, 26}, {80, 24}, {60, 20}, {45, 12}}
	for _, sz := range sizes {
		m := sampleModel()
		m.UpdateWindowSize(sz[0], sz[1])
		assertExactWidth(t, "list", plain(m.View()), sz[0])
	}
}

func TestViewLinesFitTerminalWidthSettings(t *testing.T) {
	sizes := [][2]int{{150, 40}, {120, 30}, {80, 24}, {45, 12}}
	for _, sz := range sizes {
		m := sampleModel()
		m.currentView = viewSettings
		m.UpdateWindowSize(sz[0], sz[1])
		assertExactWidth(t, "settings", plain(m.View()), sz[0])
	}
}

func TestSmallTerminalGuard(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(30, 8)
	out := plain(m.View())
	if !strings.Contains(out, "Terminal too small") {
		t.Fatalf("expected small-terminal message, got:\n%s", out)
	}
	assertExactWidth(t, "guard", out, 30)
}

func lineContaining(s, sub string) int {
	for i, line := range strings.Split(s, "\n") {
		if strings.Contains(line, sub) {
			return i
		}
	}
	return -1
}

func TestDetailPaneSideBySideWhenWide(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(150, 40)
	if !m.layout.detailVisible || m.layout.detailStacked {
		t.Fatalf("want visible side-by-side at 150 cols (visible=%v stacked=%v)",
			m.layout.detailVisible, m.layout.detailStacked)
	}
	if !strings.Contains(plain(m.View()), "Details") {
		t.Fatalf("expected Details pane in output")
	}
}

func TestDetailPaneStacksWhenSplitWouldClipTable(t *testing.T) {
	// 110 cols leaves the split table too narrow to show every value, so the
	// info panel must dock below rather than squeeze the table.
	m := sampleModel()
	m.UpdateWindowSize(110, 30)
	if !m.layout.detailStacked {
		t.Fatalf("want stacked when the split would clip (stacked=%v visible=%v)",
			m.layout.detailStacked, m.layout.detailVisible)
	}

	// Widening enough for the table to fit restores the side-by-side layout.
	m.UpdateWindowSize(160, 40)
	if m.layout.detailStacked {
		t.Fatalf("want side-by-side once the table fits (stacked=%v)", m.layout.detailStacked)
	}
}

func TestDetailPaneStacksWhenNarrow(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(90, 30)
	if !m.layout.detailVisible || !m.layout.detailStacked {
		t.Fatalf("want stacked panel at 90 cols (visible=%v stacked=%v)",
			m.layout.detailVisible, m.layout.detailStacked)
	}
	out := plain(m.View())
	buildsLine := lineContaining(out, "Builds")
	detailsLine := lineContaining(out, "Details")
	if buildsLine < 0 || detailsLine < 0 {
		t.Fatalf("expected both panes in output:\n%s", out)
	}
	if detailsLine <= buildsLine {
		t.Fatalf("Details should render below Builds (builds@%d details@%d)", buildsLine, detailsLine)
	}
	assertExactWidth(t, "stacked", out, 90)
}

func TestToggleDetailKey(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(150, 40)
	if !m.layout.detailVisible {
		t.Fatalf("panel should start visible")
	}

	// Tab hides it.
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.detailShown {
		t.Fatalf("detailShown should be false after Tab")
	}
	if m.layout.detailVisible {
		t.Fatalf("panel should be hidden after toggling off")
	}
	if strings.Contains(plain(m.View()), "Details") {
		t.Fatalf("Details pane should be gone after toggling off")
	}

	// Tab shows it again.
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !m.detailShown || !m.layout.detailVisible {
		t.Fatalf("panel should be visible again after second Tab (shown=%v visible=%v)",
			m.detailShown, m.layout.detailVisible)
	}

	// Width stays exactly right in both states.
	assertExactWidth(t, "hidden", plain(m.View()), 150)
}

func TestListVisibleRowsMatchesLayout(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(120, 30)
	if m.List.VisibleRows != m.layout.listRows {
		t.Fatalf("VisibleRows %d != listRows %d", m.List.VisibleRows, m.layout.listRows)
	}
	if m.List.GetVisibleRowsCount() != m.layout.listRows {
		t.Fatalf("GetVisibleRowsCount %d != listRows %d", m.List.GetVisibleRowsCount(), m.layout.listRows)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("abc", 5); got != "abc" {
		t.Errorf("truncate short = %q", got)
	}
	if got := width(truncate("hello world", 5)); got > 5 {
		t.Errorf("truncate width = %d, want <= 5", got)
	}
	styled := "\x1b[31mhello world\x1b[0m"
	if got := width(truncate(styled, 5)); got > 5 {
		t.Errorf("truncate styled width = %d, want <= 5", got)
	}
}

func TestGradientTextPreservesWidth(t *testing.T) {
	for _, s := range []string{"", "a", "Blender", "◈ Blender"} {
		if got, want := width(gradientText(s, "#ff79c6", "#8be9fd")), utf8.RuneCountInString(s); got != want {
			t.Errorf("gradientText(%q) width = %d, want %d", s, got, want)
		}
	}
}

func TestGetBuildColumnsSumToWidth(t *testing.T) {
	desired := []int{9, 13, 8, 7, 12, 8, 16}
	for _, w := range []int{39, 60, 100, 160} {
		cols := GetBuildColumns(w, desired)
		if got := sumColumnWidths(cols); got != w {
			t.Errorf("GetBuildColumns(%d) sum = %d", w, got)
		}
	}
}

func TestColumnFitAvoidsTruncation(t *testing.T) {
	builds := sampleModel().List.Builds
	desired := columnDesiredWidths(builds, 0)
	fit := columnsFitWidth(desired)

	// At the fit width no column is narrower than it needs to be.
	for i, col := range GetBuildColumns(fit, desired) {
		if col.Width < desired[i] {
			t.Errorf("at fit=%d, column %s width %d < desired %d", fit, col.Name, col.Width, desired[i])
		}
	}

	// One cell narrower and at least one column must truncate.
	clipped := false
	for i, col := range GetBuildColumns(fit-1, desired) {
		if col.Width < desired[i] {
			clipped = true
			break
		}
	}
	if !clipped {
		t.Fatalf("expected truncation at fit-1=%d", fit-1)
	}

	// The builds pane must fill exactly the requested column width.
	if got := sumColumnWidths(GetBuildColumns(fit+7, desired)); got != fit+7 {
		t.Errorf("widths should fill requested area, got %d want %d", got, fit+7)
	}
}

func TestPaletteResolution(t *testing.T) {
	for _, name := range PaletteNames() {
		if _, ok := PaletteByName(name); !ok {
			t.Errorf("palette %q not found", name)
		}
	}
	if ResolvePalette("nope").Name != DefaultPaletteName {
		t.Errorf("unknown palette should fall back to %q", DefaultPaletteName)
	}
	if ResolvePalette("").Name != DefaultPaletteName {
		t.Errorf("empty palette should fall back to %q", DefaultPaletteName)
	}
}

func TestNewStyleFromEveryPalette(t *testing.T) {
	for _, name := range PaletteNames() {
		p := ResolvePalette(name)
		s := NewStyleFromPalette(p)
		if s.Palette.Name != name {
			t.Errorf("style palette = %q, want %q", s.Palette.Name, name)
		}
	}
}

func TestSettingsThemeSelectionAndLivePreview(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := sampleModel()
	m.UpdateWindowSize(120, 30)

	// Open settings.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if m.currentView != viewSettings {
		t.Fatalf("expected settings view, got %v", m.currentView)
	}
	if m.Settings.Theme != "dracula" {
		t.Fatalf("initial theme = %q, want dracula", m.Settings.Theme)
	}

	// Move focus to the Theme row.
	for i := 0; i < len(m.Settings.Inputs)+1; i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if want := len(m.Settings.Inputs) + 1; m.Settings.FocusIndex != want {
		t.Fatalf("focus = %d, want %d", m.Settings.FocusIndex, want)
	}

	// Cycle to the next palette; the live UI must update immediately.
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.Settings.Theme != "gruvbox" {
		t.Fatalf("theme after cycle = %q, want gruvbox", m.Settings.Theme)
	}
	if m.Style.Palette.Name != "gruvbox" {
		t.Fatalf("model style palette = %q, want gruvbox (live preview)", m.Style.Palette.Name)
	}
	if m.Settings.Style.Palette.Name != "gruvbox" || m.List.Style.Palette.Name != "gruvbox" {
		t.Fatalf("sub-model styles not updated: settings=%q list=%q",
			m.Settings.Style.Palette.Name, m.List.Style.Palette.Name)
	}

	// Left should wrap back to dracula.
	m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.Settings.Theme != "dracula" {
		t.Fatalf("theme after left = %q, want dracula", m.Settings.Theme)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRight})

	// Saving persists the selection.
	if err := m.SaveSettings(); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if m.config.Theme != "gruvbox" {
		t.Fatalf("config theme = %q, want gruvbox", m.config.Theme)
	}
	if _, _, _, theme := m.Settings.GetValues(); theme != "gruvbox" {
		t.Fatalf("GetValues theme = %q, want gruvbox", theme)
	}
}

func TestRenderAllPalettes(t *testing.T) {
	for _, name := range PaletteNames() {
		m := sampleModel()
		m.config.Theme = name
		m.setTheme(ResolvePalette(name))
		m.UpdateWindowSize(130, 34)
		assertExactWidth(t, "palette "+name, plain(m.View()), 130)
	}
}

func TestSettingsSetValuesNormalizesTheme(t *testing.T) {
	cfg := config.Config{DownloadDir: "/tmp/x", BuildType: "daily", Theme: "dracula"}
	s := NewSettingsModel(cfg, NewStyle())
	s.SetValues("/tmp/y", "4.2", "patch", "does-not-exist")
	if s.Theme != DefaultPaletteName {
		t.Fatalf("unknown theme normalized to %q, want %q", s.Theme, DefaultPaletteName)
	}
	if s.ThemeIndex < 0 || s.ThemeIndex >= len(s.ThemeOptions) || s.ThemeOptions[s.ThemeIndex] != s.Theme {
		t.Fatalf("ThemeIndex %d out of sync with Theme %q", s.ThemeIndex, s.Theme)
	}
}
