package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// testRepo creates a temporary git repository for testing.
// Returns the repo path and a cleanup function.
func testRepo(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		cleanup()
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	cmd.Run()

	return tmpDir, cleanup
}

// createFile creates a file in the test repo
func createFile(t *testing.T, repoDir, filename, content string) {
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

// gitAdd stages a file
func gitAdd(t *testing.T, repoDir string, files ...string) {
	t.Helper()
	args := append([]string{"add"}, files...)
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %s: %v", string(out), err)
	}
}

// gitCommit creates a commit
func gitCommit(t *testing.T, repoDir, message string) {
	t.Helper()
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %s: %v", string(out), err)
	}
}

func TestFindGitRoot(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create a subdirectory
	subDir := filepath.Join(repoDir, "sub", "dir")
	os.MkdirAll(subDir, 0755)

	// Should find root from subdirectory
	root, err := FindGitRoot(subDir)
	if err != nil {
		t.Fatalf("FindGitRoot failed: %v", err)
	}

	// Resolve symlinks for comparison (macOS /var -> /private/var)
	expectedRoot, _ := filepath.EvalSymlinks(repoDir)
	actualRoot, _ := filepath.EvalSymlinks(root)

	if actualRoot != expectedRoot {
		t.Errorf("expected root %q, got %q", expectedRoot, actualRoot)
	}
}

func TestFindGitRoot_NotARepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "not-git-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, err = FindGitRoot(tmpDir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestIsGitRepo(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	if !IsGitRepo(repoDir) {
		t.Error("expected IsGitRepo to return true for git repo")
	}

	tmpDir, _ := os.MkdirTemp("", "not-git-*")
	defer os.RemoveAll(tmpDir)

	if IsGitRepo(tmpDir) {
		t.Error("expected IsGitRepo to return false for non-git directory")
	}
}

func TestCollector_Status_Empty(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	collector := NewCollector(repoDir)
	status, err := collector.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if status.HasChanges() {
		t.Error("expected no changes in empty repo")
	}
}

func TestCollector_Status_Untracked(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	createFile(t, repoDir, "new.txt", "content")

	collector := NewCollector(repoDir)
	status, err := collector.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if len(status.Untracked) != 1 {
		t.Errorf("expected 1 untracked file, got %d", len(status.Untracked))
	}

	if status.Untracked[0] != "new.txt" {
		t.Errorf("expected 'new.txt', got %q", status.Untracked[0])
	}
}

func TestCollector_Status_Modified(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, repoDir, "file.txt", "initial")
	gitAdd(t, repoDir, "file.txt")
	gitCommit(t, repoDir, "initial commit")

	// Modify file
	createFile(t, repoDir, "file.txt", "modified")

	collector := NewCollector(repoDir)
	status, err := collector.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if len(status.Modified) != 1 {
		t.Errorf("expected 1 modified file, got %d", len(status.Modified))
	}
}

func TestCollector_Status_Staged(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create and stage a file
	createFile(t, repoDir, "staged.txt", "content")
	gitAdd(t, repoDir, "staged.txt")

	collector := NewCollector(repoDir)
	status, err := collector.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if len(status.Staged) != 1 {
		t.Errorf("expected 1 staged file, got %d", len(status.Staged))
	}

	if len(status.Added) != 1 {
		t.Errorf("expected 1 added file, got %d", len(status.Added))
	}
}

func TestCollector_Diff(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, repoDir, "file.txt", "line1\nline2\n")
	gitAdd(t, repoDir, "file.txt")
	gitCommit(t, repoDir, "initial")

	// Modify file
	createFile(t, repoDir, "file.txt", "line1\nline2\nline3\n")

	collector := NewCollector(repoDir)
	diff, err := collector.Diff(false)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if diff == "" {
		t.Error("expected non-empty diff")
	}

	if !containsString(diff, "+line3") {
		t.Errorf("expected diff to contain '+line3', got: %s", diff)
	}
}

