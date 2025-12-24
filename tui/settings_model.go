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
	// We'll set width dynamically in View or SetWidth if possible,
	// but for now initialized width is okay.
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

	m.updateFocusStyles()

	return m
}

// Init initializes the model.
func (m SettingsModel) Init() tea.Cmd {
	return nil
}

// SetWidth updates the width of the settings model
func (m *SettingsModel) SetWidth(w int) {
	m.width = w
}

// View returns the string representation of the model.
func (m SettingsModel) View() string {
	var b strings.Builder

	// Calculate effective width for alignment
	effectiveWidth := m.width
	if effectiveWidth <= 0 {
		effectiveWidth = 80 // Fallback
	}

	// Styles
	// Helper to get alignment based on index
	getAlign := func(index int) lp.Position {
		switch index {
		case 0:
			return lp.Left
		case 1:
			return lp.Center
		case 2:
			return lp.Right
		default:
			return lp.Left
		}
	}

	// Common base styles
	labelBase := lp.NewStyle().Bold(true).Foreground(lp.Color(highlightColor))
	labelFocusedBase := lp.NewStyle().Bold(true).Background(lp.Color(highlightColor)).Foreground(lp.Color(backgroundColor))

	// Content styles - Always Left Aligned as requested ("setting portion ... make them all left aligned")
	inputBase := lp.NewStyle().MarginLeft(2).Align(lp.Left)
	descBase := lp.NewStyle().Italic(true).Foreground(lp.Color("241")).Align(lp.Left)

	// Section takes full width
	sectionBase := lp.NewStyle().MarginBottom(2).Width(effectiveWidth)

	optionStyle := lp.NewStyle().MarginRight(1).Padding(0, 1)
	selectedOptionStyle := lp.NewStyle().MarginRight(1).Padding(0, 1).
		Foreground(lp.Color(textColor)).Background(lp.Color(highlightColor))

	// Helper to render a text input setting
	renderTextSetting := func(index int, label, description string) string {
		labelAlign := getAlign(index)

		// Labels: Mixed Alignment
		lblStyle := labelBase.Align(labelAlign).Width(effectiveWidth)
		lblStyleFocused := labelFocusedBase.Align(labelAlign).Width(effectiveWidth)

		var sb strings.Builder
		isFocused := (m.FocusIndex == index)

		if isFocused {
			sb.WriteString(lblStyleFocused.Render(label))
		} else {
			sb.WriteString(lblStyle.Render(label))
		}
		sb.WriteString("\n")

		// Input: Always Left Aligned
		inputView := m.Inputs[index].View()
		inpStyle := inputBase.Width(effectiveWidth)

		sb.WriteString(inpStyle.Render(inputView))
		sb.WriteString("\n")

		// Description: Always Left Aligned
		dStyle := descBase.Width(effectiveWidth)
		sb.WriteString(dStyle.Render(description))

		// Wrap in section style
		return sectionBase.Render(sb.String())
	}

	renderBuildTypeSetting := func(label, description string) string {
		index := 2                    // Hardcoded as 3rd item
		labelAlign := getAlign(index) // Right

		// Labels: Mixed Alignment
		lblStyle := labelBase.Align(labelAlign).Width(effectiveWidth)
		lblStyleFocused := labelFocusedBase.Align(labelAlign).Width(effectiveWidth)

		var sb strings.Builder
		isFocused := (m.FocusIndex == len(m.Inputs))

		if isFocused {
			sb.WriteString(lblStyleFocused.Render(label))
		} else {
			sb.WriteString(lblStyle.Render(label))
		}
		sb.WriteString("\n")

		var horizontalOptions strings.Builder
		selectedBuildType := m.BuildType
		for _, option := range m.BuildTypeOptions {
			if option == selectedBuildType {
				horizontalOptions.WriteString(selectedOptionStyle.Render(option))
			} else {
				horizontalOptions.WriteString(optionStyle.Render(option))
			}
		}

		// Options: Always Left Aligned
		// Using MarginLeft(2) to match inputBase for consistency or just Left?
		// User said "make them all left aligned". Input has MarginLeft(2). Let's match it usually.
		optsStyle := lp.NewStyle().MarginLeft(2).Align(lp.Left).Width(effectiveWidth)
		sb.WriteString(optsStyle.Render(horizontalOptions.String()))
		sb.WriteString("\n")

		// Description: Always Left Aligned
		dStyle := descBase.Width(effectiveWidth)
		sb.WriteString(dStyle.Render(description))

		return sectionBase.Render(sb.String())
	}

	// Render each setting
	b.WriteString(renderTextSetting(0, "Download Directory", "Path where Blender builds will be stored."))
	b.WriteString(renderTextSetting(1, "Version Filter", "Filter versions (e.g., '4.2', '3.6'). Leave empty for all."))
	b.WriteString(renderBuildTypeSetting("Build Type", "Select default build type to fetch."))

	// Final container
	return lp.NewStyle().Width(effectiveWidth).Padding(1, 2).Render(b.String())
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
						totalItems := len(m.Inputs) + 1
						m.FocusIndex = (m.FocusIndex - 1 + totalItems) % totalItems
						m.updateFocusStyles()
						return m, nil
					}

				case CmdMoveDown:
					if !m.EditMode {
						totalItems := len(m.Inputs) + 1
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

				case CmdMoveRight:
					if !m.EditMode && m.FocusIndex == len(m.Inputs) {
						m.BuildTypeIndex = (m.BuildTypeIndex + 1) % len(m.BuildTypeOptions)
						m.BuildType = m.BuildTypeOptions[m.BuildTypeIndex]
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

func (m *SettingsModel) updateFocusStyles() {
	for i := range m.Inputs {
		if i == m.FocusIndex {
			// m.Inputs[i].PromptStyle = m.Style.SelectedRow // This was causing some issues with textinput style maybe?
			// Let's use specific textinput styles if possible or keep simple
			// The cursor style is handled by textinput itself.
			if m.EditMode {
				m.Inputs[i].Focus()
				m.Inputs[i].TextStyle = m.Style.SelectedRow
			} else {
				m.Inputs[i].Blur()
				m.Inputs[i].TextStyle = m.Style.RegularRow
			}
		} else {
			m.Inputs[i].Blur()
			m.Inputs[i].TextStyle = m.Style.RegularRow
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
func (m *SettingsModel) GetValues() (downloadDir string, versionFilter string, buildType string) {
	return m.Inputs[0].Value(), m.Inputs[1].Value(), m.BuildType
}

// SetValues sets the values (e.g., when reloading config)
func (m *SettingsModel) SetValues(downloadDir, versionFilter, buildType string) {
	m.Inputs[0].SetValue(downloadDir)
	m.Inputs[1].SetValue(versionFilter)

	m.BuildType = buildType
	for i, opt := range m.BuildTypeOptions {
		if opt == buildType {
			m.BuildTypeIndex = i
			break
		}
	}
}
