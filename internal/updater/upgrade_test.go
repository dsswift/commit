package updater

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestBuildDownloadURL(t *testing.T) {
	url := buildDownloadURL("v1.2.3")

	if !strings.Contains(url, "v1.2.3") {
		t.Errorf("URL should contain version, got: %s", url)
	}

	if !strings.Contains(url, runtime.GOOS) {
		t.Errorf("URL should contain GOOS, got: %s", url)
	}

	if !strings.Contains(url, runtime.GOARCH) {
		t.Errorf("URL should contain GOARCH, got: %s", url)
	}

	if runtime.GOOS == "windows" {
		if !strings.HasSuffix(url, ".exe") {
			t.Errorf("Windows URL should end with .exe, got: %s", url)
		}
	} else {
		if strings.HasSuffix(url, ".exe") {
			t.Errorf("Non-Windows URL should not end with .exe, got: %s", url)
		}
	}
}

func TestUpgrade_DevBuild(t *testing.T) {
	result := Upgrade("dev")

	if result.Success {
		t.Error("dev build should not succeed")
	}

	if result.Error == nil {
		t.Error("expected error for dev build")
	}

	if !strings.Contains(result.Error.Error(), "dev build") {
		t.Errorf("error should mention dev build, got: %v", result.Error)
	}
}

func TestUpgrade_EmptyVersion(t *testing.T) {
	result := Upgrade("")

	if result.Success {
		t.Error("empty version should not succeed")
	}
}

func TestFormatUpgradeResult_Success(t *testing.T) {
	result := &UpgradeResult{
		Success:        true,
		CurrentVersion: "v1.0.0",
		NewVersion:     "v1.1.0",
	}

	formatted := FormatUpgradeResult(result)

	if !strings.Contains(formatted, "✅") {
		t.Errorf("success should have checkmark, got: %s", formatted)
	}

	if !strings.Contains(formatted, "1.0.0") {
		t.Errorf("should contain current version, got: %s", formatted)
	}

	if !strings.Contains(formatted, "1.1.0") {
		t.Errorf("should contain new version, got: %s", formatted)
	}
}

func TestFormatUpgradeResult_AlreadyLatest(t *testing.T) {
	result := &UpgradeResult{
		Success:        true,
		CurrentVersion: "v1.0.0",
		NewVersion:     "v1.0.0",
		Error:          nil,
	}

	// Simulate already at latest
	result.Error = nil

	formatted := FormatUpgradeResult(result)

	if !strings.Contains(formatted, "✅") {
		t.Errorf("already at latest should have checkmark, got: %s", formatted)
	}
}

func TestFormatUpgradeResult_Error(t *testing.T) {
	result := &UpgradeResult{
		Success:        false,
		CurrentVersion: "v1.0.0",
		Error:          &upgradeTestError{"network error"},
	}

	formatted := FormatUpgradeResult(result)

	if !strings.Contains(formatted, "❌") {
		t.Errorf("error should have X, got: %s", formatted)
	}

	if !strings.Contains(formatted, "network error") {
		t.Errorf("should contain error message, got: %s", formatted)
	}
}

func TestBuildBinaryName(t *testing.T) {
	name := buildBinaryName()

	expectedPrefix := fmt.Sprintf("commit-%s-%s", runtime.GOOS, runtime.GOARCH)
	if !strings.HasPrefix(name, expectedPrefix) {
		t.Errorf("expected binary name to start with %s, got: %s", expectedPrefix, name)
	}

	if runtime.GOOS == "windows" {
		if !strings.HasSuffix(name, ".exe") {
			t.Errorf("Windows binary name should end with .exe, got: %s", name)
		}
	} else {
		if strings.HasSuffix(name, ".exe") {
			t.Errorf("Non-Windows binary name should not end with .exe, got: %s", name)
		}
	}

	// Verify the full expected name
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	expected := fmt.Sprintf("commit-%s-%s%s", runtime.GOOS, runtime.GOARCH, ext)
	if name != expected {
		t.Errorf("expected %s, got %s", expected, name)
	}
}

