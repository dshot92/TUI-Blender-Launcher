package api

import (
	"strings"

	"github.com/dshot92/TUI-Blender-Launcher/model"
)

// allowedExtensions is the set of file extensions we consider downloadable
// Blender builds. Checksum and installer-only artifacts (e.g. sha256, msi,
// msix) are excluded.
var allowedExtensions = map[string]bool{
	"zip": true, "tar.gz": true, "tar.xz": true, "tar.bz2": true,
	"xz": true, "dmg": true, "pkg": true,
}

// allowedArch returns the set of API architecture strings accepted for the
// given GOOS/GOARCH. GOOS values (linux, windows, darwin) match the API
// "platform" field directly. Known aliases are included: Blender labels Linux
// ARM builds "aarch64" while Go reports "arm64".
func allowedArch(goos, goarch string) map[string]bool {
	var apiArch string

	switch goos {
	case "linux":
		switch goarch {
		case "amd64":
			apiArch = "x86_64" // Map Go's amd64 to API's x86_64
		case "arm64":
			// The API uses "arm64" for Linux ARM; verified data did not contain
			// Linux ARM builds from this endpoint.
			apiArch = "arm64"
		default:
			apiArch = goarch
		}
	case "darwin": // macOS
		switch goarch {
		case "amd64":
			apiArch = "x86_64"
		case "arm64":
			apiArch = "arm64"
		default:
			apiArch = goarch
		}
	case "windows":
		switch goarch {
		case "amd64":
			apiArch = "amd64"
		case "arm64":
			apiArch = "arm64"
		default:
			apiArch = goarch
		}
	default:
		// For an unknown OS, use Go's arch name; the OS check below filters it.
		apiArch = goarch
	}

	allowed := map[string]bool{apiArch: true}
	if goos == "linux" && goarch == "arm64" {
		allowed["aarch64"] = true
	}
	return allowed
}

// matchesPlatform reports whether a build targets the current OS and one of the
// allowed architectures.
func matchesPlatform(build model.BlenderBuild, currentOS string, allowed map[string]bool) bool {
	return build.OperatingSystem == currentOS && allowed[build.Architecture]
}

// hasAllowedExtension reports whether the build's file extension is one we can
// download and extract. Matching is case-insensitive.
func hasAllowedExtension(ext string) bool {
	return allowedExtensions[strings.ToLower(ext)]
}
