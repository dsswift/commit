package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dsswift/commit/internal/testutil"
	"github.com/dsswift/commit/pkg/types"
)

func TestFindGitRoot(t *testing.T) {
	repoDir := testutil.TestRepo(t)

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
	repoDir := testutil.TestRepo(t)

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
	repoDir := testutil.TestRepo(t)

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
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "new.txt", "content")

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
	repoDir := testutil.TestRepo(t)

	// Create initial commit
	testutil.CreateFile(t, repoDir, "file.txt", "initial")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "initial commit")

	// Modify file
	testutil.CreateFile(t, repoDir, "file.txt", "modified")

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
	repoDir := testutil.TestRepo(t)

	// Create and stage a file
	testutil.CreateFile(t, repoDir, "staged.txt", "content")
	testutil.GitAdd(t, repoDir, "staged.txt")

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
	repoDir := testutil.TestRepo(t)

	// Create initial commit
	testutil.CreateFile(t, repoDir, "file.txt", "line1\nline2\n")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "initial")

	// Modify file
	testutil.CreateFile(t, repoDir, "file.txt", "line1\nline2\nline3\n")

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
	repoDir := testutil.TestRepo(t)

	// Create several commits
	for i := 1; i <= 5; i++ {
		testutil.CreateFile(t, repoDir, "file.txt", string(rune('0'+i)))
		testutil.GitAdd(t, repoDir, "file.txt")
		testutil.GitCommit(t, repoDir, "commit "+string(rune('0'+i)))
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
	repoDir := testutil.TestRepo(t)

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
	repoDir := testutil.TestRepo(t)

	// Need at least one commit to have a branch
	testutil.CreateFile(t, repoDir, "file.txt", "content")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "initial")

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
	repoDir := testutil.TestRepo(t)

	collector := NewCollector(repoDir)

	// Empty repo - no commits yet
	// IsInitialCommit checks if HEAD~1 exists, which it won't in empty repo
	// But we can't call it on empty repo since HEAD doesn't exist

	// Create first commit
	testutil.CreateFile(t, repoDir, "file.txt", "content")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "initial")

	// Now HEAD exists but HEAD~1 doesn't
	if !collector.IsInitialCommit() {
		t.Error("expected IsInitialCommit to be true for first commit")
	}

	// Create second commit
	testutil.CreateFile(t, repoDir, "file.txt", "modified")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "second")

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
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file1.txt", "content1")
	testutil.CreateFile(t, repoDir, "file2.txt", "content2")

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
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file1.txt", "content1")
	testutil.CreateFile(t, repoDir, "file2.txt", "content2")

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
	repoDir := testutil.TestRepo(t)

	stager := NewStager(repoDir)

	has, _ := stager.HasStagedChanges()
	if has {
		t.Error("expected no staged changes initially")
	}

	testutil.CreateFile(t, repoDir, "file.txt", "content")
	_ = stager.StageFiles([]string{"file.txt"})

	has, _ = stager.HasStagedChanges()
	if !has {
		t.Error("expected staged changes after staging")
	}
}

func TestCommitter_Commit(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file.txt", "content")

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
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file.txt", "content")

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
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file.txt", "content")

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
	repoDir := testutil.TestRepo(t)

	// Create initial commit
	testutil.CreateFile(t, repoDir, "file.txt", "initial")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "initial commit")

	// Create second commit
	testutil.CreateFile(t, repoDir, "file.txt", "modified")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "second commit")

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
	repoDir := testutil.TestRepo(t)

	// Create only one commit
	testutil.CreateFile(t, repoDir, "file.txt", "content")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "initial commit")

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
	testutil.CreateFile(t, repoDir, ".gitignore", content)
}

