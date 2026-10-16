package tui

import (
	"strings"

	"github.com/dshot92/TUI-Blender-Launcher/config"
	"github.com/dshot92/TUI-Blender-Launcher/model"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	lp "github.com/charmbracelet/lipgloss"
)

// Settings rows. The order matches the focus order (FocusIndex) so navigation
// and rendering stay in sync; the two text inputs always occupy the first
// indices, followed by the Build Channel, Release Types and Theme option rows.
const (
	settingDownloadDir = iota
	settingVersionFilter
	settingBuildType
	settingReleaseFilter
	settingTheme
	settingCount
)

// SettingsModel handles the state and logic for the settings view. The view is
// rendered as a left list / right detail layout (list on top when narrow), with
// the selected setting's value and description shown in the detail pane.
type SettingsModel struct {
	Inputs             []textinput.Model
	FocusIndex         int
	EditMode           bool
	BuildType          string
	BuildTypeOptions   []string
	BuildTypeIndex     int
	ReleaseTypes       []string
	ReleaseTypeOptions []string
	ReleaseCursor      int
	releaseEditing     bool
	releaseBackup      []string
	Theme              string
	ThemeOptions       []string
	ThemeIndex         int
	Style              Style
	Config             config.Config
	width              int

	// editBackup holds the value an input had when editing began, so Esc can
	// discard the edit and revert.
	editBackup string

	// Tab-completion state for the Download Directory field: the candidates
	// offered by the current session, the value the session started from, and
	// the index of the candidate currently shown (-1 while the value holds a
	// shared prefix rather than a full match).
	dirMatches      []string
	dirCompleteBase string
	dirMatchIdx     int
}

// SettingsValues is a snapshot of the settings a user can edit. Building it
// (and applying it) through a struct keeps call sites stable as rows are added.
type SettingsValues struct {
	DownloadDir   string
	VersionFilter string
	BuildType     string
	ReleaseTypes  []string
	Theme         string
}

// SettingsValuesFromConfig snapshots the editable settings out of cfg.
func SettingsValuesFromConfig(cfg config.Config) SettingsValues {
	return SettingsValues{
		DownloadDir:   cfg.DownloadDir,
		VersionFilter: cfg.VersionFilter,
		BuildType:     cfg.BuildType,
		ReleaseTypes:  cfg.ReleaseTypes,
		Theme:         cfg.Theme,
	}
}

// settingLabel returns the display label for a settings row.
func settingLabel(i int) string {
	switch i {
	case settingDownloadDir:
		return "Download Directory"
	case settingVersionFilter:
		return "Version Filter"
	case settingBuildType:
		return "Build Channel"
	case settingReleaseFilter:
		return "Release Types"
	case settingTheme:
		return "Theme"
	}
	return ""
}

// settingDescription returns the help text for a settings row.
func settingDescription(i int) string {
	switch i {
	case settingDownloadDir:
		return "Folder where downloaded Blender builds are stored. Press Tab while editing to complete the path."
	case settingVersionFilter:
		return "Only show builds at or above this version (e.g., 4.0, 3.6, or 4.x). Defaults to 4.x. Leave empty to show all."
	case settingBuildType:
		return "Blender channel fetched from the BuildBot: daily, experimental or patch."
	case settingReleaseFilter:
		return "Only show builds of the selected release types. Space toggles, enter saves; none selected shows all."
	case settingTheme:
		return "Interface color palette. Applied immediately, saved on exit."
	}
	return ""
}

