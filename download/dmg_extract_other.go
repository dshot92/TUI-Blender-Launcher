//go:build !darwin
// +build !darwin

package download

import (
	"fmt"
	"runtime"
)

// extractDMG is a stub for platforms without hdiutil and ditto. Blender only
// ships disk images for macOS, so builds are filtered to this OS before they are
// ever downloaded; the stub exists so the package still compiles everywhere and
// fails loudly if this path is somehow reached.
func extractDMG(dmgPath, destDir string, progressCb ExtractionProgressCallback, cancelCh <-chan struct{}) (string, error) {
	return "", fmt.Errorf("DMG extraction is only supported on macOS, not %s", runtime.GOOS)
}