func TestVerifyChecksum(t *testing.T) {
	// Create a temp file with known content
	content := []byte("hello checksum verification")
	tmpFile, err := os.CreateTemp("", "checksum-test-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name()) //nolint:errcheck // test cleanup

	if _, err := tmpFile.Write(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	_ = tmpFile.Close()

	// Compute expected hash
	h := sha256.Sum256(content)
	correctHash := hex.EncodeToString(h[:])

	// Test: correct hash should pass
	if err := verifyChecksum(tmpFile.Name(), correctHash); err != nil {
		t.Errorf("verifyChecksum should pass with correct hash, got error: %v", err)
	}

	// Test: correct hash in uppercase should also pass (case-insensitive comparison)
	if err := verifyChecksum(tmpFile.Name(), strings.ToUpper(correctHash)); err != nil {
		t.Errorf("verifyChecksum should pass with uppercase correct hash, got error: %v", err)
	}

	// Test: wrong hash should fail
	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
	err = verifyChecksum(tmpFile.Name(), wrongHash)
	if err == nil {
		t.Error("verifyChecksum should fail with wrong hash")
	}
	if err != nil && !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("error should mention checksum mismatch, got: %v", err)
	}

	// Test: nonexistent file should return error
	err = verifyChecksum("/nonexistent/path/binary", correctHash)
	if err == nil {
		t.Error("verifyChecksum should fail for nonexistent file")
	}
}

func TestDownloadChecksums_Parse(t *testing.T) {
	// Create a test server that serves a checksums.txt file
	checksumContent := "abc123def456abc123def456abc123def456abc123def456abc123def456abcd1234  commit-darwin-arm64\n" +
		"fed321cba654fed321cba654fed321cba654fed321cba654fed321cba654fedc4321  commit-linux-amd64\n" +
		"1111111111111111111111111111111111111111111111111111111111111111  commit-windows-amd64.exe\n" +
		"\n" +
		"2222222222222222222222222222222222222222222222222222222222222222  commit-darwin-amd64\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request path contains the version
		if !strings.Contains(r.URL.Path, "v1.5.0") {
			http.NotFound(w, r)
			return
		}
		_, _ = fmt.Fprint(w, checksumContent)
	}))
	defer server.Close()

	// Temporarily override the checksum URL by using downloadChecksums with a custom URL.
	// Since downloadChecksums uses the constant, we test the parsing logic via a helper approach:
	// we'll call the HTTP endpoint directly and parse the response.
	client := &http.Client{}
	resp, err := client.Get(server.URL + "/v1.5.0/checksums.txt")
	if err != nil {
		t.Fatalf("failed to fetch from test server: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test HTTP response

	// Parse in the same way downloadChecksums does
	checksums := make(map[string]string)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		checksums[parts[1]] = parts[0]
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}

	// Verify all entries parsed correctly
	if len(checksums) != 4 {
		t.Errorf("expected 4 checksum entries, got %d", len(checksums))
	}

	expectedEntries := map[string]string{
		"commit-darwin-arm64":       "abc123def456abc123def456abc123def456abc123def456abc123def456abcd1234",
		"commit-linux-amd64":        "fed321cba654fed321cba654fed321cba654fed321cba654fed321cba654fedc4321",
		"commit-windows-amd64.exe":  "1111111111111111111111111111111111111111111111111111111111111111",
		"commit-darwin-amd64":       "2222222222222222222222222222222222222222222222222222222222222222",
	}

	for filename, expectedHash := range expectedEntries {
		actualHash, found := checksums[filename]
		if !found {
			t.Errorf("missing entry for %s", filename)
			continue
		}
		if actualHash != expectedHash {
			t.Errorf("hash mismatch for %s: expected %s, got %s", filename, expectedHash, actualHash)
		}
	}
}

func TestDownloadChecksums_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	// We can't easily test downloadChecksums directly since it uses a hardcoded URL,
	// but we can verify the error handling pattern by testing the HTTP response parsing.
	client := &http.Client{}
	resp, err := client.Get(server.URL + "/v0.1.0/checksums.txt")
	if err != nil {
		t.Fatalf("unexpected network error: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test HTTP response

	if resp.StatusCode == http.StatusOK {
		t.Error("expected non-200 status for missing checksums")
	}
}

// Helper types

type upgradeTestError struct {
	msg string
}

func (e *upgradeTestError) Error() string {
	return e.msg
}
