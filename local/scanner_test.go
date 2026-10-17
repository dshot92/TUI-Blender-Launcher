package local

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dshot92/TUI-Blender-Launcher/download"
	"github.com/dshot92/TUI-Blender-Launcher/model"
)

// writeBuild creates a fake extracted build directory containing version.json.
func writeBuild(t *testing.T, root, dirName, version, hash string) string {
	t.Helper()
	dir := filepath.Join(root, dirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	meta := `{"version":"` + version + `","hash":"` + hash + `","branch":"main","platform":"linux","architecture":"x86_64"}`
	if err := os.WriteFile(filepath.Join(dir, "version.json"), []byte(meta), 0644); err != nil {
		t.Fatalf("write version.json: %v", err)
	}
	return dir
}

func TestScanLocalBuilds(t *testing.T) {
	root := t.TempDir()
	writeBuild(t, root, "blender-4.2.1-linux-x86_64", "4.2.1", "aaa111")
	writeBuild(t, root, "blender-4.0.0-linux-x86_64", "4.0.0", "bbb222")
	// Directories without version.json must be ignored.
	if err := os.MkdirAll(filepath.Join(root, "not-a-build"), 0755); err != nil {
		t.Fatal(err)
	}
	// The old-builds dir must be ignored.
	if err := os.MkdirAll(filepath.Join(root, download.OldBuildsDir, "old"), 0755); err != nil {
		t.Fatal(err)
	}

	builds, err := ScanLocalBuilds(root)
	if err != nil {
		t.Fatalf("ScanLocalBuilds: %v", err)
	}
	if len(builds) != 2 {
		t.Fatalf("got %d builds, want 2", len(builds))
	}
	// Sorted descending by version.
	if builds[0].Version != "4.2.1" || builds[1].Version != "4.0.0" {
		t.Fatalf("unexpected order: %q, %q", builds[0].Version, builds[1].Version)
	}
	for _, b := range builds {
		if b.Status != model.StateLocal {
			t.Errorf("build %s status = %v, want Local", b.Version, b.Status)
		}
	}
}

func TestScanLocalBuildsMissingDir(t *testing.T) {
	builds, err := ScanLocalBuilds(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("expected no error for missing dir, got %v", err)
	}
	if len(builds) != 0 {
		t.Fatalf("expected no builds, got %d", len(builds))
	}
}

func TestDeleteBuild(t *testing.T) {
	root := t.TempDir()
	writeBuild(t, root, "blender-4.2.1-linux-x86_64", "4.2.1", "aaa111")

	ok, err := DeleteBuild(root, "aaa111")
	if err != nil {
		t.Fatalf("DeleteBuild: %v", err)
	}
	if !ok {
		t.Fatal("DeleteBuild returned false, want true")
	}

	// The build must no longer be discoverable.
	builds, err := ScanLocalBuilds(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(builds) != 0 {
		t.Fatalf("build still present after delete: %d", len(builds))
	}

	// It should have been archived into .oldbuilds.
	entries, err := os.ReadDir(filepath.Join(root, download.OldBuildsDir))
	if err != nil {
		t.Fatalf("read oldbuilds: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 archived build, got %d", len(entries))
	}
}

func TestDeleteBuildNotFound(t *testing.T) {
	root := t.TempDir()
	writeBuild(t, root, "blender-4.2.1-linux-x86_64", "4.2.1", "aaa111")

	ok, err := DeleteBuild(root, "missing")
	if err == nil {
		t.Fatal("expected an error for a missing build")
	}
	if ok {
		t.Fatal("expected ok=false for a missing build")
	}
}

func TestBlenderExecutableCandidates(t *testing.T) {
	cases := []struct {
		goos string
		want []string
	}{
		{"windows", []string{filepath.Join("/lib", "blender-launcher.exe")}},
		{"linux", []string{filepath.Join("/lib", "blender")}},
		{"darwin", []string{
			filepath.Join("/lib", "Blender.app", "Contents", "MacOS", "Blender"),
			filepath.Join("/lib", "Blender", "Blender.app", "Contents", "MacOS", "Blender"),
			filepath.Join("/lib", "blender"),
		}},
	}
	for _, tc := range cases {
		got := blenderExecutableCandidates(tc.goos, "/lib")
		if len(got) != len(tc.want) {
			t.Fatalf("%s: got %v, want %v", tc.goos, got, tc.want)
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Errorf("%s candidate[%d] = %q, want %q", tc.goos, i, got[i], tc.want[i])
			}
		}
	}
}

// TestFindBlenderExecutable verifies discovery for the current platform's layout.
func TestFindBlenderExecutable(t *testing.T) {
	root := t.TempDir()
	candidates := blenderExecutableCandidates(runtime.GOOS, root)
	want := candidates[0]
	if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(want, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}

	if got := findBlenderExecutable(root); got != want {
		t.Errorf("findBlenderExecutable = %q, want %q", got, want)
	}
}

// TestFindBlenderExecutableMacOSFallback covers the nested, non-DMG archive
// layout, which only exists on macOS.
func TestFindBlenderExecutableMacOSFallback(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-only layout")
	}
	root := t.TempDir()
	nested := filepath.Join(root, "Blender", "Blender.app", "Contents", "MacOS", "Blender")
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(nested, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}

	if got := findBlenderExecutable(root); got != nested {
		t.Errorf("findBlenderExecutable = %q, want %q", got, nested)
	}
}
