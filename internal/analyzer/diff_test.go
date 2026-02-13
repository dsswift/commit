package analyzer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildDiffRequest_DefaultRefs(t *testing.T) {
	req := BuildDiffRequest("/repo", "file.go", "", "")

	if req.FilePath != "file.go" {
		t.Errorf("expected file.go, got %q", req.FilePath)
	}

	if req.ToRef != "HEAD" {
		t.Errorf("expected ToRef 'HEAD', got %q", req.ToRef)
	}
}

func TestBuildDiffRequest_FromRefOnly(t *testing.T) {
	req := BuildDiffRequest("/repo", "file.go", "main", "")

	if req.FromRef != "main" {
		t.Errorf("expected FromRef 'main', got %q", req.FromRef)
	}
}

func TestBuildDiffRequest_BothRefs(t *testing.T) {
	req := BuildDiffRequest("/repo", "file.go", "main", "feature")

	if req.FromRef != "main" {
		t.Errorf("expected FromRef 'main', got %q", req.FromRef)
	}

	if req.ToRef != "feature" {
		t.Errorf("expected ToRef 'feature', got %q", req.ToRef)
	}
}

func TestBuildDiffPrompt_SystemPrompt(t *testing.T) {
	result := &DiffResult{
		FilePath:    "src/main.go",
		FromRef:     "HEAD~1",
		ToRef:       "HEAD",
		Diff:        "some diff content",
		NumStats:    "+10 -5",
		LinesAdded:  10,
		LinesRemove: 5,
	}

	system, user := BuildDiffPrompt(result)

	if system == "" {
		t.Error("expected non-empty system prompt")
	}

	if !strings.Contains(system, "code change analyst") {
		t.Error("system prompt should mention role")
	}

	if user == "" {
		t.Error("expected non-empty user prompt")
	}

	if !strings.Contains(user, "src/main.go") {
		t.Error("user prompt should contain file path")
	}

	if !strings.Contains(user, "+10 -5") {
		t.Error("user prompt should contain stats")
	}
}

func TestBuildDiffPrompt_UncommittedChanges(t *testing.T) {
	result := &DiffResult{
		FilePath: "file.go",
		FromRef:  "",
		ToRef:    "HEAD",
		Diff:     "diff",
		NumStats: "+1 -1",
	}

	_, user := BuildDiffPrompt(result)

	if !strings.Contains(user, "uncommitted changes") {
		t.Errorf("should mention uncommitted changes, got: %s", user)
	}
}

func TestBuildDiffPrompt_WorkingCopy(t *testing.T) {
	result := &DiffResult{
		FilePath: "file.go",
		FromRef:  "main",
		ToRef:    "",
		Diff:     "diff",
		NumStats: "+1 -1",
	}

	_, user := BuildDiffPrompt(result)

	if !strings.Contains(user, "working copy") {
		t.Errorf("should mention working copy, got: %s", user)
	}
}

func TestGetDiff_Integration(t *testing.T) {
	// Create temp git repo
	tmpDir, err := os.MkdirTemp("", "diff-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

	// Initialize git repo
	runGit(t, tmpDir, "init")
	runGit(t, tmpDir, "config", "user.email", "test@test.com")
	runGit(t, tmpDir, "config", "user.name", "Test")

	// Create initial file
	filePath := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(filePath, []byte("line 1\n"), 0644)
	runGit(t, tmpDir, "add", "test.txt")
	runGit(t, tmpDir, "commit", "-m", "initial")

	// Modify file
	_ = os.WriteFile(filePath, []byte("line 1\nline 2\n"), 0644)

	// Get diff
	req := BuildDiffRequest(tmpDir, "test.txt", "", "")
	result, err := GetDiff(req)
	if err != nil {
		t.Fatalf("GetDiff failed: %v", err)
	}

	if result.Diff == "" {
		t.Error("expected non-empty diff")
	}

	if result.LinesAdded != 1 {
		t.Errorf("expected 1 line added, got %d", result.LinesAdded)
	}
}

func TestGetDiff_NoDifference(t *testing.T) {
	// Create temp git repo
	tmpDir, err := os.MkdirTemp("", "diff-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

	// Initialize git repo
	runGit(t, tmpDir, "init")
	runGit(t, tmpDir, "config", "user.email", "test@test.com")
	runGit(t, tmpDir, "config", "user.name", "Test")

	// Create and commit file
	filePath := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(filePath, []byte("content\n"), 0644)
	runGit(t, tmpDir, "add", "test.txt")
	runGit(t, tmpDir, "commit", "-m", "initial")

	// Get diff with no changes
	req := BuildDiffRequest(tmpDir, "test.txt", "", "")
	result, err := GetDiff(req)
	if err != nil {
		t.Fatalf("GetDiff failed: %v", err)
	}

	if result.Diff != "" {
		t.Errorf("expected empty diff, got: %s", result.Diff)
	}
}

func TestNewDiffAnalyzer(t *testing.T) {
	analyzer := NewDiffAnalyzer("/repo")

	if analyzer == nil {
		t.Fatal("expected non-nil analyzer")
		return
	}

	if analyzer.gitRoot != "/repo" {
		t.Errorf("expected gitRoot '/repo', got %q", analyzer.gitRoot)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
}
