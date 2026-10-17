package tui

import (
	"strings"
	"testing"

	"github.com/dshot92/TUI-Blender-Launcher/model"

	tea "github.com/charmbracelet/bubbletea"
)

// headerColumnX returns the x coordinate at the centre of column i on the
// builds table header row.
func headerColumnX(m *Model, i int) int {
	cols := m.columnsLayout()
	x := 1 + panePadX + gutterWidth
	for j := 0; j < i && j < len(cols); j++ {
		x += cols[j].Width
	}
	return x + cols[i].Width/2
}

func click(model *Model, x, y int, button tea.MouseButton) {
	model.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: button,
		X:      x,
		Y:      y,
	})
}

func TestColumnHeaderHitTest(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(150, 40)

	y := headerBoxHeight + 1
	cols := m.columnsLayout()
	for i := range cols {
		x := headerColumnX(m, i)
		got, ok := m.columnAt(x, y)
		if !ok || got != i {
			t.Fatalf("columnAt(%d,%d) = (%d,%v), want (%d,true)", x, y, got, ok, i)
		}
	}

	// A data row is not a header.
	if _, ok := m.columnAt(10, headerBoxHeight+2); ok {
		t.Fatalf("data row must not hit a header column")
	}
	// To the left of the first column (border/padding) is a miss.
	if _, ok := m.columnAt(0, y); ok {
		t.Fatalf("click left of the first column must miss")
	}
}

func TestSortByColumnTogglesAndSwitches(t *testing.T) {
	m := sampleModel()
	m.List.SortColumn = 0
	m.List.SortReversed = true

	// Re-clicking the active column flips the direction.
	m.List.SortByColumn(0)
	if m.List.SortReversed {
		t.Fatalf("re-clicking the active column should sort ascending")
	}
	m.List.SortByColumn(0)
	if !m.List.SortReversed {
		t.Fatalf("second re-click should sort descending again")
	}

	// Switching columns selects it and starts ascending.
	m.List.SortByColumn(5)
	if m.List.SortColumn != 5 || m.List.SortReversed {
		t.Fatalf("switch to column 5 = (column %d, reversed %v), want (5,false)",
			m.List.SortColumn, m.List.SortReversed)
	}

	// Ascending size order is honoured.
	for i := 1; i < len(m.List.Builds); i++ {
		if m.List.Builds[i-1].Size > m.List.Builds[i].Size {
			t.Fatalf("builds not sorted ascending by size: %d > %d",
				m.List.Builds[i-1].Size, m.List.Builds[i].Size)
		}
	}
}

func TestHeaderClickSortsViaMouse(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(150, 40)

	const sizeColumn = 5
	x := headerColumnX(m, sizeColumn)
	click(m, x, headerBoxHeight+1, tea.MouseButtonLeft)

	if m.List.SortColumn != sizeColumn {
		t.Fatalf("clicking the Size header set SortColumn=%d, want %d", m.List.SortColumn, sizeColumn)
	}
	if m.List.SortReversed {
		t.Fatalf("first click on a new column should sort ascending")
	}

	// A second click on the same header reverses the order.
	click(m, x, headerBoxHeight+1, tea.MouseButtonLeft)
	if !m.List.SortReversed {
		t.Fatalf("second click should sort the active column descending")
	}
}

func TestLeftClickSelectsBuildRow(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(150, 40)
	m.List.Cursor = 0

	// The third visible data row is at y = header + 2 + 2.
	click(m, 5, headerBoxHeight+2+2, tea.MouseButtonLeft)
	if m.List.Cursor != 2 {
		t.Fatalf("clicking the third row selected cursor %d, want 2", m.List.Cursor)
	}
}

func TestWheelScrollsBuildList(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(150, 40)
	m.List.Cursor = 0

	m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	if m.List.Cursor != 1 {
		t.Fatalf("wheel down moved cursor to %d, want 1", m.List.Cursor)
	}
	m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	if m.List.Cursor != 0 {
		t.Fatalf("wheel up moved cursor to %d, want 0", m.List.Cursor)
	}
}

