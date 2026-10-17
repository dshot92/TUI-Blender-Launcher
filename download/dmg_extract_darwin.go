//go:build darwin
// +build darwin

package download

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// dmgCommandTimeout bounds each hdiutil/ditto invocation so a wedged disk image
// cannot hang the download worker forever. It is deliberately generous: copying
// an app bundle from a slow external volume can take a while.
const dmgCommandTimeout = 15 * time.Minute

// dmgDetachTimeout bounds the best-effort detach of a mounted image.
const dmgDetachTimeout = 2 * time.Minute

// extractDMG mounts a macOS disk image, copies every .app bundle it contains
// into destDir/<image name without extension>, and detaches the image. It
// returns the directory now holding the copied bundles. The image is always
// detached, even when an error occurs, and cancellation kills the running helper.
//
// The volume is mounted read-only at a private mount point, so the volume name
// is irrelevant, no Finder window opens, and nothing is ever run from the
// read-only image. Bundles are copied with ditto because it preserves the
// executable bits, symlinks, extended attributes and resource forks a .app
// bundle needs; a plain recursive copy would corrupt it.
func extractDMG(dmgPath, destDir string, progressCb ExtractionProgressCallback, cancelCh <-chan struct{}) (string, error) {
	rootDir := filepath.Join(destDir, dmgRootDirName(dmgPath))

	mountPoint, err := os.MkdirTemp("", "blender-dmg-")
	if err != nil {
		return rootDir, fmt.Errorf("failed to create mount point: %w", err)
	}
	// macOS makes /var and $TMPDIR symlinked paths; hdiutil wants a canonical
	// mount point, so resolve it before mounting.
	if resolved, err := filepath.EvalSymlinks(mountPoint); err == nil {
		mountPoint = resolved
	}
	// os.Remove (not RemoveAll): if the volume were still mounted, RemoveAll
	// would walk the read-only image. An empty-dir Remove simply fails instead.
	defer os.Remove(mountPoint)
	defer detachDMG(mountPoint)

	if out, err := runDMGCommand(cancelCh, "hdiutil",
		"attach", "-nobrowse", "-noautoopen", "-readonly", "-mountpoint", mountPoint, dmgPath,
	); err != nil {
		return rootDir, fmt.Errorf("failed to mount disk image: %w (hdiutil: %s)", err, out)
	}

	appBundles, err := findAppBundles(mountPoint)
	if err != nil {
		return rootDir, err
	}
	if len(appBundles) == 0 {
		return rootDir, fmt.Errorf("no .app bundle found in disk image %s", filepath.Base(dmgPath))
	}

	report := func(p float64) {
		if progressCb == nil {
			return
		}
		if p < 0 {
			p = 0
		} else if p > 1 {
			p = 1
		}
		progressCb(p)
	}
	report(0.0)

	var totalSize int64
	for _, bundle := range appBundles {
		totalSize += dirSize(bundle)
	}

	// Start from a clean destination so a previous failed extraction cannot
	// leave a half-copied, corrupt bundle behind.
	if err := os.RemoveAll(rootDir); err != nil {
		return rootDir, fmt.Errorf("failed to clear extraction directory %s: %w", rootDir, err)
	}
	if err := os.MkdirAll(rootDir, privateDirPerm); err != nil {
		return rootDir, fmt.Errorf("failed to create extraction directory %s: %w", rootDir, err)
	}

	var copied int64
	for _, bundle := range appBundles {
		dest := filepath.Join(rootDir, filepath.Base(bundle))
		if out, err := runDMGCommand(cancelCh, "ditto", bundle, dest); err != nil {
			return rootDir, fmt.Errorf("failed to copy %s: %w (ditto: %s)", filepath.Base(bundle), err, out)
		}
		copied += dirSize(bundle)
		if totalSize > 0 {
			report(float64(copied) / float64(totalSize))
		}
	}
	report(1.0)

	return rootDir, nil
}

// detachDMG ejects a mounted image, retrying with -force on failure. Errors are
// intentionally ignored: detaching is best-effort cleanup, and the caller
// reports the extraction result, not the ejection result.
func detachDMG(mountPoint string) {
	ctx, cancel := context.WithTimeout(context.Background(), dmgDetachTimeout)
	defer cancel()
	if err := exec.CommandContext(ctx, "hdiutil", "detach", mountPoint).Run(); err == nil {
		return
	}

	forceCtx, forceCancel := context.WithTimeout(context.Background(), dmgDetachTimeout)
	defer forceCancel()
	_ = exec.CommandContext(forceCtx, "hdiutil", "detach", "-force", mountPoint).Run()
}

// runDMGCommand runs name with args and returns its combined output. When
// cancelCh is closed the running process is killed and ErrCancelled is returned.
func runDMGCommand(cancelCh <-chan struct{}, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dmgCommandTimeout)
	defer cancel()

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-cancelCh:
			cancel()
		case <-done:
		}
	}()

	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if ctx.Err() == context.Canceled {
		return string(out), ErrCancelled
	}
	return string(out), err
}
