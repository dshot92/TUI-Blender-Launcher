//go:build linux
// +build linux

package launch

import (
	"fmt"
	"os/exec"
	"syscall"
)

// BlenderInNewTerminal launches Blender in a new terminal window (Linux-specific)
func BlenderInNewTerminal(blenderExe string) error {
	// The executable path is passed as a positional argument to `bash -c`
	// and referenced as "$0". This avoids interpolating it into the shell
	// script, so paths containing spaces or shell metacharacters are safe.
	// `exec "$0"` replaces bash with Blender so the terminal stays attached
	// to its output.
	terminals := []struct {
		name string
		args []string
	}{
		{"x-terminal-emulator", []string{"-e", "bash", "-c", `exec "$0"`, blenderExe}},
		{"gnome-terminal", []string{"--", "bash", "-c", `exec "$0"`, blenderExe}},
		{"alacritty", []string{"-e", "bash", "-c", `exec "$0"`, blenderExe}},
		{"xterm", []string{"-e", "bash", "-c", `exec "$0"`, blenderExe}},
		{"konsole", []string{"-e", "bash", "-c", `exec "$0"`, blenderExe}},
	}

	for _, term := range terminals {
		cmd := exec.Command(term.name, term.args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
		err := cmd.Start()
		if err == nil {
			cmd.Process.Release()
			return nil
		}
	}

	return fmt.Errorf("failed to launch Blender: no terminal emulator worked")
}
