package model

import (
	"testing"
)

func TestSortBuildsByVersion(t *testing.T) {
	builds := []BlenderBuild{
		{Version: "4.0.0"},
		{Version: "3.6.2"},
		{Version: "4.2.1"},
	}

	asc := SortBuilds(builds, 0, false)
	wantAsc := []string{"3.6.2", "4.0.0", "4.2.1"}
	for i, w := range wantAsc {
		if asc[i].Version != w {
			t.Fatalf("ascending sort: index %d = %q, want %q", i, asc[i].Version, w)
		}
	}

	desc := SortBuilds(builds, 0, true)
	wantDesc := []string{"4.2.1", "4.0.0", "3.6.2"}
	for i, w := range wantDesc {
		if desc[i].Version != w {
			t.Fatalf("descending sort: index %d = %q, want %q", i, desc[i].Version, w)
		}
	}

	// The input slice must not be mutated.
	if builds[0].Version != "4.0.0" {
		t.Fatalf("SortBuilds mutated the input slice: %q", builds[0].Version)
	}
}

func TestSortBuildsBySize(t *testing.T) {
	builds := []BlenderBuild{
		{Version: "a", Size: 300},
		{Version: "b", Size: 100},
		{Version: "c", Size: 200},
	}

	sorted := SortBuilds(builds, 5, false)
	want := []int64{100, 200, 300}
	for i, w := range want {
		if sorted[i].Size != w {
			t.Fatalf("size sort: index %d = %d, want %d", i, sorted[i].Size, w)
		}
	}
}

func TestFormatByteSize(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{1024 * 1024, "1.0MB"},
	}
	for _, c := range cases {
		if got := FormatByteSize(c.in); got != c.want {
			t.Errorf("FormatByteSize(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