// NewSettingsModel creates a new SettingsModel.
func NewSettingsModel(cfg config.Config, style Style) SettingsModel {
	p := style.Palette

	m := SettingsModel{
		Config:             cfg,
		Style:              style,
		BuildTypeOptions:   model.BuildTypeNames(),
		BuildType:          cfg.BuildType,
		ReleaseTypeOptions: model.ReleaseFilterOptions(),
		ReleaseTypes:       []string(model.ParseReleaseTypes(cfg.ReleaseTypes)),
		ThemeOptions:       PaletteNames(),
		FocusIndex:         settingDownloadDir,
		EditMode:           false,
		dirMatchIdx:        -1,
	}

	// Theme the inputs so they match the active palette instead of the bubbles
	// defaults (which ship with an unstyled "> " prompt and an invisible block).
	fg := lp.NewStyle().Foreground(lp.Color(p.Foreground))
	muted := lp.NewStyle().Foreground(lp.Color(p.Muted))
	accent := lp.NewStyle().Foreground(lp.Color(p.Accent)).Bold(true)
	placeholder := lp.NewStyle().Foreground(lp.Color(p.Muted)).Italic(true)

	newInput := func(placeholderText, value string, limit int) textinput.Model {
		t := textinput.New()
		t.Prompt = "❯ "
		t.PromptStyle = accent
		t.TextStyle = fg
		t.PlaceholderStyle = placeholder
		t.Placeholder = placeholderText
		t.CharLimit = limit
		t.SetValue(value)
		// A solid, accent-tinted block cursor. Blink is disabled so the cursor
		// is visible without running an extra blink command.
		t.Cursor.Blink = false
		t.Cursor.Style = lp.NewStyle().
			Foreground(lp.Color(p.Accent)).
			Background(lp.Color(p.Background))
		t.Cursor.TextStyle = muted
		return t
	}

	m.Inputs = []textinput.Model{
		newInput(cfg.DownloadDir, cfg.DownloadDir, 256),
		newInput("e.g., 4.0 or 5.x (empty = none)", cfg.VersionFilter, 10),
	}

	// Resolve the configured build type so the selection is always valid.
	for i, opt := range m.BuildTypeOptions {
		if opt == cfg.BuildType {
			m.BuildTypeIndex = i
			break
		}
	}

	// Resolve the configured theme so the selection is always valid.
	m.setTheme(cfg.Theme)

	m.updateInputWidths()
	m.updateFocusStyles()

	return m
}

// Init initializes the model.
func (m SettingsModel) Init() tea.Cmd {
	return nil
}

// SetWidth updates the content width available to the settings view.
func (m *SettingsModel) SetWidth(w int) {
	if w < 20 {
		w = 20
	}
	m.width = w
	m.updateInputWidths()
}

// panes returns the settings list and detail widths for the current width,
// reporting whether the two panes sit side by side. Narrow terminals fall back
// to a stacked layout (list over detail).
func (m SettingsModel) panes() (listW, detailW int, twoColumn bool) {
	w := m.width
	if w <= 0 {
		w = 60
	}
	if w < settingsMinTwoColumnWidth {
		return w, w, false
	}

	listW = settingsListWidth
	if listW > w-settingsColumnGap-settingsMinDetailWidth {
		listW = w - settingsColumnGap - settingsMinDetailWidth
	}
	if listW < settingsMinListWidth {
		listW = settingsMinListWidth
	}

	detailW = w - listW - settingsColumnGap
	if detailW < settingsMinDetailWidth {
		detailW = settingsMinDetailWidth
		listW = w - detailW - settingsColumnGap
	}
	if listW < 1 {
		listW = 1
	}
	return listW, detailW, true
}

// View returns the settings content: a left list and a right detail pane when
// there is room, stacked otherwise.
func (m SettingsModel) View() string {
	p := m.Style.Palette
	listW, detailW, twoColumn := m.panes()

	dividerStyle := lp.NewStyle().Foreground(lp.Color(p.Subtle))

	list := m.renderList(listW)
	detail := m.renderDetail(detailW)

	if !twoColumn {
		divider := dividerStyle.Render(strings.Repeat("─", m.contentWidth()))
		return list + "\n" + divider + "\n" + detail
	}

	divider := dividerStyle.Render("│")
	return joinColumns(list, detail, " "+divider+" ")
}

// contentWidth returns the width of the settings content area.
func (m SettingsModel) contentWidth() int {
	if m.width <= 0 {
		return 60
	}
	return m.width
}

