package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// renderPageForView assembles the full frame: the single-line header, the body (builds +
// optional detail pane, or the settings modal), and the footer. Any transient
// status/error message is layered over the bottom of the frame as a toast.
func (m *Model) renderPageForView() string {
	if m.terminalWidth <= 0 || m.terminalHeight <= 0 {
		return "Loading…"
	}
	if m.terminalWidth < minTermWidth || m.terminalHeight < minTermHeight {
		msg := m.Style.Empty.Render(fmt.Sprintf("Terminal too small — need at least %d×%d", minTermWidth, minTermHeight))
		msg = truncate(msg, m.terminalWidth)
		return lp.Place(m.terminalWidth, m.terminalHeight, lp.Center, lp.Center, msg)
	}

	if m.contextMenuVisible {
		return m.renderContextMenuOverlay()
	}

	if m.deleteConfirm {
		return m.renderDeleteConfirmOverlay()
	}

	m.computeLayout()

	// The settings view is a modal: it replaces the header/body/footer chrome
	// with a centered box over a dimmed background. Both this and the list view
	// layer the transient status/error toast over the bottom of the frame.
	if m.currentView == viewInitialSetup || m.currentView == viewSettings {
		return m.overlayToast(m.renderSettingsModalOverlay())
	}

	header := renderHeader(m)
	body := m.renderBody()
	footer := m.renderBuildFooter()

	return m.overlayToast(strings.Join([]string{header, body, footer}, "\n"))
}

// renderBody lays out the builds pane and, when shown, the detail pane either
// beside it (wide terminals) or docked below it (narrow terminals).
func (m *Model) renderBody() string {
	builds := m.renderBuildsPane(m.layout.listBoxWidth, m.layout.listBoxHeight)
	if !m.layout.detailVisible {
		return builds
	}
	detail := m.renderDetailPane(m.layout.detailBoxWidth, m.layout.detailBoxHeight)
	if m.layout.detailStacked {
		return builds + "\n" + detail
	}
	return joinColumns(builds, detail, strings.Repeat(" ", paneGap))
}

// renderSettingsModalOverlay renders the settings (or initial setup) view as a
// centered box overlaid on top of the build list, so the surrounding context
// remains visible.
func (m *Model) renderSettingsModalOverlay() string {
	p := m.Style.Palette
	lay := m.settingsModalLayout()

	// The settings modal replaces the header chrome, so leave a blank (but
	// width-correct) top line instead of the wordmark.
	header := lp.NewStyle().Width(m.terminalWidth).Render("")
	body := m.renderBody()
	footer := m.renderBuildFooter()
	base := strings.Join([]string{header, body, footer}, "\n")
	title := " Settings "
	if m.currentView == viewInitialSetup {
		title = " Welcome — Initial Setup "
	}

	content := m.Settings.View()

	help := m.renderHints(m.settingsHints())
	help = lp.NewStyle().Width(lay.innerW).Align(lp.Center).Render(truncate(help, lay.innerW))

	// Build the box content. The status/error toast is layered over the bottom
	// of the frame, not inside the box.
	lines := make([]string, 0, 16)
	lines = append(lines, "")
	lines = append(lines, strings.Split(content, "\n")...)
	lines = append(lines, "", help)

	box := renderBox("",
		m.Style.ActivePaneTitle.Render(title),
		strings.Join(lines, "\n"),
		lay.boxW,
		lay.boxH,
		lp.Color(p.Accent),
	)

	frame := overlayBlock(base, box, lay.boxX, lay.boxY, m.terminalWidth)
	return m.overlayCompletionDropdown(frame, lay.boxX, lay.boxY, lay.boxW, lay.boxH)
}

