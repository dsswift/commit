package updater

import (
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

// Helper types

type upgradeTestError struct {
	msg string
}

func (e *upgradeTestError) Error() string {
	return e.msg
}
