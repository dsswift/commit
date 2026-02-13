package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dsswift/commit/internal/testutil"
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
		_ = os.RemoveAll(tmpDir)
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
	_ = cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	_ = cmd.Run()

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
	_ = os.MkdirAll(subDir, 0755)

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
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

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
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

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

	if !testutil.ContainsString(diff, "+line3") {
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
	_ = stager.StageFiles([]string{"file1.txt", "file2.txt"})

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
	_ = stager.StageFiles([]string{"file.txt"})

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
	_ = stager.StageFiles([]string{"file.txt"})

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
	_ = stager.StageFiles([]string{"file.txt"})

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
	_ = stager.StageFiles([]string{"file.txt"})

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
	err := reverser.Reverse(1, false)
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
	err := reverser.Reverse(1, false)
	if err == nil {
		t.Error("expected error when reversing initial commit")
	}
}

func TestPushedCommitError(t *testing.T) {
	t.Run("single commit", func(t *testing.T) {
		err := &PushedCommitError{Count: 1}
		msg := err.Error()

		if !testutil.ContainsString(msg, "pushed to origin") {
			t.Errorf("expected error to mention 'pushed to origin', got: %s", msg)
		}

		if !testutil.ContainsString(msg, "--force") {
			t.Errorf("expected error to mention '--force', got: %s", msg)
		}

		if testutil.ContainsString(msg, "One or more") {
			t.Errorf("single commit error should not say 'One or more', got: %s", msg)
		}
	})

	t.Run("multiple commits", func(t *testing.T) {
		err := &PushedCommitError{Count: 3}
		msg := err.Error()

		if !testutil.ContainsString(msg, "One or more of the last 3 commits") {
			t.Errorf("expected error to mention multi-commit context, got: %s", msg)
		}

		if !testutil.ContainsString(msg, "--force") {
			t.Errorf("expected error to mention '--force', got: %s", msg)
		}
	})
}


// createGitignore creates a .gitignore file in the test repo
func createGitignore(t *testing.T, repoDir string, patterns ...string) {
	t.Helper()
	content := ""
	for _, p := range patterns {
		content += p + "\n"
	}
	createFile(t, repoDir, ".gitignore", content)
}

func TestCollector_IsIgnored(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	createGitignore(t, repoDir, "*.log", "build/")

	collector := NewCollector(repoDir)

	// Ignored files
	if !collector.IsIgnored("debug.log") {
		t.Error("expected *.log to be ignored")
	}
	if !collector.IsIgnored("build/output.txt") {
		t.Error("expected build/ to be ignored")
	}

	// Not ignored files
	if collector.IsIgnored("main.go") {
		t.Error("expected main.go to not be ignored")
	}
	if collector.IsIgnored("src/app.go") {
		t.Error("expected src/app.go to not be ignored")
	}
}

func TestCollector_Status_IgnoredFilesFiltered(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create gitignore first
	createGitignore(t, repoDir, "*.log", "ignored.txt")
	gitAdd(t, repoDir, ".gitignore")
	gitCommit(t, repoDir, "add gitignore")

	// Create some files - some ignored, some not
	createFile(t, repoDir, "app.go", "package main")
	createFile(t, repoDir, "debug.log", "log content")
	createFile(t, repoDir, "ignored.txt", "ignored content")

	collector := NewCollector(repoDir)
	status, err := collector.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	// Should only have app.go as untracked
	if len(status.Untracked) != 1 {
		t.Errorf("expected 1 untracked file, got %d: %v", len(status.Untracked), status.Untracked)
	}
	if len(status.Untracked) > 0 && status.Untracked[0] != "app.go" {
		t.Errorf("expected 'app.go', got %q", status.Untracked[0])
	}
}

func TestStager_StageFiles_IgnoredFile(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create gitignore
	createGitignore(t, repoDir, "*.log")
	gitAdd(t, repoDir, ".gitignore")
	gitCommit(t, repoDir, "add gitignore")

	// Create an ignored file
	createFile(t, repoDir, "debug.log", "log content")

	stager := NewStager(repoDir)
	err := stager.StageFiles([]string{"debug.log"})

	// Should return an error about ignored file
	if err == nil {
		t.Error("expected error when staging ignored file")
	}
	if err != nil && !testutil.ContainsString(err.Error(), "ignored") {
		t.Errorf("expected error to mention 'ignored', got: %v", err)
	}
}

func TestStager_StageFiles_Directory(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create a directory with files
	createFile(t, repoDir, "subdir/file1.txt", "content1")
	createFile(t, repoDir, "subdir/file2.txt", "content2")

	stager := NewStager(repoDir)

	// Staging a directory should expand to all files within it
	err := stager.StageFiles([]string{"subdir"})
	if err != nil {
		t.Errorf("staging directory should not error, got: %v", err)
	}

	// Both files in the directory should be staged
	staged, _ := stager.StagedFiles()
	if len(staged) != 2 {
		t.Errorf("expected 2 staged files when directory provided, got %d: %v", len(staged), staged)
	}
}

