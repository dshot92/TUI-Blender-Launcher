package local

import (
	"encoding/json"
	"fmt"
	"github.com/dshot92/TUI-Blender-Launcher/download"
	"github.com/dshot92/TUI-Blender-Launcher/model"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

const versionMetaFilename = "version.json"

// ReadBuildInfo reads build information from version.json in the given directory.
// Returns nil if version.json does not exist.
func ReadBuildInfo(dirPath string) (*model.BlenderBuild, error) {
	metaPath := filepath.Join(dirPath, versionMetaFilename)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", metaPath, err)
	}

	var build model.BlenderBuild
	if err := json.Unmarshal(data, &build); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", metaPath, err)
	}
	build.Status = model.StateLocal
	build.FileName = filepath.Base(dirPath)
	return &build, nil
}

// ScanLocalBuilds scans the download directory for local Blender builds using version.json.
func ScanLocalBuilds(downloadDir string) ([]model.BlenderBuild, error) {
	var localBuilds []model.BlenderBuild
	entries, err := os.ReadDir(downloadDir)
	if err != nil {
		if os.IsNotExist(err) {
			return localBuilds, nil
		}
		return nil, fmt.Errorf("failed to read download directory %s: %w", downloadDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != download.OldBuildsDir {
			dirPath := filepath.Join(downloadDir, entry.Name())
			buildInfo, err := ReadBuildInfo(dirPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error processing directory %s: %v\n", dirPath, err)
				continue
			}
			if buildInfo != nil {
				localBuilds = append(localBuilds, *buildInfo)
			}
		}
	}

	sort.Slice(localBuilds, func(i, j int) bool {
		return localBuilds[i].Version > localBuilds[j].Version
	})

	return localBuilds, nil
}

// FindBuild returns the first local build matching the predicate, along with
// its directory path. Returns (nil, "", nil) when nothing matches.
func FindBuild(downloadDir string, match func(model.BlenderBuild) bool) (*model.BlenderBuild, string, error) {
	entries, err := os.ReadDir(downloadDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("failed to read download directory %s: %w", downloadDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == download.OldBuildsDir || entry.Name() == download.DownloadingDir {
			continue
		}
		dirPath := filepath.Join(downloadDir, entry.Name())
		buildInfo, err := ReadBuildInfo(dirPath)
		if err != nil || buildInfo == nil {
			continue
		}
		if match(*buildInfo) {
			return buildInfo, dirPath, nil
		}
	}

	return nil, "", nil
}

// DeleteBuild archives the build with the given hash into .oldbuilds.
func DeleteBuild(downloadDir string, hash string) (bool, error) {
	buildInfo, dirPath, err := FindBuild(downloadDir, func(b model.BlenderBuild) bool {
		return b.Hash == hash
	})
	if err != nil {
		return false, err
	}
	if buildInfo == nil {
		return false, fmt.Errorf("build not found with hash: %s", hash)
	}

	if err := download.ArchiveBuildDir(downloadDir, dirPath); err != nil {
		return false, fmt.Errorf("failed to move build to %s: %w", download.OldBuildsDir, err)
	}
	return true, nil
}

// LaunchBlenderCmd creates a command to launch Blender for a specific version.
func LaunchBlenderCmd(downloadDir string, version string) tea.Cmd {
	return func() tea.Msg {
		buildInfo, dirPath, err := FindBuild(downloadDir, func(b model.BlenderBuild) bool {
			return b.Version == version
		})
		if err != nil {
			return err
		}
		if buildInfo == nil {
			return fmt.Errorf("blender version %s not found", version)
		}

		blenderExe := findBlenderExecutable(dirPath)
		if blenderExe == "" {
			return fmt.Errorf("could not find Blender executable in %s", dirPath)
		}
		return model.BlenderExecMsg{
			Version:    version,
			Executable: blenderExe,
		}
	}
}

// findBlenderExecutable locates the Blender executable in the installation directory.
func findBlenderExecutable(installDir string) string {
	var candidate string
	switch runtime.GOOS {
	case "windows":
		candidate = filepath.Join(installDir, "blender-launcher.exe")
	case "linux":
		candidate = filepath.Join(installDir, "blender")
	default:
		candidate = filepath.Join(installDir, "blender")
	}

	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// OpenFileExplorer opens the default file explorer to the specified directory.
func OpenFileExplorer(dir string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("explorer", dir)
	} else {
		if _, err := exec.LookPath("xdg-open"); err == nil {
			cmd = exec.Command("xdg-open", dir)
		} else if _, err := exec.LookPath("gnome-open"); err == nil {
			cmd = exec.Command("gnome-open", dir)
		} else if _, err := exec.LookPath("kde-open"); err == nil {
			cmd = exec.Command("kde-open", dir)
		} else {
			cmd = exec.Command("xdg-open", dir)
		}
	}

	cmd.Stdout = nil
	cmd.Stderr = nil

	// Detach the process (implementation provided elsewhere)
	detachProcess(cmd)

	return cmd.Start()
}

// OpenURL opens url in the user's default browser.
func OpenURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		if _, err := exec.LookPath("xdg-open"); err == nil {
			cmd = exec.Command("xdg-open", url)
		} else if _, err := exec.LookPath("gnome-open"); err == nil {
			cmd = exec.Command("gnome-open", url)
		} else if _, err := exec.LookPath("kde-open"); err == nil {
			cmd = exec.Command("kde-open", url)
		} else {
			cmd = exec.Command("xdg-open", url)
		}
	}

	cmd.Stdout = nil
	cmd.Stderr = nil

	detachProcess(cmd)

	return cmd.Start()
}

// CleanOldBuilds removes all builds from the .oldbuilds directory.
// Returns the number of cleaned builds and any error encountered.
func CleanOldBuilds(downloadDir string) (int, error) {
	oldBuildsDir := filepath.Join(downloadDir, download.OldBuildsDir)

	// Check if the old builds directory exists
	if _, err := os.Stat(oldBuildsDir); os.IsNotExist(err) {
		// If it doesn't exist, there's nothing to clean
		return 0, nil
	}

	// Read the contents of the old builds directory
	entries, err := os.ReadDir(oldBuildsDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read %s directory: %w", download.OldBuildsDir, err)
	}

	cleanedCount := 0

	// Delete each old build
	for _, entry := range entries {
		if entry.IsDir() {
			dirPath := filepath.Join(oldBuildsDir, entry.Name())
			if err := os.RemoveAll(dirPath); err != nil {
				return cleanedCount, fmt.Errorf("failed to delete old build %s: %w", entry.Name(), err)
			}
			cleanedCount++
		}
	}

	return cleanedCount, nil
}
