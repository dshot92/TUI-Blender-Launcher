package tui

import (
	"strings"

	"TUI-Blender-Launcher/config"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	lp "github.com/charmbracelet/lipgloss"
)

// SettingsModel handles the state and logic for the settings view.
type SettingsModel struct {
	Inputs           []textinput.Model
	FocusIndex       int
	EditMode         bool
	BuildType        string
	BuildTypeOptions []string
	BuildTypeIndex   int
	Theme            string
	ThemeOptions     []string
	ThemeIndex       int
	Style            Style
	Config           config.Config
	width            int
}

// NewSettingsModel creates a new SettingsModel.
func NewSettingsModel(cfg config.Config, style Style) SettingsModel {
	m := SettingsModel{
		Config:           cfg,
		Style:            style,
		BuildTypeOptions: []string{"daily", "experimental", "patch"},
		BuildType:        cfg.BuildType,
		ThemeOptions:     PaletteNames(),
		FocusIndex:       0,
		EditMode:         false,
	}

	// Initialize inputs
	m.Inputs = make([]textinput.Model, 2)

	// Download Dir input
	t := textinput.New()
	t.Placeholder = cfg.DownloadDir
	t.SetValue(cfg.DownloadDir)
	t.CharLimit = 256
	t.Width = 50
	m.Inputs[0] = t

	// Version Filter input
	t = textinput.New()
	t.Placeholder = "e.g., 4.0, 3.6 (leave empty for none)"
	t.SetValue(cfg.VersionFilter)
	t.CharLimit = 10
	t.Width = 50
	m.Inputs[1] = t

	// Find initial build type index
	for i, opt := range m.BuildTypeOptions {
		if opt == cfg.BuildType {
			m.BuildTypeIndex = i
			break
		}
	}

	// Resolve the configured theme so the selection is always valid.
	m.setTheme(cfg.Theme)

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
	for i := range m.Inputs {
		iw := w - 6
		if iw < 10 {
			iw = 10
		}
		m.Inputs[i].Width = iw
	}
}

// View returns the settings content, consistently left-aligned.
func (m SettingsModel) View() string {
	effectiveWidth := m.width
	if effectiveWidth <= 0 {
		effectiveWidth = 60
	}

	p := m.Style.Palette
	labelStyle := lp.NewStyle().Bold(true).Foreground(lp.Color(p.Accent))
	labelFocused := lp.NewStyle().Bold(true).
		Foreground(lp.Color(p.Background)).
		Background(lp.Color(p.Accent))
	descStyle := lp.NewStyle().Italic(true).Foreground(lp.Color(p.Muted))
	optionStyle := lp.NewStyle().Padding(0, 1).Foreground(lp.Color(p.Muted))
	optionActive := lp.NewStyle().Padding(0, 1).Bold(true).
		Foreground(lp.Color(p.Background)).
		Background(lp.Color(p.Accent))

	// section renders a label, a control line, and a description, all left
	// aligned and indented consistently.
	section := func(index int, label, description, control string) string {
		lbl := labelStyle
		if m.FocusIndex == index {
			lbl = labelFocused
		}
		var b strings.Builder
		b.WriteString(lbl.Width(effectiveWidth).Render("  " + label))
		b.WriteString("\n")
		b.WriteString(lp.NewStyle().Width(effectiveWidth).Render(truncate("  "+control, effectiveWidth)))
		b.WriteString("\n")
		b.WriteString(descStyle.Width(effectiveWidth).Render(truncate("  "+description, effectiveWidth)))
		b.WriteString("\n\n")
		return b.String()
	}

	var b strings.Builder
	b.WriteString(section(0, "Download Directory",
		"Path where Blender builds are stored.",
		m.Inputs[0].View()))

	b.WriteString(section(1, "Version Filter",
		"Only show builds at or above this version (empty = all).",
		m.Inputs[1].View()))

	var options strings.Builder
	for _, option := range m.BuildTypeOptions {
		if option == m.BuildType {
			options.WriteString(optionActive.Render(option))
		} else {
			options.WriteString(optionStyle.Render(option))
		}
	}
	b.WriteString(section(len(m.Inputs), "Build Type",
		"Default build type to fetch from the Blender BuildBot.", options.String()))

	var themeOptions strings.Builder
	for _, option := range m.ThemeOptions {
		if option == m.Theme {
			themeOptions.WriteString(optionActive.Render(option))
		} else {
			themeOptions.WriteString(optionStyle.Render(option))
		}
	}
	b.WriteString(section(len(m.Inputs)+1, "Theme",
		"UI color palette (previewed immediately, saved on exit).", themeOptions.String()))

	return b.String()
}