// renderList renders the left-hand list of settings, one row per setting with
// the focused row highlighted.
func (m SettingsModel) renderList(w int) string {
	p := m.Style.Palette
	base := lp.NewStyle().Foreground(lp.Color(p.Foreground))
	selected := lp.NewStyle().Foreground(lp.Color(p.Accent)).Bold(true)

	rows := make([]string, 0, settingCount)
	for i := 0; i < settingCount; i++ {
		style := base
		prefix := "  "
		if i == m.FocusIndex {
			style = selected
			prefix = "▸ "
		}
		line := style.Render(truncate(prefix+settingLabel(i), w))
		rows = append(rows, lp.NewStyle().Width(w).Render(line))
	}
	return strings.Join(rows, "\n")
}

// renderDetail renders the right-hand detail pane for the focused setting: the
// label, its current value (an editable input while editing), a divider, and a
// description.
func (m SettingsModel) renderDetail(w int) string {
	p := m.Style.Palette

	titleStyle := lp.NewStyle().Foreground(lp.Color(p.Accent)).Bold(true)
	labelStyle := lp.NewStyle().Foreground(lp.Color(p.Muted)).Bold(true)
	valueStyle := lp.NewStyle().Foreground(lp.Color(p.Foreground))
	descStyle := lp.NewStyle().Foreground(lp.Color(p.Muted))
	dividerStyle := lp.NewStyle().Foreground(lp.Color(p.Subtle))

	idx := m.FocusIndex
	if idx < 0 || idx >= settingCount {
		idx = 0
	}

	// The pane is inset by one column on each side for breathing room.
	contentW := w - 2*panePadX
	if contentW < minValueWidth {
		contentW = minValueWidth
	}

	lines := []string{
		" " + titleStyle.Render(settingLabel(idx)),
		"",
		" " + m.detailValue(contentW, labelStyle, valueStyle),
		"",
		" " + dividerStyle.Render(strings.Repeat("─", contentW)),
		"",
	}
	// Wrap the description to the pane width so longer hints (such as the
	// version filter examples) remain visible instead of being clipped. Always
	// emit exactly two rows (padding or truncating as needed) so navigating
	// between settings never changes the modal height and the UI does not jump.
	descLines := wrapText(settingDescription(idx), contentW)
	if len(descLines) > 2 {
		descLines = descLines[:2]
		descLines[1] = truncate(descLines[1]+"…", contentW)
	}
	for len(descLines) < 2 {
		descLines = append(descLines, "")
	}
	for _, dl := range descLines {
		lines = append(lines, descStyle.Padding(0, 1).Render(dl))
	}
	return strings.Join(lines, "\n")
}

// detailValue renders the "Value: ..." line for the focused setting.
func (m SettingsModel) detailValue(w int, labelStyle, valueStyle lp.Style) string {
	label := labelStyle.Render(valueLabel)
	avail := w - width(valueLabel)
	if avail < minValueWidth {
		avail = minValueWidth
	}

	switch m.FocusIndex {
	case settingDownloadDir, settingVersionFilter:
		if m.EditMode {
			return label + m.Inputs[m.FocusIndex].View()
		}
		v := m.Inputs[m.FocusIndex].Value()
		if strings.TrimSpace(v) == "" {
			return label + m.Style.Muted.Render("(none)")
		}
		return label + valueStyle.Render(truncate(v, avail))
	case settingBuildType:
		return label + m.renderOptions(m.BuildTypeOptions, m.BuildType, avail)
	case settingReleaseFilter:
		return label + m.renderReleaseOptions(avail)
	case settingTheme:
		return label + m.renderOptions(m.ThemeOptions, m.Theme, avail)
	}
	return label
}

// renderOptions renders a segmented control for an option setting, falling back
// to a "< current >" marker when the options do not fit the available width.
func (m SettingsModel) renderOptions(options []string, current string, avail int) string {
	p := m.Style.Palette
	active := lp.NewStyle().Padding(0, 1).Bold(true).
		Foreground(lp.Color(p.Background)).Background(lp.Color(p.Accent))
	inactive := lp.NewStyle().Padding(0, 1).Foreground(lp.Color(p.Muted))

	var b strings.Builder
	for _, o := range options {
		if o == current {
			b.WriteString(active.Render(o))
		} else {
			b.WriteString(inactive.Render(o))
		}
	}
	s := b.String()
	if width(s) > avail {
		return active.Render("< " + current + " >")
	}
	return s
}