// overlayCompletionDropdown composites an IDE-style popup of the Download
// Directory completion candidates over the settings modal, anchored on the
// input's cursor. It only paints when a completion session with at least two
// candidates is active.
func (m *Model) overlayCompletionDropdown(frame string, boxX, boxY, boxWidth, boxHeight int) string {
	setting := &m.Settings
	if !setting.EditMode || setting.FocusIndex != settingDownloadDir {
		return frame
	}
	matches := setting.dirMatches
	if len(matches) < 2 {
		return frame
	}

	listW, _, twoCol := setting.panes()
	var valueViewLine, inputCol int
	if twoCol {
		valueViewLine = 2
		inputCol = listW + settingsColumnGap + 1 + width(valueLabel) + 2
	} else {
		valueViewLine = settingCount + 1 + 2
		inputCol = 1 + width(valueLabel) + 2
	}

	input := setting.Inputs[settingDownloadDir]
	pos := input.Position()
	inWidth := input.Width
	if inWidth < 1 {
		inWidth = 1
	}
	visCol := pos
	if visCol > inWidth-1 {
		visCol = inWidth - 1
	}
	if visCol < 0 {
		visCol = 0
	}

	anchorX := boxX + 1 + inputCol + visCol
	if anchorX < 0 {
		anchorX = 0
	}
	anchorY := boxY + valueViewLine + 3

	// Cap the number of rows we actually display.
	shown := len(matches)
	availRows := m.terminalHeight - anchorY - 2
	if availRows < 1 {
		availRows = 1
	}
	if shown > availRows {
		shown = availRows
	}
	if shown > settingsMaxDirMatches {
		shown = settingsMaxDirMatches
	}
	if shown < 1 {
		shown = 1
	}
	if shown > len(matches) {
		shown = len(matches)
	}

	names := make([]string, shown)
	for i := 0; i < shown; i++ {
		names[i] = filepath.Base(matches[i])
	}

	dropW := 1
	for _, n := range names {
		if w := width(n); w > dropW {
			dropW = w
		}
	}
	dropW += dropdownChrome
	if dropW > boxWidth-2 {
		dropW = boxWidth - 2
	}
	if dropW < minDropdownWidth {
		dropW = minDropdownWidth
	}
	inner := dropW - boxBorderFrameWidth
	if inner < 1 {
		inner = 1
	}

	p := m.Style.Palette
	accent := lp.NewStyle().Foreground(lp.Color(p.Accent)).Bold(true)
	normal := lp.NewStyle().Foreground(lp.Color(p.Foreground))
	muted := lp.NewStyle().Foreground(lp.Color(p.Muted))

	cur := setting.dirMatchIdx
	if cur < 0 {
		cur = 0
	}

	contentLines := make([]string, 0, shown+1)
	for i := 0; i < shown; i++ {
		marker := "  "
		st := normal
		if i == cur {
			marker = "\u25b8 "
			st = accent
		}
		line := marker + names[i]
		if w := width(line); w < inner {
			line += strings.Repeat(" ", inner-w)
		}
		contentLines = append(contentLines, st.Render(truncate(line, inner)))
	}
	if len(matches) > shown {
		overflow := fmt.Sprintf("\u2026 +%d more", len(matches)-shown)
		contentLines = append(contentLines, muted.Render(truncate(overflow, inner)))
	}

	dropHeight := len(contentLines) + boxBorderFrameHeight
	dropdown := renderBox("", "", strings.Join(contentLines, "\n"), dropW, dropHeight, lp.Color(p.Accent))

	return overlayBlock(frame, dropdown, anchorX, anchorY, m.terminalWidth)
}

// deleteConfirmLayout describes the on-screen geometry of the confirm modal so
// rendering and mouse hit-testing agree on the button positions.
type deleteConfirmLayout struct {
	boxX, boxY int
	boxW, boxH int
	innerW     int
	buttonY    int
	noX, yesX  int
}

// confirmNoLabel and confirmYesLabel are the button labels. Yes sits to the
// right of No, and No is the default choice.
const (
	confirmNoLabel  = "[ No ]"
	confirmYesLabel = "[ Yes ]"
)

// deleteConfirmLayout computes the confirm modal geometry for the current
// terminal size.
func (m *Model) deleteConfirmLayout() deleteConfirmLayout {
	boxWidth := m.layout.settingsWidth
	if boxWidth < settingsMinWidth {
		boxWidth = settingsMinWidth
	}
	if boxWidth > m.terminalWidth {
		boxWidth = m.terminalWidth
	}
	if boxWidth < minConfirmWidth {
		boxWidth = minConfirmWidth
	}

	innerW := boxWidth - boxBorderFrameWidth
	if innerW < 1 {
		innerW = 1
	}

	buttons := confirmNoLabel + "   " + confirmYesLabel
	btnsW := width(buttons)
	if btnsW > innerW {
		btnsW = innerW
	}
	pad := (innerW - btnsW) / 2
	if pad < 0 {
		pad = 0
	}

	// Box lines: blank, content, blank, buttons, blank, help.
	boxHeight := 6 + boxBorderFrameHeight
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

	startX := boxX + 1 + pad
	return deleteConfirmLayout{
		boxX: boxX, boxY: boxY,
		boxW: boxWidth, boxH: boxHeight,
		innerW:  innerW,
		buttonY: boxY + 4,
		noX:     startX,
		yesX:    startX + width(confirmNoLabel) + 3,
	}
}

