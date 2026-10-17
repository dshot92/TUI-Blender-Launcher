package model

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// BuildState represents the current state of a Blender build
type BuildState int

const (
	// StateNone is the default state
	StateNone BuildState = iota
	StateDownloading
	StateExtracting
	StateLocal
	StateOnline
	StateUpdate
	StateFailed
	StateCancelled
)

// String returns the string representation of the BuildState
func (s BuildState) String() string {
	switch s {
	case StateNone:
		return "None"
	case StateDownloading:
		return "Downloading"
	case StateExtracting:
		return "Extracting"
	case StateLocal:
		return "Local"
	case StateOnline:
		return "Online"
	case StateUpdate:
		return "Update"
	case StateFailed:
		return "Failed"
	case StateCancelled:
		return "Cancelled"
	default:
		return "Unknown"
	}
}

// Timestamp is a custom type to handle Unix timestamp decoding from JSON numbers.
type Timestamp time.Time

// UnmarshalJSON implements the json.Unmarshaler interface for Timestamp.
func (t *Timestamp) UnmarshalJSON(b []byte) error {
	// Try to unmarshal as an integer (Unix timestamp)
	var timestamp int64
	if err := json.Unmarshal(b, &timestamp); err == nil {
		// It's a Unix timestamp (seconds)
		*t = Timestamp(time.Unix(timestamp, 0))
		return nil
	}

	// If not an integer, try a string with RFC3339 format
	var timeStr string
	if err := json.Unmarshal(b, &timeStr); err == nil {
		parsedTime, err := time.Parse(time.RFC3339, timeStr)
		if err == nil {
			*t = Timestamp(parsedTime)
			return nil
		}
	}

	// Neither an integer Unix timestamp nor an RFC3339 string: reject the value
	// rather than silently substituting the current time, which would make
	// malformed data look freshly built.
	return fmt.Errorf("unsupported timestamp value %s", string(b))
}

// MarshalJSON implements the json.Marshaler interface for Timestamp.
// This ensures the timestamp is properly saved in version.json as RFC3339 formatted string.
func (t Timestamp) MarshalJSON() ([]byte, error) {
	// Convert to RFC3339 string format for consistent serialization
	return json.Marshal(time.Time(t).Format(time.RFC3339))
}

// Time returns the underlying time.Time value.
// This provides convenience for using the value as a standard time.Time.
func (t Timestamp) Time() time.Time {
	return time.Time(t)
}

// shortHashLen is the number of hash characters used in a build ID.
const shortHashLen = 8

// buildColumnCount is the number of sortable table columns. It must match
// len(buildColumnNames) in the tui package.
const buildColumnCount = 7

// BlenderBuild represents the structure of a single build entry from the API
// plus internal state for the TUI.
type BlenderBuild struct {
	// Fields from API
	Version         string    `json:"version"`
	Branch          string    `json:"branch"`
	Hash            string    `json:"hash"`           // Git commit hash short identifier
	BuildDate       Timestamp `json:"file_mtime"`     // Use custom Timestamp type
	DownloadURL     string    `json:"url"`            // URL for the specific file (can be build or checksum)
	OperatingSystem string    `json:"platform"`       // e.g., "linux", "windows", "macos"
	Architecture    string    `json:"architecture"`   // e.g., "amd64", "arm64"
	Size            int64     `json:"file_size"`      // File size in bytes
	FileName        string    `json:"file_name"`      // Full name of the downloadable file
	FileExtension   string    `json:"file_extension"` // e.g., "zip", "tar.gz", "sha256", "msi"
	ReleaseCycle    string    `json:"release_cycle"`  // e.g., "daily", "stable", "candidate" (replaces previous 'Type')

	// Internal state (not from API)
	Status BuildState

	// ChecksumURL points at the API's .sha256 sidecar for DownloadURL when the
	// API provides one. Populated by the api package; empty means the download
	// is not verifiable.
	ChecksumURL string `json:"-"`
}