// releaseChip markers: a two-cell checkbox prefix followed by the option label.
const (
	releaseChipOff = "▢ "
	releaseChipOn  = "▣ "
)

// releaseChip is one rendered release-type option: its full label (marker plus
// option text) and whether it is currently selected.
type releaseChip struct {
	label    string
	selected bool
}

// releaseChips builds the ordered release-type chips for the current selection.
func (m SettingsModel) releaseChips() []releaseChip {
	selected := make(map[string]bool, len(m.ReleaseTypes))
	for _, s := range m.ReleaseTypes {
		selected[s] = true
	}
	chips := make([]releaseChip, len(m.ReleaseTypeOptions))
	for i, opt := range m.ReleaseTypeOptions {
		on := selected[opt]
		mark := releaseChipOff
		if on {
			mark = releaseChipOn
		}
		chips[i] = releaseChip{label: mark + opt, selected: on}
	}
	return chips
}

// renderReleaseOptions renders the release-type multi-select as a row of
// toggle chips. When the row is in toggle mode the cursor chip is underlined;
// if the chips do not fit the available width a compact summary is shown.
func (m SettingsModel) renderReleaseOptions(avail int) string {
	p := m.Style.Palette
	on := lp.NewStyle().Padding(0, 1).Bold(true).
		Foreground(lp.Color(p.Background)).Background(lp.Color(p.Accent))
	off := lp.NewStyle().Padding(0, 1).Foreground(lp.Color(p.Muted))
	cursor := lp.NewStyle().Padding(0, 1).Bold(true).Underline(true).
		Foreground(lp.Color(p.Accent))

	var b strings.Builder
	for i, chip := range m.releaseChips() {
		style := off
		if chip.selected {
			style = on
		}
		if m.releaseEditing && i == m.ReleaseCursor {
			style = cursor
		}
		b.WriteString(style.Render(chip.label))
	}
	s := b.String()
	if width(s) > avail {
		return m.Style.Muted.Render(truncate(m.releaseSummary(), avail))
	}
	return s
}

// releaseSummary is a plain-text description of the current release selection,
// used when the toggle chips do not fit.
func (m SettingsModel) releaseSummary() string {
	if len(m.ReleaseTypes) == 0 {
		return "(all)"
	}
	return strings.Join(m.ReleaseTypes, ", ")
}

