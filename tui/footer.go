package tui

import (
	"TUI-Blender-Launcher/download"
	"TUI-Blender-Launcher/model"
	"os"
	"path/filepath"
	"strings"
)

// hint is a single footer key/label pair.
type hint struct {
	key   string
	label string
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

// renderBuildFooter renders the footer for the build list view.
func (m *Model) renderBuildFooter() string {
	infoLabel := "Show Info"
	if m.layout.detailVisible {
		infoLabel = "Hide Info"
	}
	general := []hint{
		{PrimaryKey(CmdFetchBuilds), "Fetch"},
		{PrimaryKey(CmdToggleSortOrder), "Reverse Sort"},
		{PrimaryKey(CmdToggleDetail), infoLabel},
		{PrimaryKey(CmdShowSettings), "Settings"},
		{PrimaryKey(CmdQuit), "Quit"},
	}

	// Contextual commands based on the highlighted build.
	contextual := []hint{}
	if build := m.List.GetSelectedBuild(); build != nil {
		downloading := build.Status == model.StateDownloading || build.Status == model.StateExtracting
		if !downloading && m.commands != nil && m.commands.downloads != nil {
			if state := m.commands.downloads.GetState(build.ID()); state != nil &&
				(state.BuildState == model.StateDownloading || state.BuildState == model.StateExtracting) {
				downloading = true
			}
		}

		switch {
		case downloading:
			contextual = append(contextual, hint{PrimaryKey(CmdDeleteBuild), "Cancel"})
		default:
			switch build.Status {
			case model.StateLocal:
				contextual = append(contextual,
					hint{PrimaryKey(CmdLaunchBuild), "Launch"},
					hint{PrimaryKey(CmdOpenBuildDir), "Open Dir"},
					hint{PrimaryKey(CmdDeleteBuild), "Delete"},
				)
			case model.StateUpdate:
				contextual = append(contextual,
					hint{PrimaryKey(CmdDownloadBuild), "Download"},
					hint{PrimaryKey(CmdLaunchBuild), "Launch"},
					hint{PrimaryKey(CmdOpenBuildDir), "Open Dir"},
					hint{PrimaryKey(CmdDeleteBuild), "Delete"},
				)
			case model.StateOnline, model.StateCancelled, model.StateFailed:
				contextual = append(contextual, hint{PrimaryKey(CmdDownloadBuild), "Download"})
			}
		}
	}

	return m.renderFooter(m.renderHints(contextual), m.renderHints(general))
}

// settingsHints returns the key hints shown inside the settings modal.
func (m *Model) settingsHints() []hint {
	// While editing a text field, most keys go to the input, so advertise the
	// two ways out: save (enter) or discard (esc).
	if m.Settings.EditMode {
		return []hint{
			{"esc", "cancel"},
			{"enter", "save"},
		}
	}

	// While toggling release types, advertise the toggle controls.
	if m.Settings.releaseEditing {
		return []hint{
			{"←/→", "move"},
			{"space", "toggle"},
			{"esc", "done"},
		}
	}

	// Only offer the clean option when there is something to clean.
	showClean := false
	oldBuildsDir := filepath.Join(m.config.DownloadDir, download.OldBuildsDir)
	if entries, err := os.ReadDir(oldBuildsDir); err == nil && len(entries) > 0 {
		showClean = true
	}

	commands := []hint{
		{PrimaryKey(CmdToggleEditMode), "edit"},
		{PrimaryKey(CmdMoveLeft), "change"},
		{PrimaryKey(CmdSaveSettings), "save"},
	}
	if showClean {
		commands = append(commands, hint{PrimaryKey(CmdCleanOldBuilds), "clean"})
	}
	commands = append(commands, hint{PrimaryKey(CmdQuit), "quit"})

	return commands
}
