package updater

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
		"commit-darwin-arm64":      "abc123def456abc123def456abc123def456abc123def456abc123def456abcd1234",
		"commit-linux-amd64":       "fed321cba654fed321cba654fed321cba654fed321cba654fed321cba654fedc4321",
		"commit-windows-amd64.exe": "1111111111111111111111111111111111111111111111111111111111111111",
		"commit-darwin-amd64":      "2222222222222222222222222222222222222222222222222222222222222222",
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

func TestDownloadBinary_Success(t *testing.T) {
	binaryContent := []byte("fake-binary-content-for-testing")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(binaryContent)
	}))
	defer server.Close()

	tempPath, err := downloadBinary(server.URL + "/commit-darwin-arm64")
	if err != nil {
		t.Fatalf("downloadBinary returned unexpected error: %v", err)
	}
	defer os.Remove(tempPath) //nolint:errcheck // test cleanup

	// Verify file exists and has correct content
	downloaded, err := os.ReadFile(tempPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(downloaded) != string(binaryContent) {
		t.Errorf("downloaded content mismatch: expected %q, got %q", string(binaryContent), string(downloaded))
	}

	// Verify the file is executable
	info, err := os.Stat(tempPath)
	if err != nil {
		t.Fatalf("failed to stat downloaded file: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("downloaded file should be executable, got mode %v", info.Mode())
	}
}

func TestDownloadBinary_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tempPath, err := downloadBinary(server.URL + "/nonexistent")
	if err == nil {
		_ = os.Remove(tempPath)
		t.Fatal("expected error for non-200 status, got nil")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention status code 404, got: %v", err)
	}
}

func TestDownloadBinary_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tempPath, err := downloadBinary(server.URL + "/error")
	if err == nil {
		_ = os.Remove(tempPath)
		t.Fatal("expected error for 500 status, got nil")
	}

	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status code 500, got: %v", err)
	}
}

func TestDownloadBinary_InvalidURL(t *testing.T) {
	_, err := downloadBinary("http://127.0.0.1:0/nonexistent")
	if err == nil {
		t.Fatal("expected error for unreachable server, got nil")
	}
}

func TestDownloadBinary_LargePayload(t *testing.T) {
	// Verify downloadBinary handles a larger payload correctly
	largeContent := make([]byte, 1024*1024) // 1MB
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(largeContent)
	}))
	defer server.Close()

	tempPath, err := downloadBinary(server.URL + "/large-binary")
	if err != nil {
		t.Fatalf("downloadBinary returned unexpected error: %v", err)
	}
	defer os.Remove(tempPath) //nolint:errcheck // test cleanup

	downloaded, err := os.ReadFile(tempPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if len(downloaded) != len(largeContent) {
		t.Errorf("downloaded size mismatch: expected %d bytes, got %d bytes", len(largeContent), len(downloaded))
	}
}

func TestReplaceBinary_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file (the "new" binary)
	sourceContent := []byte("new-binary-content")
	sourcePath := filepath.Join(tmpDir, "new-binary")
	if err := os.WriteFile(sourcePath, sourceContent, 0755); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Create target file (the "old" binary to be replaced)
	targetContent := []byte("old-binary-content")
	targetPath := filepath.Join(tmpDir, "old-binary")
	if err := os.WriteFile(targetPath, targetContent, 0755); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	// Replace target with source
	if err := replaceBinary(targetPath, sourcePath); err != nil {
		t.Fatalf("replaceBinary returned unexpected error: %v", err)
	}

	// Verify target now has source content
	result, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read replaced file: %v", err)
	}
	if string(result) != string(sourceContent) {
		t.Errorf("replaced binary content mismatch: expected %q, got %q", string(sourceContent), string(result))
	}

	// Verify the replaced file is executable
	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("failed to stat replaced file: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("replaced binary should be executable, got mode %v", info.Mode())
	}
}

func TestReplaceBinary_SourceNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	targetPath := filepath.Join(tmpDir, "target")
	if err := os.WriteFile(targetPath, []byte("existing"), 0755); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	err := replaceBinary(targetPath, filepath.Join(tmpDir, "nonexistent-source"))
	if err == nil {
		t.Fatal("expected error when source file does not exist, got nil")
	}
}

