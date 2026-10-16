package api

import (
	"testing"

	"github.com/dshot92/TUI-Blender-Launcher/model"
)

func TestAllowedArch(t *testing.T) {
	cases := []struct {
		goos, goarch string
		want         []string
	}{
		{"linux", "amd64", []string{"x86_64"}},
		{"linux", "arm64", []string{"arm64", "aarch64"}},
		{"darwin", "amd64", []string{"x86_64"}},
		{"darwin", "arm64", []string{"arm64"}},
		{"windows", "amd64", []string{"amd64"}},
		{"windows", "arm64", []string{"arm64"}},
	}
	for _, c := range cases {
		got := allowedArch(c.goos, c.goarch)
		if len(got) != len(c.want) {
			t.Errorf("allowedArch(%s,%s) has %d entries, want %d", c.goos, c.goarch, len(got), len(c.want))
			continue
		}
		for _, arch := range c.want {
			if !got[arch] {
				t.Errorf("allowedArch(%s,%s) missing %q", c.goos, c.goarch, arch)
			}
		}
	}
}

func TestMatchesPlatform(t *testing.T) {
	allowed := allowedArch("linux", "amd64")

	if !matchesPlatform(model.BlenderBuild{OperatingSystem: "linux", Architecture: "x86_64"}, "linux", allowed) {
		t.Error("linux/x86_64 should match")
	}
	if matchesPlatform(model.BlenderBuild{OperatingSystem: "windows", Architecture: "x86_64"}, "linux", allowed) {
		t.Error("wrong OS should not match")
	}
	if matchesPlatform(model.BlenderBuild{OperatingSystem: "linux", Architecture: "arm64"}, "linux", allowed) {
		t.Error("wrong arch should not match")
	}
}

func TestHasAllowedExtension(t *testing.T) {
	for _, ext := range []string{"zip", "tar.xz", "tar.gz", "tar.bz2", "xz", "dmg", "pkg", "ZIP"} {
		if !hasAllowedExtension(ext) {
			t.Errorf("%q should be allowed", ext)
		}
	}
	for _, ext := range []string{"sha256", "msi", "msix", ""} {
		if hasAllowedExtension(ext) {
			t.Errorf("%q should not be allowed", ext)
		}
	}
}
