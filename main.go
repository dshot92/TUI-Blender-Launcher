package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dshot92/TUI-Blender-Launcher/config" // Import config package
	"github.com/dshot92/TUI-Blender-Launcher/tui"    // Import the tui package

	tea "github.com/charmbracelet/bubbletea"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

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
		fmt.Printf("tui-blender-launcher %s\n", version)
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