// ID returns a stable identifier for the build: its version, optionally
// suffixed with the short commit hash when present.
func (b BlenderBuild) ID() string {
	if b.Hash == "" {
		return b.Version
	}
	h := b.Hash
	if len(h) > shortHashLen {
		h = h[:shortHashLen]
	}
	return b.Version + "-" + h
}

// BlenderExecMsg is sent when Blender should be executed directly
// This will cause the TUI to exit and exec Blender in its place
type BlenderExecMsg struct {
	Executable string // The path to the Blender executable
}

// DownloadState holds progress info for an active download
type DownloadState struct {
	BuildID     string        // Unique identifier for build (version + hash)
	Progress    float64       // Progress from 0.0 to 1.0
	Current     int64         // Bytes downloaded so far (renamed from CurrentBytes)
	Total       int64         // Total bytes to download (renamed from TotalBytes)
	Speed       float64       // Download speed in bytes/sec
	BuildState  BuildState    // Changed from Message to BuildState
	LastUpdated time.Time     // Timestamp of last progress update
	StartTime   time.Time     // When the download started
	CancelCh    chan struct{} // Per-download cancel channel
}

// Clone returns a shallow copy of the download state. Callers that read
// state outside the owning manager's lock should use this to avoid data
// races with the download goroutine mutating the shared instance.
func (s *DownloadState) Clone() *DownloadState {
	if s == nil {
		return nil
	}
	c := *s
	return &c
}

// FormatByteSize converts bytes to human-readable sizes
func FormatByteSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatBuildDate formats a build date in yyyy-mm-dd-hh-mm format
func FormatBuildDate(t Timestamp) string {
	return t.Time().Format("2006-01-02-15:04")
}

// compareByColumn compares two builds on the given column, returning -1, 0 or 1.
func compareByColumn(a, b BlenderBuild, column int) int {
	switch column {
	case 0: // Version
		return strings.Compare(a.Version, b.Version)
	case 1: // Status
		switch {
		case a.Status < b.Status:
			return -1
		case a.Status > b.Status:
			return 1
		default:
			return 0
		}
	case 2: // Branch
		return strings.Compare(a.Branch, b.Branch)
	case 3: // Type/ReleaseCycle
		return strings.Compare(a.ReleaseCycle, b.ReleaseCycle)
	case 4: // Hash
		return strings.Compare(a.Hash, b.Hash)
	case 5: // Size
		switch {
		case a.Size < b.Size:
			return -1
		case a.Size > b.Size:
			return 1
		default:
			return 0
		}
	case 6: // Build Date
		switch {
		case a.BuildDate.Time().Before(b.BuildDate.Time()):
			return -1
		case a.BuildDate.Time().After(b.BuildDate.Time()):
			return 1
		default:
			return 0
		}
	default:
		return 0
	}
}

// SortBuilds sorts the builds based on the selected column and sort order.
func SortBuilds(builds []BlenderBuild, column int, reverse bool) []BlenderBuild {
	// Create a copy of builds to avoid modifying the original
	sortedBuilds := make([]BlenderBuild, len(builds))
	copy(sortedBuilds, builds)

	// Order of columns to compare as tiebreakers (all columns, ascending).
	allColumns := make([]int, buildColumnCount)
	for i := range allColumns {
		allColumns[i] = i
	}

	sort.SliceStable(sortedBuilds, func(i, j int) bool {
		a, b := sortedBuilds[i], sortedBuilds[j]

		// First compare using the primary column.
		if c := compareByColumn(a, b, column); c != 0 {
			if reverse {
				return c > 0
			}
			return c < 0
		}

		// Values are equal: use the other columns as ascending tiebreakers.
		for _, secondaryCol := range allColumns {
			if secondaryCol == column {
				continue
			}
			if c := compareByColumn(a, b, secondaryCol); c != 0 {
				return c < 0
			}
		}

		// All values equal: maintain original order for stability.
		return i < j
	})

	return sortedBuilds
}
