package model

// BuildType identifies which Blender BuildBot channel builds are fetched from.
type BuildType string

const (
	BuildTypeDaily        BuildType = "daily"
	BuildTypeExperimental BuildType = "experimental"
	BuildTypePatch        BuildType = "patch"
)

// DefaultBuildType is used when a configured value is empty or unknown.
const DefaultBuildType = BuildTypeDaily

// BuildTypes lists the supported build types in display order. It is the single
// source of truth shared by config validation, the settings UI and the API.
var BuildTypes = []BuildType{BuildTypeDaily, BuildTypeExperimental, BuildTypePatch}

// Valid reports whether the build type is one of the supported values.
func (b BuildType) Valid() bool {
	for _, t := range BuildTypes {
		if t == b {
			return true
		}
	}
	return false
}

// ParseBuildType normalizes an arbitrary string to a supported BuildType,
// falling back to DefaultBuildType when it is empty or unknown.
func ParseBuildType(s string) BuildType {
	b := BuildType(s)
	if b.Valid() {
		return b
	}
	return DefaultBuildType
}

// BuildTypeNames returns the supported build types as plain strings, in display
// order, for use in settings UIs.
func BuildTypeNames() []string {
	names := make([]string, len(BuildTypes))
	for i, t := range BuildTypes {
		names[i] = string(t)
	}
	return names
}