// deleteConfirmButtonAt returns the choice under (x, y), or -1 when the click
// does not land on a button.
func (m *Model) deleteConfirmButtonAt(x, y int) int {
	lay := m.deleteConfirmLayout()
	if y != lay.buttonY {
		return -1
	}
	if x >= lay.noX && x < lay.noX+width(confirmNoLabel) {
		return confirmChoiceNo
	}
	if x >= lay.yesX && x < lay.yesX+width(confirmYesLabel) {
		return confirmChoiceYes
	}
	return -1
}

// renderDeleteConfirmOverlay renders a centered confirmation popup as an overlay
// on top of the build list, leaving the surrounding content visible.
func (m *Model) renderDeleteConfirmOverlay() string {
	p := m.Style.Palette
	title := " Confirm "

	text := "Delete build " + m.deleteConfirmBuild.Version + "?"
	if m.deleteConfirmCancel {
		text = "Cancel download of " + m.deleteConfirmBuild.Version + "?"
	}

	m.computeLayout()
	lay := m.deleteConfirmLayout()

	header := renderHeader(m)
	body := m.renderBody()
	footer := m.renderBuildFooter()
	base := strings.Join([]string{header, body, footer}, "\n")

	content := m.Style.HelpDesc.Render(text)
	content = lp.NewStyle().Width(lay.innerW).Align(lp.Center).Render(truncate(content, lay.innerW))

	// Two selectable choices, navigable with left/right (or h/l). No is on the
	// left and is selected by default, so Yes must be chosen deliberately.
	yesStyle := m.Style.Muted
	noStyle := m.Style.Muted
	if m.deleteConfirmChoice == confirmChoiceYes {
		yesStyle = m.Style.ActivePaneTitle
	} else {
		noStyle = m.Style.ActivePaneTitle
	}
	noBtn := noStyle.Render(confirmNoLabel)
	yesBtn := yesStyle.Render(confirmYesLabel)
	buttons := noBtn + "   " + yesBtn
	buttons = lp.NewStyle().Width(lay.innerW).Align(lp.Center).Render(truncate(buttons, lay.innerW))

	help := m.renderHints([]hint{
		{key: "←/→", label: "choose"},
		{key: "enter", label: "confirm"},
		{key: "esc", label: "cancel"},
	})
	help = lp.NewStyle().Width(lay.innerW).Align(lp.Center).Render(truncate(help, lay.innerW))

	lines := make([]string, 0, 8)
	lines = append(lines, "")
	lines = append(lines, content)
	lines = append(lines, "", buttons, "", help)

	box := renderBox("",
		m.Style.ActivePaneTitle.Render(title),
		strings.Join(lines, "\n"),
		lay.boxW,
		lay.boxH,
		lp.Color(p.Accent),
	)
	return overlayBlock(base, box, lay.boxX, lay.boxY, m.terminalWidth)
}

// renderContextMenuOverlay renders a small popup listing the available actions
// for the build that was right-clicked.
func (m *Model) renderContextMenuOverlay() string {
	p := m.Style.Palette
	m.computeLayout()

	header := renderHeader(m)
	body := m.renderBody()
	footer := m.renderBuildFooter()
	base := strings.Join([]string{header, body, footer}, "\n")

	w, h := m.contextMenuSize()
	ox, oy := m.contextMenuOrigin()

	lines := make([]string, 0, len(m.contextMenuItems))
	for i, it := range m.contextMenuItems {
		label := it.label
		style := m.Style.RegularRow
		if i == m.contextMenuChoice {
			style = m.Style.SelectedRow
			label = "▸ " + label
		} else {
			label = "  " + label
		}
		inner := w - boxBorderFrameWidth
		if inner < 1 {
			inner = 1
		}
		line := style.Width(inner).Render(truncate(label, inner))
		lines = append(lines, line)
	}

	box := renderBox("", m.Style.ActivePaneTitle.Render(" Build Options "), strings.Join(lines, "\n"), w, h, lp.Color(p.Accent))
	frame := overlayBlock(base, box, ox, oy, m.terminalWidth)
	return m.overlayToast(frame)
}
