package planner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dsswift/commit/pkg/types"
)

func TestValidator_Validate_ValidPlan(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "planner-test-*")
	defer os.RemoveAll(tmpDir)

	// Create test files
	_ = os.WriteFile(filepath.Join(tmpDir, "file1.go"), []byte("content"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "file2.go"), []byte("content"), 0644)

	config := &types.RepoConfig{
		CommitTypes: types.CommitTypeConfig{
			Mode:  "whitelist",
			Types: []string{"feat", "fix", "docs"},
		},
	}

	validator := NewValidator(tmpDir, config, []string{"file1.go", "file2.go"})

	scope := "api"
	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{
				Type:    "feat",
				Scope:   &scope,
				Message: "add new feature",
				Files:   []string{"file1.go"},
			},
			{
				Type:    "fix",
				Scope:   nil,
				Message: "fix bug",
				Files:   []string{"file2.go"},
			},
		},
	}

	result := validator.Validate(plan)

	if !result.Valid {
		t.Errorf("expected valid plan, got errors: %v", result.Errors)
	}
}

func TestValidator_Validate_NilPlan(t *testing.T) {
	config := &types.RepoConfig{}
	validator := NewValidator(".", config, nil)

	result := validator.Validate(nil)

	if result.Valid {
		t.Error("expected invalid result for nil plan")
	}

	if len(result.Errors) == 0 {
		t.Error("expected errors for nil plan")
	}
}

func TestValidator_Validate_EmptyCommits(t *testing.T) {
	config := &types.RepoConfig{}
	validator := NewValidator(".", config, nil)

	plan := &types.CommitPlan{Commits: []types.PlannedCommit{}}

	result := validator.Validate(plan)

	if result.Valid {
		t.Error("expected invalid result for empty commits")
	}
}

func TestValidator_Validate_InvalidCommitType(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "planner-test-*")
	defer os.RemoveAll(tmpDir)
	_ = os.WriteFile(filepath.Join(tmpDir, "file.go"), []byte("content"), 0644)

	config := &types.RepoConfig{
		CommitTypes: types.CommitTypeConfig{
			Mode:  "whitelist",
			Types: []string{"feat", "fix"},
		},
	}

	validator := NewValidator(tmpDir, config, []string{"file.go"})

	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{
				Type:    "invalid",
				Message: "some message",
				Files:   []string{"file.go"},
			},
		},
	}

	result := validator.Validate(plan)

	if result.Valid {
		t.Error("expected invalid result for invalid commit type")
	}

	hasTypeError := false
	for _, err := range result.Errors {
		if containsString(err.Message, "not allowed") {
			hasTypeError = true
			break
		}
	}

	if !hasTypeError {
		t.Errorf("expected error about invalid type, got: %v", result.Errors)
	}
}

func TestValidator_Validate_MessageTooLong(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "planner-test-*")
	defer os.RemoveAll(tmpDir)
	_ = os.WriteFile(filepath.Join(tmpDir, "file.go"), []byte("content"), 0644)

	config := &types.RepoConfig{}
	validator := NewValidator(tmpDir, config, []string{"file.go"})

	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{
				Type:    "feat",
				Message: "this message is way too long and exceeds the fifty character limit",
				Files:   []string{"file.go"},
			},
		},
	}

	result := validator.Validate(plan)

	if result.Valid {
		t.Error("expected invalid result for long message")
	}
}

func TestValidator_Validate_EmptyMessage(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "planner-test-*")
	defer os.RemoveAll(tmpDir)
	_ = os.WriteFile(filepath.Join(tmpDir, "file.go"), []byte("content"), 0644)

	config := &types.RepoConfig{}
	validator := NewValidator(tmpDir, config, []string{"file.go"})

	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{
				Type:    "feat",
				Message: "",
				Files:   []string{"file.go"},
			},
		},
	}

	result := validator.Validate(plan)

	if result.Valid {
		t.Error("expected invalid result for empty message")
	}
}