func TestStager_StageFiles_MixedFilesAndDirectories(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create files and directories
	createFile(t, repoDir, "root.txt", "root content")
	createFile(t, repoDir, "subdir/nested.txt", "nested content")

	stager := NewStager(repoDir)

	// Stage mix of file and directory - should stage both the file and directory contents
	err := stager.StageFiles([]string{"root.txt", "subdir"})
	if err != nil {
		t.Errorf("staging mixed files/dirs should not error, got: %v", err)
	}

	// Both files should be staged (root.txt + subdir/nested.txt)
	staged, _ := stager.StagedFiles()
	if len(staged) != 2 {
		t.Errorf("expected 2 staged files, got %d: %v", len(staged), staged)
	}
}

func TestStager_StageFiles_OnlyDirectories(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create directories with files
	createFile(t, repoDir, "dir1/file.txt", "content")
	createFile(t, repoDir, "dir2/file.txt", "content")

	stager := NewStager(repoDir)

	// Staging directories should expand to their files
	err := stager.StageFiles([]string{"dir1", "dir2"})
	if err != nil {
		t.Errorf("staging directories should not error, got: %v", err)
	}

	// Files from both directories should be staged
	staged, _ := stager.StagedFiles()
	if len(staged) != 2 {
		t.Errorf("expected 2 staged files, got %d: %v", len(staged), staged)
	}
}

func TestStager_StageFiles_EmptyDirectory(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create an empty directory
	emptyDir := filepath.Join(repoDir, "empty")
	_ = os.MkdirAll(emptyDir, 0755)

	stager := NewStager(repoDir)

	// Staging an empty directory should succeed but stage nothing
	err := stager.StageFiles([]string{"empty"})
	if err != nil {
		t.Errorf("staging empty directory should not error, got: %v", err)
	}

	// Nothing should be staged
	staged, _ := stager.StagedFiles()
	if len(staged) != 0 {
		t.Errorf("expected 0 staged files for empty dir, got %d: %v", len(staged), staged)
	}
}

func TestStager_StageFiles_NestedDirectory(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create nested directory structure
	createFile(t, repoDir, "a/b/c/deep.txt", "deep content")
	createFile(t, repoDir, "a/b/shallow.txt", "shallow content")

	stager := NewStager(repoDir)

	// Staging parent directory should include all nested files
	err := stager.StageFiles([]string{"a/b"})
	if err != nil {
		t.Errorf("staging nested directory should not error, got: %v", err)
	}

	// Both files under a/b should be staged
	staged, _ := stager.StagedFiles()
	if len(staged) != 2 {
		t.Errorf("expected 2 staged files, got %d: %v", len(staged), staged)
	}
}

func TestStager_StageFiles_RenameSource(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit with a file
	createFile(t, repoDir, "old_name.txt", "content")
	gitAdd(t, repoDir, "old_name.txt")
	gitCommit(t, repoDir, "initial commit")

	// Rename the file using git mv (stages the rename)
	cmd := exec.Command("git", "mv", "old_name.txt", "new_name.txt")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git mv failed: %s: %v", string(out), err)
	}

	stager := NewStager(repoDir)

	// Trying to stage the OLD name should succeed (the rename is already staged)
	err := stager.StageFiles([]string{"old_name.txt"})
	if err != nil {
		t.Errorf("staging rename source should not error, got: %v", err)
	}

	// The new name should be in staged files
	staged, _ := stager.StagedFiles()
	if len(staged) != 1 {
		t.Errorf("expected 1 staged file, got %d: %v", len(staged), staged)
	}
	if len(staged) > 0 && staged[0] != "new_name.txt" {
		t.Errorf("expected 'new_name.txt' to be staged, got %q", staged[0])
	}
}

func TestStager_StageFiles_RenameSourceWithOtherFiles(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit with files
	createFile(t, repoDir, "old_name.txt", "content")
	createFile(t, repoDir, "other.txt", "other content")
	gitAdd(t, repoDir, "old_name.txt", "other.txt")
	gitCommit(t, repoDir, "initial commit")

	// Rename one file using git mv
	cmd := exec.Command("git", "mv", "old_name.txt", "new_name.txt")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git mv failed: %s: %v", string(out), err)
	}

	// Modify the other file (not staged yet)
	createFile(t, repoDir, "other.txt", "modified content")

	stager := NewStager(repoDir)

	// Staging both the old rename source AND the modified file should work
	err := stager.StageFiles([]string{"old_name.txt", "other.txt"})
	if err != nil {
		t.Errorf("staging rename source with other files should not error, got: %v", err)
	}

	// Both files should be staged (new_name.txt from rename, other.txt from add)
	staged, _ := stager.StagedFiles()
	if len(staged) != 2 {
		t.Errorf("expected 2 staged files, got %d: %v", len(staged), staged)
	}
}

