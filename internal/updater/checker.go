// Package updater handles version checking and self-update functionality.
package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dsswift/commit/internal/config"
)

const (
	// GitHubReleasesURL is the API endpoint for checking releases.
	GitHubReleasesURL = "https://api.github.com/repos/dsswift/commit/releases/latest"

	// CacheFileName is the name of the version check cache file.
	CacheFileName = ".version-check"

	// CacheDuration is how long to cache version check results.
	CacheDuration = 24 * time.Hour

	// CheckTimeout is the timeout for the version check HTTP request.
	CheckTimeout = 5 * time.Second
)

// VersionInfo contains information about available versions.
type VersionInfo struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	ReleaseURL      string
}

// VersionCache stores the cached version check result.
type VersionCache struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
	ReleaseURL    string    `json:"release_url"`
}

// GitHubRelease represents a GitHub release API response.
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// CheckVersion checks if a newer version is available.
// This function is designed to be called in a goroutine and not block the main execution.
func CheckVersion(currentVersion string) *VersionInfo {
	info := &VersionInfo{
		CurrentVersion:  currentVersion,
		UpdateAvailable: false,
	}

	// Don't check for dev builds
	if currentVersion == "dev" || currentVersion == "" {
		return info
	}

	// Check cache first
	cached, err := loadCache()
	if err == nil && time.Since(cached.CheckedAt) < CacheDuration {
		info.LatestVersion = cached.LatestVersion
		info.ReleaseURL = cached.ReleaseURL
		info.UpdateAvailable = isNewerVersion(cached.LatestVersion, currentVersion)
		return info
	}

	// Fetch from GitHub
	release, err := fetchLatestRelease()
	if err != nil {
		return info
	}

	// Update cache
	saveCache(&VersionCache{
		CheckedAt:     time.Now(),
		LatestVersion: release.TagName,
		ReleaseURL:    release.HTMLURL,
	})

	info.LatestVersion = release.TagName
	info.ReleaseURL = release.HTMLURL
	info.UpdateAvailable = isNewerVersion(release.TagName, currentVersion)

	return info
}

// fetchLatestRelease fetches the latest release from GitHub.
func fetchLatestRelease() (*GitHubRelease, error) {
	client := &http.Client{
		Timeout: CheckTimeout,
	}

	req, err := http.NewRequest("GET", GitHubReleasesURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "commit-tool")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// loadCache loads the version check cache from disk.
func loadCache() (*VersionCache, error) {
	cachePath, err := getCachePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	var cache VersionCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	return &cache, nil
}

// saveCache saves the version check cache to disk.
func saveCache(cache *VersionCache) error {
	cachePath, err := getCachePath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0600)
}

// getCachePath returns the path to the version cache file.
func getCachePath() (string, error) {
	configPath, err := config.ConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(configPath, CacheFileName), nil
}

// isNewerVersion compares two version strings.
// Returns true if latest is newer than current.
func isNewerVersion(latest, current string) bool {
	// Normalize versions (remove 'v' prefix if present)
	latest = strings.TrimPrefix(latest, "v")
	current = strings.TrimPrefix(current, "v")

	// Simple comparison - split by dots and compare numerically
	latestParts := strings.Split(latest, ".")
	currentParts := strings.Split(current, ".")

	// Compare each part
	maxLen := len(latestParts)
	if len(currentParts) > maxLen {
		maxLen = len(currentParts)
	}

	for i := 0; i < maxLen; i++ {
		var latestNum, currentNum int

		if i < len(latestParts) {
			fmt.Sscanf(latestParts[i], "%d", &latestNum)
		}
		if i < len(currentParts) {
			fmt.Sscanf(currentParts[i], "%d", &currentNum)
		}

		if latestNum > currentNum {
			return true
		}
		if latestNum < currentNum {
			return false
		}
	}

	return false
}

// FormatUpdateNotice returns a formatted string for the update notice.
func FormatUpdateNotice(info *VersionInfo) string {
	if !info.UpdateAvailable {
		return ""
	}

	return fmt.Sprintf("\n💡 New version available: %s → %s\n   Run `commit --upgrade` to update",
		info.CurrentVersion, info.LatestVersion)
}
