package tui

import "strings"

// AppVersion is the application version reported in the header. It defaults to
// "dev" for local builds and is set by main from the build-time version (which
// releases inject via -ldflags "-X main.version=...").
var AppVersion = "dev"

// versionLabel returns the header's version string. Numeric release versions
// are prefixed with "v" (e.g. "1.2.3" -> "v1.2.3", "1.0.0-5-gabc" ->
// "v1.0.0-5-gabc"), while dev/named versions such as "dev", "dev+583da1c", or
// a bare short SHA are shown as-is.
func versionLabel() string {
	v := strings.TrimSpace(AppVersion)
	if v == "" {
		return "dev"
	}
	if v[0] >= '0' && v[0] <= '9' && strings.Contains(v, ".") {
		return "v" + v
	}
	return v
}
