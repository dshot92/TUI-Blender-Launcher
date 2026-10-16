package tui

import (
	"fmt"
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// renderPageForView assembles the full frame: header box, optional status line,
// the body (builds + optional detail pane, or the settings modal), and the
// footer.
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

	// The settings view is a modal: it replaces the header/body/footer chrome
	// with a centered box over a dimmed background.
	if m.currentView == viewInitialSetup || m.currentView == viewSettings {
		return m.renderSettingsModal()
	}

	header := renderHeader(m)
	body := m.renderBody()
	footer := m.renderBuildFooter()

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

// renderSettingsModal renders the settings (or initial setup) view as a centered
// box over a dimmed background, Surge-style. The key hints and any status/error
// line live inside the box because the overlay replaces the usual chrome.
func (m *Model) renderSettingsModal() string {
	p := m.Style.Palette
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

	// Status / error line, since the overlay hides the usual status bar.
	var messageLine string
	if m.err != nil {
		messageLine = lp.NewStyle().Foreground(lp.Color(p.Error)).Bold(true).
			Render(truncate("✖ "+m.err.Error(), innerW))
	} else if m.status != "" {
		messageLine = lp.NewStyle().Foreground(lp.Color(p.Success)).
			Render(truncate("✓ "+m.status, innerW))
	}

	help := m.renderHints(m.settingsHints())
	help = lp.NewStyle().Width(innerW).Align(lp.Center).Render(truncate(help, innerW))

	// Build the box content and size the modal to it, so it stays snug rather
	// than leaving a large empty gap above the help line.
	lines := make([]string, 0, 16)
	lines = append(lines, "")
	if messageLine != "" {
		lines = append(lines, messageLine, "")
	}
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

	return lp.Place(m.terminalWidth, m.terminalHeight, lp.Center, lp.Center, box,
		lp.WithWhitespaceChars(" "),
		lp.WithWhitespaceBackground(lp.Color(p.Background)))
}