func TestCollector_RecentCommits(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create several commits
	for i := 1; i <= 5; i++ {
		createFile(t, repoDir, "file.txt", string(rune('0'+i)))
		gitAdd(t, repoDir, "file.txt")
		gitCommit(t, repoDir, "commit "+string(rune('0'+i)))
	}

	collector := NewCollector(repoDir)
	commits, err := collector.RecentCommits(3)
	if err != nil {
		t.Fatalf("RecentCommits failed: %v", err)
	}

	if len(commits) != 3 {
		t.Errorf("expected 3 commits, got %d", len(commits))
	}

	// Most recent should be first
	if commits[0] != "commit 5" {
		t.Errorf("expected first commit to be 'commit 5', got %q", commits[0])
	}
}

func TestCollector_RecentCommits_Empty(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	collector := NewCollector(repoDir)
	commits, err := collector.RecentCommits(10)
	if err != nil {
		t.Fatalf("RecentCommits failed: %v", err)
	}

	if len(commits) != 0 {
		t.Errorf("expected 0 commits in empty repo, got %d", len(commits))
	}
}

func TestCollector_CurrentBranch(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Need at least one commit to have a branch
	createFile(t, repoDir, "file.txt", "content")
	gitAdd(t, repoDir, "file.txt")
	gitCommit(t, repoDir, "initial")

	collector := NewCollector(repoDir)
	branch, err := collector.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch failed: %v", err)
	}

	// Default branch is usually main or master
	if branch != "main" && branch != "master" {
		t.Errorf("expected 'main' or 'master', got %q", branch)
	}
}

func TestCollector_IsInitialCommit(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	collector := NewCollector(repoDir)

	// Empty repo - no commits yet
	// IsInitialCommit checks if HEAD~1 exists, which it won't in empty repo
	// But we can't call it on empty repo since HEAD doesn't exist

	// Create first commit
	createFile(t, repoDir, "file.txt", "content")
	gitAdd(t, repoDir, "file.txt")
	gitCommit(t, repoDir, "initial")

	// Now HEAD exists but HEAD~1 doesn't
	if !collector.IsInitialCommit() {
		t.Error("expected IsInitialCommit to be true for first commit")
	}

	// Create second commit
	createFile(t, repoDir, "file.txt", "modified")
	gitAdd(t, repoDir, "file.txt")
	gitCommit(t, repoDir, "second")

	// Now HEAD~1 exists
	if collector.IsInitialCommit() {
		t.Error("expected IsInitialCommit to be false after second commit")
	}
}

