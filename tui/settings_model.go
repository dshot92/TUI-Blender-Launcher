package tui

import (
	"TUI-Blender-Launcher/config"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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
	Config           config.Config // Store a copy or reference if needed for validation/saving
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

// View returns the string representation of the model.
func (m SettingsModel) View() string {
	return ""
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
					return m, nil // Return *SettingsModel as tea.Model? No, usually sub-models return self and cmd

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
			m.Inputs[i].PromptStyle = m.Style.SelectedRow
			if m.EditMode {
				m.Inputs[i].Focus()
			} else {
				m.Inputs[i].Blur()
			}
		} else {
			m.Inputs[i].PromptStyle = m.Style.RegularRow
			m.Inputs[i].Blur()
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
