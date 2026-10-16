package model

import (
	version "github.com/hashicorp/go-version"
)

// VersionFilter applies a minimum-version filter to builds using semantic
// version comparison, so e.g. "4.10" is correctly treated as newer than "4.9".
type VersionFilter struct {
	min *version.Version
}

// ParseVersionFilter parses a filter string into a VersionFilter. An empty
// string yields (nil, nil), meaning "no filtering". A malformed string yields an
// error so callers can decide whether to surface it or ignore it.
func ParseVersionFilter(s string) (*VersionFilter, error) {
	if s == "" {
		return nil, nil
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