// Update handles update messages for the settings model.
func (m *SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Esc stops editing the focused input without saving, reverting it to
		// the value it had before editing began.
		if m.EditMode && msg.Type == tea.KeyEsc {
			m.cancelEdit()
			return m, nil
		}

		// Tab completes the directory path while it is being edited.
		if m.EditMode && m.FocusIndex == settingDownloadDir && msg.Type == tea.KeyTab {
			return m.handleDirCompletion()
		}

		// The release-type multi-select captures navigation while it is active.
		if m.releaseEditing {
			return m.handleReleaseKey(msg)
		}

		for _, cmd := range GetCommandsForView(viewSettings) {
			if key.Matches(msg, GetKeyBinding(cmd.Type)) {
				if m.handleCommand(cmd.Type) {
					return m, nil
				}
			}
		}

		// Route keystrokes to the focused text field while editing.
		if m.EditMode && m.FocusIndex < len(m.Inputs) {
			// Any edit invalidates the last completion session; Tab is handled
			// above and returns before reaching here.
			m.resetDirCompletion()
			var cmd tea.Cmd
			m.Inputs[m.FocusIndex], cmd = m.Inputs[m.FocusIndex].Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// handleCommand performs the settings-local action for cmd, reporting whether it
// handled the command. It covers edit toggling, focus movement and option
// cycling; app-level commands (quit, save, clean) are handled by the Model.
// While a text field is being edited, navigation commands are left unhandled so
// the keystroke can be routed to the input.
func (m *SettingsModel) handleCommand(cmd CommandType) bool {
	switch cmd {
	case CmdToggleEditMode:
		m.handleToggleEdit()
		return true
	case CmdMoveUp:
		if m.EditMode {
			return false
		}
		m.moveFocus(-1)
		return true
	case CmdMoveDown:
		if m.EditMode {
			return false
		}
		m.moveFocus(1)
		return true
	case CmdMoveLeft:
		if m.EditMode {
			return false
		}
		m.cycleOption(-1)
		return true
	case CmdMoveRight:
		if m.EditMode {
			return false
		}
		m.cycleOption(1)
		return true
	}
	return false
}

// handleToggleEdit enters/leaves edit mode for text settings, and cycles the
// value of option settings. Enter commits the edit; Esc (handled separately)
// discards it.
func (m *SettingsModel) handleToggleEdit() (tea.Model, tea.Cmd) {
	// The release-type row opens its own multi-select mode.
	if m.FocusIndex == settingReleaseFilter {
		m.releaseEditing = true
		m.releaseBackup = append([]string(nil), m.ReleaseTypes...)
		if m.ReleaseCursor < 0 || m.ReleaseCursor >= len(m.ReleaseTypeOptions) {
			m.ReleaseCursor = 0
		}
		return m, nil
	}
	if m.FocusIndex >= len(m.Inputs) {
		m.cycleOption(1)
		return m, nil
	}
	if m.EditMode {
		// Commit the edit.
		m.EditMode = false
		m.editBackup = ""
		m.resetDirCompletion()
		m.updateFocusStyles()
		return m, nil
	}
	// Begin editing: snapshot the value so Esc can revert it.
	m.editBackup = m.Inputs[m.FocusIndex].Value()
	m.resetDirCompletion()
	m.EditMode = true
	m.updateFocusStyles()
	return m, nil
}

// cancelEdit leaves edit mode and restores the focused input to the value it
// had before editing began.
func (m *SettingsModel) cancelEdit() {
	if m.FocusIndex < len(m.Inputs) {
		m.Inputs[m.FocusIndex].SetValue(m.editBackup)
		m.Inputs[m.FocusIndex].CursorEnd()
	}
	m.editBackup = ""
	m.EditMode = false
	m.resetDirCompletion()
	m.updateFocusStyles()
}

// moveFocus advances the focused row, wrapping around.
func (m *SettingsModel) moveFocus(dir int) {
	m.FocusIndex = (m.FocusIndex + dir + settingCount) % settingCount
}

// selectSetting focuses the settings row at index i, discarding any in-progress
// edit or release toggle so a stray click can never save a partial value.
func (m *SettingsModel) selectSetting(i int) {
	if i < 0 || i >= settingCount {
		return
	}
	if m.releaseEditing {
		m.releaseBackup = nil
		m.releaseEditing = false
	}
	if m.EditMode && m.FocusIndex != i {
		m.cancelEdit()
	}
	m.FocusIndex = i
	m.updateFocusStyles()
}

// startEdit enters edit mode for the focused text field, snapshotting its value
// so Esc can revert it. It is a no-op when already editing or when the focused
// row is not a text field.
func (m *SettingsModel) startEdit() {
	if m.EditMode || m.FocusIndex < 0 || m.FocusIndex >= len(m.Inputs) {
		return
	}
	m.editBackup = m.Inputs[m.FocusIndex].Value()
	m.resetDirCompletion()
	m.EditMode = true
	m.updateFocusStyles()
}

// optionAt returns the index of the option whose rendered segment contains the
// column relX, measured from the start of the value area. Segments carry one
// cell of horizontal padding on each side, matching renderOptions. It returns
// -1 when relX falls in the gaps or past the end.
func optionAt(options []string, relX int) int {
	cursor := 0
	for i, o := range options {
		w := width(o) + 2
		if relX >= cursor && relX < cursor+w {
			return i
		}
		cursor += w
	}
	return -1
}

// releaseOptionAt is optionAt for the release-type chips, whose labels are
// prefixed with a two-cell checkbox marker.
func (m SettingsModel) releaseOptionAt(relX int) int {
	cursor := 0
	for i, chip := range m.releaseChips() {
		w := width(chip.label) + 2
		if relX >= cursor && relX < cursor+w {
			return i
		}
		cursor += w
	}
	return -1
}

// cycleOption advances the value of the focused option setting (Build Type or
// Theme) in the given direction, wrapping around.
func (m *SettingsModel) cycleOption(dir int) {
	switch m.FocusIndex {
	case settingBuildType:
		if len(m.BuildTypeOptions) == 0 {
			return
		}
		m.BuildTypeIndex = (m.BuildTypeIndex + dir + len(m.BuildTypeOptions)) % len(m.BuildTypeOptions)
		m.BuildType = m.BuildTypeOptions[m.BuildTypeIndex]
	case settingTheme:
		if len(m.ThemeOptions) == 0 {
			return
		}
		m.ThemeIndex = (m.ThemeIndex + dir + len(m.ThemeOptions)) % len(m.ThemeOptions)
		m.Theme = m.ThemeOptions[m.ThemeIndex]
	}
}

// handleReleaseKey handles keys while the release-type multi-select is active:
// left/right (or h/l) move the cursor, space toggles the focused chip, enter
// saves the selection and esc discards it.
func (m *SettingsModel) handleReleaseKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		// Cancel: restore the selection from before toggling began.
		m.ReleaseTypes = append([]string(nil), m.releaseBackup...)
		m.releaseBackup = nil
		m.releaseEditing = false
	case tea.KeyEnter:
		// Save: keep the selection and leave toggle mode.
		m.releaseBackup = nil
		m.releaseEditing = false
	case tea.KeyLeft, tea.KeyUp:
		m.moveReleaseCursor(-1)
	case tea.KeyRight, tea.KeyDown:
		m.moveReleaseCursor(1)
	case tea.KeySpace:
		m.toggleRelease()
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "h":
			m.moveReleaseCursor(-1)
		case "l":
			m.moveReleaseCursor(1)
		case " ":
			m.toggleRelease()
		}
	}
	return m, nil
}

