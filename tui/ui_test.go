package tui

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/dshot92/TUI-Blender-Launcher/config"
	"github.com/dshot92/TUI-Blender-Launcher/model"

	tea "github.com/charmbracelet/bubbletea"
	lp "github.com/charmbracelet/lipgloss"
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

// showInfoPanel enables the build info pane for tests that exercise its layout.
func showInfoPanel(m *Model) {
	m.detailShown = true
	m.computeLayout()
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

func TestFrameHeightFitsTerminal(t *testing.T) {
	sizes := [][2]int{{150, 40}, {120, 30}, {80, 24}, {45, 12}}
	for _, sz := range sizes {
		m := sampleModel()
		m.UpdateWindowSize(sz[0], sz[1])
		lines := strings.Split(plain(m.View()), "\n")
		if len(lines) != sz[1] {
			t.Fatalf("%dx%d: frame has %d lines, want %d", sz[0], sz[1], len(lines), sz[1])
		}
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
	showInfoPanel(m)
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
	showInfoPanel(m)
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
	showInfoPanel(m)
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
	if m.detailShown || m.layout.detailVisible {
		t.Fatalf("panel should start hidden (shown=%v visible=%v)",
			m.detailShown, m.layout.detailVisible)
	}
	if strings.Contains(plain(m.View()), "Details") {
		t.Fatalf("Details pane should be absent on launch")
	}

	// Tab shows it.
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !m.detailShown {
		t.Fatalf("detailShown should be true after Tab")
	}
	if !m.layout.detailVisible {
		t.Fatalf("panel should be visible after toggling on")
	}
	if !strings.Contains(plain(m.View()), "Details") {
		t.Fatalf("expected Details pane after toggling on")
	}

	// Tab hides it again.
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.detailShown || m.layout.detailVisible {
		t.Fatalf("panel should be hidden again after second Tab (shown=%v visible=%v)",
			m.detailShown, m.layout.detailVisible)
	}
	if strings.Contains(plain(m.View()), "Details") {
		t.Fatalf("Details pane should be gone after toggling off")
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
	for i := 0; i < settingTheme; i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.Settings.FocusIndex != settingTheme {
		t.Fatalf("focus = %d, want %d", m.Settings.FocusIndex, settingTheme)
	}

	// Enter must not advance an option row; only left/right changes it.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Settings.Theme != "dracula" {
		t.Fatalf("enter should not change theme, got %q", m.Settings.Theme)
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
	if v := m.Settings.GetValues(); v.Theme != "gruvbox" {
		t.Fatalf("GetValues theme = %q, want gruvbox", v.Theme)
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
	s.SetValues(SettingsValues{DownloadDir: "/tmp/y", VersionFilter: "4.2", BuildType: "patch", Theme: "does-not-exist"})
	if s.Theme != DefaultPaletteName {
		t.Fatalf("unknown theme normalized to %q, want %q", s.Theme, DefaultPaletteName)
	}
	if s.ThemeIndex < 0 || s.ThemeIndex >= len(s.ThemeOptions) || s.ThemeOptions[s.ThemeIndex] != s.Theme {
		t.Fatalf("ThemeIndex %d out of sync with Theme %q", s.ThemeIndex, s.Theme)
	}
}

func TestSettingsModalShowsListAndDetail(t *testing.T) {
	m := sampleModel()
	m.currentView = viewSettings
	m.UpdateWindowSize(120, 34)

	out := plain(m.View())

	// The list and the detail pane for the focused setting must both render.
	if !strings.Contains(out, "Download Directory") {
		t.Fatalf("expected the settings list in the modal:\n%s", out)
	}
	if !strings.Contains(out, "Value:") {
		t.Fatalf("expected a value line in the detail pane:\n%s", out)
	}
	if !strings.Contains(out, "Folder where downloaded Blender builds") {
		t.Fatalf("expected the setting description in the detail pane:\n%s", out)
	}
}

func TestSettingsModalReplacesChrome(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(120, 34)
	if !strings.Contains(plain(m.View()), "Blender") {
		t.Fatalf("list view should show the header wordmark")
	}

	m.currentView = viewSettings
	if strings.Contains(plain(m.View()), "◈ Blender") {
		t.Fatalf("settings modal should replace the header chrome:\n%s", plain(m.View()))
	}
}

func TestSettingsCompactStacksPanesWhenNarrow(t *testing.T) {
	m := sampleModel()
	m.currentView = viewSettings
	m.UpdateWindowSize(60, 24)

	out := plain(m.View())
	listLine := lineContaining(out, "Download Directory")
	detailLine := lineContaining(out, "Folder where downloaded")
	if listLine < 0 || detailLine < 0 {
		t.Fatalf("expected both panes to render:\n%s", out)
	}
	if detailLine <= listLine {
		t.Fatalf("detail should render below the list when stacked (list@%d detail@%d)", listLine, detailLine)
	}
	assertExactWidth(t, "settings-narrow", out, 60)
}

func TestSettingsEnterOnOptionIsInert(t *testing.T) {
	m := sampleModel()
	m.currentView = viewSettings
	m.UpdateWindowSize(120, 34)
	m.Settings.SetValues(SettingsValues{DownloadDir: "/tmp/blender", BuildType: "daily", Theme: "dracula"})

	// Move to the Theme row and press Enter: option rows are changed with
	// left/right, so Enter must neither enter edit mode nor cycle the value.
	for i := 0; i < settingTheme; i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.Settings.EditMode {
		t.Fatalf("option rows must not enter edit mode")
	}
	if m.Settings.Theme != "dracula" {
		t.Fatalf("Enter must not change an option, got theme %q", m.Settings.Theme)
	}
}

func TestSettingsEscStopsEditing(t *testing.T) {
	m := sampleModel()
	m.currentView = viewSettings
	m.UpdateWindowSize(120, 34)
	m.Settings.SetValues(SettingsValues{DownloadDir: "/tmp/original", BuildType: "daily", Theme: "dracula"})

	// Enter edit mode on the first input and change the value.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Settings.EditMode {
		t.Fatalf("expected to be editing after Enter")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	if m.Settings.Inputs[0].Value() == "/tmp/original" {
		t.Fatalf("expected typing to change the input value")
	}

	// Esc stops editing and discards the change.
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.Settings.EditMode {
		t.Fatalf("expected Esc to stop editing")
	}
	if m.currentView != viewSettings {
		t.Fatalf("Esc must not leave the settings view")
	}
	if got := m.Settings.Inputs[0].Value(); got != "/tmp/original" {
		t.Fatalf("Esc should revert the edit, got %q", got)
	}
}

func TestSettingsEnterCommitsEdit(t *testing.T) {
	m := sampleModel()
	m.currentView = viewSettings
	m.UpdateWindowSize(120, 34)
	m.Settings.SetValues(SettingsValues{DownloadDir: "/tmp/original", BuildType: "daily", Theme: "dracula"})

	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.Settings.EditMode {
		t.Fatalf("Enter should leave edit mode")
	}
	got := m.Settings.Inputs[0].Value()
	if got == "/tmp/original" || !strings.Contains(got, "X") || len(got) != len("/tmp/original")+1 {
		t.Fatalf("Enter should keep the edit, got %q", got)
	}
}

func TestSettingsEditHintsShowBothEscAndEnter(t *testing.T) {
	m := sampleModel()
	m.currentView = viewSettings
	m.UpdateWindowSize(120, 34)
	m.Settings.SetValues(SettingsValues{DownloadDir: "/tmp/original", BuildType: "daily", Theme: "dracula"})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	keys := map[string]bool{}
	for _, h := range m.settingsHints() {
		keys[h.key] = true
	}
	if !keys["esc"] || !keys["enter"] {
		t.Fatalf("editing hints should offer both esc and enter, got %+v", m.settingsHints())
	}
}

// TestSettingsHintsMatchFocusedRow verifies the help line only advertises
// commands that apply to the focused row (notably, left/right is offered for
// option rows but not for text fields), and that the clean command is gone.
func TestSettingsHintsMatchFocusedRow(t *testing.T) {
	m := sampleModel()
	m.currentView = viewSettings
	m.Settings.SetValues(SettingsValues{DownloadDir: "/tmp/original", BuildType: "daily", Theme: "dracula"})

	hasHint := func(hints []hint, key, label string) bool {
		for _, h := range hints {
			if h.key == key && h.label == label {
				return true
			}
		}
		return false
	}
	hasClean := func(hints []hint) bool {
		for _, h := range hints {
			if h.label == "clean" || h.cmd == CmdCleanOldBuilds {
				return true
			}
		}
		return false
	}

	// Text rows can only be edited; left/right must not be advertised.
	m.Settings.FocusIndex = settingDownloadDir
	hints := m.settingsHints()
	if !hasHint(hints, "enter", "edit") {
		t.Fatalf("text row should offer edit, got %+v", hints)
	}
	if hasHint(hints, "←/→", "change") {
		t.Fatalf("text row must not advertise left/right change, got %+v", hints)
	}
	if hasClean(hints) {
		t.Fatalf("settings must not offer clean, got %+v", hints)
	}

	// Option rows change their value with left/right.
	m.Settings.FocusIndex = settingBuildType
	hints = m.settingsHints()
	if !hasHint(hints, "←/→", "change") {
		t.Fatalf("option row should advertise left/right change, got %+v", hints)
	}
	if hasHint(hints, "enter", "next") {
		t.Fatalf("option row must not advertise an enter action, got %+v", hints)
	}
	if hasClean(hints) {
		t.Fatalf("settings must not offer clean, got %+v", hints)
	}

	// Toggling release types advertises left/right to move the cursor.
	m.Settings.FocusIndex = settingReleaseFilter
	m.Settings.releaseEditing = true
	hints = m.settingsHints()
	if !hasHint(hints, "←/→", "move") || !hasHint(hints, "space", "toggle") {
		t.Fatalf("release toggling should advertise move and toggle, got %+v", hints)
	}
	if hasClean(hints) {
		t.Fatalf("settings must not offer clean, got %+v", hints)
	}
}

func TestSettingsInputsAreThemed(t *testing.T) {
	cfg := config.Config{DownloadDir: "/tmp/x", BuildType: "daily", Theme: "nord"}
	s := NewSettingsModel(cfg, NewStyle())

	if s.Inputs[0].Prompt != "❯ " {
		t.Fatalf("input prompt = %q, want themed prompt", s.Inputs[0].Prompt)
	}
	if s.Inputs[0].Value() != "/tmp/x" {
		t.Fatalf("input value = %q, want /tmp/x", s.Inputs[0].Value())
	}

	// Entering edit mode focuses the input, which must show a visible block
	// cursor (the cursor's Blink flag drives which style is used).
	s.EditMode = true
	s.FocusIndex = settingDownloadDir
	s.updateFocusStyles()
	if s.Inputs[0].Cursor.Blink {
		t.Fatalf("focused input cursor should be a visible block")
	}
}

func TestToastPanelShowsStatusInBothViews(t *testing.T) {
	for _, view := range []struct {
		name  string
		state viewState
	}{
		{"list", viewList},
		{"settings", viewSettings},
	} {
		m := sampleModel()
		m.currentView = view.state
		m.UpdateWindowSize(120, 34)
		m.status = "no old builds to clean"

		out := plain(m.View())
		if !strings.Contains(out, "no old builds to clean") {
			t.Fatalf("%s: toast missing status text:\n%s", view.name, out)
		}
		if !strings.Contains(out, "✓") {
			t.Fatalf("%s: toast missing success glyph:\n%s", view.name, out)
		}
		assertExactWidth(t, view.name+" toast", out, 120)
	}
}

func TestToastPrefersErrorOverStatus(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(120, 34)
	m.status = "all good"
	m.err = errors.New("download failed")

	out := plain(m.View())
	if !strings.Contains(out, "download failed") {
		t.Fatalf("toast missing error text:\n%s", out)
	}
	if strings.Contains(out, "all good") {
		t.Fatalf("status should be hidden while an error is shown:\n%s", out)
	}
	if !strings.Contains(out, "✖") {
		t.Fatalf("toast missing error glyph:\n%s", out)
	}
	assertExactWidth(t, "error toast", out, 120)
}

func TestToastHiddenWithoutMessage(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(120, 34)

	if _, ok := m.renderToast(m.terminalWidth - 2); ok {
		t.Fatal("toast should not render without a status or error")
	}
	out := plain(m.View())
	if strings.Contains(out, "✓") || strings.Contains(out, "✖") {
		t.Fatalf("no toast expected in a clean frame:\n%s", out)
	}
}

func TestToastRespectsTheme(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(120, 34)
	m.status = "hello"

	for _, name := range PaletteNames() {
		p := ResolvePalette(name)
		m.setTheme(p)

		text, border, ok := m.toastMessage()
		if !ok || text == "" {
			t.Fatalf("%s: expected a themed status message", name)
		}
		if want := lp.Color(p.Accent); border != want {
			t.Fatalf("%s: status border = %q, want accent %q", name, border, want)
		}
		if lp.Color(p.Accent) == lp.Color(p.Success) {
			t.Fatalf("%s: accent should differ from the green success color", name)
		}
	}

	// Errors use the palette's error color instead.
	m.setTheme(ResolvePalette("nord"))
	m.err = errors.New("boom")
	_, border, ok := m.toastMessage()
	if !ok {
		t.Fatal("expected an error toast")
	}
	if want := lp.Color(ResolvePalette("nord").Error); border != want {
		t.Fatalf("error border = %q, want %q", border, want)
	}
}

func TestToastLongStatusFitsWidth(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(60, 20)
	m.status = strings.Repeat("wide message ", 20)

	assertExactWidth(t, "long toast", plain(m.View()), 60)
}

func TestOverlayBlockPreservesContent(t *testing.T) {
	base := "ABCDEFGHIJ"
	block := "\x1b[31mXY\x1b[0m" // 2 columns wide once stripped

	got := overlayBlock(base, block, 4, 0, 10)
	if plain(got) != "ABCDXYGHIJ" {
		t.Fatalf("overlay = %q, want %q", plain(got), "ABCDXYGHIJ")
	}
	if width(got) != 10 {
		t.Fatalf("overlay width = %d, want 10", width(got))
	}

	// Right-aligned clamping must not overflow the line either.
	got = overlayBlock(base, block, 100, 0, 10)
	if width(got) != 10 {
		t.Fatalf("clamped overlay width = %d, want 10", width(got))
	}
}

func TestToastKeepsFooterBehindIt(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(120, 34)
	m.status = "no old builds to clean"

	out := plain(m.View())
	if !strings.Contains(out, "no old builds to clean") {
		t.Fatalf("toast missing:\n%s", out)
	}
	// Footer hints to the left of the centered panel must remain visible.
	if !strings.Contains(out, "Fetch") {
		t.Fatalf("footer content behind the toast was erased:\n%s", out)
	}
	assertExactWidth(t, "overlay keeps width", out, 120)
}

func TestVersionFilterLiveValidation(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(120, 34)

	// Open settings and focus the Version Filter row.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	for i := 0; i < settingVersionFilter; i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.Settings.FocusIndex != settingVersionFilter {
		t.Fatalf("focus = %d, want version filter", m.Settings.FocusIndex)
	}

	// Enter edit mode and type a malformed value.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Settings.EditMode {
		t.Fatal("expected to be editing the version filter")
	}
	for _, r := range "abc" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	if m.filterErr == nil {
		t.Fatal("expected a live validation error for an invalid filter")
	}
	out := plain(m.View())
	if !strings.Contains(out, "invalid version filter") {
		t.Fatalf("toast should show the invalid version text:\n%s", out)
	}
	assertExactWidth(t, "invalid filter toast", out, 120)

	// Correcting to a valid value clears the message while still editing.
	for i := 0; i < 3; i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "5.x" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if m.filterErr != nil {
		t.Fatalf("valid filter should clear the error, got %v", m.filterErr)
	}
	if strings.Contains(plain(m.View()), "invalid version filter") {
		t.Fatalf("toast should be gone for a valid filter:\n%s", plain(m.View()))
	}

	// Reintroduce the error, then move off the row: it must clear.
	for _, r := range "abc" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if m.filterErr == nil {
		t.Fatal("expected the error to return for an invalid filter")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // commit the edit
	m.Update(tea.KeyMsg{Type: tea.KeyDown})  // move to the next row
	if m.filterErr != nil {
		t.Fatalf("filter error should clear after leaving the row: %v", m.filterErr)
	}
}

// TestEmptyBuildsPaneCentersMessage verifies that an empty build list shows only
// the hint message (no column header) and that the message sits on the vertical
// midpoint of the pane's inner area.
func TestEmptyBuildsPaneCentersMessage(t *testing.T) {
	cfg := config.Config{DownloadDir: "/tmp/blender", BuildType: "daily", Theme: "dracula"}
	m := InitialModel(cfg, false)
	m.List.Builds = nil
	m.UpdateWindowSize(100, 30)
	m.computeLayout()

	boxWidth := m.layout.listBoxWidth
	boxHeight := m.layout.listBoxHeight
	out := plain(m.renderBuildsPane(boxWidth, boxHeight))
	lines := strings.Split(out, "\n")

	if len(lines) != boxHeight {
		t.Fatalf("pane has %d lines, want %d", len(lines), boxHeight)
	}

	msgRow := -1
	for i, line := range lines {
		if strings.Contains(line, "No Blender builds yet") {
			msgRow = i
			break
		}
	}
	if msgRow < 0 {
		t.Fatalf("empty-state message not found:\n%s", out)
	}

	// The column header must not be drawn while the list is empty.
	if strings.Contains(out, "Version") || strings.Contains(out, "Branch") {
		t.Fatalf("column header should be hidden when there are no builds:\n%s", out)
	}

	// Rows: one top border, then the inner area. The message should line up with
	// the middle of the inner area (upper-middle row for an even inner height).
	innerHeight := boxHeight - boxBorderFrameHeight
	want := 1 + (innerHeight-1)/2
	if msgRow != want {
		t.Fatalf("message on row %d, want the vertical midpoint %d:\n%s", msgRow, want, out)
	}
}
