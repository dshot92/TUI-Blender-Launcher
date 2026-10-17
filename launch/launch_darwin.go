//go:build darwin
// +build darwin

package launch

import (
	"fmt"
	"os/exec"
)

// BlenderInNewTerminal launches Blender in a new terminal window (macOS-specific).
//
// On macOS a Blender build is an .app bundle whose runnable binary lives at
// Blender.app/Contents/MacOS/Blender. Running that binary directly, rather than
// opening the bundle through LaunchServices, is what makes Blender's console
// output visible. Terminal is asked to run it via AppleScript; the path is passed
// as an argument and quoted inside AppleScript, so spaces and shell
// metacharacters in the path are handled safely.
func BlenderInNewTerminal(blenderExe string) error {
	const script = `on run argv
	set exePath to item 1 of argv
	tell application "Terminal"
		activate
		do script (quoted form of exePath)
	end tell
end run`

	cmd := exec.Command("osascript", "-e", script, blenderExe)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch Blender in new terminal: %w", err)
	}
	// osascript returns as soon as Terminal has been asked to run the command,
	// so release it to avoid leaving a zombie process behind.
	_ = cmd.Process.Release()
	return nil
}