// Update handles update messages for the settings model.
func (m *SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle Tab key for directory completion
		if m.EditMode && m.FocusIndex == 0 && msg.Type == tea.KeyTab {
			return m.handleDirCompletion()
		}

		for _, cmd := range GetCommandsForView(viewSettings) {
			if key.Matches(msg, GetKeyBinding(cmd.Type)) {
				switch cmd.Type {
				case CmdToggleEditMode:
					m.EditMode = !m.EditMode
					if m.FocusIndex < len(m.Inputs) {
						if m.EditMode {
							m.Inputs[m.FocusIndex].Focus()
						} else {
							m.Inputs[m.FocusIndex].Blur()
						}
					}
					m.updateFocusStyles()
					return m, nil

				case CmdMoveUp:
					if !m.EditMode {
						totalItems := len(m.Inputs) + 2
						m.FocusIndex = (m.FocusIndex - 1 + totalItems) % totalItems
						m.updateFocusStyles()
						return m, nil
					}

				case CmdMoveDown:
					if !m.EditMode {
						totalItems := len(m.Inputs) + 2
						m.FocusIndex = (m.FocusIndex + 1) % totalItems
						m.updateFocusStyles()
						return m, nil
					}

				case CmdMoveLeft:
					if !m.EditMode && m.FocusIndex == len(m.Inputs) {
						m.BuildTypeIndex = (m.BuildTypeIndex - 1 + len(m.BuildTypeOptions)) % len(m.BuildTypeOptions)
						m.BuildType = m.BuildTypeOptions[m.BuildTypeIndex]
						return m, nil
					}
					if !m.EditMode && m.FocusIndex == len(m.Inputs)+1 {
						m.ThemeIndex = (m.ThemeIndex - 1 + len(m.ThemeOptions)) % len(m.ThemeOptions)
						m.Theme = m.ThemeOptions[m.ThemeIndex]
						return m, nil
					}

				case CmdMoveRight:
					if !m.EditMode && m.FocusIndex == len(m.Inputs) {
						m.BuildTypeIndex = (m.BuildTypeIndex + 1) % len(m.BuildTypeOptions)
						m.BuildType = m.BuildTypeOptions[m.BuildTypeIndex]
						return m, nil
					}
					if !m.EditMode && m.FocusIndex == len(m.Inputs)+1 {
						m.ThemeIndex = (m.ThemeIndex + 1) % len(m.ThemeOptions)
						m.Theme = m.ThemeOptions[m.ThemeIndex]
						return m, nil
					}
				}
			}
		}

		// Pass input to text fields
		if m.EditMode && m.FocusIndex < len(m.Inputs) {
			var cmd tea.Cmd
			m.Inputs[m.FocusIndex], cmd = m.Inputs[m.FocusIndex].Update(msg)
			return m, cmd
		}
	}
	return m, nil
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

// GetValues returns the current values from the inputs
func (m *SettingsModel) GetValues() (downloadDir, versionFilter, buildType, theme string) {
	return m.Inputs[0].Value(), m.Inputs[1].Value(), m.BuildType, m.Theme
}

// SetValues sets the values (e.g., when reloading config)
func (m *SettingsModel) SetValues(downloadDir, versionFilter, buildType, theme string) {
	m.Inputs[0].SetValue(downloadDir)
	m.Inputs[1].SetValue(versionFilter)

	m.BuildType = buildType
	for i, opt := range m.BuildTypeOptions {
		if opt == buildType {
			m.BuildTypeIndex = i
			break
		}
	}

	m.setTheme(theme)
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
