package model

import (
	"fmt"
	"strconv"
	"strings"

	version "github.com/hashicorp/go-version"
)

// VersionFilter filters builds by version. It supports two forms:
//
//   - a minimum version ("4.0"), keeping builds at or above it, and
//   - a wildcard ("5.x"), keeping builds whose leading version components match,
//     so "5.x" keeps every 5.* build and "5.2.x" keeps every 5.2.* build.
//
// "x", "X" and "*" are accepted as the wildcard token.
type VersionFilter struct {
	min    *version.Version
	prefix []int // wildcard: components that must match from the front
	wild   bool
}

// ParseVersionFilter parses a filter string into a VersionFilter. An empty
// string yields (nil, nil), meaning "no filtering". A malformed string yields an
// error so callers can decide whether to surface it or ignore it.
func ParseVersionFilter(s string) (*VersionFilter, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	if isWildcard(s) {
		prefix, err := wildcardPrefix(s)
		if err != nil {
			return nil, err
		}
		return &VersionFilter{prefix: prefix, wild: true}, nil
	}

	min, err := version.NewVersion(s)
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
	if f.wild {
		segments := buildVersion.Segments()
		if len(segments) < len(f.prefix) {
			return false
		}
		for i, want := range f.prefix {
			if segments[i] != want {
				return false
			}
		}
		return true
	}
	return !buildVersion.LessThan(f.min)
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

// isWildcard reports whether any component of the filter is the wildcard token.
func isWildcard(s string) bool {
	for _, comp := range strings.Split(s, ".") {
		if isWildcardToken(comp) {
			return true
		}
	}
	return false
}

// wildcardPrefix returns the numeric components that precede the first wildcard
// token. Everything from the wildcard onward is ignored, so "5.x", "5.x.y" and
// "5.*" all yield {5}. "x" alone yields an empty prefix, which matches any build.
func wildcardPrefix(s string) ([]int, error) {
	comps := strings.Split(s, ".")
	prefix := make([]int, 0, len(comps))
	for _, comp := range comps {
		if isWildcardToken(comp) {
			break
		}
		n, err := strconv.Atoi(comp)
		if err != nil {
			return nil, fmt.Errorf("malformed version filter '%s': %q is not a number or wildcard", s, comp)
		}
		prefix = append(prefix, n)
	}
	return prefix, nil
}
