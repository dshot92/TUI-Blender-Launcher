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
	header := renderHeader(m)
	body := m.renderBody()
	footer := m.renderBuildFooter()
	base := strings.Join([]string{header, body, footer}, "\n")
	title := " Settings "
	if m.currentView == viewInitialSetup {
		title = " Welcome — Initial Setup "
	}

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

	content := m.Settings.View()

	help := m.renderHints(m.settingsHints())
	help = lp.NewStyle().Width(innerW).Align(lp.Center).Render(truncate(help, innerW))

	// Build the box content and size the modal to it, so it stays snug rather
	// than leaving a large empty gap above the help line. The status/error toast
	// is layered over the bottom of the frame, not inside the box.
	lines := make([]string, 0, 16)
	lines = append(lines, "")
	lines = append(lines, strings.Split(content, "\n")...)
	lines = append(lines, "", help)

	boxHeight := len(lines) + boxBorderFrameHeight
	if boxHeight > m.terminalHeight {
		boxHeight = m.terminalHeight
	}
	if minH := boxBorderFrameHeight + 2; boxHeight < minH {
		boxHeight = minH
	}

	box := renderBox("",
		m.Style.ActivePaneTitle.Render(title),
		strings.Join(lines, "\n"),
		boxWidth,
		boxHeight,
		lp.Color(p.Accent),
	)

	boxX := (m.terminalWidth - boxWidth) / 2
	if boxX < 0 {
		boxX = 0
	}
	boxY := (m.terminalHeight - boxHeight) / 2
	if boxY < 0 {
		boxY = 0
	}

	frame := overlayBlock(base, box, boxX, boxY, m.terminalWidth)
	return m.overlayCompletionDropdown(frame, boxX, boxY, boxWidth, boxHeight)
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
		inputCol = listW + 3 + 1 + width("Value: ") + 2
	} else {
		valueViewLine = settingCount + 1 + 2
		inputCol = 1 + width("Value: ") + 2
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
	dropW += 4
	if dropW > boxWidth-2 {
		dropW = boxWidth - 2
	}
	if dropW < 8 {
		dropW = 8
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
	header := renderHeader(m)
	body := m.renderBody()
	footer := m.renderBuildFooter()
	base := strings.Join([]string{header, body, footer}, "\n")

	boxWidth := m.layout.settingsWidth
	if boxWidth < settingsMinWidth {
		boxWidth = settingsMinWidth
	}
	if boxWidth > m.terminalWidth {
		boxWidth = m.terminalWidth
	}
	if boxWidth < 30 {
		boxWidth = 30
	}

	innerW := boxWidth - boxBorderFrameWidth
	if innerW < 1 {
		innerW = 1
	}

	content := m.Style.HelpDesc.Render(text)
	content = lp.NewStyle().Width(innerW).Align(lp.Center).Render(truncate(content, innerW))

	// Two selectable choices, navigable with left/right (or h/l).
	yesStyle := m.Style.Muted
	noStyle := m.Style.Muted
	if m.deleteConfirmChoice == 0 {
		yesStyle = m.Style.ActivePaneTitle
	} else {
		noStyle = m.Style.ActivePaneTitle
	}
	yesBtn := yesStyle.Render("[ Yes ]")
	noBtn := noStyle.Render("[ No ]")
	buttons := yesBtn + "   " + noBtn
	buttons = lp.NewStyle().Width(innerW).Align(lp.Center).Render(truncate(buttons, innerW))

	help := m.renderHints([]hint{
		{"←/→", "choose"},
		{"enter", "confirm"},
		{"esc", "cancel"},
	})
	help = lp.NewStyle().Width(innerW).Align(lp.Center).Render(truncate(help, innerW))

	lines := make([]string, 0, 8)
	lines = append(lines, "")
	lines = append(lines, content)
	lines = append(lines, "", buttons, "", help)

	boxHeight := len(lines) + boxBorderFrameHeight
	if boxHeight > m.terminalHeight {
		boxHeight = m.terminalHeight
	}
	if minH := boxBorderFrameHeight + 2; boxHeight < minH {
		boxHeight = minH
	}

	box := renderBox("",
		m.Style.ActivePaneTitle.Render(title),
		strings.Join(lines, "\n"),
		boxWidth,
		boxHeight,
		lp.Color(p.Accent),
	)

	x := (m.terminalWidth - boxWidth) / 2
	y := (m.terminalHeight - boxHeight) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return overlayBlock(base, box, x, y, m.terminalWidth)
}
