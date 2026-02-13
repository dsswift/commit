package interactive

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// testRepo creates a temporary git repository for testing.
func testRepo(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "interactive-test-*")
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

	// Configure git user
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
func gitCommit(t *testing.T, repoDir, message string) string {
	t.Helper()
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %s: %v", string(out), err)
	}

	// Get the commit hash
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get commit hash: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// getCommitMessages returns all commit messages in order (oldest first)
func getCommitMessages(t *testing.T, repoDir string) []string {
	t.Helper()
	cmd := exec.Command("git", "log", "--reverse", "--format=%s")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get commit messages: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

func TestRebaser_Execute_PickAll(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create base commit
	createFile(t, repoDir, "file.txt", "initial")
	gitAdd(t, repoDir, "file.txt")
	baseHash := gitCommit(t, repoDir, "initial commit")

	// Create commits to rebase
	createFile(t, repoDir, "file.txt", "content1")
	gitAdd(t, repoDir, "file.txt")
	gitCommit(t, repoDir, "commit 1")

	createFile(t, repoDir, "file.txt", "content2")
	gitAdd(t, repoDir, "file.txt")
	gitCommit(t, repoDir, "commit 2")

	createFile(t, repoDir, "file.txt", "content3")
	gitAdd(t, repoDir, "file.txt")
	gitCommit(t, repoDir, "commit 3")

	// Create entries with all pick operations
	entries := []RebaseEntry{
		{Commit: RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD~2"), Message: "commit 1"}, Operation: OpPick},
		{Commit: RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD~1"), Message: "commit 2"}, Operation: OpPick},
		{Commit: RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD"), Message: "commit 3"}, Operation: OpPick},
	}

	rebaser := NewRebaser(repoDir)
	err := rebaser.Execute(entries, baseHash)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify all commits still exist with same messages
	messages := getCommitMessages(t, repoDir)
	expected := []string{"initial commit", "commit 1", "commit 2", "commit 3"}
	if len(messages) != len(expected) {
		t.Fatalf("expected %d commits, got %d: %v", len(expected), len(messages), messages)
	}
	for i, msg := range messages {
		if msg != expected[i] {
			t.Errorf("commit %d: got %q, want %q", i, msg, expected[i])
		}
	}
}

func TestRebaser_Execute_Reorder(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create base commit
	createFile(t, repoDir, "base.txt", "base")
	gitAdd(t, repoDir, "base.txt")
	baseHash := gitCommit(t, repoDir, "base")

	// Create commits A, B, C (each modifies a different file to avoid conflicts)
	createFile(t, repoDir, "a.txt", "A")
	gitAdd(t, repoDir, "a.txt")
	hashA := gitCommit(t, repoDir, "A")

	createFile(t, repoDir, "b.txt", "B")
	gitAdd(t, repoDir, "b.txt")
	hashB := gitCommit(t, repoDir, "B")

	createFile(t, repoDir, "c.txt", "C")
	gitAdd(t, repoDir, "c.txt")
	gitCommit(t, repoDir, "C")

	// Reorder to C, A, B
	entries := []RebaseEntry{
		{Commit: RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD"), Message: "C"}, Operation: OpPick},
		{Commit: RebaseCommit{ShortHash: hashA[:7], Message: "A"}, Operation: OpPick},
		{Commit: RebaseCommit{ShortHash: hashB[:7], Message: "B"}, Operation: OpPick},
	}

	rebaser := NewRebaser(repoDir)
	err := rebaser.Execute(entries, baseHash)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify new order
	messages := getCommitMessages(t, repoDir)
	expected := []string{"base", "C", "A", "B"}
	if len(messages) != len(expected) {
		t.Fatalf("expected %d commits, got %d: %v", len(expected), len(messages), messages)
	}
	for i, msg := range messages {
		if msg != expected[i] {
			t.Errorf("commit %d: got %q, want %q", i, msg, expected[i])
		}
	}
}

func TestRebaser_Execute_Drop(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create base commit
	createFile(t, repoDir, "base.txt", "base")
	gitAdd(t, repoDir, "base.txt")
	baseHash := gitCommit(t, repoDir, "base")

	// Create commits
	createFile(t, repoDir, "keep.txt", "keep")
	gitAdd(t, repoDir, "keep.txt")
	gitCommit(t, repoDir, "keep this")

	createFile(t, repoDir, "drop.txt", "drop")
	gitAdd(t, repoDir, "drop.txt")
	gitCommit(t, repoDir, "drop this")

	createFile(t, repoDir, "also_keep.txt", "also keep")
	gitAdd(t, repoDir, "also_keep.txt")
	gitCommit(t, repoDir, "also keep")

	// Drop the middle commit
	entries := []RebaseEntry{
		{Commit: RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD~2"), Message: "keep this"}, Operation: OpPick},
		{Commit: RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD~1"), Message: "drop this"}, Operation: OpDrop},
		{Commit: RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD"), Message: "also keep"}, Operation: OpPick},
	}

	rebaser := NewRebaser(repoDir)
	err := rebaser.Execute(entries, baseHash)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify dropped commit is gone
	messages := getCommitMessages(t, repoDir)
	expected := []string{"base", "keep this", "also keep"}
	if len(messages) != len(expected) {
		t.Fatalf("expected %d commits, got %d: %v", len(expected), len(messages), messages)
	}

	// Verify dropped file doesn't exist
	if _, err := os.Stat(filepath.Join(repoDir, "drop.txt")); !os.IsNotExist(err) {
		t.Error("dropped file should not exist")
	}
}

func TestRebaser_Execute_Squash(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create base commit
	createFile(t, repoDir, "base.txt", "base")
	gitAdd(t, repoDir, "base.txt")
	baseHash := gitCommit(t, repoDir, "base")

	// Create commits to squash
	createFile(t, repoDir, "feature.txt", "v1")
	gitAdd(t, repoDir, "feature.txt")
	gitCommit(t, repoDir, "feat: add feature")

	createFile(t, repoDir, "feature.txt", "v2")
	gitAdd(t, repoDir, "feature.txt")
	gitCommit(t, repoDir, "fix: feature bug")

	// Squash second into first with edited message
	entries := []RebaseEntry{
		{
			Commit:        RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD~1"), Message: "feat: add feature"},
			Operation:     OpPick,
			NewMessage:    "feat: add feature with fix",
			MessageEdited: true,
		},
		{
			Commit:    RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD"), Message: "fix: feature bug"},
			Operation: OpSquash,
		},
	}

	rebaser := NewRebaser(repoDir)
	err := rebaser.Execute(entries, baseHash)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify squash result - should have 2 commits now
	messages := getCommitMessages(t, repoDir)
	if len(messages) != 2 {
		t.Fatalf("expected 2 commits, got %d: %v", len(messages), messages)
	}

	// Content should be from final state
	content, err := os.ReadFile(filepath.Join(repoDir, "feature.txt"))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "v2" {
		t.Errorf("content = %q, want %q", string(content), "v2")
	}
}

func TestRebaser_Execute_Reword(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create base commit
	createFile(t, repoDir, "base.txt", "base")
	gitAdd(t, repoDir, "base.txt")
	baseHash := gitCommit(t, repoDir, "base")

	// Create commit to reword
	createFile(t, repoDir, "feature.txt", "content")
	gitAdd(t, repoDir, "feature.txt")
	gitCommit(t, repoDir, "old message")

	// Reword with new message
	entries := []RebaseEntry{
		{
			Commit:        RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD"), Message: "old message"},
			Operation:     OpReword,
			NewMessage:    "new message",
			MessageEdited: true,
		},
	}

	rebaser := NewRebaser(repoDir)
	err := rebaser.Execute(entries, baseHash)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify message changed
	messages := getCommitMessages(t, repoDir)
	if len(messages) != 2 {
		t.Fatalf("expected 2 commits, got %d: %v", len(messages), messages)
	}
	if messages[1] != "new message" {
		t.Errorf("message = %q, want %q", messages[1], "new message")
	}

	// Verify content unchanged
	content, err := os.ReadFile(filepath.Join(repoDir, "feature.txt"))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "content" {
		t.Errorf("content = %q, want %q", string(content), "content")
	}
}

func TestRebaser_Execute_EmptyEntries(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	rebaser := NewRebaser(repoDir)
	err := rebaser.Execute([]RebaseEntry{}, "abc123")

	if err == nil {
		t.Error("expected error for empty entries")
	}
	if !strings.Contains(err.Error(), "no commits") {
		t.Errorf("error = %v, want error about no commits", err)
	}
}

func getShortHash(t *testing.T, repoDir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--short", ref)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get short hash for %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

func TestPushedCommitError(t *testing.T) {
	err := &PushedCommitError{}
	msg := err.Error()

	if !strings.Contains(msg, "pushed") {
		t.Errorf("error should mention 'pushed', got: %s", msg)
	}
	if !strings.Contains(msg, "force") {
		t.Errorf("error should mention 'force', got: %s", msg)
	}
}

func TestRebaser_Execute_RootCommit(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, repoDir, "file.txt", "initial")
	gitAdd(t, repoDir, "file.txt")
	gitCommit(t, repoDir, "initial commit")

	// Create second commit to reword
	createFile(t, repoDir, "file.txt", "updated")
	gitAdd(t, repoDir, "file.txt")
	gitCommit(t, repoDir, "second commit")

	// Rebase both commits with --root (empty baseCommit)
	entries := []RebaseEntry{
		{Commit: RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD~1"), Message: "initial commit"}, Operation: OpPick},
		{Commit: RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD"), Message: "second commit"}, Operation: OpPick},
	}

	rebaser := NewRebaser(repoDir)
	err := rebaser.Execute(entries, "") // empty baseCommit triggers --root

	if err != nil {
		t.Fatalf("Execute with root failed: %v", err)
	}

	// Verify both commits still exist
	messages := getCommitMessages(t, repoDir)
	if len(messages) != 2 {
		t.Fatalf("expected 2 commits, got %d: %v", len(messages), messages)
	}
}

func TestRebaser_Execute_MultipleRewords(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create base commit
	createFile(t, repoDir, "base.txt", "base")
	gitAdd(t, repoDir, "base.txt")
	baseHash := gitCommit(t, repoDir, "base")

	// Create multiple commits to reword
	createFile(t, repoDir, "a.txt", "a")
	gitAdd(t, repoDir, "a.txt")
	gitCommit(t, repoDir, "old message A")

	createFile(t, repoDir, "b.txt", "b")
	gitAdd(t, repoDir, "b.txt")
	gitCommit(t, repoDir, "old message B")

	createFile(t, repoDir, "c.txt", "c")
	gitAdd(t, repoDir, "c.txt")
	gitCommit(t, repoDir, "old message C")

	// Reword all commits with new messages
	entries := []RebaseEntry{
		{
			Commit:        RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD~2"), Message: "old message A"},
			Operation:     OpReword,
			NewMessage:    "new message A",
			MessageEdited: true,
		},
		{
			Commit:        RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD~1"), Message: "old message B"},
			Operation:     OpReword,
			NewMessage:    "new message B",
			MessageEdited: true,
		},
		{
			Commit:        RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD"), Message: "old message C"},
			Operation:     OpReword,
			NewMessage:    "new message C",
			MessageEdited: true,
		},
	}

	rebaser := NewRebaser(repoDir)
	err := rebaser.Execute(entries, baseHash)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify all messages changed correctly
	messages := getCommitMessages(t, repoDir)
	expected := []string{"base", "new message A", "new message B", "new message C"}
	if len(messages) != len(expected) {
		t.Fatalf("expected %d commits, got %d: %v", len(expected), len(messages), messages)
	}
	for i, msg := range messages {
		if msg != expected[i] {
			t.Errorf("commit %d: got %q, want %q", i, msg, expected[i])
		}
	}
}

func TestRebaser_Execute_MixedOperationsWithReword(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create base commit
	createFile(t, repoDir, "base.txt", "base")
	gitAdd(t, repoDir, "base.txt")
	baseHash := gitCommit(t, repoDir, "base")

	// Create commits with mixed operations
	createFile(t, repoDir, "a.txt", "a")
	gitAdd(t, repoDir, "a.txt")
	gitCommit(t, repoDir, "pick this")

	createFile(t, repoDir, "b.txt", "b")
	gitAdd(t, repoDir, "b.txt")
	gitCommit(t, repoDir, "reword this")

	createFile(t, repoDir, "c.txt", "c")
	gitAdd(t, repoDir, "c.txt")
	gitCommit(t, repoDir, "also reword")

	createFile(t, repoDir, "d.txt", "d")
	gitAdd(t, repoDir, "d.txt")
	gitCommit(t, repoDir, "pick this too")

	// Mix of pick and reword operations
	entries := []RebaseEntry{
		{
			Commit:    RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD~3"), Message: "pick this"},
			Operation: OpPick,
		},
		{
			Commit:        RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD~2"), Message: "reword this"},
			Operation:     OpReword,
			NewMessage:    "reworded first",
			MessageEdited: true,
		},
		{
			Commit:        RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD~1"), Message: "also reword"},
			Operation:     OpReword,
			NewMessage:    "reworded second",
			MessageEdited: true,
		},
		{
			Commit:    RebaseCommit{ShortHash: getShortHash(t, repoDir, "HEAD"), Message: "pick this too"},
			Operation: OpPick,
		},
	}

	rebaser := NewRebaser(repoDir)
	err := rebaser.Execute(entries, baseHash)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify messages
	messages := getCommitMessages(t, repoDir)
	expected := []string{"base", "pick this", "reworded first", "reworded second", "pick this too"}
	if len(messages) != len(expected) {
		t.Fatalf("expected %d commits, got %d: %v", len(expected), len(messages), messages)
	}
	for i, msg := range messages {
		if msg != expected[i] {
			t.Errorf("commit %d: got %q, want %q", i, msg, expected[i])
		}
	}
}
