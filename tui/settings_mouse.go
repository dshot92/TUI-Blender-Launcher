package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// settingsLayout describes the on-screen geometry of the settings modal in
// terminal coordinates, so mouse hit-testing and rendering agree on where each
// element sits.
type settingsLayout struct {
	boxX, boxY     int
	boxW, boxH     int
	innerW         int
	listW, detailW int
	twoCol         bool

	// contentX/contentY is the top-left cell of the Settings.View() output
	// inside the modal; contentH is its line count.
	contentX, contentY int
	contentH           int

	// helpY is the row holding the settings key hints.
	helpY int
}

// settingsModalLayout computes the settings modal geometry for the current
// terminal size and settings content.
func (m *Model) settingsModalLayout() settingsLayout {
	boxWidth := m.layout.settingsWidth
	if boxWidth < settingsMinWidth {
		boxWidth = settingsMinWidth
	}
	if boxWidth > m.terminalWidth {
		boxWidth = m.terminalWidth
	}

	innerW := boxWidth - boxBorderFrameWidth
	if innerW < 1 {
		innerW = 1
	}

	contentH := len(strings.Split(m.Settings.View(), "\n"))

	// Box lines: one blank, the content, one blank and the help line.
	boxHeight := contentH + 3 + boxBorderFrameHeight
	if boxHeight > m.terminalHeight {
		boxHeight = m.terminalHeight
	}
	if minH := boxBorderFrameHeight + 2; boxHeight < minH {
		boxHeight = minH
	}

	boxX := (m.terminalWidth - boxWidth) / 2
	if boxX < 0 {
		boxX = 0
	}
	boxY := (m.terminalHeight - boxHeight) / 2
	if boxY < 0 {
		boxY = 0
	}

	listW, detailW, twoCol := m.Settings.panes()

	return settingsLayout{
		boxX: boxX, boxY: boxY,
		boxW: boxWidth, boxH: boxHeight,
		innerW:   innerW,
		listW:    listW,
		detailW:  detailW,
		twoCol:   twoCol,
		contentX: boxX + 1,
		contentY: boxY + 2,
		contentH: contentH,
		helpY:    boxY + 2 + contentH + 1,
	}
}

// optionsWidth returns the rendered width of a plain segmented control.
func optionsWidth(options []string) int {
	sum := 0
	for _, o := range options {
		sum += width(o) + 2
	}
	return sum
}

// handleSettingsMouse applies a mouse event to the settings modal. It reports
// the command to run (if any) and whether the event was consumed.
func (m *Model) handleSettingsMouse(msg tea.MouseMsg) (tea.Cmd, bool) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if !m.Settings.EditMode && !m.Settings.releaseEditing {
			m.Settings.moveFocus(-1)
		}
		return nil, true
	case tea.MouseButtonWheelDown:
		if !m.Settings.EditMode && !m.Settings.releaseEditing {
			m.Settings.moveFocus(1)
		}
		return nil, true
	}

	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return nil, false
	}

	lay := m.settingsModalLayout()

	// The help line runs the hint that was clicked.
	if msg.Y == lay.helpY {
		hints := m.settingsHints()
		help := truncate(m.renderHints(hints), lay.innerW)
		left := lay.boxX + 1 + (lay.innerW-width(help))/2
		if cmd, ok := hintAt(hints, msg.X, left); ok {
			_, c := m.executeSettingsCommand(cmd)
			return c, true
		}
		return nil, true
	}

	// A click outside the modal is inert, but still consumed so it cannot fall
	// through to the (inactive) list view behind it.
	if msg.X < lay.boxX || msg.X >= lay.boxX+lay.boxW ||
		msg.Y < lay.boxY || msg.Y >= lay.boxY+lay.boxH {
		return nil, true
	}

	// List pane: focusing the clicked row.
	if row := msg.Y - lay.contentY; row >= 0 && row < settingCount &&
		msg.X >= lay.contentX && msg.X < lay.contentX+lay.listW {
		m.Settings.selectSetting(row)
		return nil, true
	}

	// Detail pane: the value line may hold a text input or option chips.
	detailX := lay.contentX
	detailTopY := lay.contentY
	if lay.twoCol {
		detailX = lay.contentX + lay.listW + settingsColumnGap
	} else {
		detailTopY = lay.contentY + settingCount + 1
	}
	if msg.Y == detailTopY+2 && msg.X >= detailX {
		m.settingsValueClick(msg.X - detailX)
		return nil, true
	}
	return nil, true
}

// settingsValueClick acts on a click on the focused setting's value line, given
// the column relative to the detail pane's left edge.
func (m *Model) settingsValueClick(detailRelX int) {
	s := &m.Settings

	valueX := 1 + width(valueLabel) // one-cell inset plus the label
	if detailRelX < valueX {
		return
	}
	relX := detailRelX - valueX

	lay := m.settingsModalLayout()
	avail := lay.detailW - 2*panePadX - width(valueLabel)
	if avail < minValueWidth {
		avail = minValueWidth
	}

	switch s.FocusIndex {
	case settingDownloadDir, settingVersionFilter:
		s.startEdit()
	case settingBuildType:
		if optionsWidth(s.BuildTypeOptions) > avail {
			s.cycleOption(1)
			return
		}
		if i := optionAt(s.BuildTypeOptions, relX); i >= 0 {
			s.BuildTypeIndex = i
			s.BuildType = s.BuildTypeOptions[i]
		}
	case settingTheme:
		if optionsWidth(s.ThemeOptions) > avail {
			s.cycleOption(1)
			return
		}
		if i := optionAt(s.ThemeOptions, relX); i >= 0 {
			s.ThemeIndex = i
			s.Theme = s.ThemeOptions[i]
		}
	case settingReleaseFilter:
		if i := s.releaseOptionAt(relX); i >= 0 {
			s.ReleaseCursor = i
			s.toggleRelease()
		}
	}
}
