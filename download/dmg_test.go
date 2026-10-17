package download

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDMGRootDirName(t *testing.T) {
	cases := map[string]string{
		"/tmp/blender-4.2.1-macos-arm64.dmg": "blender-4.2.1-macos-arm64",
		"Blender.dmg":                        "Blender",
		"weird.dmg.dmg":                      "weird.dmg",
		"no-extension":                       "no-extension",
	}
	for in, want := range cases {
		if got := dmgRootDirName(in); got != want {
			t.Errorf("dmgRootDirName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFindAppBundles(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "Blender.app", "Contents", "MacOS"))
	mustMkdirAll(t, filepath.Join(root, "Blenderplayer.app", "Contents", "MacOS"))
	// A plain directory and the "Applications" symlink baked into a DMG must be
	// ignored, as must a plain file whose name merely ends in .app.
	mustMkdirAll(t, filepath.Join(root, "extras"))
	if err := os.Symlink("/Applications", filepath.Join(root, "Applications")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "fake.app"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write fake.app: %v", err)
	}

	got, err := findAppBundles(root)
	if err != nil {
		t.Fatalf("findAppBundles: %v", err)
	}
	want := []string{
		filepath.Join(root, "Blender.app"),
		filepath.Join(root, "Blenderplayer.app"),
	}
	if len(got) != len(want) {
		t.Fatalf("findAppBundles = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("bundle[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFindAppBundlesMissingDir(t *testing.T) {
	if _, err := findAppBundles(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("expected an error for a missing directory")
	}
}

func TestDirSize(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a"), make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "b"), make([]byte, 50), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := dirSize(root); got != 150 {
		t.Errorf("dirSize = %d, want 150", got)
	}
}

// TestExtractDMGOnlyOnDarwin documents that non-macOS platforms get the stub and
// fail fast instead of silently doing nothing.
func TestExtractDMGOnlyOnDarwin(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("the darwin implementation mounts a real disk image")
	}
	if _, err := extractDMG("/nonexistent.dmg", t.TempDir(), nil, make(chan struct{})); err == nil {
		t.Fatal("expected an error on non-macOS platforms")
	}
}

func mustMkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}