// moveReleaseCursor advances the highlighted chip, wrapping around.
func (m *SettingsModel) moveReleaseCursor(dir int) {
	n := len(m.ReleaseTypeOptions)
	if n == 0 {
		return
	}
	m.ReleaseCursor = (m.ReleaseCursor + dir + n) % n
}

// toggleRelease flips the focused release cycle in the selection. The result is
// re-canonicalized so the stored order is stable.
func (m *SettingsModel) toggleRelease() {
	if m.ReleaseCursor < 0 || m.ReleaseCursor >= len(m.ReleaseTypeOptions) {
		return
	}
	opt := m.ReleaseTypeOptions[m.ReleaseCursor]
	next := make([]string, 0, len(m.ReleaseTypes)+1)
	removed := false
	for _, s := range m.ReleaseTypes {
		if s == opt {
			removed = true
			continue
		}
		next = append(next, s)
	}
	if !removed {
		next = append(next, opt)
	}
	m.ReleaseTypes = []string(model.ParseReleaseTypes(next))
}

// updateFocusStyles applies the palette to the text inputs based on focus.
func (m *SettingsModel) updateFocusStyles() {
	p := m.Style.Palette
	base := lp.NewStyle().Foreground(lp.Color(p.Foreground))
	focused := lp.NewStyle().Foreground(lp.Color(p.Accent)).Bold(true)

	for i := range m.Inputs {
		if i == m.FocusIndex && m.EditMode {
			m.Inputs[i].Focus()
			m.Inputs[i].TextStyle = focused
		} else {
			m.Inputs[i].Blur()
			m.Inputs[i].TextStyle = base
		}
	}
}

// updateInputWidths sizes the text inputs to fit the detail pane.
func (m *SettingsModel) updateInputWidths() {
	_, detailW, _ := m.panes()
	// Reserve room for the "Value: " label, the input prompt and cursor, plus a
	// little slack so the input never wraps the detail line.
	iw := detailW - width(valueLabel) - valueInputSlack
	if iw < minInputWidth {
		iw = minInputWidth
	}
	for i := range m.Inputs {
		m.Inputs[i].Width = iw
	}
}

