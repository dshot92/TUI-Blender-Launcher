package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// mkTree creates the given directories under root (paths may be nested) and
// returns root.
func mkTree(t *testing.T, root string, dirs ...string) string {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	return root
}

func TestDirCompletionsUniqueMatch(t *testing.T) {
	root := mkTree(t, t.TempDir(), "fruits", "framework")

	got, err := DirCompletions(filepath.Join(root, "fru"))
	if err != nil {
		t.Fatalf("DirCompletions: %v", err)
	}
	want := []string{filepath.Join(root, "fruits")}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDirCompletionsMultipleMatches(t *testing.T) {
	root := mkTree(t, t.TempDir(), "fruits", "framework", "formats", "notes")

	got, err := DirCompletions(filepath.Join(root, "f"))
	if err != nil {
		t.Fatalf("DirCompletions: %v", err)
	}
	want := []string{
		filepath.Join(root, "formats"),
		filepath.Join(root, "framework"),
		filepath.Join(root, "fruits"),
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestDirCompletionsTrailingSeparatorListsAll(t *testing.T) {
	root := mkTree(t, t.TempDir(), "alpha", "beta", ".hidden")

	got, err := DirCompletions(root + string(os.PathSeparator))
	if err != nil {
		t.Fatalf("DirCompletions: %v", err)
	}
	sort.Strings(got)
	want := []string{filepath.Join(root, "alpha"), filepath.Join(root, "beta")}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v (hidden dirs must be skipped)", got, want)
	}
}

func TestDirCompletionsHiddenWhenDotPrefix(t *testing.T) {
	root := mkTree(t, t.TempDir(), ".config", "visible")

	got, err := DirCompletions(root + string(os.PathSeparator) + ".")
	if err != nil {
		t.Fatalf("DirCompletions: %v", err)
	}
	if len(got) != 1 || got[0] != filepath.Join(root, ".config") {
		t.Fatalf("got %v, want [%s]", got, filepath.Join(root, ".config"))
	}
}

func TestDirCompletionsNoMatchIsEmpty(t *testing.T) {
	root := mkTree(t, t.TempDir(), "alpha")

	got, err := DirCompletions(filepath.Join(root, "zzz"))
	if err != nil {
		t.Fatalf("DirCompletions: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %v, want no matches", got)
	}
}

func TestDirCompletionsRelativeInputStaysRelative(t *testing.T) {
	root := mkTree(t, t.TempDir(), "fruits", "framework")

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(wd)

	got, err := DirCompletions("fru")
	if err != nil {
		t.Fatalf("DirCompletions: %v", err)
	}
	if len(got) != 1 || got[0] != "fruits" {
		t.Fatalf("got %v, want [fruits]", got)
	}
}

func TestDirCompletionsTildePreserved(t *testing.T) {
	root := mkTree(t, t.TempDir(), "Projects", "Pictures")
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)

	got, err := DirCompletions("~/Pro")
	if err != nil {
		t.Fatalf("DirCompletions: %v", err)
	}
	want := "~" + string(os.PathSeparator) + "Projects"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %v, want [%s] (tilde must be preserved)", got, want)
	}
}