func TestContextMenuForLiveDownloadOffersCancel(t *testing.T) {
	m := sampleModel()
	// The download manager reports a live transfer even if the list status has
	// not caught up yet.
	build := m.List.Builds[0] // StateOnline in the sample list
	build.Status = model.StateOnline
	m.commands.downloads.states[build.ID()] = &model.DownloadState{
		BuildID:    build.ID(),
		BuildState: model.StateDownloading,
	}

	if !m.isDownloading(build) {
		t.Fatalf("live manager transfer should report as downloading")
	}
	items := m.contextMenuItemsFor(build)
	if len(items) != 1 || items[0].cmd != CmdDeleteBuild {
		t.Fatalf("live download menu = %+v, want a single Cancel entry", items)
	}
}

func TestContextMenuOpenAndExecute(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(150, 40)

	// Right-click the first row to open the menu.
	click(m, 5, headerBoxHeight+2, tea.MouseButtonRight)
	if !m.contextMenuVisible {
		t.Fatalf("right click on a row should open the context menu")
	}
	if m.contextMenuBuildIdx != 0 {
		t.Fatalf("context menu opened on build %d, want 0", m.contextMenuBuildIdx)
	}

	// Esc dismisses it without acting.
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.contextMenuVisible {
		t.Fatalf("esc should dismiss the context menu")
	}
}

func TestFooterHintClickRunsCommand(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(150, 40)

	// The bottom footer line starts with the Fetch hint.
	y := m.terminalHeight - 1
	cmd, ok := m.footerCommandAt(0, y)
	if !ok || cmd != CmdFetchBuilds {
		t.Fatalf("footerCommandAt(0,%d) = (%v,%v), want (CmdFetchBuilds,true)", y, cmd, ok)
	}

	// A click well above the footer is not a hint.
	if _, ok := m.footerCommandAt(0, 0); ok {
		t.Fatalf("click above the footer should not map to a command")
	}
}

func TestDoubleClickLocalBuildLaunches(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(150, 40)

	localIdx := -1
	for i, b := range m.List.Builds {
		if b.Status == model.StateLocal {
			localIdx = i
			break
		}
	}
	if localIdx < 0 {
		t.Fatalf("sample model has no local build")
	}

	x := 5
	y := headerBoxHeight + 2 + localIdx
	click(m, x, y, tea.MouseButtonLeft) // first click only selects
	_, cmd := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: x, Y: y})
	if cmd == nil {
		t.Fatalf("double click on a local build should produce a launch command")
	}
}

func TestDoubleClickOnlineBuildDoesNotLaunch(t *testing.T) {
	m := sampleModel()
	m.UpdateWindowSize(150, 40)

	// The first sample build is online.
	x := 5
	y := headerBoxHeight + 2
	click(m, x, y, tea.MouseButtonLeft)
	_, cmd := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: x, Y: y})
	if cmd != nil {
		t.Fatalf("double click on a non-local build should not launch")
	}
}

// settingsModel returns a model showing the settings modal at a size wide
// enough for the two-column layout.
func settingsModel(t *testing.T) *Model {
	t.Helper()
	m := sampleModel()
	m.currentView = viewSettings
	m.UpdateWindowSize(120, 40)
	return m
}

func TestSettingsMouseFocusesListRow(t *testing.T) {
	m := settingsModel(t)
	lay := m.settingsModalLayout()
	click(m, lay.contentX+2, lay.contentY+settingBuildType, tea.MouseButtonLeft)
	if m.Settings.FocusIndex != settingBuildType {
		t.Fatalf("clicking the Build Channel row focused %d, want %d", m.Settings.FocusIndex, settingBuildType)
	}
}