func TestValidator_Validate_NoFiles(t *testing.T) {
	config := &types.RepoConfig{}
	validator := NewValidator(".", config, nil)

	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{
				Type:    "feat",
				Message: "add feature",
				Files:   []string{},
			},
		},
	}

	result := validator.Validate(plan)

	if result.Valid {
		t.Error("expected invalid result for commit with no files")
	}
}

func TestValidator_Validate_NonExistentFile(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "planner-test-*")
	defer os.RemoveAll(tmpDir)

	config := &types.RepoConfig{}
	validator := NewValidator(tmpDir, config, []string{})

	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{
				Type:    "feat",
				Message: "add feature",
				Files:   []string{"nonexistent.go"},
			},
		},
	}

	result := validator.Validate(plan)

	if result.Valid {
		t.Error("expected invalid result for non-existent file")
	}
}

func TestValidator_Validate_DuplicateFiles(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "planner-test-*")
	defer os.RemoveAll(tmpDir)
	_ = os.WriteFile(filepath.Join(tmpDir, "file.go"), []byte("content"), 0644)

	config := &types.RepoConfig{}
	validator := NewValidator(tmpDir, config, []string{"file.go"})

	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{
				Type:    "feat",
				Message: "add feature",
				Files:   []string{"file.go"},
			},
			{
				Type:    "fix",
				Message: "fix bug",
				Files:   []string{"file.go"}, // Same file in both commits
			},
		},
	}

	result := validator.Validate(plan)

	if result.Valid {
		t.Error("expected invalid result for duplicate files across commits")
	}
}

func TestValidateAndFix_TruncatesLongMessage(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "planner-test-*")
	defer os.RemoveAll(tmpDir)
	_ = os.WriteFile(filepath.Join(tmpDir, "file.go"), []byte("content"), 0644)

	config := &types.RepoConfig{}
	validator := NewValidator(tmpDir, config, []string{"file.go"})

	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{
				Type:    "feat",
				Message: "this message is way too long and exceeds the fifty character limit",
				Files:   []string{"file.go"},
			},
		},
	}

	fixedPlan, result := validator.ValidateAndFix(plan)

	if fixedPlan == nil {
		t.Fatal("expected non-nil fixed plan")
	}

	if len(fixedPlan.Commits[0].Message) > 50 {
		t.Errorf("expected message to be truncated, got length %d", len(fixedPlan.Commits[0].Message))
	}

	if !result.Valid {
		t.Errorf("expected valid result after fix, got errors: %v", result.Errors)
	}
}

func TestValidateAndFix_MergesOverlappingCommits(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "planner-test-*")
	defer os.RemoveAll(tmpDir)
	_ = os.WriteFile(filepath.Join(tmpDir, "shared.go"), []byte("content"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "only_a.go"), []byte("content"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "only_b.go"), []byte("content"), 0644)

	config := &types.RepoConfig{}
	validator := NewValidator(tmpDir, config, []string{"shared.go", "only_a.go", "only_b.go"})

	// LLM incorrectly put shared.go in both commits
	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{
				Type:    "feat",
				Message: "add feature a",
				Files:   []string{"shared.go", "only_a.go"},
			},
			{
				Type:    "refactor",
				Message: "refactor feature b",
				Files:   []string{"shared.go", "only_b.go"},
			},
		},
	}

	fixedPlan, result := validator.ValidateAndFix(plan)

	if fixedPlan == nil {
		t.Fatal("expected non-nil fixed plan")
	}

	// Should have merged into 1 commit
	if len(fixedPlan.Commits) != 1 {
		t.Errorf("expected 1 merged commit, got %d", len(fixedPlan.Commits))
	}

	// Should have all 3 files
	if len(fixedPlan.Commits) > 0 && len(fixedPlan.Commits[0].Files) != 3 {
		t.Errorf("expected 3 files in merged commit, got %d: %v", len(fixedPlan.Commits[0].Files), fixedPlan.Commits[0].Files)
	}

	if !result.Valid {
		t.Errorf("expected valid result after fix, got errors: %v", result.Errors)
	}
}

