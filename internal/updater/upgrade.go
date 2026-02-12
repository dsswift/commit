// Package updater handles version checking and self-update functionality.
package updater

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// DownloadTimeout is the timeout for downloading the new binary.
	DownloadTimeout = 5 * time.Minute

	// GitHubReleaseDownloadURL is the template for downloading release assets.
	GitHubReleaseDownloadURL = "https://github.com/dsswift/commit/releases/download/%s/commit-%s-%s%s"

	// GitHubChecksumDownloadURL is the template for downloading the checksums file for a release.
	GitHubChecksumDownloadURL = "https://github.com/dsswift/commit/releases/download/%s/checksums.txt"
)

// UpgradeResult contains the result of an upgrade operation.
type UpgradeResult struct {
	Success        bool
	CurrentVersion string
	NewVersion     string
	Error          error
}

// Upgrade performs a self-update of the binary.
func Upgrade(currentVersion string) *UpgradeResult {
	result := &UpgradeResult{
		CurrentVersion: currentVersion,
	}

	// Don't upgrade dev builds
	if currentVersion == "dev" || currentVersion == "" {
		result.Error = fmt.Errorf("cannot upgrade dev build - install from release")
		return result
	}

	// Check for latest version
	release, err := fetchLatestRelease()
	if err != nil {
		result.Error = fmt.Errorf("failed to check for updates: %w", err)
		return result
	}

	result.NewVersion = release.TagName

	// Compare versions
	if !isNewerVersion(release.TagName, currentVersion) {
		result.Success = true
		result.Error = fmt.Errorf("already at latest version (%s)", currentVersion)
		return result
	}

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		result.Error = fmt.Errorf("failed to get executable path: %w", err)
		return result
	}

	// Resolve symlinks
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		result.Error = fmt.Errorf("failed to resolve executable path: %w", err)
		return result
	}

	// Download new binary
	downloadURL := buildDownloadURL(release.TagName)
	tempPath, err := downloadBinary(downloadURL)
	if err != nil {
		result.Error = fmt.Errorf("failed to download update: %w", err)
		return result
	}
	defer os.Remove(tempPath)

	// Verify checksum
	checksums, checksumErr := downloadChecksums(release.TagName)
	if checksumErr != nil {
		// Older releases may not have checksums.txt -- warn but continue
		fmt.Fprintf(os.Stderr, "warning: checksum file not available for %s, skipping verification\n", release.TagName)
	} else {
		binaryName := buildBinaryName()
		expectedHash, found := checksums[binaryName]
		if !found {
			fmt.Fprintf(os.Stderr, "warning: no checksum entry for %s, skipping verification\n", binaryName)
		} else {
			if err := verifyChecksum(tempPath, expectedHash); err != nil {
				result.Error = fmt.Errorf("checksum verification failed: %w", err)
				return result
			}
		}
	}

	// Replace current binary
	if err := replaceBinary(execPath, tempPath); err != nil {
		result.Error = fmt.Errorf("failed to install update: %w", err)
		return result
	}

	// Update cache
	saveCache(&VersionCache{
		CheckedAt:     time.Now(),
		LatestVersion: release.TagName,
		ReleaseURL:    release.HTMLURL,
	})

	result.Success = true
	return result
}

// buildDownloadURL constructs the download URL for the current platform.
func buildDownloadURL(version string) string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}

	return fmt.Sprintf(GitHubReleaseDownloadURL, version, goos, goarch, ext)
}

// downloadBinary downloads a binary from the given URL to a temp file.
func downloadBinary(url string) (string, error) {
	client := &http.Client{
		Timeout: DownloadTimeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create temp file
	tempFile, err := os.CreateTemp("", "commit-update-*")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	// Copy content
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	// Make executable
	if err := os.Chmod(tempFile.Name(), 0755); err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

// replaceBinary replaces the target binary with the source.
func replaceBinary(target, source string) error {
	// On Windows, we can't replace a running binary directly
	// So we rename the old one first
	if runtime.GOOS == "windows" {
		oldPath := target + ".old"
		os.Remove(oldPath) // Remove any previous .old file
		if err := os.Rename(target, oldPath); err != nil {
			return err
		}
	}

	// Copy new binary to target location
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	_, err = io.Copy(targetFile, sourceFile)
	if err != nil {
		return err
	}

	return nil
}

// buildBinaryName returns the expected binary filename for the current platform.
func buildBinaryName() string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("commit-%s-%s%s", runtime.GOOS, runtime.GOARCH, ext)
}

// downloadChecksums downloads and parses the checksums.txt file for a given release version.
// Each line in the file has the format: <sha256hash>  <filename>
// Returns a map of filename -> hash.
func downloadChecksums(version string) (map[string]string, error) {
	url := fmt.Sprintf(GitHubChecksumDownloadURL, version)

	client := &http.Client{
		Timeout: DownloadTimeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums download failed with status %d", resp.StatusCode)
	}

	checksums := make(map[string]string)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Format: <hash>  <filename> (two spaces between hash and filename)
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}

		hash := parts[0]
		filename := parts[1]
		checksums[filename] = hash
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse checksums: %w", err)
	}

	return checksums, nil
}

// verifyChecksum computes the SHA256 hash of the file at binaryPath and compares
// it against expectedHash. Returns nil if they match, or an error if they don't.
func verifyChecksum(binaryPath, expectedHash string) error {
	f, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to open file for checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to compute checksum: %w", err)
	}

	actualHash := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// FormatUpgradeResult returns a formatted string for the upgrade result.
func FormatUpgradeResult(result *UpgradeResult) string {
	if result.Error != nil && !result.Success {
		return fmt.Sprintf("❌ Upgrade failed: %v", result.Error)
	}

	if result.Success && result.Error != nil {
		// Already at latest
		return fmt.Sprintf("✅ %v", result.Error)
	}

	if result.Success {
		return fmt.Sprintf("✅ Upgraded: %s → %s",
			strings.TrimPrefix(result.CurrentVersion, "v"),
			strings.TrimPrefix(result.NewVersion, "v"))
	}

	return "❌ Upgrade failed"
}
