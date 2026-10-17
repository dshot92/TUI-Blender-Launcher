package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/dshot92/TUI-Blender-Launcher/config" // Import config package
	"github.com/dshot92/TUI-Blender-Launcher/tui"    // Import the tui package

	tea "github.com/charmbracelet/bubbletea"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

// resolveVersion returns the version to display. Release and local builds
// inject it via -ldflags "-X main.version=..."; when that is absent (e.g.
// `go run .` or `go install module@vX.Y.Z`) it falls back to the embedded VCS
// revision or the module version so the header never shows a bare "dev" for a
// known checkout.
func resolveVersion() string {
	if v := strings.TrimSpace(version); v != "" && v != "dev" {
		return v
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}

	// Prefer the embedded VCS revision for source builds: a plain `go build`
	// in a checkout otherwise reports the module as a noisy pseudo-version.
	var rev, dirty string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "-dirty"
			}
		}
	}
	if rev != "" {
		if len(rev) > 7 {
			rev = rev[:7]
		}
		return "dev+" + rev + dirty
	}

	// Fall back to the module version, which Go sets for `go install pkg@vX.Y.Z`.
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return strings.TrimPrefix(v, "v")
	}

	return "dev"
}

func printUsage() {
	fmt.Fprintf(os.Stdout, "TUI Blender Launcher\n\n")
	fmt.Fprintf(os.Stdout, "A terminal UI for finding, downloading, managing, and launching Blender builds.\n\n")
	fmt.Fprintf(os.Stdout, "Usage:\n  %s [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stdout, "Options:\n")
	flag.PrintDefaults()
}

func main() {
	showVersion := flag.Bool("version", false, "Print version and exit")
	showHelp := flag.Bool("help", false, "Print usage and exit")
	flag.BoolVar(showVersion, "v", false, "Print version and exit (shorthand)")
	flag.BoolVar(showHelp, "h", false, "Print usage and exit (shorthand)")
	flag.Usage = printUsage
	flag.Parse()

	if *showVersion {
		fmt.Printf("tui-blender-launcher %s\n", resolveVersion())
		return
	}
	if *showHelp {
		printUsage()
		return
	}

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Check if config file *actually* exists (LoadConfig returns defaults if not)
	configFilePath, _ := config.GetConfigPath()
	needsInitialSetup := false
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		needsInitialSetup = true
	}

	// Initialize the TUI model, passing the config and setup flag
	tui.AppVersion = resolveVersion()
	m := tui.InitialModel(cfg, needsInitialSetup)

	// Create and run the Bubble Tea program
	p := tea.NewProgram(m,
		tea.WithAltScreen(),       // Use AltScreen
		tea.WithMouseCellMotion(), // Enable mouse support
	)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
