package tui

import (
	"fmt"
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// renderPageForView assembles the full frame: header box, optional status line,
// the body (builds + optional detail pane, or settings), and the footer.
func (m *Model) renderPageForView() string {
	if m.terminalWidth <= 0 || m.terminalHeight <= 0 {
		return "Loading…"
	}
	if m.terminalWidth < minTermWidth || m.terminalHeight < minTermHeight {
		msg := m.Style.Empty.Render(fmt.Sprintf("Terminal too small — need at least %d×%d", minTermWidth, minTermHeight))
		msg = truncate(msg, m.terminalWidth)
		return lp.Place(m.terminalWidth, m.terminalHeight, lp.Center, lp.Center, msg)
	}

	m.computeLayout()

	header := renderHeader(m)

	var body, footer string
	if m.currentView == viewInitialSetup || m.currentView == viewSettings {
		body = m.renderSettingsPane()
		footer = m.renderSettingsFooter()
	} else {
		body = m.renderBody()
		footer = m.renderBuildFooter()
	}

	var statusLine string
	if m.err != nil {
		statusLine = m.Style.Error.Render(truncate(" ! "+m.err.Error(), m.terminalWidth))
	} else if m.status != "" {
		statusLine = m.Style.Status.Render(truncate(" "+m.status, m.terminalWidth))
	}

	parts := make([]string, 0, 4)
	parts = append(parts, header)
	if statusLine != "" {
		parts = append(parts, statusLine)
	}
	parts = append(parts, body, footer)

	return strings.Join(parts, "\n")
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

// renderSettingsPane wraps the settings content in a centered bordered pane.
func (m *Model) renderSettingsPane() string {
	title := " Settings "
	if m.currentView == viewInitialSetup {
		title = " Welcome — Initial Setup "
	}
	box := renderBox("",
		m.Style.ActivePaneTitle.Render(title),
		m.Settings.View(),
		m.layout.settingsWidth,
		m.layout.bodyHeight,
		lp.Color(m.Style.Palette.Accent),
	)
	return lp.Place(m.terminalWidth, m.layout.bodyHeight, lp.Center, lp.Top, box)
}
