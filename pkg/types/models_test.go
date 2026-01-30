package types

import (
	"testing"
)

func TestDefaultCommitTypes(t *testing.T) {
	types := DefaultCommitTypes()

	expected := []string{"feat", "fix", "docs", "refactor", "test", "chore", "perf", "style"}

	if len(types) != len(expected) {
		t.Errorf("expected %d types, got %d", len(expected), len(types))
	}

	for i, typ := range expected {
		if types[i] != typ {
			t.Errorf("expected types[%d] = %q, got %q", i, typ, types[i])
		}
	}
}

func TestRepoConfig_IsTypeAllowed_DefaultConfig(t *testing.T) {
	config := &RepoConfig{}

	// All default types should be allowed
	for _, typ := range DefaultCommitTypes() {
		if !config.IsTypeAllowed(typ) {
			t.Errorf("expected %q to be allowed with default config", typ)
		}
	}

	// Unknown type should not be allowed
	if config.IsTypeAllowed("unknown") {
		t.Error("expected 'unknown' to not be allowed")
	}
}

func TestRepoConfig_IsTypeAllowed_Whitelist(t *testing.T) {
	config := &RepoConfig{
		CommitTypes: CommitTypeConfig{
			Mode:  "whitelist",
			Types: []string{"docs", "fix"},
		},
	}

	// Only whitelisted types should be allowed
	if !config.IsTypeAllowed("docs") {
		t.Error("expected 'docs' to be allowed")
	}
	if !config.IsTypeAllowed("fix") {
		t.Error("expected 'fix' to be allowed")
	}
	if config.IsTypeAllowed("feat") {
		t.Error("expected 'feat' to not be allowed")
	}
	if config.IsTypeAllowed("refactor") {
		t.Error("expected 'refactor' to not be allowed")
	}
}

func TestRepoConfig_IsTypeAllowed_Blacklist(t *testing.T) {
	config := &RepoConfig{
		CommitTypes: CommitTypeConfig{
			Mode:  "blacklist",
			Types: []string{"refactor"},
		},
	}

	// All types except blacklisted should be allowed
	if !config.IsTypeAllowed("feat") {
		t.Error("expected 'feat' to be allowed")
	}
	if !config.IsTypeAllowed("fix") {
		t.Error("expected 'fix' to be allowed")
	}
	if config.IsTypeAllowed("refactor") {
		t.Error("expected 'refactor' to not be allowed")
	}
}

func TestRepoConfig_AllowedTypes_Default(t *testing.T) {
	config := &RepoConfig{}

	allowed := config.AllowedTypes()
	expected := DefaultCommitTypes()

	if len(allowed) != len(expected) {
		t.Errorf("expected %d allowed types, got %d", len(expected), len(allowed))
	}
}

func TestRepoConfig_AllowedTypes_Whitelist(t *testing.T) {
	config := &RepoConfig{
		CommitTypes: CommitTypeConfig{
			Mode:  "whitelist",
			Types: []string{"docs", "fix"},
		},
	}

	allowed := config.AllowedTypes()

	if len(allowed) != 2 {
		t.Errorf("expected 2 allowed types, got %d", len(allowed))
	}

	// Check that both types are present
	hasDoc, hasFix := false, false
	for _, typ := range allowed {
		if typ == "docs" {
			hasDoc = true
		}
		if typ == "fix" {
			hasFix = true
		}
	}

	if !hasDoc || !hasFix {
		t.Errorf("expected docs and fix in allowed types, got %v", allowed)
	}
}

func TestRepoConfig_AllowedTypes_Blacklist(t *testing.T) {
	config := &RepoConfig{
		CommitTypes: CommitTypeConfig{
			Mode:  "blacklist",
			Types: []string{"refactor", "style"},
		},
	}

	allowed := config.AllowedTypes()

	// Should have all default types except refactor and style
	expectedLen := len(DefaultCommitTypes()) - 2
	if len(allowed) != expectedLen {
		t.Errorf("expected %d allowed types, got %d", expectedLen, len(allowed))
	}

	// refactor and style should not be in allowed
	for _, typ := range allowed {
		if typ == "refactor" || typ == "style" {
			t.Errorf("expected %q to be excluded from allowed types", typ)
		}
	}
}

func TestGitStatus_HasChanges(t *testing.T) {
	tests := []struct {
		name     string
		status   GitStatus
		expected bool
	}{
		{
			name:     "empty status",
			status:   GitStatus{},
			expected: false,
		},
		{
			name:     "only modified",
			status:   GitStatus{Modified: []string{"file.go"}},
			expected: true,
		},
		{
			name:     "only added",
			status:   GitStatus{Added: []string{"new.go"}},
			expected: true,
		},
		{
			name:     "only deleted",
			status:   GitStatus{Deleted: []string{"old.go"}},
			expected: true,
		},
		{
			name:     "only renamed",
			status:   GitStatus{Renamed: []string{"renamed.go"}},
			expected: true,
		},
		{
			name:     "only untracked",
			status:   GitStatus{Untracked: []string{"untracked.go"}},
			expected: true,
		},
		{
			name: "multiple changes",
			status: GitStatus{
				Modified:  []string{"a.go"},
				Added:     []string{"b.go"},
				Untracked: []string{"c.go"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.HasChanges()
			if result != tt.expected {
				t.Errorf("HasChanges() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestGitStatus_AllFiles(t *testing.T) {
	status := GitStatus{
		Modified:  []string{"a.go", "b.go"},
		Added:     []string{"c.go"},
		Deleted:   []string{"d.go"},
		Renamed:   []string{"e.go"},
		Untracked: []string{"f.go"},
	}

	files := status.AllFiles()

	// Should have 6 unique files
	if len(files) != 6 {
		t.Errorf("expected 6 files, got %d: %v", len(files), files)
	}

	// Check all files are present
	expected := map[string]bool{
		"a.go": true, "b.go": true, "c.go": true,
		"d.go": true, "e.go": true, "f.go": true,
	}

	for _, f := range files {
		if !expected[f] {
			t.Errorf("unexpected file %q in result", f)
		}
		delete(expected, f)
	}

	if len(expected) > 0 {
		t.Errorf("missing files: %v", expected)
	}
}

func TestGitStatus_AllFiles_NoDuplicates(t *testing.T) {
	// Same file appears in multiple categories
	status := GitStatus{
		Modified: []string{"a.go"},
		Staged:   []string{"a.go"},
	}

	files := status.AllFiles()

	// AllFiles doesn't include Staged, but let's verify no duplicates
	// from the categories it does include
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d: %v", len(files), files)
	}
}
