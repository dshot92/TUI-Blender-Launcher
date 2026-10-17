package download

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dshot92/TUI-Blender-Launcher/model"
)

func TestReadVersionMetadata(t *testing.T) {
	dir := t.TempDir()
	build := model.BlenderBuild{
		Version:      "4.2.0",
		Branch:       "main",
		ReleaseCycle: "daily",
		BuildDate:    model.Timestamp(time.Unix(1633046400, 0)),
	}
	if err := saveVersionMetadata(build, dir); err != nil {
		t.Fatalf("saveVersionMetadata: %v", err)
	}

	got, err := ReadVersionMetadata(dir)
	if err != nil {
		t.Fatalf("ReadVersionMetadata: %v", err)
	}
	if got == nil {
		t.Fatal("expected metadata, got nil")
	}
	if got.Version != build.Version || got.Branch != build.Branch || got.ReleaseCycle != build.ReleaseCycle {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, build)
	}
	if got.BuildDate.Time().Unix() != 1633046400 {
		t.Fatalf("build date = %v, want unix 1633046400", got.BuildDate.Time())
	}
}

func TestReadVersionMetadataMissing(t *testing.T) {
	got, err := ReadVersionMetadata(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error for missing metadata: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for missing metadata, got %+v", got)
	}
}

func TestReadVersionMetadataMalformed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, versionMetaFilename), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
	if _, err := ReadVersionMetadata(dir); err == nil {
		t.Fatal("expected an error for malformed metadata")
	}
}
