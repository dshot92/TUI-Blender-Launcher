package tui

import (
	"os"
	"testing"
)

// TestMain redirects the config path to a throwaway directory for the whole
// package. The settings view auto-saves, so any test that opens it and changes
// a value would otherwise write the developer's real
// ~/.config/tui-blender-launcher/config.toml. Individual tests may still
// override the location with t.Setenv.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tui-config-test")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", dir)

	code := m.Run()

	os.RemoveAll(dir)
	os.Exit(code)
}
