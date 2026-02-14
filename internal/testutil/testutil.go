// Package testutil provides shared test helpers.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ContainsString reports whether substr is within s.
func ContainsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestRepo creates a temporary git repository for testing.
// Returns the repo path. The directory is cleaned up automatically via t.Cleanup.
func TestRepo(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	return tmpDir
}

// CreateFile creates a file in the given directory, creating parent directories as needed.
func CreateFile(t *testing.T, repoDir, filename, content string) {
	t.Helper()
	path := filepath.Join(repoDir, filename)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
}

// GitAdd stages files in the given repository.
func GitAdd(t *testing.T, repoDir string, files ...string) {
	t.Helper()
	args := append([]string{"add"}, files...)
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %s: %v", string(out), err)
	}
}

// GitCommit creates a commit and returns the short hash.
func GitCommit(t *testing.T, repoDir, message string) string {
	t.Helper()
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %s: %v", string(out), err)
	}

	cmd = exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get commit hash: %v", err)
	}
	return strings.TrimSpace(string(out))
}
