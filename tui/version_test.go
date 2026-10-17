package tui

import "testing"

func TestVersionLabel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"release version", "1.2.3", "v1.2.3"},
		{"release version with v", "v1.2.3", "v1.2.3"},
		{"git-describe between tags", "1.0.0-5-gabc1234", "v1.0.0-5-gabc1234"},
		{"prerelease", "1.0.0-rc.1", "v1.0.0-rc.1"},
		{"dev default", "dev", "dev"},
		{"dev with revision", "dev+583da1c", "dev+583da1c"},
		{"dev dirty", "dev+583da1c-dirty", "dev+583da1c-dirty"},
		{"bare short sha", "583da1c", "583da1c"},
		{"bare short sha dirty", "583da1c-dirty", "583da1c-dirty"},
		{"empty falls back to dev", "", "dev"},
		{"whitespace falls back to dev", "   ", "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := AppVersion
			AppVersion = tt.in
			defer func() { AppVersion = old }()

			if got := versionLabel(); got != tt.want {
				t.Fatalf("versionLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}
