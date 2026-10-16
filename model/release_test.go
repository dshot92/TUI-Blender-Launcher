package model

import (
	"reflect"
	"testing"
)

func TestParseReleaseTypesCanonicalizes(t *testing.T) {
	// Unknown entries are dropped, duplicates and case folded, and the result is
	// ordered by ReleaseCycles.
	got := []string(ParseReleaseTypes([]string{"stable", "alpha", "bogus", "alpha", "STABLE"}))
	want := []string{"alpha", "stable"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseReleaseTypes = %v, want %v", got, want)
	}

	if len(ParseReleaseTypes(nil)) != 0 {
		t.Fatal("empty selection should stay empty (meaning all)")
	}
}

func TestReleaseFilterMatches(t *testing.T) {
	all := ParseReleaseTypes(nil)
	if !all.Matches(BlenderBuild{ReleaseCycle: "alpha"}) {
		t.Error("empty filter should match everything")
	}

	f := ParseReleaseTypes([]string{"stable", "candidate"})
	if !f.Matches(BlenderBuild{ReleaseCycle: "stable"}) {
		t.Error("stable should match")
	}
	if !f.Matches(BlenderBuild{ReleaseCycle: "Candidate"}) {
		t.Error("matching should be case-insensitive")
	}
	if f.Matches(BlenderBuild{ReleaseCycle: "alpha"}) {
		t.Error("alpha should not match")
	}
}

func TestReleaseFilterApplyKeepsLocal(t *testing.T) {
	f := ParseReleaseTypes([]string{"stable"})
	builds := []BlenderBuild{
		{Version: "5.3.0", ReleaseCycle: "alpha", Status: StateOnline},
		{Version: "5.1.2", ReleaseCycle: "stable", Status: StateOnline},
		{Version: "4.2.0", ReleaseCycle: "alpha", Status: StateLocal},
		{Version: "5.4.0", ReleaseCycle: "alpha", Status: StateDownloading},
	}
	got := f.Apply(builds)
	if len(got) != 3 {
		t.Fatalf("Apply kept %d builds, want 3 (stable + local + downloading)", len(got))
	}
}

func TestReleaseFilterOptions(t *testing.T) {
	if got := ReleaseFilterOptions(); len(got) != len(ReleaseCycles) {
		t.Fatalf("ReleaseFilterOptions has %d entries, want %d", len(got), len(ReleaseCycles))
	}
}
