package api

import (
	"TUI-Blender-Launcher/config"
	"TUI-Blender-Launcher/model"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
)

// API endpoint URLs for different build types
const (
	dailyBlenderAPIURL        = "https://builder.blender.org/download/daily/?format=json&v=1"
	patchBlenderAPIURL        = "https://builder.blender.org/download/patch/?format=json&v=1"
	experimentalBlenderAPIURL = "https://builder.blender.org/download/experimental/?format=json&v=1"
)

// buildTypeURLs maps each supported build type to its API endpoint. It is keyed
// by model.BuildType so the set of build types has a single source of truth.
var buildTypeURLs = map[model.BuildType]string{
	model.BuildTypeDaily:        dailyBlenderAPIURL,
	model.BuildTypePatch:        patchBlenderAPIURL,
	model.BuildTypeExperimental: experimentalBlenderAPIURL,
}

// API represents the Blender API client
type API struct {
	client *http.Client
}

// NewAPI creates a new API client. The HTTP client is resolved at request
// time (falling back to http.DefaultClient), so tests can inject a mock
// RoundTripper via http.DefaultClient even after NewAPI is called.
func NewAPI() *API {
	return &API{}
}

// NewAPIWithClient creates a new API client using the provided HTTP client.
// Useful for testing or for callers that need custom transport settings.
func NewAPIWithClient(client *http.Client) *API {
	return &API{client: client}
}

// FetchBuilds fetches the list of Blender builds from the official API,
// filtering for the current OS/architecture, file extensions, and minimum version.
func (a *API) FetchBuilds(versionFilter string, buildType string) ([]model.BlenderBuild, error) {
	// Get config
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Determine which API URL to use based on buildType, defaulting to daily
	// builds if the value is unknown.
	apiURL, ok := buildTypeURLs[model.ParseBuildType(buildType)]
	if !ok {
		apiURL = dailyBlenderAPIURL
	}

	// Add UUID to request headers
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-Client-UUID", cfg.UUID)

	// Resolve the HTTP client lazily so tests can override http.DefaultClient
	// after NewAPI() has been called.
	client := a.client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch data: status code %d", resp.StatusCode)
	}

	var allBuildEntries []model.BlenderBuild
	if err := json.NewDecoder(resp.Body).Decode(&allBuildEntries); err != nil {
		return nil, fmt.Errorf("failed to decode JSON (check API response structure): %w", err)
	}

	// --- Filtering Setup ---
	currentOS := runtime.GOOS
	currentArch := runtime.GOARCH
	allowed := allowedArch(currentOS, currentArch)

	// Parse the version filter once. An invalid filter is a user error, so it is
	// surfaced to the caller (which shows it in the TUI).
	vf, err := model.ParseVersionFilter(versionFilter)
	if err != nil {
		return nil, fmt.Errorf("invalid version filter format '%s': %w", versionFilter, err)
	}

	// --- Filtering Loop ---
	var platformFilteredBuilds []model.BlenderBuild
	for _, build := range allBuildEntries {
		if !matchesPlatform(build, currentOS, allowed) {
			continue
		}
		if !hasAllowedExtension(build.FileExtension) {
			continue
		}
		if !vf.Matches(build, false) {
			continue
		}

		// Passed all filters
		build.Status = model.StateOnline
		platformFilteredBuilds = append(platformFilteredBuilds, build)
	}

	return platformFilteredBuilds, nil
}