func TestCollector_IsIgnored(t *testing.T) {
	repoDir := testutil.TestRepo(t)

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
	repoDir := testutil.TestRepo(t)

	// Create gitignore first
	createGitignore(t, repoDir, "*.log", "ignored.txt")
	testutil.GitAdd(t, repoDir, ".gitignore")
	testutil.GitCommit(t, repoDir, "add gitignore")

	// Create some files - some ignored, some not
	testutil.CreateFile(t, repoDir, "app.go", "package main")
	testutil.CreateFile(t, repoDir, "debug.log", "log content")
	testutil.CreateFile(t, repoDir, "ignored.txt", "ignored content")

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
	repoDir := testutil.TestRepo(t)

	// Create gitignore
	createGitignore(t, repoDir, "*.log")
	testutil.GitAdd(t, repoDir, ".gitignore")
	testutil.GitCommit(t, repoDir, "add gitignore")

	// Create an ignored file
	testutil.CreateFile(t, repoDir, "debug.log", "log content")

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
	repoDir := testutil.TestRepo(t)

	// Create a directory with files
	testutil.CreateFile(t, repoDir, "subdir/file1.txt", "content1")
	testutil.CreateFile(t, repoDir, "subdir/file2.txt", "content2")

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
	repoDir := testutil.TestRepo(t)

	// Create files and directories
	testutil.CreateFile(t, repoDir, "root.txt", "root content")
	testutil.CreateFile(t, repoDir, "subdir/nested.txt", "nested content")

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
	repoDir := testutil.TestRepo(t)

	// Create directories with files
	testutil.CreateFile(t, repoDir, "dir1/file.txt", "content")
	testutil.CreateFile(t, repoDir, "dir2/file.txt", "content")

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
	repoDir := testutil.TestRepo(t)

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
	repoDir := testutil.TestRepo(t)

	// Create nested directory structure
	testutil.CreateFile(t, repoDir, "a/b/c/deep.txt", "deep content")
	testutil.CreateFile(t, repoDir, "a/b/shallow.txt", "shallow content")

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
	repoDir := testutil.TestRepo(t)

	// Create initial commit with a file
	testutil.CreateFile(t, repoDir, "old_name.txt", "content")
	testutil.GitAdd(t, repoDir, "old_name.txt")
	testutil.GitCommit(t, repoDir, "initial commit")

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
	repoDir := testutil.TestRepo(t)

	// Create initial commit with files
	testutil.CreateFile(t, repoDir, "old_name.txt", "content")
	testutil.CreateFile(t, repoDir, "other.txt", "other content")
	testutil.GitAdd(t, repoDir, "old_name.txt", "other.txt")
	testutil.GitCommit(t, repoDir, "initial commit")

	// Rename one file using git mv
	cmd := exec.Command("git", "mv", "old_name.txt", "new_name.txt")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git mv failed: %s: %v", string(out), err)
	}

	// Modify the other file (not staged yet)
	testutil.CreateFile(t, repoDir, "other.txt", "modified content")

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
	repoDir := testutil.TestRepo(t)

	// Create initial commit with a file
	testutil.CreateFile(t, repoDir, "original.txt", "content")
	testutil.GitAdd(t, repoDir, "original.txt")
	testutil.GitCommit(t, repoDir, "initial commit")

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
	repoDir := testutil.TestRepo(t)

	// Create initial commit with a file
	testutil.CreateFile(t, repoDir, "old_name.txt", "some content that git can match")
	testutil.GitAdd(t, repoDir, "old_name.txt")
	testutil.GitCommit(t, repoDir, "initial commit")

	// Manually delete the old file and create a new file with same content
	// (simulating what happens in a Unity refactor where the file is renamed outside git)
	_ = os.Remove(filepath.Join(repoDir, "old_name.txt"))
	testutil.CreateFile(t, repoDir, "new_name.txt", "some content that git can match")

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
	repoDir := testutil.TestRepo(t)

	// Create 4 commits
	for i := 1; i <= 4; i++ {
		testutil.CreateFile(t, repoDir, "file.txt", fmt.Sprintf("version %d", i))
		testutil.GitAdd(t, repoDir, "file.txt")
		testutil.GitCommit(t, repoDir, fmt.Sprintf("commit %d", i))
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
	repoDir := testutil.TestRepo(t)

	// Create 2 commits
	testutil.CreateFile(t, repoDir, "file.txt", "version 1")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "commit 1")

	testutil.CreateFile(t, repoDir, "file.txt", "version 2")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "commit 2")

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
	repoDir := testutil.TestRepo(t)

	// Create 3 commits
	for i := 1; i <= 3; i++ {
		testutil.CreateFile(t, repoDir, "file.txt", fmt.Sprintf("version %d", i))
		testutil.GitAdd(t, repoDir, "file.txt")
		testutil.GitCommit(t, repoDir, fmt.Sprintf("commit %d", i))
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

func TestCommitter_VerifyCommit(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	// Create an initial commit so diff-tree has a parent to compare against
	testutil.CreateFile(t, repoDir, "init.txt", "init")
	testutil.GitAdd(t, repoDir, "init.txt")
	testutil.GitCommit(t, repoDir, "initial commit")

	// Create a file, stage it, and commit
	testutil.CreateFile(t, repoDir, "file.go", "package main")
	testutil.GitAdd(t, repoDir, "file.go")
	hash := testutil.GitCommit(t, repoDir, "add file")

	committer := NewCommitter(repoDir)

	// Verify with correct hash and file should succeed
	err := committer.VerifyCommit(hash, []string{"file.go"})
	if err != nil {
		t.Fatalf("VerifyCommit should succeed with correct hash and file: %v", err)
	}

	// Verify with wrong hash should fail with mismatch
	err = committer.VerifyCommit("badhash", []string{"file.go"})
	if err == nil {
		t.Fatal("VerifyCommit should fail with wrong hash")
	}
	if !testutil.ContainsString(err.Error(), "mismatch") {
		t.Errorf("expected error to mention 'mismatch', got: %s", err.Error())
	}

	// Verify with wrong file should fail with not in commit
	err = committer.VerifyCommit(hash, []string{"nonexistent.go"})
	if err == nil {
		t.Fatal("VerifyCommit should fail with wrong file")
	}
	if !testutil.ContainsString(err.Error(), "not in commit") {
		t.Errorf("expected error to mention 'not in commit', got: %s", err.Error())
	}
}

func TestCommitter_ExecutePlannedCommit(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	// Create an initial commit so unstage works
	testutil.CreateFile(t, repoDir, "init.txt", "init")
	testutil.GitAdd(t, repoDir, "init.txt")
	testutil.GitCommit(t, repoDir, "initial commit")

	// Create a file for the planned commit
	testutil.CreateFile(t, repoDir, "main.go", "package main")

	committer := NewCommitter(repoDir)
	result, err := committer.ExecutePlannedCommit(types.PlannedCommit{
		Type:    "feat",
		Message: "add main",
		Files:   []string{"main.go"},
	})
	if err != nil {
		t.Fatalf("ExecutePlannedCommit failed: %v", err)
	}

	if result.Hash == "" {
		t.Error("expected non-empty commit hash")
	}

	if result.Message != "feat: add main" {
		t.Errorf("expected message 'feat: add main', got %q", result.Message)
	}

	foundMainGo := false
	for _, f := range result.Files {
		if f == "main.go" {
			foundMainGo = true
			break
		}
	}
	if !foundMainGo {
		t.Errorf("expected result.Files to contain 'main.go', got %v", result.Files)
	}
}

func TestCommitter_ExecutePlannedCommit_WithScope(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	// Create an initial commit so unstage works
	testutil.CreateFile(t, repoDir, "init.txt", "init")
	testutil.GitAdd(t, repoDir, "init.txt")
	testutil.GitCommit(t, repoDir, "initial commit")

	// Create a file for the planned commit
	testutil.CreateFile(t, repoDir, "endpoint.go", "package api")

	scope := "api"
	committer := NewCommitter(repoDir)
	result, err := committer.ExecutePlannedCommit(types.PlannedCommit{
		Type:    "feat",
		Scope:   &scope,
		Message: "add endpoint",
		Files:   []string{"endpoint.go"},
	})
	if err != nil {
		t.Fatalf("ExecutePlannedCommit with scope failed: %v", err)
	}

	if result.Message != "feat(api): add endpoint" {
		t.Errorf("expected message 'feat(api): add endpoint', got %q", result.Message)
	}
}

func TestNoStagedFilesError(t *testing.T) {
	err := &NoStagedFilesError{PlannedFiles: []string{"a.go", "b.go"}}
	msg := err.Error()

	if !testutil.ContainsString(msg, "2 paths") {
		t.Errorf("expected error to contain '2 paths', got: %s", msg)
	}

	if !testutil.ContainsString(msg, "directories") {
		t.Errorf("expected error to contain 'directories', got: %s", msg)
	}
}

func TestStager_UnstageFiles(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	// Create initial commit so reset HEAD works
	testutil.CreateFile(t, repoDir, "file.txt", "initial")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "initial commit")

	// Modify and stage the file
	testutil.CreateFile(t, repoDir, "file.txt", "modified content")
	testutil.GitAdd(t, repoDir, "file.txt")

	stager := NewStager(repoDir)

	// Verify file is staged
	staged, err := stager.StagedFiles()
	if err != nil {
		t.Fatalf("StagedFiles failed: %v", err)
	}
	if len(staged) != 1 || staged[0] != "file.txt" {
		t.Fatalf("expected file.txt to be staged, got %v", staged)
	}

	// Unstage the file
	err = stager.UnstageFiles([]string{"file.txt"})
	if err != nil {
		t.Fatalf("UnstageFiles failed: %v", err)
	}

	// Verify file is no longer staged
	staged, err = stager.StagedFiles()
	if err != nil {
		t.Fatalf("StagedFiles after unstage failed: %v", err)
	}
	if len(staged) != 0 {
		t.Errorf("expected no staged files after unstage, got %v", staged)
	}
}