func TestTruncateDiff(t *testing.T) {
	tests := []struct {
		name     string
		diff     string
		max      int
		expected string
	}{
		{
			name:     "no truncation needed",
			diff:     "short diff",
			max:      100,
			expected: "short diff",
		},
		{
			name:     "truncates at line boundary",
			diff:     "line1\nline2\nline3\nline4",
			max:      15,
			expected: "line1\nline2\n\n... (truncated)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateDiff(tt.diff, tt.max)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestStager_StageFiles(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	createFile(t, repoDir, "file1.txt", "content1")
	createFile(t, repoDir, "file2.txt", "content2")

	stager := NewStager(repoDir)
	err := stager.StageFiles([]string{"file1.txt"})
	if err != nil {
		t.Fatalf("StageFiles failed: %v", err)
	}

	// Verify file is staged
	staged, err := stager.StagedFiles()
	if err != nil {
		t.Fatalf("StagedFiles failed: %v", err)
	}

	if len(staged) != 1 || staged[0] != "file1.txt" {
		t.Errorf("expected ['file1.txt'], got %v", staged)
	}
}

func TestStager_UnstageAll(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	createFile(t, repoDir, "file1.txt", "content1")
	createFile(t, repoDir, "file2.txt", "content2")

	stager := NewStager(repoDir)
	stager.StageFiles([]string{"file1.txt", "file2.txt"})

	err := stager.UnstageAll()
	if err != nil {
		t.Fatalf("UnstageAll failed: %v", err)
	}

	staged, _ := stager.StagedFiles()
	if len(staged) != 0 {
		t.Errorf("expected no staged files, got %v", staged)
	}
}

func TestStager_HasStagedChanges(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	stager := NewStager(repoDir)

	has, _ := stager.HasStagedChanges()
	if has {
		t.Error("expected no staged changes initially")
	}

	createFile(t, repoDir, "file.txt", "content")
	stager.StageFiles([]string{"file.txt"})

	has, _ = stager.HasStagedChanges()
	if !has {
		t.Error("expected staged changes after staging")
	}
}

func TestCommitter_Commit(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	createFile(t, repoDir, "file.txt", "content")

	stager := NewStager(repoDir)
	stager.StageFiles([]string{"file.txt"})

	committer := NewCommitter(repoDir)
	hash, err := committer.Commit("test: add file")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if hash == "" {
		t.Error("expected non-empty commit hash")
	}

	// Verify commit message
	msg, _ := committer.GetLastCommitMessage()
	if msg != "test: add file" {
		t.Errorf("expected message 'test: add file', got %q", msg)
	}
}

func TestCommitter_CommitWithScope(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	createFile(t, repoDir, "file.txt", "content")

	stager := NewStager(repoDir)
	stager.StageFiles([]string{"file.txt"})

	committer := NewCommitter(repoDir)
	scope := "api"
	hash, err := committer.CommitWithScope("feat", &scope, "add endpoint")
	if err != nil {
		t.Fatalf("CommitWithScope failed: %v", err)
	}

	if hash == "" {
		t.Error("expected non-empty commit hash")
	}

	msg, _ := committer.GetLastCommitMessage()
	if msg != "feat(api): add endpoint" {
		t.Errorf("expected 'feat(api): add endpoint', got %q", msg)
	}
}

func TestCommitter_CommitWithScope_NoScope(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	createFile(t, repoDir, "file.txt", "content")

	stager := NewStager(repoDir)
	stager.StageFiles([]string{"file.txt"})

	committer := NewCommitter(repoDir)
	hash, err := committer.CommitWithScope("docs", nil, "update readme")
	if err != nil {
		t.Fatalf("CommitWithScope failed: %v", err)
	}

	if hash == "" {
		t.Error("expected non-empty commit hash")
	}

	msg, _ := committer.GetLastCommitMessage()
	if msg != "docs: update readme" {
		t.Errorf("expected 'docs: update readme', got %q", msg)
	}
}

func TestReverser_Reverse(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, repoDir, "file.txt", "initial")
	gitAdd(t, repoDir, "file.txt")
	gitCommit(t, repoDir, "initial commit")

	// Create second commit
	createFile(t, repoDir, "file.txt", "modified")
	gitAdd(t, repoDir, "file.txt")
	gitCommit(t, repoDir, "second commit")

	reverser := NewReverser(repoDir)
	err := reverser.Reverse(false)
	if err != nil {
		t.Fatalf("Reverse failed: %v", err)
	}

	// Verify HEAD is back to first commit
	collector := NewCollector(repoDir)
	commits, _ := collector.RecentCommits(1)
	if len(commits) == 0 || commits[0] != "initial commit" {
		t.Errorf("expected HEAD to be 'initial commit', got %v", commits)
	}

	// Verify changes are in working directory
	status, _ := collector.Status()
	if len(status.Modified) != 1 {
		t.Errorf("expected 1 modified file, got %d", len(status.Modified))
	}
}

func TestReverser_Reverse_InitialCommit(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create only one commit
	createFile(t, repoDir, "file.txt", "content")
	gitAdd(t, repoDir, "file.txt")
	gitCommit(t, repoDir, "initial commit")

	reverser := NewReverser(repoDir)
	err := reverser.Reverse(false)
	if err == nil {
		t.Error("expected error when reversing initial commit")
	}
}

func TestPushedCommitError(t *testing.T) {
	err := &PushedCommitError{}
	msg := err.Error()

	if !containsString(msg, "pushed to origin") {
		t.Errorf("expected error to mention 'pushed to origin', got: %s", msg)
	}

	if !containsString(msg, "--force") {
		t.Errorf("expected error to mention '--force', got: %s", msg)
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
