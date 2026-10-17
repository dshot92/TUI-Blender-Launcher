package model

import "testing"

func TestParseBuildType(t *testing.T) {
	cases := []struct {
		in   string
		want BuildType
	}{
		{"daily", BuildTypeDaily},
		{"experimental", BuildTypeExperimental},
		{"patch", BuildTypePatch},
		{"", DefaultBuildType},
		{"stable", DefaultBuildType},
		{"DAILY", DefaultBuildType}, // case-sensitive by design
	}
	for _, c := range cases {
		if got := ParseBuildType(c.in); got != c.want {
			t.Errorf("ParseBuildType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildTypeValid(t *testing.T) {
	if !BuildTypeDaily.Valid() {
		t.Error("daily should be valid")
	}
	if BuildType("nightly").Valid() {
		t.Error("nightly should not be valid")
	}
}

func TestBuildTypeNamesOrder(t *testing.T) {
	got := BuildTypeNames()
	want := []string{"daily", "experimental", "patch"}
	if len(got) != len(want) {
		t.Fatalf("BuildTypeNames() length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("BuildTypeNames()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