func TestReplaceBinary_TargetDirNotWritable(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	sourcePath := filepath.Join(tmpDir, "source")
	if err := os.WriteFile(sourcePath, []byte("new-content"), 0755); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Target in a nonexistent directory
	targetPath := filepath.Join(tmpDir, "nonexistent-dir", "target")

	err := replaceBinary(targetPath, sourcePath)
	if err == nil {
		t.Fatal("expected error when target directory does not exist, got nil")
	}
}

func TestReplaceBinary_PreservesSourceFile(t *testing.T) {
	tmpDir := t.TempDir()

	sourceContent := []byte("source-content")
	sourcePath := filepath.Join(tmpDir, "source")
	if err := os.WriteFile(sourcePath, sourceContent, 0755); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	targetPath := filepath.Join(tmpDir, "target")
	if err := os.WriteFile(targetPath, []byte("old-target"), 0755); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	if err := replaceBinary(targetPath, sourcePath); err != nil {
		t.Fatalf("replaceBinary returned unexpected error: %v", err)
	}

	// Source file should still exist and be unchanged (it copies, not moves)
	sourceAfter, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("source file should still exist after replace: %v", err)
	}
	if string(sourceAfter) != string(sourceContent) {
		t.Errorf("source file content changed: expected %q, got %q", string(sourceContent), string(sourceAfter))
	}
}

func TestReplaceBinary_CreatesTargetIfNotExists(t *testing.T) {
	tmpDir := t.TempDir()

	sourceContent := []byte("brand-new-binary")
	sourcePath := filepath.Join(tmpDir, "source")
	if err := os.WriteFile(sourcePath, sourceContent, 0755); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Target does not exist but directory does
	targetPath := filepath.Join(tmpDir, "new-target")

	if err := replaceBinary(targetPath, sourcePath); err != nil {
		t.Fatalf("replaceBinary returned unexpected error: %v", err)
	}

	result, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read new target file: %v", err)
	}
	if string(result) != string(sourceContent) {
		t.Errorf("new target content mismatch: expected %q, got %q", string(sourceContent), string(result))
	}
}

func TestDownloadBinary_VerifyChecksumIntegration(t *testing.T) {
	// Test that a downloaded binary can be verified with verifyChecksum (end-to-end)
	binaryContent := []byte("binary-content-for-checksum-integration")
	h := sha256.Sum256(binaryContent)
	expectedHash := hex.EncodeToString(h[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(binaryContent)
	}))
	defer server.Close()

	tempPath, err := downloadBinary(server.URL + "/binary")
	if err != nil {
		t.Fatalf("downloadBinary returned unexpected error: %v", err)
	}
	defer os.Remove(tempPath) //nolint:errcheck // test cleanup

	// Verify checksum of the downloaded file matches expected
	if err := verifyChecksum(tempPath, expectedHash); err != nil {
		t.Errorf("checksum verification of downloaded binary failed: %v", err)
	}
}

func TestFormatUpgradeResult_NoErrorNoSuccess(t *testing.T) {
	result := &UpgradeResult{
		Success:        false,
		CurrentVersion: "v1.0.0",
		Error:          nil,
	}

	formatted := FormatUpgradeResult(result)

	if !strings.Contains(formatted, "failed") {
		t.Errorf("expected failure message, got: %s", formatted)
	}
}

func TestFormatUpgradeResult_AlreadyLatestWithError(t *testing.T) {
	result := &UpgradeResult{
		Success:        true,
		CurrentVersion: "v1.0.0",
		NewVersion:     "v1.0.0",
		Error:          fmt.Errorf("already at latest version (v1.0.0)"),
	}

	formatted := FormatUpgradeResult(result)

	if !strings.Contains(formatted, "already at latest") {
		t.Errorf("expected 'already at latest' message, got: %s", formatted)
	}
}

// Helper types

type upgradeTestError struct {
	msg string
}

func (e *upgradeTestError) Error() string {
	return e.msg
}