func TestFilterSensitiveFiles(t *testing.T) {
	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{
				Type:    "feat",
				Message: "add config",
				Files:   []string{"main.go", "appsettings.json", ".env", "config.go"},
			},
		},
	}

	filtered := FilterSensitiveFiles(plan)

	// Should have filtered out appsettings.json and .env
	if len(filtered) != 2 {
		t.Errorf("expected 2 filtered files, got %d: %v", len(filtered), filtered)
	}

	// Remaining files should be main.go and config.go
	if len(plan.Commits[0].Files) != 2 {
		t.Errorf("expected 2 remaining files, got %d", len(plan.Commits[0].Files))
	}
}

func TestFilterSensitiveFiles_RemovesEmptyCommits(t *testing.T) {
	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{
				Type:    "feat",
				Message: "add code",
				Files:   []string{"main.go"},
			},
			{
				Type:    "chore",
				Message: "update config",
				Files:   []string{".env", "appsettings.json"}, // All sensitive
			},
		},
	}

	FilterSensitiveFiles(plan)

	// Second commit should be removed (all files were sensitive)
	if len(plan.Commits) != 1 {
		t.Errorf("expected 1 commit after filtering, got %d", len(plan.Commits))
	}

	if plan.Commits[0].Message != "add code" {
		t.Error("wrong commit remained")
	}
}

func TestFilterSensitiveFiles_Patterns(t *testing.T) {
	tests := []struct {
		filename string
		isSensitive bool
	}{
		{"appsettings.json", true},
		{"appsettings.Development.json", true},
		{"local.settings.json", true},
		{".env", true},
		{".env.local", true},
		{"credentials.json", true},
		{"secrets.json", true},
		{"key.pem", true},
		{"private.key", true},
		{"cert.p12", true},
		{"main.go", false},
		{"config.yaml", false},
		{"settings.json", false}, // Not appsettings.json
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := isSensitiveFile(tt.filename)
			if result != tt.isSensitive {
				t.Errorf("%q: expected sensitive=%v, got %v", tt.filename, tt.isSensitive, result)
			}
		})
	}
}

func TestPreviewPlan(t *testing.T) {
	scope := "api"
	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{
				Type:      "feat",
				Scope:     &scope,
				Message:   "add endpoint",
				Files:     []string{"handler.go"},
				Reasoning: "New API endpoint",
			},
			{
				Type:    "docs",
				Scope:   nil,
				Message: "update readme",
				Files:   []string{"README.md"},
			},
		},
	}

	preview := PreviewPlan(plan)

	if preview == "" {
		t.Error("expected non-empty preview")
	}

	if !containsString(preview, "2 commits planned") {
		t.Errorf("expected '2 commits planned', got: %s", preview)
	}

	if !containsString(preview, "feat(api): add endpoint") {
		t.Errorf("expected scoped commit message, got: %s", preview)
	}

	if !containsString(preview, "docs: update readme") {
		t.Errorf("expected unscoped commit message, got: %s", preview)
	}
}

func TestPreviewPlan_Empty(t *testing.T) {
	preview := PreviewPlan(nil)
	if preview != "No commits planned" {
		t.Errorf("expected 'No commits planned', got: %s", preview)
	}

	preview = PreviewPlan(&types.CommitPlan{})
	if preview != "No commits planned" {
		t.Errorf("expected 'No commits planned', got: %s", preview)
	}
}

func TestExecutionError(t *testing.T) {
	scope := "api"
	err := &ExecutionError{
		CommitIndex: 1,
		Planned: types.PlannedCommit{
			Type:    "feat",
			Scope:   &scope,
			Message: "add feature",
		},
		Err: &testError{"git error"},
	}

	msg := err.Error()

	if !containsString(msg, "commit 2") {
		t.Errorf("expected 'commit 2' (1-indexed), got: %s", msg)
	}

	if !containsString(msg, "feat(api): add feature") {
		t.Errorf("expected commit message in error, got: %s", msg)
	}

	if !containsString(msg, "git error") {
		t.Errorf("expected wrapped error message, got: %s", msg)
	}

	if err.Unwrap() == nil {
		t.Error("expected Unwrap to return wrapped error")
	}
}

// Helper types and functions

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