func TestStager_StageAll(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	// Create initial commit so diff --cached works properly
	testutil.CreateFile(t, repoDir, "init.txt", "init")
	testutil.GitAdd(t, repoDir, "init.txt")
	testutil.GitCommit(t, repoDir, "initial commit")

	// Create 3 new files
	testutil.CreateFile(t, repoDir, "a.go", "package a")
	testutil.CreateFile(t, repoDir, "b.go", "package b")
	testutil.CreateFile(t, repoDir, "c.go", "package c")

	stager := NewStager(repoDir)
	err := stager.StageAll()
	if err != nil {
		t.Fatalf("StageAll failed: %v", err)
	}

	// Verify all 3 files are staged
	staged, err := stager.StagedFiles()
	if err != nil {
		t.Fatalf("StagedFiles failed: %v", err)
	}

	if len(staged) != 3 {
		t.Errorf("expected 3 staged files, got %d: %v", len(staged), staged)
	}

	// Verify each file is present
	stagedSet := make(map[string]bool)
	for _, f := range staged {
		stagedSet[f] = true
	}
	for _, expected := range []string{"a.go", "b.go", "c.go"} {
		if !stagedSet[expected] {
			t.Errorf("expected %q to be staged, staged files: %v", expected, staged)
		}
	}
}

