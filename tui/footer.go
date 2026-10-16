package tui

import (
	"github.com/dshot92/TUI-Blender-Launcher/download"
	"github.com/dshot92/TUI-Blender-Launcher/model"
	"os"
	"path/filepath"
	"strings"
)

// hint is a single footer key/label pair.
type hint struct {
	key   string
	label string
	cmd   CommandType
}

// renderHints renders a slice of hints separated by a subtle divider.
func (m *Model) renderHints(hints []hint) string {
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		if h.key == "" {
			parts = append(parts, m.Style.HelpDesc.Render(h.label))
			continue
		}
		parts = append(parts, m.Style.HelpKey.Render(h.key)+m.Style.HelpDesc.Render(" "+h.label))
	}
	return strings.Join(parts, m.Style.HelpSep.Render("  ·  "))
}

// renderFooter lays the two hint lines out, aligned to the terminal width and
// padded so the footer is always exactly footerHeight lines.
func (m *Model) renderFooter(line1, line2 string) string {
	line1 = truncate(line1, m.terminalWidth)
	line2 = truncate(line2, m.terminalWidth)
	return m.Style.Footer.Width(m.terminalWidth).Render(line1 + "\n" + line2)
}

// footerHints returns the contextual (top) and general (bottom) footer hint
// lists, each annotated with its command type for mouse hit-testing.
func (m *Model) footerHints() (contextual, general []hint) {
	infoLabel := "Show Info"
	if m.layout.detailVisible {
		infoLabel = "Hide Info"
	}
	general = []hint{
		{key: PrimaryKey(CmdFetchBuilds), label: "Fetch", cmd: CmdFetchBuilds},
		{key: PrimaryKey(CmdToggleSortOrder), label: "Reverse Sort", cmd: CmdToggleSortOrder},
		{key: PrimaryKey(CmdToggleDetail), label: infoLabel, cmd: CmdToggleDetail},
		{key: PrimaryKey(CmdShowSettings), label: "Settings", cmd: CmdShowSettings},
		{key: PrimaryKey(CmdCleanOldBuilds), label: "Clean Old", cmd: CmdCleanOldBuilds},
		{key: PrimaryKey(CmdQuit), label: "Quit", cmd: CmdQuit},
	}

	contextual = []hint{}
	if build := m.List.GetSelectedBuild(); build != nil {
		downloading := m.isDownloading(*build)

		switch {
		case downloading:
			contextual = append(contextual, hint{key: PrimaryKey(CmdDeleteBuild), label: "Cancel", cmd: CmdDeleteBuild})
		default:
			switch build.Status {
			case model.StateLocal:
				contextual = append(contextual,
					hint{key: PrimaryKey(CmdLaunchBuild), label: "Launch", cmd: CmdLaunchBuild},
					hint{key: PrimaryKey(CmdOpenBuildDir), label: "Open Dir", cmd: CmdOpenBuildDir},
					hint{key: PrimaryKey(CmdDeleteBuild), label: "Delete", cmd: CmdDeleteBuild},
				)
			case model.StateUpdate:
				contextual = append(contextual,
					hint{key: PrimaryKey(CmdDownloadBuild), label: "Download", cmd: CmdDownloadBuild},
					hint{key: PrimaryKey(CmdLaunchBuild), label: "Launch", cmd: CmdLaunchBuild},
					hint{key: PrimaryKey(CmdOpenBuildDir), label: "Open Dir", cmd: CmdOpenBuildDir},
					hint{key: PrimaryKey(CmdDeleteBuild), label: "Delete", cmd: CmdDeleteBuild},
				)
			case model.StateOnline, model.StateCancelled, model.StateFailed:
				contextual = append(contextual, hint{key: PrimaryKey(CmdDownloadBuild), label: "Download", cmd: CmdDownloadBuild})
			}
			if build.DownloadURL != "" && build.Status != model.StateLocal {
				contextual = append(contextual, hint{key: PrimaryKey(CmdOpenURL), label: "Open URL", cmd: CmdOpenURL})
			}
		}
	}
	return contextual, general
}

// renderBuildFooter renders the footer for the build list view.
func (m *Model) renderBuildFooter() string {
	contextual, general := m.footerHints()
	return m.renderFooter(m.renderHints(contextual), m.renderHints(general))
}

// footerCommandAt maps a click in the footer region to the command it invokes.
// It returns false when (x, y) does not land on a rendered hint.
func (m *Model) footerCommandAt(x, y int) (CommandType, bool) {
	if m.terminalHeight <= 0 {
		return 0, false
	}
	topY := m.terminalHeight - footerHeight
	if y < topY || y >= m.terminalHeight {
		return 0, false
	}
	contextual, general := m.footerHints()
	var hints []hint
	if y == topY {
		hints = contextual
	} else {
		hints = general
	}
	if len(hints) == 0 {
		return 0, false
	}
	return hintAt(hints, x, 0)
}

// hintAt maps an x coordinate to the command of the hint it lands on, given the
// column where the hint line starts. Hints are laid out as "key label" segments
// separated by "  ·  ", matching renderHints.
func hintAt(hints []hint, x, lineStart int) (CommandType, bool) {
	sepWidth := width("  ·  ")
	cursor := lineStart
	for i, h := range hints {
		segment := h.label
		if h.key != "" {
			segment = h.key + " " + h.label
		}
		segWidth := width(segment)
		if x >= cursor && x < cursor+segWidth {
			return h.cmd, true
		}
		cursor += segWidth
		if i < len(hints)-1 {
			cursor += sepWidth
		}
	}
	return 0, false
}

// settingsHints returns the key hints shown inside the settings modal.
func (m *Model) settingsHints() []hint {
	// While editing a text field, most keys go to the input, so advertise the
	// two ways out: save (enter) or discard (esc).
	if m.Settings.EditMode {
		return []hint{
			{key: "esc", label: "cancel", cmd: CmdNone},
			{key: "enter", label: "save", cmd: CmdNone},
		}
	}

	// While toggling release types, advertise the toggle controls.
	if m.Settings.releaseEditing {
		return []hint{
			{key: "space", label: "toggle", cmd: CmdNone},
			{key: "enter", label: "save", cmd: CmdNone},
			{key: "esc", label: "cancel", cmd: CmdNone},
		}
	}

	// Only offer the clean option when there is something to clean.
	showClean := false
	oldBuildsDir := filepath.Join(m.config.DownloadDir, download.OldBuildsDir)
	if entries, err := os.ReadDir(oldBuildsDir); err == nil && len(entries) > 0 {
		showClean = true
	}

	commands := []hint{
		{key: PrimaryKey(CmdToggleEditMode), label: "edit", cmd: CmdToggleEditMode},
		{key: PrimaryKey(CmdMoveLeft), label: "change", cmd: CmdMoveLeft},
		{key: PrimaryKey(CmdSaveSettings), label: "save", cmd: CmdSaveSettings},
	}
	if showClean {
		commands = append(commands, hint{key: PrimaryKey(CmdCleanOldBuilds), label: "clean", cmd: CmdCleanOldBuilds})
	}
	commands = append(commands, hint{key: PrimaryKey(CmdQuit), label: "quit", cmd: CmdQuit})

	return commands
}