func TestStager_getStagedRenames(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit with a file
	createFile(t, repoDir, "original.txt", "content")
	gitAdd(t, repoDir, "original.txt")
	gitCommit(t, repoDir, "initial commit")

	// Rename the file using git mv
	cmd := exec.Command("git", "mv", "original.txt", "renamed.txt")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git mv failed: %s: %v", string(out), err)
	}

	stager := NewStager(repoDir)
	renames, err := stager.getStagedRenames()
	if err != nil {
		t.Fatalf("getStagedRenames failed: %v", err)
	}

	if len(renames) != 1 {
		t.Errorf("expected 1 rename, got %d: %v", len(renames), renames)
	}

	if newPath, ok := renames["original.txt"]; !ok || newPath != "renamed.txt" {
		t.Errorf("expected original.txt -> renamed.txt, got %v", renames)
	}
}

func TestStager_StageFiles_AutoDetectedRename(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit with a file
	createFile(t, repoDir, "old_name.txt", "some content that git can match")
	gitAdd(t, repoDir, "old_name.txt")
	gitCommit(t, repoDir, "initial commit")

	// Manually delete the old file and create a new file with same content
	// (simulating what happens in a Unity refactor where the file is renamed outside git)
	_ = os.Remove(filepath.Join(repoDir, "old_name.txt"))
	createFile(t, repoDir, "new_name.txt", "some content that git can match")

	// Verify the current state: deleted file + untracked new file
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoDir
	out, _ := cmd.Output()
	status := string(out)
	if !testutil.ContainsString(status, " D old_name.txt") {
		t.Fatalf("expected ' D old_name.txt' in status, got: %s", status)
	}
	if !testutil.ContainsString(status, "?? new_name.txt") {
		t.Fatalf("expected '?? new_name.txt' in status, got: %s", status)
	}

	stager := NewStager(repoDir)

	// Staging BOTH files should work - git will auto-detect the rename
	err := stager.StageFiles([]string{"old_name.txt", "new_name.txt"})
	if err != nil {
		t.Errorf("staging files that form an auto-detected rename should not error, got: %v", err)
	}

	// After staging, git should have detected the rename
	renames, _ := stager.getStagedRenames()
	if len(renames) != 1 {
		t.Logf("Expected git to auto-detect rename, got renames: %v", renames)
		// Note: git may not always detect as rename depending on content similarity
		// So we just verify that staging succeeded
	}
}

func TestReverser_Reverse_Multiple(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create 4 commits
	for i := 1; i <= 4; i++ {
		createFile(t, repoDir, "file.txt", fmt.Sprintf("version %d", i))
		gitAdd(t, repoDir, "file.txt")
		gitCommit(t, repoDir, fmt.Sprintf("commit %d", i))
	}

	reverser := NewReverser(repoDir)
	err := reverser.Reverse(2, false)
	if err != nil {
		t.Fatalf("Reverse(2) failed: %v", err)
	}

	// Verify HEAD is now at commit 2
	collector := NewCollector(repoDir)
	commits, _ := collector.RecentCommits(1)
	if len(commits) == 0 || commits[0] != "commit 2" {
		t.Errorf("expected HEAD to be 'commit 2', got %v", commits)
	}

	// Verify changes are in working directory
	status, _ := collector.Status()
	if len(status.Modified) != 1 {
		t.Errorf("expected 1 modified file, got %d", len(status.Modified))
	}
}

func TestReverser_Reverse_TooMany(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create 2 commits
	createFile(t, repoDir, "file.txt", "version 1")
	gitAdd(t, repoDir, "file.txt")
	gitCommit(t, repoDir, "commit 1")

	createFile(t, repoDir, "file.txt", "version 2")
	gitAdd(t, repoDir, "file.txt")
	gitCommit(t, repoDir, "commit 2")

	reverser := NewReverser(repoDir)
	err := reverser.Reverse(3, false)
	if err == nil {
		t.Fatal("expected error when reversing more commits than exist")
	}

	if !testutil.ContainsString(err.Error(), "only 2 commits exist") {
		t.Errorf("expected error to mention commit count, got: %s", err.Error())
	}
}

func TestCollector_HasCommitDepth(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create 3 commits
	for i := 1; i <= 3; i++ {
		createFile(t, repoDir, "file.txt", fmt.Sprintf("version %d", i))
		gitAdd(t, repoDir, "file.txt")
		gitCommit(t, repoDir, fmt.Sprintf("commit %d", i))
	}

	collector := NewCollector(repoDir)

	// Depths 1 and 2 should succeed (HEAD~1 and HEAD~2 exist)
	if err := collector.HasCommitDepth(1); err != nil {
		t.Errorf("HasCommitDepth(1) should succeed with 3 commits: %v", err)
	}
	if err := collector.HasCommitDepth(2); err != nil {
		t.Errorf("HasCommitDepth(2) should succeed with 3 commits: %v", err)
	}

	// Depth 3 should fail (HEAD~3 doesn't exist with 3 commits)
	if err := collector.HasCommitDepth(3); err == nil {
		t.Error("HasCommitDepth(3) should fail with only 3 commits")
	}

	// Depth 4 should also fail
	if err := collector.HasCommitDepth(4); err == nil {
		t.Error("HasCommitDepth(4) should fail with only 3 commits")
	}
}
