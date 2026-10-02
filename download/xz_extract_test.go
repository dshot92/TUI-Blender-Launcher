package download

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ulikunitz/xz"
)

type expectedEntry struct {
	isDir      bool
	isLink     bool
	linkTarget string
	perm       os.FileMode
	data       []byte
}

func testLargeData() []byte {
	// 5MB, compressible: exercises the >4MB streaming path without a slow fixture build.
	return bytes.Repeat([]byte("abcdefgh12345678"), 5*1024*1024/16)
}

// buildTarXzFixture creates a .tar.xz archive (compressed with the pure-Go
// writer, so no external binary is needed) and returns its path plus the
// expected extracted tree keyed by path relative to destDir.
func buildTarXzFixture(t *testing.T) (string, map[string]expectedEntry) {
	t.Helper()

	files := []struct {
		name string
		mode int64
		data []byte
	}{
		{"root/small.txt", 0644, []byte("hello")},
		{"root/nested/deep.txt", 0644, []byte("deep content")},
		{"root/empty.bin", 0644, []byte{}},
		{"root/large.bin", 0644, testLargeData()},
		{"root/run.sh", 0755, []byte("#!/bin/sh\necho hi\n")},
	}

	archivePath := filepath.Join(t.TempDir(), "fixture.tar.xz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	xzw, err := xz.NewWriter(f)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(xzw)

	expected := map[string]expectedEntry{}

	writeFile := func(name string, mode int64, data []byte) {
		if err := tw.WriteHeader(&tar.Header{
			Name: name, Mode: mode, Size: int64(len(data)), Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
		expected[name] = expectedEntry{perm: os.FileMode(mode).Perm(), data: data}
	}

	for _, dir := range []string{"root/", "root/nested/"} {
		if err := tw.WriteHeader(&tar.Header{
			Name: dir, Mode: 0755, Typeflag: tar.TypeDir,
		}); err != nil {
			t.Fatal(err)
		}
		expected[dir] = expectedEntry{isDir: true, perm: 0755}
	}
	for _, fl := range files {
		writeFile(fl.name, fl.mode, fl.data)
	}
	if err := tw.WriteHeader(&tar.Header{
		Name: "root/link", Typeflag: tar.TypeSymlink, Linkname: "small.txt",
	}); err != nil {
		t.Fatal(err)
	}
	expected["root/link"] = expectedEntry{isLink: true, linkTarget: "small.txt"}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := xzw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return archivePath, expected
}

func compareTrees(t *testing.T, destDir string, expected map[string]expectedEntry) {
	t.Helper()

	seen := map[string]bool{}
	err := filepath.Walk(destDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(destDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		seen[rel] = true
		want, ok := expected[rel]
		if !ok {
			// Allow both "root" and "root/" keys for directories.
			if want, ok = expected[rel+"/"]; !ok {
				t.Errorf("unexpected entry: %s", rel)
				return nil
			}
		}
		switch {
		case want.isLink:
			if info.Mode()&os.ModeSymlink == 0 {
				t.Errorf("%s: not a symlink", rel)
				return nil
			}
			target, err := os.Readlink(path)
			if err != nil {
				t.Fatal(err)
			}
			if target != want.linkTarget {
				t.Errorf("%s: symlink target = %q, want %q", rel, target, want.linkTarget)
			}
		case want.isDir:
			if !info.IsDir() {
				t.Errorf("%s: not a directory", rel)
			}
		default:
			if info.IsDir() {
				t.Errorf("%s: is a directory, want file", rel)
				return nil
			}
			if got := info.Mode().Perm(); got != want.perm {
				t.Errorf("%s: perm = %o, want %o", rel, got, want.perm)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, want.data) {
				t.Errorf("%s: contents differ (%d vs %d bytes)", rel, len(data), len(want.data))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for name := range expected {
		key := name
		if expected[name].isDir && key[len(key)-1] == '/' {
			key = key[:len(key)-1]
		}
		if !seen[key] {
			t.Errorf("missing entry: %s", name)
		}
	}
}

func runWithTimeout(t *testing.T, timeout time.Duration, fn func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		t.Fatal("extraction hung (timed out)")
		return nil
	}
}

func TestExtractTarXzRoundTrip(t *testing.T) {
	archive, expected := buildTarXzFixture(t)
	dest := t.TempDir()

	var progress []float64
	err := runWithTimeout(t, 60*time.Second, func() error {
		return extractTarXz(archive, dest, func(p float64) { progress = append(progress, p) }, make(chan struct{}))
	})
	if err != nil {
		t.Fatalf("extractTarXz: %v", err)
	}
	compareTrees(t, dest, expected)

	if len(progress) == 0 || progress[0] != 0.0 {
		t.Errorf("progress did not start at 0.0: %v", progress[:min(3, len(progress))])
	}
	if last := progress[len(progress)-1]; last != 1.0 {
		t.Errorf("progress did not end at 1.0, got %v", last)
	}
	for _, p := range progress {
		if p < 0.0 || p > 1.0 {
			t.Errorf("progress out of range: %v", p)
			break
		}
	}
}

func TestExtractTarXzPureGoFallback(t *testing.T) {
	old := disableSystemXz
	disableSystemXz = true
	defer func() { disableSystemXz = old }()

	if findSystemXz() != "" {
		t.Fatal("disableSystemXz did not take effect")
	}

	archive, expected := buildTarXzFixture(t)
	dest := t.TempDir()
	err := runWithTimeout(t, 120*time.Second, func() error {
		return extractTarXz(archive, dest, nil, make(chan struct{}))
	})
	if err != nil {
		t.Fatalf("extractTarXz (fallback): %v", err)
	}
	compareTrees(t, dest, expected)
}

func TestSystemXzReaderMatchesPureGo(t *testing.T) {
	if findSystemXz() == "" {
		t.Skip("system xz not available")
	}
	archive, _ := buildTarXzFixture(t)
	raw, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}

	stream, err := newSystemXzReader(bytes.NewReader(raw), make(chan struct{}))
	if err != nil {
		t.Fatalf("newSystemXzReader: %v", err)
	}
	sysOut, readErr := io.ReadAll(stream.rc)
	cleanupErr := stream.cleanup()
	if readErr != nil {
		t.Fatalf("system xz read: %v", readErr)
	}
	if cleanupErr != nil {
		t.Fatalf("system xz cleanup: %v", cleanupErr)
	}

	// Decode the same archive with the pure-Go reader and compare bytes.
	f, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	goOut, err := io.ReadAll(mustXzReader(t, f))
	if err != nil {
		t.Fatalf("pure-Go xz read: %v", err)
	}

	if !bytes.Equal(sysOut, goOut) {
		t.Fatalf("system xz output differs: %d vs %d bytes", len(sysOut), len(goOut))
	}
}

func mustXzReader(t *testing.T, r io.Reader) io.Reader {
	t.Helper()
	xzr, err := xz.NewReader(r)
	if err != nil {
		t.Fatal(err)
	}
	return xzr
}

func TestFindSystemXzEmptyPath(t *testing.T) {
	old := disableSystemXz
	disableSystemXz = false
	defer func() { disableSystemXz = old }()

	t.Setenv("PATH", "")
	if got := findSystemXz(); got != "" {
		t.Errorf("findSystemXz with empty PATH = %q, want empty", got)
	}
}

func TestExtractTarXzCancel(t *testing.T) {
	archive, _ := buildTarXzFixture(t)
	cancelCh := make(chan struct{})
	close(cancelCh) // already cancelled

	err := runWithTimeout(t, 30*time.Second, func() error {
		return extractTarXz(archive, t.TempDir(), nil, cancelCh)
	})
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("err = %v, want ErrCancelled", err)
	}
}

func TestExtractTarXzErrorBurst(t *testing.T) {
	// Regression test: more failing small-file workers than the old
	// errChan buffer (4) must return an error, not hang.
	archive, _ := buildTarXzFixture(t)
	dest := t.TempDir()

	// Block every small-file write by pre-creating directories at the
	// target file paths (WriteFile then fails with EISDIR).
	for _, name := range []string{"root/small.txt", "root/nested/deep.txt", "root/empty.bin", "root/run.sh"} {
		p := filepath.Join(dest, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(p, 0750); err != nil {
			t.Fatal(err)
		}
	}
	// Add enough extra failing files to exceed any small channel buffer.
	for i := 0; i < 20; i++ {
		p := filepath.Join(dest, "root", "blocked")
		if err := os.MkdirAll(p, 0750); err != nil {
			t.Fatal(err)
		}
		_ = i
	}

	err := runWithTimeout(t, 60*time.Second, func() error {
		return extractTarXz(archive, dest, nil, make(chan struct{}))
	})
	if err == nil {
		t.Fatal("expected error from blocked writes, got nil")
	}
}

func TestExtractZipErrorBurst(t *testing.T) {
	// Same regression test for the zip small-file worker path.
	archivePath := filepath.Join(t.TempDir(), "burst.zip")
	zf, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	for i := 0; i < 20; i++ {
		name := "a/" + string(rune('a'+i)) + ".txt"
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zf.Close(); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	// Pre-create directories at every target file path.
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range r.File {
		p := filepath.Join(dest, filepath.FromSlash(f.Name))
		if err := os.MkdirAll(p, 0750); err != nil {
			t.Fatal(err)
		}
	}
	r.Close()

	err = runWithTimeout(t, 60*time.Second, func() error {
		return extractZip(archivePath, dest, nil, make(chan struct{}))
	})
	if err == nil {
		t.Fatal("expected error from blocked writes, got nil")
	}
}

func TestExtractZipRoundTrip(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "rt.zip")
	zf, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	for _, name := range []string{"b/one.txt", "b/two.txt", "b/empty.txt"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("data:" + name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zf.Close(); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	err = runWithTimeout(t, 60*time.Second, func() error {
		return extractZip(archivePath, dest, nil, make(chan struct{}))
	})
	if err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	for _, name := range []string{"b/one.txt", "b/two.txt", "b/empty.txt"} {
		data, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "data:"+name {
			t.Errorf("%s: got %q", name, data)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
