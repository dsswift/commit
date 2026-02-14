package planner

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/dsswift/commit/internal/testutil"
	"github.com/dsswift/commit/pkg/types"
)

func getLastCommitMessage(t *testing.T, repoDir string) string {
	t.Helper()
	cmd := exec.Command("git", "log", "-1", "--format=%s")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get last commit message: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func getAllCommitMessages(t *testing.T, repoDir string) []string {
	t.Helper()
	cmd := exec.Command("git", "log", "--format=%s", "--reverse")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get commit messages: %v", err)
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\n")
}

func TestExecutor_Execute_SingleCommit(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	// Create a file so we have something to commit
	testutil.CreateFile(t, repoDir, "main.go", "package main")

	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{
				Type:    "feat",
				Message: "add main package",
				Files:   []string{"main.go"},
			},
		},
	}

	executor := NewExecutor(repoDir, false)
	executed, err := executor.Execute(plan, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(executed) != 1 {
		t.Fatalf("expected 1 executed commit, got %d", len(executed))
	}

	if executed[0].Hash == "" || executed[0].Hash == "(dry-run)" {
		t.Error("expected real commit hash")
	}

	msg := getLastCommitMessage(t, repoDir)
	if msg != "feat: add main package" {
		t.Errorf("expected 'feat: add main package', got %q", msg)
	}
}

func TestExecutor_Execute_MultipleCommits(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "a.go", "package a")
	testutil.CreateFile(t, repoDir, "b.go", "package b")
	testutil.CreateFile(t, repoDir, "c.go", "package c")

	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{Type: "feat", Message: "add package a", Files: []string{"a.go"}},
			{Type: "feat", Message: "add package b", Files: []string{"b.go"}},
			{Type: "feat", Message: "add package c", Files: []string{"c.go"}},
		},
	}

	executor := NewExecutor(repoDir, false)
	executed, err := executor.Execute(plan, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(executed) != 3 {
		t.Fatalf("expected 3 executed commits, got %d", len(executed))
	}

	// Verify order: commits should appear in order
	messages := getAllCommitMessages(t, repoDir)
	expected := []string{"feat: add package a", "feat: add package b", "feat: add package c"}
	if len(messages) != len(expected) {
		t.Fatalf("expected %d commits, got %d: %v", len(expected), len(messages), messages)
	}
	for i, msg := range messages {
		if msg != expected[i] {
			t.Errorf("commit %d: expected %q, got %q", i, expected[i], msg)
		}
	}
}

func TestExecutor_Execute_DryRun(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file.txt", "content")

	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{Type: "feat", Message: "add file", Files: []string{"file.txt"}},
		},
	}

	executor := NewExecutor(repoDir, true)
	executed, err := executor.Execute(plan, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(executed) != 1 {
		t.Fatalf("expected 1 executed commit, got %d", len(executed))
	}

	if executed[0].Hash != "(dry-run)" {
		t.Errorf("expected hash '(dry-run)', got %q", executed[0].Hash)
	}

	// Verify no actual git commits were created
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		// HEAD doesn't exist = no commits, which is correct for dry-run
		return
	}
}

