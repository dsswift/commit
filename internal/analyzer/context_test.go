package analyzer

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dsswift/commit/pkg/types"
)

// testRepo creates a temporary git repository for testing.
func testRepo(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "analyzer-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		cleanup()
		t.Fatalf("failed to init git repo: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	return tmpDir, cleanup
}

func createFile(t *testing.T, repoDir, filename, content string) {
	t.Helper()
	path := filepath.Join(repoDir, filename)
	dir := filepath.Dir(path)
	_ = os.MkdirAll(dir, 0755)
	_ = os.WriteFile(path, []byte(content), 0644)
}

func gitAdd(t *testing.T, repoDir string, files ...string) {
	t.Helper()
	args := append([]string{"add"}, files...)
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	_ = cmd.Run()
}

func gitCommit(t *testing.T, repoDir, message string) {
	t.Helper()
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoDir
	_ = cmd.Run()
}

func TestContextBuilder_Build_NoChanges(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	config := &types.RepoConfig{}
	builder := NewContextBuilder(repoDir, config)

	_, err := builder.Build(false)
	if err == nil {
		t.Error("expected error for empty repo")
	}

	if _, ok := err.(*NoChangesError); !ok {
		t.Errorf("expected NoChangesError, got %T: %v", err, err)
	}
}

func TestContextBuilder_Build_WithChanges(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, repoDir, "existing.txt", "initial")
	gitAdd(t, repoDir, "existing.txt")
	gitCommit(t, repoDir, "initial commit")

	// Create new file and modify existing
	createFile(t, repoDir, "new.txt", "new content")
	createFile(t, repoDir, "existing.txt", "modified")

	config := &types.RepoConfig{}
	builder := NewContextBuilder(repoDir, config)

	req, err := builder.Build(false)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if len(req.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(req.Files))
	}

	if req.HasScopes {
		t.Error("expected HasScopes to be false")
	}
}

func TestContextBuilder_Build_WithScopes(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit (required for git diff)
	createFile(t, repoDir, "init.txt", "init")
	gitAdd(t, repoDir, "init.txt")
	gitCommit(t, repoDir, "initial commit")

	// Create files in different directories and stage them
	// (staged files show as individual paths, untracked might show as directories)
	createFile(t, repoDir, "src/api/handler.go", "api code")
	createFile(t, repoDir, "src/core/main.go", "core code")
	gitAdd(t, repoDir, "src/api/handler.go", "src/core/main.go")

	// Note: Scopes must be sorted by specificity (longest first) for proper matching
	// In production, LoadRepoConfig does this sorting
	config := &types.RepoConfig{
		Scopes: []types.ScopeConfig{
			{Path: "src/api/", Scope: "api"},   // More specific - matches src/api/*
			{Path: "src/core/", Scope: "core"}, // Specific - matches src/core/*
		},
	}

	builder := NewContextBuilder(repoDir, config)

	// Use staged-only mode since we staged the files
	req, err := builder.Build(true)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if !req.HasScopes {
		t.Error("expected HasScopes to be true")
	}

	// Check scopes were resolved
	scopeMap := make(map[string]string)
	for _, f := range req.Files {
		scopeMap[f.Path] = f.Scope
	}

	if scopeMap["src/api/handler.go"] != "api" {
		t.Errorf("expected api scope for handler.go, got %q", scopeMap["src/api/handler.go"])
	}

	if scopeMap["src/core/main.go"] != "core" {
		t.Errorf("expected core scope for main.go, got %q", scopeMap["src/core/main.go"])
	}
}

func TestContextBuilder_Build_StagedOnly(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, repoDir, "committed.txt", "committed")
	gitAdd(t, repoDir, "committed.txt")
	gitCommit(t, repoDir, "initial")

	// Create staged and unstaged files
	createFile(t, repoDir, "staged.txt", "staged")
	createFile(t, repoDir, "unstaged.txt", "unstaged")
	gitAdd(t, repoDir, "staged.txt")

	config := &types.RepoConfig{}
	builder := NewContextBuilder(repoDir, config)

	// Build with staged only
	req, err := builder.Build(true)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should only have staged file
	if len(req.Files) != 1 {
		t.Errorf("expected 1 file, got %d: %v", len(req.Files), req.Files)
	}

	if req.Files[0].Path != "staged.txt" {
		t.Errorf("expected staged.txt, got %q", req.Files[0].Path)
	}
}

func TestContextBuilder_Build_IncludesRecentCommits(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create several commits
	for i := 1; i <= 3; i++ {
		createFile(t, repoDir, "file.txt", string(rune('0'+i)))
		gitAdd(t, repoDir, "file.txt")
		gitCommit(t, repoDir, "commit "+string(rune('0'+i)))
	}

	// Create a new change
	createFile(t, repoDir, "new.txt", "new")

	config := &types.RepoConfig{}
	builder := NewContextBuilder(repoDir, config)

	req, err := builder.Build(false)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if len(req.RecentCommits) == 0 {
		t.Error("expected recent commits to be populated")
	}
}

func TestContextBuilder_Build_IncludesRules(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit (required for git diff)
	createFile(t, repoDir, "init.txt", "init")
	gitAdd(t, repoDir, "init.txt")
	gitCommit(t, repoDir, "initial commit")

	createFile(t, repoDir, "file.txt", "content")

	config := &types.RepoConfig{
		CommitTypes: types.CommitTypeConfig{
			Mode:  "whitelist",
			Types: []string{"feat", "fix"},
		},
	}
	builder := NewContextBuilder(repoDir, config)

	req, err := builder.Build(false)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if len(req.Rules.Types) != 2 {
		t.Errorf("expected 2 allowed types, got %d", len(req.Rules.Types))
	}

	if req.Rules.MaxMessageLength != 50 {
		t.Errorf("expected max length 50, got %d", req.Rules.MaxMessageLength)
	}
}

func TestContextBuilder_BuildForFiles(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit
	createFile(t, repoDir, "file1.txt", "initial1")
	createFile(t, repoDir, "file2.txt", "initial2")
	gitAdd(t, repoDir, "file1.txt", "file2.txt")
	gitCommit(t, repoDir, "initial")

	// Modify files
	createFile(t, repoDir, "file1.txt", "modified1")
	createFile(t, repoDir, "file2.txt", "modified2")

	config := &types.RepoConfig{}
	builder := NewContextBuilder(repoDir, config)

	// Build for specific file
	req, err := builder.BuildForFiles([]string{"file1.txt"})
	if err != nil {
		t.Fatalf("BuildForFiles failed: %v", err)
	}

	if len(req.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(req.Files))
	}

	if req.Files[0].Path != "file1.txt" {
		t.Errorf("expected file1.txt, got %q", req.Files[0].Path)
	}
}

func TestNoChangesError(t *testing.T) {
	err := &NoChangesError{}
	msg := err.Error()

	if msg != "nothing to commit - working tree is clean" {
		t.Errorf("unexpected error message: %q", msg)
	}
}

func TestSummary(t *testing.T) {
	req := &types.AnalysisRequest{
		Files: []types.FileChange{
			{Path: "a.go", Scope: "api"},
			{Path: "b.go", Scope: "api"},
			{Path: "c.go", Scope: "core"},
		},
		Diff: "diff content here",
	}

	summary := Summary(req)

	if summary == "" {
		t.Error("expected non-empty summary")
	}

	// Should mention file count
	if !containsString(summary, "3 files") {
		t.Errorf("expected summary to mention '3 files', got: %s", summary)
	}

	// Should mention scope count
	if !containsString(summary, "2 scopes") {
		t.Errorf("expected summary to mention '2 scopes', got: %s", summary)
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
