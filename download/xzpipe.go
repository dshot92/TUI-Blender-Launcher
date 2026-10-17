package download

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"
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
// The tar reader consumes s.rc; cleanup reaps the child and joins the
// goroutine that feeds the child's stdin.
type systemXzStream struct {
	rc         io.ReadCloser
	cmd        *exec.Cmd
	stderr     *bytes.Buffer
	feederDone chan struct{}
	// done is closed by cleanup to stop the cancellation watcher.
	done      chan struct{}
	closeOnce sync.Once
	// killed records that teardown interrupted a still-running child, so its
	// exit status reflects our signal rather than a decode failure.
	killed atomic.Bool
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

	cmd := exec.Command(xzPath, "-dc")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create xz stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create xz stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start xz: %w", err)
	}

	s := &systemXzStream{
		rc:         stdout,
		cmd:        cmd,
		stderr:     &stderr,
		feederDone: make(chan struct{}),
		done:       make(chan struct{}),
	}

	// Feed the archive to the child. Exits on EOF, on child death (EPIPE),
	// or when stdin is closed after the tar loop ends. feederDone lets cleanup
	// join this goroutine so no progress callback can outlive extraction.
	go func() {
		defer close(s.feederDone)
		_, _ = io.Copy(stdin, src)
		_ = stdin.Close()
	}()

	// Propagate user cancellation by killing the child. Exits via s.done on
	// the normal path, so no goroutine leak.
	go func() {
		select {
		case <-cancelCh:
			s.kill()
		case <-s.done:
		}
	}()

	return s, nil
}

// kill terminates the child. It records whether a still-running process was
// interrupted; a child that already exited returns os.ErrProcessDone and
// leaves the genuine exit status intact for cleanup to report.
func (s *systemXzStream) kill() {
	if err := s.cmd.Process.Kill(); err == nil {
		s.killed.Store(true)
	}
}

// cleanup reaps the child process. Call on every exit path, including
// early ones: it kills the child first so Wait cannot block on a child
// stuck writing to an unconsumed stdout pipe, then reaps it and joins the
// stdin feeder goroutine.
func (s *systemXzStream) cleanup() error {
	s.kill()
	err := s.cmd.Wait()
	s.closeOnce.Do(func() { close(s.done) })
	if s.feederDone != nil {
		<-s.feederDone
	}

	if s.killed.Load() {
		// We interrupted a still-running decoder during teardown; its exit
		// status reflects our signal, not a decode failure.
		return nil
	}
	if err != nil {
		if s.stderr.Len() > 0 {
			return fmt.Errorf("xz -dc failed: %v: %s", err, s.stderr.String())
		}
		return fmt.Errorf("xz -dc failed: %w", err)
	}
	return nil
}
