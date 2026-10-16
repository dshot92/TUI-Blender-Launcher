package model

import "testing"

func TestParseVersionFilter(t *testing.T) {
	if f, err := ParseVersionFilter(""); f != nil || err != nil {
		t.Errorf("empty filter = (%v, %v), want (nil, nil)", f, err)
	}
	if _, err := ParseVersionFilter("4.0"); err != nil {
		t.Errorf("valid filter returned error: %v", err)
	}
	if _, err := ParseVersionFilter("not-a-version"); err == nil {
		t.Error("invalid filter should return an error")
	}
}

func TestVersionFilterSemantic(t *testing.T) {
	f, err := ParseVersionFilter("4.9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !f.Matches(BlenderBuild{Version: "4.10"}, false) {
		t.Error("4.10 should be >= 4.9 (semantic compare)")
	}
	if f.Matches(BlenderBuild{Version: "4.8.0"}, false) {
		t.Error("4.8.0 should be < 4.9")
	}
	if f.Matches(BlenderBuild{Version: "not-a-version"}, false) {
		t.Error("unparseable versions should be dropped")
	}
}

func TestVersionFilterKeepsLocalAndInProgress(t *testing.T) {
	f, err := ParseVersionFilter("5.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, status := range []BuildState{StateLocal, StateDownloading, StateExtracting} {
		if !f.Matches(BlenderBuild{Version: "3.6.0", Status: status}, true) {
			t.Errorf("status %s should be kept regardless of version", status)
		}
	}
	// Without keepLocal, an old online build is filtered out.
	if f.Matches(BlenderBuild{Version: "3.6.0", Status: StateOnline}, false) {
		t.Error("old online build should be filtered out")
	}
}

func TestVersionFilterApplyNilIsNoop(t *testing.T) {
	builds := []BlenderBuild{{Version: "3.6.0"}, {Version: "4.0.0"}}
	got := (*VersionFilter)(nil).Apply(builds)
	if len(got) != 2 {
		t.Fatalf("nil filter should be a no-op, got %d builds", len(got))
	}
}

func TestVersionFilterApply(t *testing.T) {
	f, err := ParseVersionFilter("4.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	builds := []BlenderBuild{
		{Version: "3.6.2", Status: StateOnline},
		{Version: "4.2.1", Status: StateOnline},
		{Version: "3.0.0", Status: StateLocal},
	}
	got := f.Apply(builds)
	if len(got) != 2 {
		t.Fatalf("Apply kept %d builds, want 2 (4.2.1 + local 3.0.0)", len(got))
	}
}
