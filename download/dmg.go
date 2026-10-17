package download

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// dmgExtension is the macOS disk-image extension handled by extractDMG.
const dmgExtension = ".dmg"

// dmgAppBundleExt is the suffix of the macOS application bundles found inside a
// Blender disk image.
const dmgAppBundleExt = ".app"

// dmgRootDirName derives the directory a disk image is extracted into from the
// image's file name, e.g. "blender-4.2.1-macos-arm64.dmg" becomes
// "blender-4.2.1-macos-arm64". This mirrors the single-root-directory layout of
// the .zip and .tar.xz archives, so scanner and metadata code work unchanged.
func dmgRootDirName(dmgPath string) string {
	return strings.TrimSuffix(filepath.Base(dmgPath), dmgExtension)
}

// findAppBundles returns the .app bundles directly inside dir, sorted by name.
// A Blender disk image holds one bundle (Blender.app) at its root; UPBGE images
// also carry Blenderplayer.app. Non-directory matches and the "Applications"
// symlink baked into the image are ignored.
func findAppBundles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", dir, err)
	}

	var bundles []string
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), dmgAppBundleExt) {
			continue
		}
		// Stat (not entry.IsDir) so a bundle reached through a symlink is still
		// recognised as a directory.
		full := filepath.Join(dir, entry.Name())
		info, err := os.Stat(full)
		if err != nil || !info.IsDir() {
			continue
		}
		bundles = append(bundles, full)
	}

	sort.Strings(bundles)
	return bundles, nil
}

// dirSize returns the total size in bytes of the regular files under root. It is
// best-effort: entries that cannot be read are skipped so progress reporting can
// never fail an extraction.
func dirSize(root string) int64 {
	var total int64
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}
