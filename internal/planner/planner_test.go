package planner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dsswift/commit/internal/testutil"
	"github.com/dsswift/commit/pkg/types"
)

func TestValidator_Validate_ValidPlan(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "planner-test-*")
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

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
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
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
		if testutil.ContainsString(err.Message, "not allowed") {
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
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
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
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
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
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

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
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
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
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
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

	fixedPlan, result := validator.ValidateAndFix(plan, false)

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
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
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

	fixedPlan, result := validator.ValidateAndFix(plan, false)

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
		filename    string
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

	if !testutil.ContainsString(preview, "2 commits planned") {
		t.Errorf("expected '2 commits planned', got: %s", preview)
	}

	if !testutil.ContainsString(preview, "feat(api): add endpoint") {
		t.Errorf("expected scoped commit message, got: %s", preview)
	}

	if !testutil.ContainsString(preview, "docs: update readme") {
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

	if !testutil.ContainsString(msg, "commit 2") {
		t.Errorf("expected 'commit 2' (1-indexed), got: %s", msg)
	}

	if !testutil.ContainsString(msg, "feat(api): add feature") {
		t.Errorf("expected commit message in error, got: %s", msg)
	}

	if !testutil.ContainsString(msg, "git error") {
		t.Errorf("expected wrapped error message, got: %s", msg)
	}

	if err.Unwrap() == nil {
		t.Error("expected Unwrap to return wrapped error")
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Field:   "commits[0].type",
		Message: "invalid type 'foo'",
	}

	got := err.Error()
	expected := "validation error in commits[0].type: invalid type 'foo'"

	if got != expected {
		t.Errorf("ValidationError.Error() = %q, want %q", got, expected)
	}
}

func TestIsPathSafe(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"absolute path", "/etc/passwd", false},
		{"dotdot only", "..", false},
		{"dotdot prefix", "../secret.txt", false},
		{"embedded dotdot", "foo/../../etc/passwd", false},
		{"normal relative path", "main.go", true},
		{"nested relative path", "internal/planner/validator.go", true},
		{"current dir prefix", "./main.go", true},
		{"deeply nested", "a/b/c/d/e/file.go", true},
		{"dotdot in middle resolving safe", "foo/bar/../baz.go", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPathSafe(tt.path)
			if got != tt.expected {
				t.Errorf("isPathSafe(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

// Helper types and functions

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestValidator_ValidateAndFix_RecoversOmittedDeletions(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "planner-test-*")
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

	config := &types.RepoConfig{
		CommitTypes: types.CommitTypeConfig{
			Mode:  "whitelist",
			Types: []string{"feat", "docs"},
		},
	}

	knownFiles := []string{
		"docs/plans/applied/a.md",
		"docs/plans/drafts/a.md",
		"src/api/handler.go",
	}
	validator := NewValidator(tmpDir, config, knownFiles)

	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{Type: "docs", Message: "apply plan a", Files: []string{"docs/plans/applied/a.md"}},
			{Type: "feat", Message: "add handler", Files: []string{"src/api/handler.go"}},
		},
	}

	fixed, result := validator.ValidateAndFix(plan, false)

	if !result.Valid {
		t.Fatalf("expected valid plan, got errors: %v", result.Errors)
	}

	if len(result.Recovered) != 1 || result.Recovered[0] != "docs/plans/drafts/a.md" {
		t.Fatalf("expected drafts/a.md to be recovered, got %v", result.Recovered)
	}

	var found bool
	for _, c := range fixed.Commits {
		for _, f := range c.Files {
			if f != "docs/plans/drafts/a.md" {
				continue
			}
			found = true
			if c.Message != "apply plan a" {
				t.Errorf("expected deletion grouped with nearest path, landed in %q", c.Message)
			}
		}
	}
	if !found {
		t.Error("deleted file missing from fixed plan")
	}
}

func TestValidator_ValidateAndFix_NoRecoveryWhenComplete(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "planner-test-*")
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

	config := &types.RepoConfig{
		CommitTypes: types.CommitTypeConfig{Mode: "whitelist", Types: []string{"feat"}},
	}
	validator := NewValidator(tmpDir, config, []string{"a.go"})

	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{Type: "feat", Message: "add a", Files: []string{"a.go"}},
		},
	}

	_, result := validator.ValidateAndFix(plan, false)

	if len(result.Recovered) != 0 {
		t.Errorf("expected no recovery, got %v", result.Recovered)
	}
}

// scopedConfig resolves workloads/api/ to api and infra/ to infra, the way
// LoadRepoConfig leaves a loaded .commit.json.
func scopedConfig() *types.RepoConfig {
	return &types.RepoConfig{Scopes: []types.ScopeConfig{{Path: "workloads/api/", Scope: "api"}, {Path: "infra/", Scope: "infra"}}}
}

func TestValidateAndFix_SplitsMixedScopes(t *testing.T) {
	infra := "infra"
	files := []string{"infra/main.tf", "workloads/api/README.md", "infra/vars.tf"}
	validator := NewValidator(".", scopedConfig(), files)

	// The model planned one commit across two scopes, as it did for a
	// Terraform file and a workload README.
	plan := &types.CommitPlan{Commits: []types.PlannedCommit{
		{Type: "feat", Scope: &infra, Message: "preauthorize cli scopes", Files: files},
	}}

	fixed, result := validator.ValidateAndFix(plan, false)

	if len(fixed.Commits) != 2 {
		t.Fatalf("expected one commit per scope, got %d: %+v", len(fixed.Commits), fixed.Commits)
	}
	for i, want := range []struct {
		scope string
		files []string
	}{{"infra", []string{"infra/main.tf", "infra/vars.tf"}}, {"api", []string{"workloads/api/README.md"}}} {
		got := fixed.Commits[i]
		if got.Scope == nil || *got.Scope != want.scope {
			t.Errorf("commit %d: scope %v, want %s", i, got.Scope, want.scope)
		}
		if strings.Join(got.Files, ",") != strings.Join(want.files, ",") {
			t.Errorf("commit %d: files %v, want %v", i, got.Files, want.files)
		}
		if got.Type != "feat" || got.Message != "preauthorize cli scopes" {
			t.Errorf("commit %d: type and message not kept: %+v", i, got)
		}
	}
	if len(result.Split) != 1 {
		t.Errorf("expected the split reported once, got %v", result.Split)
	}
}

func TestValidateAndFix_SingleKeepsMixedScopes(t *testing.T) {
	files := []string{"infra/main.tf", "workloads/api/README.md"}
	validator := NewValidator(".", scopedConfig(), files)
	plan := &types.CommitPlan{Commits: []types.PlannedCommit{{Type: "chore", Message: "one commit", Files: files}}}

	fixed, result := validator.ValidateAndFix(plan, true)

	if len(fixed.Commits) != 1 || len(result.Split) != 0 {
		t.Errorf("--single must keep one commit, got %d commits, split %v", len(fixed.Commits), result.Split)
	}
}

func TestValidateAndFix_CorrectsWrongScope(t *testing.T) {
	infra := "infra"
	validator := NewValidator(".", scopedConfig(), []string{"workloads/api/a.cs"})
	plan := &types.CommitPlan{Commits: []types.PlannedCommit{{Type: "fix", Scope: &infra, Message: "fix api", Files: []string{"workloads/api/a.cs"}}}}

	fixed, _ := validator.ValidateAndFix(plan, false)

	if fixed.Commits[0].Scope == nil || *fixed.Commits[0].Scope != "api" {
		t.Errorf("expected the file's own scope, got %v", fixed.Commits[0].Scope)
	}
}
