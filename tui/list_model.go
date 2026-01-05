package tui

import (
	"TUI-Blender-Launcher/model"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// ListModel handles the state and logic for the build list view.
type ListModel struct {
	Builds          []model.BlenderBuild
	Cursor          int
	StartIndex      int
	SortColumn      int
	SortReversed    bool
	TerminalHeight  int
	Style           Style // Keep Style here as well if needed for List specific rendering
	LastRenderState map[string]float64
}

// NewListModel creates a new ListModel.
func NewListModel(style Style) ListModel {
	return ListModel{
		SortColumn:      0,
		SortReversed:    true,
		Style:           style,
		Builds:          []model.BlenderBuild{},
		LastRenderState: make(map[string]float64),
	}
}

// Init initializes the model.
func (m ListModel) Init() tea.Cmd {
	return nil
}

// View returns the string representation of the model.
func (m ListModel) View() string {
	return ""
}

// Update handles update messages for the list model.
func (m *ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		visibleRowsCount := m.GetVisibleRowsCount()

		for _, cmd := range GetCommandsForView(viewList) {
			if key.Matches(msg, GetKeyBinding(cmd.Type)) {
				switch cmd.Type {
				case CmdToggleSortOrder:
					m.SortReversed = !m.SortReversed
					m.SortBuilds()
					m.EnsureCursorVisible()
					return m, nil

				case CmdMoveUp:
					m.UpdateCursor("up", visibleRowsCount)
					return m, nil

				case CmdMoveDown:
					m.UpdateCursor("down", visibleRowsCount)
					return m, nil

				case CmdMoveLeft:
					m.UpdateSortColumn("left")
					m.SortBuilds()
					m.EnsureCursorVisible()
					return m, nil

				case CmdMoveRight:
					m.UpdateSortColumn("right")
					m.SortBuilds()
					m.EnsureCursorVisible()
					return m, nil

				case CmdPageUp:
					m.UpdateCursor("pageup", visibleRowsCount)
					return m, nil

				case CmdPageDown:
					m.UpdateCursor("pagedown", visibleRowsCount)
					return m, nil

				case CmdHome:
					m.UpdateCursor("home", visibleRowsCount)
					return m, nil

				case CmdEnd:
					m.UpdateCursor("end", visibleRowsCount)
					return m, nil
				}
			}
		}
	}
	return m, nil
}

func (m *ListModel) GetVisibleRowsCount() int {
	if m.TerminalHeight < 7 {
		return 1
	}
	return m.TerminalHeight - 7
}

// UpdateCursor moves the cursor
func (m *ListModel) UpdateCursor(direction string, visibleRowsCount int) {
	if len(m.Builds) == 0 {
		return
	}

	switch direction {
	case "up":
		m.Cursor--
		if m.Cursor < 0 {
			m.Cursor = len(m.Builds) - 1
		}
	case "down":
		m.Cursor++
		if m.Cursor >= len(m.Builds) {
			m.Cursor = 0
		}
	case "home":
		m.Cursor = 0
	case "end":
		m.Cursor = len(m.Builds) - 1
	case "pageup":
		m.Cursor -= visibleRowsCount
		if m.Cursor < 0 {
			m.Cursor = 0
		}
	case "pagedown":
		m.Cursor += visibleRowsCount
		if m.Cursor >= len(m.Builds) {
			m.Cursor = len(m.Builds) - 1
		}
	}

	m.EnsureCursorVisible()
}

// EnsureCursorVisible ensures the cursor is visible within the scrolling window
func (m *ListModel) EnsureCursorVisible() {
	visibleRowsCount := m.GetVisibleRowsCount()

	if len(m.Builds) == 0 {
		m.StartIndex = 0
		return
	}

	// Ensure cursor is within bounds
	if m.Cursor >= len(m.Builds) {
		m.Cursor = len(m.Builds) - 1
	} else if m.Cursor < 0 {
		m.Cursor = 0
	}

	// Adjust startIndex to ensure cursor is visible
	if m.Cursor < m.StartIndex {
		// Cursor moved above visible area, scroll up
		m.StartIndex = m.Cursor
	} else if m.Cursor >= m.StartIndex+visibleRowsCount {
		// Cursor moved below visible area, scroll down
		m.StartIndex = m.Cursor - visibleRowsCount + 1
		if m.StartIndex < 0 {
			m.StartIndex = 0
		}
	}
}

// UpdateSortColumn changes the sort column
func (m *ListModel) UpdateSortColumn(direction string) {
	// Total columns: Version, Status, Branch, Type, Hash, Size, Build Date
	numColumns := 7

	if direction == "left" {
		m.SortColumn--
		if m.SortColumn < 0 {
			m.SortColumn = numColumns - 1
		}
	} else if direction == "right" {
		m.SortColumn++
		if m.SortColumn >= numColumns {
			m.SortColumn = 0
		}
	}
}

// SortBuilds sorts the build list
func (m *ListModel) SortBuilds() {
	m.Builds = model.SortBuilds(m.Builds, m.SortColumn, m.SortReversed)
}

// GetSelectedBuild returns the currently selected build, or nil if none
func (m *ListModel) GetSelectedBuild() *model.BlenderBuild {
	if len(m.Builds) > 0 && m.Cursor >= 0 && m.Cursor < len(m.Builds) {
		return &m.Builds[m.Cursor]
	}
	return nil
}
