package tui

import (
	"github.com/dshot92/TUI-Blender-Launcher/config"
	"github.com/dshot92/TUI-Blender-Launcher/model"
)

// Palette holds the semantic colors used across the TUI. Keeping them in one
// place (instead of scattering hardcoded color codes through the rendering
// code) makes the whole UI themeable and easy to reason about.
type Palette struct {
	// Name is the human-readable palette identifier shown in the header.
	Name string

	// Core surfaces
	Background string // app background
	Foreground string // primary text
	Muted      string // secondary text (descriptions, inactive labels)
	Subtle     string // very low-emphasis text / separators
	Accent     string // primary highlight: selection, keys, active borders
	Border     string // inactive pane borders

	// Semantic build/download states
	Local       string
	Update      string
	Online      string
	Error       string
	Downloading string
	Success     string

	// Gradient endpoints used for the logo and the download progress bar.
	GradientStart string
	GradientEnd   string
}

// DefaultPaletteName is the palette selected when the config value is empty or
// unknown. It shares the canonical name with config.DefaultThemeName.
const DefaultPaletteName = config.DefaultThemeName

// builtinPalettes is the registry of palettes that ship with the app. Users
// pick one via the `theme` key in config.toml.
var builtinPalettes = map[string]Palette{
	// Dracula palette (default).
	"dracula": {
		Name:          "dracula",
		Background:    "#282a36",
		Foreground:    "#f8f8f2",
		Muted:         "#6272a4",
		Subtle:        "#44475a",
		Accent:        "#ff79c6",
		Border:        "#44475a",
		Local:         "#bd93f9",
		Update:        "#50fa7b",
		Online:        "#ffb86c",
		Error:         "#ff5555",
		Downloading:   "#8be9fd",
		Success:       "#50fa7b",
		GradientStart: "#ff79c6",
		GradientEnd:   "#8be9fd",
	},
	"catppuccin": {
		Name:          "catppuccin",
		Background:    "#1e1e2e",
		Foreground:    "#cdd6f4",
		Muted:         "#7f849c",
		Subtle:        "#45475a",
		Accent:        "#cba6f7",
		Border:        "#45475a",
		Local:         "#b4befe",
		Update:        "#a6e3a1",
		Online:        "#fab387",
		Error:         "#f38ba8",
		Downloading:   "#89dceb",
		Success:       "#a6e3a1",
		GradientStart: "#cba6f7",
		GradientEnd:   "#89b4fa",
	},
	"nord": {
		Name:          "nord",
		Background:    "#2e3440",
		Foreground:    "#eceff4",
		Muted:         "#7b88a1",
		Subtle:        "#434c5e",
		Accent:        "#88c0d0",
		Border:        "#434c5e",
		Local:         "#b48ead",
		Update:        "#a3be8c",
		Online:        "#ebcb8b",
		Error:         "#bf616a",
		Downloading:   "#81a1c1",
		Success:       "#a3be8c",
		GradientStart: "#88c0d0",
		GradientEnd:   "#b48ead",
	},
	"gruvbox": {
		Name:          "gruvbox",
		Background:    "#282828",
		Foreground:    "#ebdbb2",
		Muted:         "#928374",
		Subtle:        "#3c3836",
		Accent:        "#fabd2f",
		Border:        "#3c3836",
		Local:         "#d3869b",
		Update:        "#b8bb26",
		Online:        "#fe8019",
		Error:         "#fb4934",
		Downloading:   "#83a598",
		Success:       "#b8bb26",
		GradientStart: "#fabd2f",
		GradientEnd:   "#fb4934",
	},
	"tokyonight": {
		Name:          "tokyonight",
		Background:    "#1a1b26",
		Foreground:    "#c0caf5",
		Muted:         "#565f89",
		Subtle:        "#292e42",
		Accent:        "#bb9af7",
		Border:        "#292e42",
		Local:         "#7aa2f7",
		Update:        "#9ece6a",
		Online:        "#e0af68",
		Error:         "#f7768e",
		Downloading:   "#7dcfff",
		Success:       "#9ece6a",
		GradientStart: "#bb9af7",
		GradientEnd:   "#7dcfff",
	},
}

// PaletteNames returns the sorted list of built-in palette names. Used for
// validation and documentation.
func PaletteNames() []string {
	return []string{"catppuccin", "dracula", "gruvbox", "nord", "tokyonight"}
}

// PaletteByName returns the built-in palette for the given name. The second
// return value reports whether the name was recognized.
func PaletteByName(name string) (Palette, bool) {
	p, ok := builtinPalettes[name]
	return p, ok
}

// DefaultPalette returns the palette used when no (valid) theme is configured.
func DefaultPalette() Palette {
	return builtinPalettes[DefaultPaletteName]
}

// ResolvePalette returns the palette for the given name, falling back to the
// default palette when the name is empty or unknown.
func ResolvePalette(name string) Palette {
	if p, ok := PaletteByName(name); ok {
		return p
	}
	return DefaultPalette()
}

// StateColor returns the semantic color associated with a build state.
func (p Palette) StateColor(s model.BuildState) string {
	switch s {
	case model.StateLocal:
		return p.Local
	case model.StateUpdate:
		return p.Update
	case model.StateOnline:
		return p.Online
	case model.StateFailed, model.StateCancelled:
		return p.Error
	case model.StateDownloading, model.StateExtracting:
		return p.Downloading
	default:
		return p.Muted
	}
}

// stateGlyph returns a compact leading marker for a build state. All glyphs are
// single-width so they never disturb column alignment.
func stateGlyph(s model.BuildState) string {
	switch s {
	case model.StateLocal:
		return "●"
	case model.StateUpdate:
		return "↑"
	case model.StateOnline:
		return "↓"
	case model.StateDownloading:
		return "↓"
	case model.StateExtracting:
		return "◆"
	case model.StateFailed:
		return "✕"
	case model.StateCancelled:
		return "○"
	default:
		return "·"
	}
}
