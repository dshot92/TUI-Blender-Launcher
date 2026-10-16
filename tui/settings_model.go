package tui

import (
	"strings"

	"TUI-Blender-Launcher/config"
	"TUI-Blender-Launcher/model"

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
		return "Folder where downloaded Blender builds are stored."
	case settingVersionFilter:
		return "Only show builds at or above this version (e.g., 4.0, 3.6). Leave empty to show all."
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
		newInput("e.g., 4.0, 3.6 (empty = none)", cfg.VersionFilter, 10),
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
	contentW := w - 2
	if contentW < 8 {
		contentW = 8
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
	// version filter examples) remain visible instead of being clipped.
	for _, dl := range wrapText(settingDescription(idx), contentW) {
		lines = append(lines, descStyle.Padding(0, 1).Render(dl))
	}
	return strings.Join(lines, "\n")
}

// detailValue renders the "Value: ..." line for the focused setting.
func (m SettingsModel) detailValue(w int, labelStyle, valueStyle lp.Style) string {
	label := labelStyle.Render("Value: ")
	avail := w - width(label)
	if avail < 8 {
		avail = 8
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

	selected := make(map[string]bool, len(m.ReleaseTypes))
	for _, s := range m.ReleaseTypes {
		selected[s] = true
	}

	var b strings.Builder
	for i, opt := range m.ReleaseTypeOptions {
		mark := "▢ "
		style := off
		if selected[opt] {
			mark = "▣ "
			style = on
		}
		if m.releaseEditing && i == m.ReleaseCursor {
			style = cursor
		}
		b.WriteString(style.Render(mark + opt))
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
				switch cmd.Type {
				case CmdToggleEditMode:
					return m.handleToggleEdit()

				case CmdMoveUp:
					if !m.EditMode {
						m.moveFocus(-1)
						return m, nil
					}

				case CmdMoveDown:
					if !m.EditMode {
						m.moveFocus(1)
						return m, nil
					}

				case CmdMoveLeft:
					if !m.EditMode {
						m.cycleOption(-1)
						return m, nil
					}

				case CmdMoveRight:
					if !m.EditMode {
						m.cycleOption(1)
						return m, nil
					}
				}
			}
		}

		// Route keystrokes to the focused text field while editing.
		if m.EditMode && m.FocusIndex < len(m.Inputs) {
			var cmd tea.Cmd
			m.Inputs[m.FocusIndex], cmd = m.Inputs[m.FocusIndex].Update(msg)
			return m, cmd
		}
	}
	return m, nil
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
		m.updateFocusStyles()
		return m, nil
	}
	// Begin editing: snapshot the value so Esc can revert it.
	m.editBackup = m.Inputs[m.FocusIndex].Value()
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
	m.updateFocusStyles()
}

// moveFocus advances the focused row, wrapping around.
func (m *SettingsModel) moveFocus(dir int) {
	m.FocusIndex = (m.FocusIndex + dir + settingCount) % settingCount
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
	iw := detailW - width("Value: ") - 5
	if iw < 10 {
		iw = 10
	}
	for i := range m.Inputs {
		m.Inputs[i].Width = iw
	}
}

func (m *SettingsModel) handleDirCompletion() (tea.Model, tea.Cmd) {
	input := m.Inputs[0].Value()
	matches, err := DirCompletions(input)
	if err == nil && len(matches) > 0 {
		if len(matches) == 1 {
			m.Inputs[0].SetValue(matches[0] + "/")
			m.Inputs[0].CursorEnd()
		} else {
			// Find common prefix
			prefix := matches[0]
			for _, mpath := range matches[1:] {
				max := len(prefix)
				if len(mpath) < max {
					max = len(mpath)
				}
				for i := 0; i < max; i++ {
					if prefix[i] != mpath[i] {
						prefix = prefix[:i]
						break
					}
				}
			}
			m.Inputs[0].SetValue(prefix)
			m.Inputs[0].CursorEnd()
		}
	}
	return m, nil
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
