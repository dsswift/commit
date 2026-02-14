package updater

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dsswift/commit/internal/testutil"
)

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		latest   string
		current  string
		expected bool
	}{
		{"1.0.0", "1.0.0", false},
		{"1.0.1", "1.0.0", true},
		{"1.1.0", "1.0.0", true},
		{"2.0.0", "1.0.0", true},
		{"1.0.0", "1.0.1", false},
		{"1.0.0", "2.0.0", false},
		{"v1.0.1", "v1.0.0", true},
		{"v1.0.1", "1.0.0", true},
		{"1.0.1", "v1.0.0", true},
		{"1.2.3", "1.2.2", true},
		{"1.2.3", "1.2.4", false},
		{"1.10.0", "1.9.0", true},
		{"1.0", "1.0.0", false},
		{"1.0.0", "1.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.latest+"_vs_"+tt.current, func(t *testing.T) {
			result := isNewerVersion(tt.latest, tt.current)
			if result != tt.expected {
				t.Errorf("isNewerVersion(%q, %q) = %v, expected %v",
					tt.latest, tt.current, result, tt.expected)
			}
		})
	}
}

func TestCheckVersion_DevBuild(t *testing.T) {
	info := CheckVersion("dev")

	if info.UpdateAvailable {
		t.Error("dev builds should never show update available")
	}

	info = CheckVersion("")

	if info.UpdateAvailable {
		t.Error("empty version should never show update available")
	}
}

func TestFormatUpdateNotice_NoUpdate(t *testing.T) {
	info := &VersionInfo{
		CurrentVersion:  "1.0.0",
		LatestVersion:   "1.0.0",
		UpdateAvailable: false,
	}

	notice := FormatUpdateNotice(info)
	if notice != "" {
		t.Errorf("expected empty notice, got %q", notice)
	}
}

func TestFormatUpdateNotice_UpdateAvailable(t *testing.T) {
	info := &VersionInfo{
		CurrentVersion:  "1.0.0",
		LatestVersion:   "1.1.0",
		UpdateAvailable: true,
	}

	notice := FormatUpdateNotice(info)

	if notice == "" {
		t.Error("expected non-empty notice")
	}

	if !testutil.ContainsString(notice, "1.0.0") {
		t.Error("notice should contain current version")
	}

	if !testutil.ContainsString(notice, "1.1.0") {
		t.Error("notice should contain latest version")
	}

	if !testutil.ContainsString(notice, "--upgrade") {
		t.Error("notice should mention --upgrade")
	}
}

func TestCacheSaveAndLoad(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "updater-test-*")
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	t.Setenv("HOME", tmpDir)

	// Create config directory
	configDir := filepath.Join(tmpDir, ".commit-tool")
	_ = os.MkdirAll(configDir, 0700)

	cache := &VersionCache{
		CheckedAt:     time.Now(),
		LatestVersion: "v1.2.3",
		ReleaseURL:    "https://github.com/dsswift/commit/releases/tag/v1.2.3",
	}

	err := saveCache(cache)
	if err != nil {
		t.Fatalf("saveCache failed: %v", err)
	}

	loaded, err := loadCache()
	if err != nil {
		t.Fatalf("loadCache failed: %v", err)
	}

	if loaded.LatestVersion != cache.LatestVersion {
		t.Errorf("expected version %q, got %q", cache.LatestVersion, loaded.LatestVersion)
	}

	if loaded.ReleaseURL != cache.ReleaseURL {
		t.Errorf("expected URL %q, got %q", cache.ReleaseURL, loaded.ReleaseURL)
	}
}

func TestLoadCache_NoFile(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "updater-test-*")
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	t.Setenv("HOME", tmpDir)

	_, err := loadCache()
	if err == nil {
		t.Error("expected error for missing cache file")
	}
}

func TestCheckVersion_UsesCache(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "updater-test-*")
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	t.Setenv("HOME", tmpDir)

	// Create config directory and cache
	configDir := filepath.Join(tmpDir, ".commit-tool")
	_ = os.MkdirAll(configDir, 0700)

	cache := &VersionCache{
		CheckedAt:     time.Now(), // Recent cache
		LatestVersion: "v2.0.0",
		ReleaseURL:    "https://example.com",
	}
	_ = saveCache(cache)

	// Check version should use cache (not make HTTP request)
	info := CheckVersion("v1.0.0")

	if info.LatestVersion != "v2.0.0" {
		t.Errorf("expected cached version v2.0.0, got %q", info.LatestVersion)
	}

	if !info.UpdateAvailable {
		t.Error("expected update to be available")
	}
}

func TestCheckVersion_ExpiredCache(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "updater-test-*")
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	t.Setenv("HOME", tmpDir)

	// Create config directory and expired cache
	configDir := filepath.Join(tmpDir, ".commit-tool")
	_ = os.MkdirAll(configDir, 0700)

	cache := &VersionCache{
		CheckedAt:     time.Now().Add(-48 * time.Hour), // Expired
		LatestVersion: "v2.0.0",
		ReleaseURL:    "https://example.com",
	}
	_ = saveCache(cache)

	// Check version - cache is expired, will try to fetch
	// (will fail since no network, but should not panic)
	info := CheckVersion("v1.0.0")

	// Should still have current version set
	if info.CurrentVersion != "v1.0.0" {
		t.Errorf("expected current version v1.0.0, got %q", info.CurrentVersion)
	}
}

func TestCheckVersionFresh_BypassesCache(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "updater-test-*")
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	t.Setenv("HOME", tmpDir)

	// Create config directory and fresh cache
	configDir := filepath.Join(tmpDir, ".commit-tool")
	_ = os.MkdirAll(configDir, 0700)

	cache := &VersionCache{
		CheckedAt:     time.Now(), // Recent cache - would normally be used
		LatestVersion: "v2.0.0",
		ReleaseURL:    "https://example.com",
	}
	_ = saveCache(cache)

	// CheckVersionFresh should bypass the cache and try to fetch
	// (will fail since no network mock, so LatestVersion will be empty)
	info := CheckVersionFresh("v1.0.0")

	// Should NOT use cached version - proves cache was bypassed
	if info.LatestVersion == "v2.0.0" {
		t.Error("CheckVersionFresh should bypass cache, but used cached version")
	}

	// Current version should still be set
	if info.CurrentVersion != "v1.0.0" {
		t.Errorf("expected current version v1.0.0, got %q", info.CurrentVersion)
	}
}
