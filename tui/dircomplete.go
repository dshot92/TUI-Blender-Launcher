package tui

import (
	"os"
	"path/filepath"
	"strings"
)

// DirCompletions returns the subdirectories of the directory named by input
// that match the trailing path element the user has typed. The returned paths
// preserve the spelling of input, so a leading "~", "./" or "/" survives and
// the platform's path separator is used.
func DirCompletions(input string) ([]string, error) {
	dir, prefix := splitDirPrefix(input)

	scanDir := dir
	if scanDir == "" {
		scanDir = "."
	}
	scanDir = expandHome(scanDir)

	entries, err := os.ReadDir(scanDir)
	if err != nil {
		return nil, err
	}

	// Hidden directories are only offered once the user has started typing a
	// dot, matching shell behavior.
	showHidden := strings.HasPrefix(prefix, ".")

	matches := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		matches = append(matches, joinPath(dir, name))
	}
	return matches, nil
}

// splitDirPrefix splits a partially typed path into the directory part and the
// name prefix being completed. A trailing separator, or "." / "..", means the
// user has not started typing a name yet.
func splitDirPrefix(input string) (dir, prefix string) {
	if input == "" {
		return "", ""
	}
	if hasTrailingSeparator(input) {
		return input, ""
	}
	switch input {
	case ".", "..":
		return input, ""
	}
	dir = filepath.Dir(input)
	if dir == "." && !strings.HasPrefix(input, ".") {
		// A plain relative name ("Down") has no directory component; keeping
		// "." here would leak into the result ("Down" -> "Down/Downloads").
		dir = ""
	}
	return dir, filepath.Base(input)
}

// joinPath appends name to dir without cleaning the result, so the user's
// spelling ("~/", "./", "/") is preserved.
func joinPath(dir, name string) string {
	if dir == "" {
		return name
	}
	if hasTrailingSeparator(dir) {
		return dir + name
	}
	return dir + string(os.PathSeparator) + name
}

// expandHome rewrites a leading "~" (or "~/" / "~\\") to the user's home
// directory. Paths without a leading tilde are returned unchanged.
func expandHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if len(path) >= 2 && path[0] == '~' && (path[1] == '/' || path[1] == '\\') {
		return filepath.Join(home, path[2:])
	}
	return path
}

// hasTrailingSeparator reports whether path ends with a path separator (either
// kind, so Windows accepts "/" too).
func hasTrailingSeparator(path string) bool {
	return strings.HasSuffix(path, "/") || strings.HasSuffix(path, string(os.PathSeparator))
}

// withTrailingSeparator appends a path separator to path unless one is already
// present.
func withTrailingSeparator(path string) string {
	if hasTrailingSeparator(path) {
		return path
	}
	return path + string(os.PathSeparator)
}
