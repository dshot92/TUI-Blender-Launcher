//go:build !linux && !darwin && !windows
// +build !linux,!darwin,!windows

package launch

import (
	"fmt"
	"runtime"
)

// BlenderInNewTerminal is a fallback for platforms without a dedicated
// terminal-launch implementation. It reports that launching is unsupported
// rather than failing to compile.
func BlenderInNewTerminal(blenderExe string) error {
	return fmt.Errorf("launching Blender in a new terminal is not supported on %s", runtime.GOOS)
}
