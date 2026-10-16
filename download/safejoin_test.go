package download

import (
	"path/filepath"
	"testing"
)

// TestSafeJoinRejectsTraversal verifies that archive entries which would escape
// the destination directory are rejected. Archive entry names always use forward
// slashes, so the traversal cases are expressed that way.
func TestSafeJoinRejectsTraversal(t *testing.T) {
	dest := t.TempDir()
	bad := []string{
		"../evil",
		"../../etc/passwd",
		"a/../../b",
		"a/b/../../../c",
	}
	for _, name := range bad {
		if _, err := safeJoin(dest, name); err == nil {
			t.Errorf("safeJoin(%q) = nil error, want rejection", name)
		}
	}
}

// TestSafeJoinAllowsContained verifies that ordinary, contained entries resolve
// to the expected path under the destination directory.
func TestSafeJoinAllowsContained(t *testing.T) {
	dest := t.TempDir()
	got, err := safeJoin(dest, "blender-4.2/x/y.txt")
	if err != nil {
		t.Fatalf("safeJoin: %v", err)
	}
	want := filepath.Join(dest, "blender-4.2", "x", "y.txt")
	if got != want {
		t.Errorf("safeJoin = %q, want %q", got, want)
	}
}