// handleDirCompletion completes the Download Directory field against the
// filesystem, in the spirit of a shell's Tab completion:
//
//   - a unique match is completed and given a trailing separator;
//   - several matches extend the value to their longest shared prefix;
//   - when no further extension is possible, repeated Tab presses cycle
//     through the individual matches.
//
// The candidates are kept on the model so the detail pane can list them.
func (m *SettingsModel) handleDirCompletion() (tea.Model, tea.Cmd) {
	input := m.Inputs[settingDownloadDir].Value()

	// Continue the current session while the field still holds its starting
	// prefix or one of the candidates we last offered.
	continuing := input == m.dirCompleteBase
	if !continuing {
		for _, match := range m.dirMatches {
			if input == match || input == withTrailingSeparator(match) {
				continuing = true
				break
			}
		}
	}
	if !continuing {
		m.dirCompleteBase = input
		m.dirMatchIdx = -1
		m.dirMatches = nil
	}

	matches, err := DirCompletions(m.dirCompleteBase)
	if err != nil || len(matches) == 0 {
		m.dirMatches = nil
		m.dirMatchIdx = -1
		return m, nil
	}
	m.dirMatches = matches

	if len(matches) == 1 {
		m.setDownloadDir(withTrailingSeparator(matches[0]))
		m.dirMatchIdx = 0
		return m, nil
	}

	if common := commonPathPrefix(matches); len(common) > len(m.dirCompleteBase) {
		// Extend to the shared prefix; a following Tab starts cycling.
		m.dirCompleteBase = common
		m.setDownloadDir(common)
		m.dirMatchIdx = -1
		return m, nil
	}

	// No further extension is possible: cycle through the candidates.
	m.dirMatchIdx = (m.dirMatchIdx + 1) % len(matches)
	m.setDownloadDir(withTrailingSeparator(matches[m.dirMatchIdx]))
	return m, nil
}

// setDownloadDir replaces the Download Directory input value and parks the
// cursor at the end of the completed text.
func (m *SettingsModel) setDownloadDir(value string) {
	m.Inputs[settingDownloadDir].SetValue(value)
	m.Inputs[settingDownloadDir].CursorEnd()
}

// resetDirCompletion forgets the current Tab-completion session.
func (m *SettingsModel) resetDirCompletion() {
	m.dirMatches = nil
	m.dirCompleteBase = ""
	m.dirMatchIdx = -1
}

// commonPathPrefix returns the longest prefix shared by every path in paths.
func commonPathPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	prefix := paths[0]
	for _, p := range paths[1:] {
		max := len(prefix)
		if len(p) < max {
			max = len(p)
		}
		i := 0
		for i < max && prefix[i] == p[i] {
			i++
		}
		prefix = prefix[:i]
		if prefix == "" {
			return ""
		}
	}
	return prefix
}

// GetValues returns a snapshot of the current settings.
func (m *SettingsModel) GetValues() SettingsValues {
	return SettingsValues{
		DownloadDir:   m.Inputs[settingDownloadDir].Value(),
		VersionFilter: m.Inputs[settingVersionFilter].Value(),
		BuildType:     m.BuildType,
		ReleaseTypes:  append([]string(nil), m.ReleaseTypes...),
		Theme:         m.Theme,
	}
}

// SetValues applies a snapshot of settings (e.g., when reloading config).
func (m *SettingsModel) SetValues(v SettingsValues) {
	m.Inputs[settingDownloadDir].SetValue(v.DownloadDir)
	m.Inputs[settingVersionFilter].SetValue(v.VersionFilter)

	m.BuildType = v.BuildType
	for i, opt := range m.BuildTypeOptions {
		if opt == v.BuildType {
			m.BuildTypeIndex = i
			break
		}
	}

	m.ReleaseTypes = []string(model.ParseReleaseTypes(v.ReleaseTypes))
	m.ReleaseCursor = 0

	m.setTheme(v.Theme)

	// Leaving and re-entering the view must never resume in edit or toggle mode.
	m.EditMode = false
	m.editBackup = ""
	m.releaseEditing = false
	m.releaseBackup = nil
	m.resetDirCompletion()
	m.updateFocusStyles()
}

// setTheme selects a palette by name, normalizing unknown names to the default
// so the selection always points at a valid option.
func (m *SettingsModel) setTheme(theme string) {
	m.Theme = ResolvePalette(theme).Name
	m.ThemeIndex = 0
	for i, opt := range m.ThemeOptions {
		if opt == m.Theme {
			m.ThemeIndex = i
			break
		}
	}
}
