package tui

import (
	"testing"

	"github.com/dshot92/TUI-Blender-Launcher/config"
	"github.com/dshot92/TUI-Blender-Launcher/model"
)

func newTestModel(versionFilter string, builds []model.BlenderBuild) *Model {
	return &Model{
		config: config.Config{VersionFilter: versionFilter},
		List:   ListModel{Builds: builds},
	}
}

func TestMergeBuildsPreservesAndDeduplicates(t *testing.T) {
	m := newTestModel("", []model.BlenderBuild{
		{Version: "4.2.0", Hash: "aaaa", Status: model.StateLocal},
	})
	incoming := []model.BlenderBuild{
		{Version: "4.2.0", Hash: "aaaa", Status: model.StateOnline}, // duplicate of local
		{Version: "4.0.0", Hash: "bbbb", Status: model.StateOnline},
	}

	got := m.mergeBuilds(incoming)
	if len(got) != 2 {
		t.Fatalf("got %d builds, want 2: %+v", len(got), got)
	}
	// The local-only build must not be dropped, and the duplicate must not be
	// duplicated. (Data is refreshed from the incoming entry; status is
	// recomputed later by UpdateBuildStatus.)
	seen := map[string]int{}
	for _, b := range got {
		seen[b.Version]++
	}
	if seen["4.2.0"] != 1 || seen["4.0.0"] != 1 {
		t.Errorf("unexpected merge result: %+v", got)
	}
}

func TestMergeBuildsPreservesInProgressStatus(t *testing.T) {
	m := newTestModel("", []model.BlenderBuild{
		{Version: "4.2.0", Hash: "aaaa", Status: model.StateDownloading},
	})
	// Same build arrives from the API as Online; status must stay Downloading.
	got := m.mergeBuilds([]model.BlenderBuild{{Version: "4.2.0", Hash: "aaaa", Status: model.StateOnline}})
	if len(got) != 1 {
		t.Fatalf("got %d builds, want 1", len(got))
	}
	if got[0].Status != model.StateDownloading {
		t.Errorf("status = %v, want Downloading", got[0].Status)
	}
}

func TestApplyVersionFilterSemantic(t *testing.T) {
	m := newTestModel("4.9", nil)
	builds := []model.BlenderBuild{
		{Version: "4.10.0", Status: model.StateOnline}, // newer than 4.9
		{Version: "4.8.0", Status: model.StateOnline},  // older
		{Version: "4.9.0", Status: model.StateOnline},  // equal, kept
		{Version: "1.0.0", Status: model.StateLocal},   // local always kept
	}

	got := m.applyVersionFilter(builds)
	versions := map[string]bool{}
	for _, b := range got {
		versions[b.Version] = true
	}
	if !versions["4.10.0"] || !versions["4.9.0"] || !versions["1.0.0"] {
		t.Errorf("expected 4.10.0, 4.9.0 and 1.0.0 to pass, got %+v", got)
	}
	if versions["4.8.0"] {
		t.Errorf("4.8.0 should have been filtered out, got %+v", got)
	}
}

func TestApplyVersionFilterInvalidIsNoop(t *testing.T) {
	m := newTestModel("not-a-version", nil)
	builds := []model.BlenderBuild{{Version: "4.2.0", Status: model.StateOnline}}
	got := m.applyVersionFilter(builds)
	if len(got) != 1 {
		t.Fatalf("invalid filter should not drop builds, got %d", len(got))
	}
}

func TestCommandsSetConfigPreservesDownloadStates(t *testing.T) {
	c := NewCommands(config.Config{DownloadDir: "/old"})

	buildID := "4.2.0-abcdef12"
	c.downloads.mu.Lock()
	c.downloads.states[buildID] = &model.DownloadState{
		BuildID:    buildID,
		BuildState: model.StateDownloading,
		Progress:   0.42,
		CancelCh:   make(chan struct{}),
	}
	c.downloads.mu.Unlock()

	newCfg := config.Config{DownloadDir: "/new", VersionFilter: "4.2"}
	c.SetConfig(newCfg)

	if c.cfg.DownloadDir != "/new" || c.cfg.VersionFilter != "4.2" {
		t.Errorf("commands config not updated: %+v", c.cfg)
	}
	if c.downloads.cfg.DownloadDir != "/new" {
		t.Errorf("download manager config not updated: %+v", c.downloads.cfg)
	}

	states := c.downloads.GetAllStates()
	state, ok := states[buildID]
	if !ok {
		t.Fatalf("download state lost after SetConfig: %+v", states)
	}
	if state.Progress != 0.42 {
		t.Errorf("progress = %v, want 0.42", state.Progress)
	}
}

func TestSaveSettingsAndReturnPreservesDownloads(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg := config.Config{DownloadDir: t.TempDir(), BuildType: "daily"}
	style := NewStyle()
	m := &Model{
		config:   cfg,
		commands: NewCommands(cfg),
		Settings: NewSettingsModel(cfg, style),
		List:     NewListModel(style),
		Progress: NewProgressModel(),
		Style:    style,
	}

	buildID := "4.2.0-abcdef12"
	m.commands.downloads.mu.Lock()
	m.commands.downloads.states[buildID] = &model.DownloadState{
		BuildID:    buildID,
		BuildState: model.StateDownloading,
		Progress:   0.5,
		CancelCh:   make(chan struct{}),
	}
	m.commands.downloads.mu.Unlock()

	// Mirror the cache the tick loop keeps in sync while in settings.
	m.Progress.DownloadStates = m.commands.downloads.GetAllStates()

	if _, cmd := m.SaveSettingsAndReturn(); cmd == nil {
		t.Fatal("expected a refresh command after saving settings")
	}

	states := m.commands.downloads.GetAllStates()
	if _, ok := states[buildID]; !ok {
		t.Fatalf("download state lost after SaveSettingsAndReturn: %+v", states)
	}
}

func TestStatusMessageExpiry(t *testing.T) {
	m := &Model{}

	// A status message records the current token.
	_, _ = m.Update(statusMsg{text: "first"})
	if m.status != "first" {
		t.Fatalf("status = %q, want %q", m.status, "first")
	}
	firstToken := m.statusToken

	// A newer status message bumps the token.
	_, _ = m.Update(statusMsg{text: "second"})
	if m.statusToken == firstToken {
		t.Fatal("expected token to change for a new status message")
	}
	secondToken := m.statusToken

	// A stale timer must not clear the newer status.
	_, _ = m.Update(clearStatusMsg{token: firstToken})
	if m.status != "second" {
		t.Errorf("stale clear wiped current status: %q", m.status)
	}

	// The timer for the current status clears it.
	_, _ = m.Update(clearStatusMsg{token: secondToken})
	if m.status != "" {
		t.Errorf("matching clear did not wipe status: %q", m.status)
	}
}
