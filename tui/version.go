package tui

import "strings"

// AppVersion is the application version reported in the header. It defaults to
// "dev" for local builds and is set by main from the build-time version (which
// releases inject via -ldflags "-X main.version=...").
var AppVersion = "dev"

// versionLabel returns the header's version string. Numeric release versions
// are prefixed with "v" (e.g. "1.2.3" -> "v1.2.3"), while named versions such
// as the default "dev" are shown as-is.
func versionLabel() string {
	v := strings.TrimSpace(AppVersion)
	if v == "" {
		return "dev"
	}
	if v[0] >= '0' && v[0] <= '9' {
		return "v" + v
	}
	return v
}
