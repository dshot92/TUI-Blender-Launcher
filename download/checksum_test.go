package download

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeTempFile(t *testing.T, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive.tar.xz")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func TestVerifyChecksumMatches(t *testing.T) {
	content := []byte("blender archive contents")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Real sidecars are "<digest>  <filename>".
		fmt.Fprintf(w, "%s  archive.tar.xz\n", digest)
	}))
	defer server.Close()

	ok, err := verifyChecksum(writeTempFile(t, content), server.URL, nil)
	if err != nil {
		t.Fatalf("verifyChecksum: %v", err)
	}
	if !ok {
		t.Fatal("expected the checksum to verify")
	}
}

func TestVerifyChecksumMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s\n", "0000000000000000000000000000000000000000000000000000000000000000")
	}))
	defer server.Close()

	ok, err := verifyChecksum(writeTempFile(t, []byte("some content")), server.URL, nil)
	if err == nil {
		t.Fatal("expected a mismatch error")
	}
	if ok {
		t.Fatal("a mismatching checksum must not report verified")
	}
}

func TestVerifyChecksumUnavailableSkips(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	ok, err := verifyChecksum(writeTempFile(t, []byte("some content")), server.URL, nil)
	if err != nil {
		t.Fatalf("missing checksum should not be an error, got %v", err)
	}
	if ok {
		t.Fatal("a missing checksum should report not-verified")
	}
}

func TestVerifyChecksumCancelled(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer server.Close()
	defer close(release)

	cancelCh := make(chan struct{})
	close(cancelCh)

	_, err := verifyChecksum(writeTempFile(t, []byte("some content")), server.URL, cancelCh)
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("expected ErrCancelled, got %v", err)
	}
}
