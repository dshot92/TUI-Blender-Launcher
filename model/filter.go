package model

import (
	"fmt"
	"strconv"
	"strings"

	version "github.com/hashicorp/go-version"
)

// VersionFilter filters builds by version, treating the configured value as the
// minimum accepted version. Builds at or above it are kept; older builds are
// dropped.
//
// Wildcard tokens ("x", "X", "*") may be used for the "any component" form of a
// version (e.g. "4.x"). In that form every wildcard is normalized to 0 before
// the minimum comparison, so "4.x" means "4.0 or newer".
type VersionFilter struct {
	min *version.Version
}

// ParseVersionFilter parses a filter string into a VersionFilter. An empty
// string yields (nil, nil), meaning "no filtering". A malformed string yields an
// error so callers can decide whether to surface it or ignore it.
func ParseVersionFilter(s string) (*VersionFilter, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	comps := strings.Split(s, ".")
	normalized := make([]string, len(comps))
	for i, comp := range comps {
		if isWildcardToken(comp) {
			normalized[i] = "0"
			continue
		}
		if _, err := strconv.Atoi(comp); err != nil {
			return nil, fmt.Errorf("malformed version filter '%s': %q is not a number or wildcard", s, comp)
		}
		normalized[i] = comp
	}

	min, err := version.NewVersion(strings.Join(normalized, "."))
	if err != nil {
		return nil, err
	}
	return &VersionFilter{min: min}, nil
}

// Matches reports whether a build passes the filter. A nil filter matches
// everything. When keepLocal is true, local and in-progress builds always match
// regardless of version. Builds whose version cannot be parsed are dropped.
func (f *VersionFilter) Matches(b BlenderBuild, keepLocal bool) bool {
	if f == nil {
		return true
	}
	if keepLocal && (b.Status == StateLocal || b.Status == StateDownloading || b.Status == StateExtracting) {
		return true
	}
	buildVersion, err := version.NewVersion(b.Version)
	if err != nil {
		return false
	}
	return !versionLess(buildVersion.Segments(), f.min.Segments())
}

// versionLess reports whether the left numeric version is strictly less than the
// right one using zero-padded component comparison, so a missing component is
// treated as 0 and pre-release metadata does not hide a build that sits at the
// same numeric version as the minimum (e.g. "5.0.0-alpha" still passes "5.0").
func versionLess(left, right []int) bool {
	n := len(left)
	if len(right) > n {
		n = len(right)
	}
	for i := 0; i < n; i++ {
		l := 0
		if i < len(left) {
			l = left[i]
		}
		r := 0
		if i < len(right) {
			r = right[i]
		}
		if l != r {
			return l < r
		}
	}
	return false
}

// Apply returns the builds that pass the filter, always keeping local and
// in-progress builds. A nil filter returns the input unchanged.
func (f *VersionFilter) Apply(builds []BlenderBuild) []BlenderBuild {
	if f == nil {
		return builds
	}
	filtered := make([]BlenderBuild, 0, len(builds))
	for _, b := range builds {
		if f.Matches(b, true) {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

// isWildcardToken reports whether a single dot-separated component is the
// wildcard placeholder.
func isWildcardToken(s string) bool {
	return s == "x" || s == "X" || s == "*"
}