func TestSettingsMouseSelectsBuildTypeOption(t *testing.T) {
	m := settingsModel(t)
	m.Settings.FocusIndex = settingBuildType

	const target = "patch"
	idx := -1
	for i, o := range m.Settings.BuildTypeOptions {
		if o == target {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatalf("build type option %q missing", target)
	}

	lay := m.settingsModalLayout()
	rel := 0
	for i := 0; i < idx; i++ {
		rel += width(m.Settings.BuildTypeOptions[i]) + 2
	}
	rel += (width(target) + 2) / 2
	x := lay.contentX + lay.listW + 3 + 1 + 7 + rel

	click(m, x, lay.contentY+2, tea.MouseButtonLeft)
	if m.Settings.BuildType != target {
		t.Fatalf("clicking the %q chip set BuildType=%q", target, m.Settings.BuildType)
	}
}

func TestSettingsMouseEditsTextField(t *testing.T) {
	m := settingsModel(t)
	m.Settings.FocusIndex = settingVersionFilter

	lay := m.settingsModalLayout()
	x := lay.contentX + lay.listW + 3 + 1 + 7 + 2
	click(m, x, lay.contentY+2, tea.MouseButtonLeft)
	if !m.Settings.EditMode {
		t.Fatalf("clicking a text field should start editing")
	}
}

func TestSettingsMouseHelpHintSaves(t *testing.T) {
	m := settingsModel(t)
	lay := m.settingsModalLayout()
	hints := m.settingsHints()
	help := truncate(m.renderHints(hints), lay.innerW)
	left := lay.boxX + 1 + (lay.innerW-width(help))/2

	x := hintX(hints, left, CmdSaveSettings)
	if x < 0 {
		t.Fatalf("save hint not found")
	}
	click(m, x, lay.helpY, tea.MouseButtonLeft)
	if m.currentView != viewList {
		t.Fatalf("clicking save should return to the list view, got view %d", m.currentView)
	}
}

// hintX returns an x coordinate inside the segment for the given command in a
// rendered hint line that starts at lineStart.
func hintX(hints []hint, lineStart int, target CommandType) int {
	cursor := lineStart
	sep := width("  ·  ")
	for i, h := range hints {
		seg := h.label
		if h.key != "" {
			seg = h.key + " " + h.label
		}
		w := width(seg)
		if h.cmd == target {
			return cursor + w/2
		}
		cursor += w
		if i < len(hints)-1 {
			cursor += sep
		}
	}
	return -1
}

// openDeleteConfirm returns a model with the delete confirmation open on a local
// build.
func openDeleteConfirm(t *testing.T) *Model {
	t.Helper()
	m := sampleModel()
	m.UpdateWindowSize(150, 40)

	localIdx := -1
	for i, b := range m.List.Builds {
		if b.Status == model.StateLocal {
			localIdx = i
			break
		}
	}
	if localIdx < 0 {
		t.Fatalf("sample model has no local build")
	}
	m.List.Cursor = localIdx

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if !m.deleteConfirm {
		t.Fatalf("pressing x on a local build should open the confirmation")
	}
	return m
}

func TestDeleteConfirmDefaultsToNo(t *testing.T) {
	m := openDeleteConfirm(t)
	if m.deleteConfirmChoice != confirmChoiceNo {
		t.Fatalf("confirmation default choice = %d, want No (%d)", m.deleteConfirmChoice, confirmChoiceNo)
	}

	// Enter with the default must not delete anything.
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Fatalf("enter on the default (No) should not produce a delete command")
	}
	if m.deleteConfirm {
		t.Fatalf("enter should dismiss the confirmation")
	}
}

func TestDeleteConfirmButtonsOrderedNoThenYes(t *testing.T) {
	m := openDeleteConfirm(t)
	lay := m.deleteConfirmLayout()
	if lay.noX >= lay.yesX {
		t.Fatalf("expected No left of Yes, got noX=%d yesX=%d", lay.noX, lay.yesX)
	}
	out := plain(m.View())
	if i, j := strings.Index(out, confirmNoLabel), strings.Index(out, confirmYesLabel); i < 0 || j < 0 || i > j {
		t.Fatalf("expected [ No ] before [ Yes ] in the overlay:\n%s", out)
	}
}

func TestDeleteConfirmMouseNoCancels(t *testing.T) {
	m := openDeleteConfirm(t)
	lay := m.deleteConfirmLayout()
	_, cmd := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
		X: lay.noX + 1, Y: lay.buttonY,
	})
	if cmd != nil {
		t.Fatalf("clicking No should not delete")
	}
	if m.deleteConfirm {
		t.Fatalf("clicking No should dismiss the confirmation")
	}
}

func TestDeleteConfirmMouseYesDeletes(t *testing.T) {
	m := openDeleteConfirm(t)
	lay := m.deleteConfirmLayout()
	_, cmd := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
		X: lay.yesX + 1, Y: lay.buttonY,
	})
	if cmd == nil {
		t.Fatalf("clicking Yes should produce a delete command")
	}
	if m.deleteConfirm {
		t.Fatalf("clicking Yes should dismiss the confirmation")
	}
}

func TestDeleteConfirmMouseIgnoresBackground(t *testing.T) {
	m := openDeleteConfirm(t)
	// A click well away from the buttons leaves the confirmation open.
	_, cmd := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
		X: 1, Y: 1,
	})
	if cmd != nil || !m.deleteConfirm {
		t.Fatalf("background click should be ignored (cmd=%v, open=%v)", cmd, m.deleteConfirm)
	}
}
