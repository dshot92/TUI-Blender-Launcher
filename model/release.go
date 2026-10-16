package model

import "strings"

// ReleaseCycles lists the release cycles the Blender API reports in the
// "release_cycle" field (rendered as the table's Type column), in display
// order. It is the single source of truth shared by the settings UI and the
// filtering logic.
var ReleaseCycles = []string{"alpha", "beta", "candidate", "stable"}

// ReleaseFilterOptions returns a copy of the selectable release cycles.
func ReleaseFilterOptions() []string {
	return append([]string(nil), ReleaseCycles...)
}

// ReleaseFilter is a selection of release cycles to show. An empty filter means
// "all release cycles".
type ReleaseFilter []string

// ParseReleaseTypes normalizes a selection of release cycles: unknown entries are
// dropped, duplicates removed and the result ordered canonically (matching
// ReleaseCycles). An empty result means "all".
func ParseReleaseTypes(selected []string) ReleaseFilter {
	set := make(map[string]bool, len(selected))
	for _, s := range selected {
		set[strings.ToLower(s)] = true
	}
	out := make(ReleaseFilter, 0, len(set))
	for _, c := range ReleaseCycles {
		if set[c] {
			out = append(out, c)
		}
	}
	return out
}

// Matches reports whether a build passes the release filter. An empty filter
// matches everything. Matching is case-insensitive.
func (f ReleaseFilter) Matches(b BlenderBuild) bool {
	if len(f) == 0 {
		return true
	}
	for _, s := range f {
		if strings.EqualFold(b.ReleaseCycle, s) {
			return true
		}
	}
	return false
}

// Apply returns the builds that pass the release filter, always keeping local
// and in-progress builds. An empty filter returns the input unchanged.
func (f ReleaseFilter) Apply(builds []BlenderBuild) []BlenderBuild {
	if len(f) == 0 {
		return builds
	}
	filtered := make([]BlenderBuild, 0, len(builds))
	for _, b := range builds {
		if b.Status == StateLocal || b.Status == StateDownloading || b.Status == StateExtracting || f.Matches(b) {
			filtered = append(filtered, b)
		}
	}
	return filtered
}