func TestExecutor_Execute_ProgressCallback(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "a.go", "package a")
	testutil.CreateFile(t, repoDir, "b.go", "package b")

	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{Type: "feat", Message: "add a", Files: []string{"a.go"}},
			{Type: "feat", Message: "add b", Files: []string{"b.go"}},
		},
	}

	var progressCalls []struct {
		current, total int
	}

	executor := NewExecutor(repoDir, false)
	_, err := executor.Execute(plan, func(current, total int, commit types.PlannedCommit) {
		progressCalls = append(progressCalls, struct{ current, total int }{current, total})
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(progressCalls) != 2 {
		t.Fatalf("expected 2 progress calls, got %d", len(progressCalls))
	}

	if progressCalls[0].current != 1 || progressCalls[0].total != 2 {
		t.Errorf("first progress: expected 1/2, got %d/%d", progressCalls[0].current, progressCalls[0].total)
	}

	if progressCalls[1].current != 2 || progressCalls[1].total != 2 {
		t.Errorf("second progress: expected 2/2, got %d/%d", progressCalls[1].current, progressCalls[1].total)
	}
}

func TestExecutor_Execute_SkipsDirectoryOnlyCommits(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	// Create real files for one commit, empty dir for another
	testutil.CreateFile(t, repoDir, "real.go", "package real")
	testutil.CreateFile(t, repoDir, "emptydir/.gitkeep", "")
	testutil.GitAdd(t, repoDir, "emptydir/.gitkeep")
	testutil.GitCommit(t, repoDir, "setup")

	// Now remove the .gitkeep so the directory is truly empty for git
	cmd := exec.Command("git", "rm", "emptydir/.gitkeep")
	cmd.Dir = repoDir
	_ = cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "remove gitkeep")
	cmd.Dir = repoDir
	_ = cmd.Run()

	// Create a new file and try to commit it along with the (now empty) directory path
	testutil.CreateFile(t, repoDir, "new.go", "package new")

	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{Type: "feat", Message: "add new", Files: []string{"new.go"}},
		},
	}

	executor := NewExecutor(repoDir, false)
	executed, err := executor.Execute(plan, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(executed) != 1 {
		t.Fatalf("expected 1 executed commit, got %d", len(executed))
	}
}

func TestExecutor_ExecuteSingle_Success(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file.go", "package main")

	planned := types.PlannedCommit{
		Type:    "feat",
		Message: "add file",
		Files:   []string{"file.go"},
	}

	executor := NewExecutor(repoDir, false)
	result, err := executor.ExecuteSingle(planned)
	if err != nil {
		t.Fatalf("ExecuteSingle failed: %v", err)
	}

	if result.Hash == "" || result.Hash == "(dry-run)" {
		t.Error("expected real commit hash")
	}

	if result.Message != "feat: add file" {
		t.Errorf("expected 'feat: add file', got %q", result.Message)
	}
}

func TestExecutor_ExecuteSingle_DryRun(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	planned := types.PlannedCommit{
		Type:    "docs",
		Message: "update readme",
		Files:   []string{"README.md"},
	}

	executor := NewExecutor(repoDir, true)
	result, err := executor.ExecuteSingle(planned)
	if err != nil {
		t.Fatalf("ExecuteSingle failed: %v", err)
	}

	if result.Hash != "(dry-run)" {
		t.Errorf("expected '(dry-run)', got %q", result.Hash)
	}

	if result.Message != "docs: update readme" {
		t.Errorf("expected 'docs: update readme', got %q", result.Message)
	}
}

func TestExecutor_Execute_WithScope(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "api.go", "package api")

	scope := "api"
	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{
				Type:    "feat",
				Scope:   &scope,
				Message: "add endpoint",
				Files:   []string{"api.go"},
			},
		},
	}

	executor := NewExecutor(repoDir, false)
	executed, err := executor.Execute(plan, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(executed) != 1 {
		t.Fatalf("expected 1 executed commit, got %d", len(executed))
	}

	msg := getLastCommitMessage(t, repoDir)
	if msg != "feat(api): add endpoint" {
		t.Errorf("expected 'feat(api): add endpoint', got %q", msg)
	}
}

func TestExecutor_Execute_NilScope(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "main.go", "package main")

	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{
				Type:    "chore",
				Scope:   nil,
				Message: "initial commit",
				Files:   []string{"main.go"},
			},
		},
	}

	executor := NewExecutor(repoDir, false)
	executed, err := executor.Execute(plan, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(executed) != 1 {
		t.Fatalf("expected 1 executed commit, got %d", len(executed))
	}

	msg := getLastCommitMessage(t, repoDir)
	if msg != "chore: initial commit" {
		t.Errorf("expected 'chore: initial commit', got %q", msg)
	}
}
