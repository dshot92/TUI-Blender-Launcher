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

func TestTimestampUnmarshalJSON(t *testing.T) {
	t.Run("unix seconds", func(t *testing.T) {
		var ts Timestamp
		if err := ts.UnmarshalJSON([]byte("1633046400")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, want := ts.Time().Unix(), int64(1633046400); got != want {
			t.Fatalf("got %d, want %d", got, want)
		}
	})

	t.Run("rfc3339 string", func(t *testing.T) {
		var ts Timestamp
		if err := ts.UnmarshalJSON([]byte(`"2021-10-01T00:00:00Z"`)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, want := ts.Time().UTC().Format("2006-01-02T15:04:05Z"), "2021-10-01T00:00:00Z"; got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("malformed value is rejected", func(t *testing.T) {
		var ts Timestamp
		if err := ts.UnmarshalJSON([]byte(`"not-a-timestamp"`)); err == nil {
			t.Fatal("expected an error for a malformed timestamp")
		}
		if !ts.Time().IsZero() {
			t.Fatalf("timestamp should stay zero on error, got %v", ts.Time())
		}
	})
}
