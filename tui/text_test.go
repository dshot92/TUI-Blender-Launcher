package tui

import (
	"strings"
	"testing"
)

func TestWrapTextBreaksOnWidth(t *testing.T) {
	got := wrapText("only show builds at or above this version", 12)
	for _, line := range got {
		if width(line) > 12 {
			t.Errorf("line %q exceeds max width 12", line)
		}
	}
	if joined := strings.Join(got, " "); joined != "only show builds at or above this version" {
		t.Errorf("wrapped text lost content: %q", joined)
	}
	if len(got) < 2 {
		t.Fatalf("expected text to wrap onto multiple lines, got %d", len(got))
	}
}

func TestWrapTextKeepsLongWord(t *testing.T) {
	got := wrapText("supercalifragilistic", 5)
	if len(got) != 1 || got[0] != "supercalifragilistic" {
		t.Fatalf("long word should stay intact, got %q", got)
	}
}

func TestVersionFilterDescriptionShowsHint(t *testing.T) {
	desc := settingDescription(settingVersionFilter)
	for _, want := range []string{"e.g.", "4.0", "3.6", "Leave empty"} {
		if !strings.Contains(desc, want) {
			t.Errorf("version filter description %q missing %q", desc, want)
		}
	}
}