func TestStager_StageAll_ErrorPath(t *testing.T) {
	// Point stager at a directory that is not a git repo to force git add -A failure
	tmpDir := t.TempDir()

	stager := NewStager(tmpDir)
	err := stager.StageAll()
	if err == nil {
		t.Fatal("expected error when running StageAll in non-git directory")
	}
	if !strings.Contains(err.Error(), "failed to stage all files") {
		t.Errorf("expected error to mention 'failed to stage all files', got: %v", err)
	}
}

func TestStager_isIgnored(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	// Create .gitignore that ignores *.log files and the build/ directory
	testutil.CreateFile(t, repoDir, ".gitignore", "*.log\nbuild/\n")
	testutil.GitAdd(t, repoDir, ".gitignore")
	testutil.GitCommit(t, repoDir, "add gitignore")

	// Create ignored files
	testutil.CreateFile(t, repoDir, "debug.log", "log content")
	testutil.CreateFile(t, repoDir, "build/output.bin", "binary")

	// Create a non-ignored file
	testutil.CreateFile(t, repoDir, "main.go", "package main")

	stager := NewStager(repoDir)

	t.Run("ignored file by extension", func(t *testing.T) {
		if !stager.isIgnored("debug.log") {
			t.Error("expected debug.log to be ignored")
		}
	})

	t.Run("ignored file in directory", func(t *testing.T) {
		if !stager.isIgnored("build/output.bin") {
			t.Error("expected build/output.bin to be ignored")
		}
	})

	t.Run("non-ignored file", func(t *testing.T) {
		if stager.isIgnored("main.go") {
			t.Error("expected main.go to not be ignored")
		}
	})

	t.Run("nonexistent non-ignored file", func(t *testing.T) {
		if stager.isIgnored("doesnotexist.go") {
			t.Error("expected nonexistent non-ignored file to return false")
		}
	})
}

