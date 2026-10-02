package download

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
)

// disableSystemXz forces the pure-Go xz fallback. Tests set it to exercise
// the fallback path on machines that have xz installed.
var disableSystemXz bool

// findSystemXz returns the path to the system xz binary, or "" when it is
// unavailable (missing binary, forced fallback, or Windows where the zip
// path never needs it).
func findSystemXz() string {
	if disableSystemXz {
		return ""
	}
	if runtime.GOOS == "windows" {
		return ""
	}
	path, err := exec.LookPath("xz")
	if err != nil {
		return ""
	}
	return path
}

// systemXzStream is a `xz -dc` child decoding the archive on stdin.
// The tar reader consumes s.rc; cleanup reaps the child.
type systemXzStream struct {
	rc     io.ReadCloser
	cancel context.CancelFunc
	cmd    *exec.Cmd
	stderr *bytes.Buffer
}

// newSystemXzReader starts `xz -dc` (liblzma) with src as its stdin and
// returns the child's stdout as the tar input stream.
//
// src is typically the progress-tracked file reader, so extraction progress
// keeps measuring compressed bytes fed to the decoder. The caller must call
// cleanup on every exit path to reap the child. Any error means the caller
// should fall back to the pure-Go decoder.
func newSystemXzReader(src io.Reader, cancelCh <-chan struct{}) (*systemXzStream, error) {
	xzPath := findSystemXz()
	if xzPath == "" {
		return nil, fmt.Errorf("system xz not available")
	}

	ctx, stop := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, xzPath, "-dc")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		stop()
		return nil, fmt.Errorf("failed to create xz stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stop()
		return nil, fmt.Errorf("failed to create xz stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		stop()
		return nil, fmt.Errorf("failed to start xz: %w", err)
	}

	// Feed the archive to the child. Exits on EOF, on child death (EPIPE),
	// or when stdin is closed after the tar loop ends.
	go func() {
		_, _ = io.Copy(stdin, src)
		_ = stdin.Close()
	}()

	// Propagate user cancellation to the child (CommandContext kills it).
	// Exits via ctx.Done on the normal path, so no goroutine leak.
	go func() {
		select {
		case <-cancelCh:
			stop()
		case <-ctx.Done():
		}
	}()

	return &systemXzStream{rc: stdout, cancel: stop, cmd: cmd, stderr: &stderr}, nil
}

// cleanup reaps the child process. Call on every exit path, including
// early ones: it kills the child first so Wait cannot block on a child
// stuck writing to an unconsumed stdout pipe, then reaps it.
func (s *systemXzStream) cleanup() error {
	s.cancel()
	err := s.cmd.Wait()
	if err != nil {
		if s.stderr.Len() > 0 {
			return fmt.Errorf("xz -dc failed: %v: %s", err, s.stderr.String())
		}
		return fmt.Errorf("xz -dc failed: %w", err)
	}
	return nil
}