func TestStager_diagnoseFile(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	// Create initial commit with a tracked file
	testutil.CreateFile(t, repoDir, "tracked.txt", "content")
	testutil.GitAdd(t, repoDir, "tracked.txt")
	testutil.GitCommit(t, repoDir, "initial commit")

	stager := NewStager(repoDir)

	t.Run("existing untracked file", func(t *testing.T) {
		testutil.CreateFile(t, repoDir, "untracked.txt", "new content")

		result := stager.diagnoseFile("untracked.txt")
		if result == "" {
			t.Fatal("expected non-empty diagnosis")
		}
		if !strings.Contains(result, "file exists on disk") {
			t.Errorf("expected diagnosis to mention file exists, got: %s", result)
		}
		if !strings.Contains(result, "NOT tracked") {
			t.Errorf("expected diagnosis to mention not tracked, got: %s", result)
		}
	})

	t.Run("tracked file with no changes", func(t *testing.T) {
		result := stager.diagnoseFile("tracked.txt")
		if result == "" {
			t.Fatal("expected non-empty diagnosis")
		}
		if !strings.Contains(result, "file exists on disk") {
			t.Errorf("expected diagnosis to mention file exists, got: %s", result)
		}
		if !strings.Contains(result, "file is tracked by git") {
			t.Errorf("expected diagnosis to mention tracked, got: %s", result)
		}
		if !strings.Contains(result, "no changes detected") {
			t.Errorf("expected diagnosis to mention no changes, got: %s", result)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		result := stager.diagnoseFile("ghost.txt")
		if result == "" {
			t.Fatal("expected non-empty diagnosis")
		}
		if !strings.Contains(result, "does not exist on disk") {
			t.Errorf("expected diagnosis to mention file missing, got: %s", result)
		}
	})

	t.Run("file involved in rename", func(t *testing.T) {
		// Stage a rename so diagnoseFile can detect it
		cmd := exec.Command("git", "mv", "tracked.txt", "renamed.txt")
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git mv failed: %s: %v", string(out), err)
		}

		result := stager.diagnoseFile("tracked.txt")
		if result == "" {
			t.Fatal("expected non-empty diagnosis")
		}
		if !strings.Contains(result, "SOURCE of a staged rename") {
			t.Errorf("expected diagnosis to mention rename source, got: %s", result)
		}
		if !strings.Contains(result, "renamed.txt") {
			t.Errorf("expected diagnosis to mention rename destination, got: %s", result)
		}

		// Also test the destination side
		resultDest := stager.diagnoseFile("renamed.txt")
		if !strings.Contains(resultDest, "DESTINATION of a staged rename") {
			t.Errorf("expected diagnosis to mention rename destination, got: %s", resultDest)
		}
	})
}

func TestStager_expandDirectory(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	// Create .gitignore to test filtering
	testutil.CreateFile(t, repoDir, ".gitignore", "*.log\n")
	testutil.GitAdd(t, repoDir, ".gitignore")
	testutil.GitCommit(t, repoDir, "add gitignore")

	stager := NewStager(repoDir)

	t.Run("directory with mixed files", func(t *testing.T) {
		// Create a directory with both tracked-eligible and ignored files
		testutil.CreateFile(t, repoDir, "src/app.go", "package main")
		testutil.CreateFile(t, repoDir, "src/util.go", "package util")
		testutil.CreateFile(t, repoDir, "src/debug.log", "log content") // ignored

		files, err := stager.expandDirectory("src")
		if err != nil {
			t.Fatalf("expandDirectory failed: %v", err)
		}

		// Should include the .go files but not the .log file
		if len(files) != 2 {
			t.Errorf("expected 2 non-ignored files, got %d: %v", len(files), files)
		}

		fileSet := make(map[string]bool)
		for _, f := range files {
			fileSet[f] = true
		}
		if !fileSet["src/app.go"] {
			t.Error("expected src/app.go in expanded files")
		}
		if !fileSet["src/util.go"] {
			t.Error("expected src/util.go in expanded files")
		}
		if fileSet["src/debug.log"] {
			t.Error("did not expect src/debug.log (ignored) in expanded files")
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		emptyDir := filepath.Join(repoDir, "emptydir")
		if err := os.MkdirAll(emptyDir, 0755); err != nil {
			t.Fatalf("failed to create empty dir: %v", err)
		}

		files, err := stager.expandDirectory("emptydir")
		if err != nil {
			t.Fatalf("expandDirectory failed: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 files for empty dir, got %d: %v", len(files), files)
		}
	})

	t.Run("nested directory", func(t *testing.T) {
		testutil.CreateFile(t, repoDir, "deep/a/b/file.txt", "nested")

		files, err := stager.expandDirectory("deep")
		if err != nil {
			t.Fatalf("expandDirectory failed: %v", err)
		}
		if len(files) != 1 {
			t.Errorf("expected 1 file, got %d: %v", len(files), files)
		}
		if len(files) > 0 && files[0] != "deep/a/b/file.txt" {
			t.Errorf("expected deep/a/b/file.txt, got %s", files[0])
		}
	})

	t.Run("directory with only ignored files", func(t *testing.T) {
		testutil.CreateFile(t, repoDir, "logs/server.log", "log1")
		testutil.CreateFile(t, repoDir, "logs/error.log", "log2")

		files, err := stager.expandDirectory("logs")
		if err != nil {
			t.Fatalf("expandDirectory failed: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 files (all ignored), got %d: %v", len(files), files)
		}
	})
}
