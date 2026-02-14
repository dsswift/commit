package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dsswift/commit/pkg/types"
)

func TestLoadRepoConfig_NoFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "repo-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

	config, err := LoadRepoConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config == nil {
		t.Fatal("expected non-nil config")
	}

	// Should return default config
	if len(config.Scopes) != 0 {
		t.Errorf("expected no scopes, got %d", len(config.Scopes))
	}

	if config.CommitTypes.Mode != "whitelist" {
		t.Errorf("expected whitelist mode, got %q", config.CommitTypes.Mode)
	}
}

func TestLoadRepoConfig_ValidConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "repo-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

	configContent := `{
  "scopes": [
    { "path": "src/api/", "scope": "api" },
    { "path": "src/", "scope": "core" },
    { "path": "docs/", "scope": "docs" }
  ],
  "defaultScope": "repo",
  "commitTypes": {
    "mode": "whitelist",
    "types": ["feat", "fix", "docs"]
  }
}`

	configPath := filepath.Join(tmpDir, RepoConfigFile)
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	config, err := LoadRepoConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check scopes
	if len(config.Scopes) != 3 {
		t.Errorf("expected 3 scopes, got %d", len(config.Scopes))
	}

	// Check default scope
	if config.DefaultScope == nil || *config.DefaultScope != "repo" {
		t.Errorf("expected default scope 'repo'")
	}

	// Check commit types
	if config.CommitTypes.Mode != "whitelist" {
		t.Errorf("expected whitelist mode, got %q", config.CommitTypes.Mode)
	}
	if len(config.CommitTypes.Types) != 3 {
		t.Errorf("expected 3 commit types, got %d", len(config.CommitTypes.Types))
	}
}

func TestLoadRepoConfig_SortsBySpecificity(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "repo-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

	// Define scopes in non-sorted order
	configContent := `{
  "scopes": [
    { "path": "a/", "scope": "short" },
    { "path": "a/b/c/d/", "scope": "longest" },
    { "path": "a/b/", "scope": "medium" }
  ]
}`

	configPath := filepath.Join(tmpDir, RepoConfigFile)
	_ = os.WriteFile(configPath, []byte(configContent), 0644)

	config, err := LoadRepoConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Scopes should be sorted by path length (longest first)
	if config.Scopes[0].Scope != "longest" {
		t.Errorf("expected first scope to be 'longest', got %q", config.Scopes[0].Scope)
	}
	if config.Scopes[1].Scope != "medium" {
		t.Errorf("expected second scope to be 'medium', got %q", config.Scopes[1].Scope)
	}
	if config.Scopes[2].Scope != "short" {
		t.Errorf("expected third scope to be 'short', got %q", config.Scopes[2].Scope)
	}
}

func TestLoadRepoConfig_NormalizesTrailingSlash(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "repo-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

	configContent := `{
  "scopes": [
    { "path": "src/api", "scope": "api" }
  ]
}`

	configPath := filepath.Join(tmpDir, RepoConfigFile)
	_ = os.WriteFile(configPath, []byte(configContent), 0644)

	config, err := LoadRepoConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Path should have trailing slash added
	if config.Scopes[0].Path != "src/api/" {
		t.Errorf("expected path 'src/api/', got %q", config.Scopes[0].Path)
	}
}

func TestLoadRepoConfig_InvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "repo-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

	configPath := filepath.Join(tmpDir, RepoConfigFile)
	_ = os.WriteFile(configPath, []byte("not valid json"), 0644)

	_, err = LoadRepoConfig(tmpDir)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadRepoConfig_DuplicatePaths(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "repo-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

	configContent := `{
  "scopes": [
    { "path": "src/", "scope": "a" },
    { "path": "src/", "scope": "b" }
  ]
}`

	configPath := filepath.Join(tmpDir, RepoConfigFile)
	_ = os.WriteFile(configPath, []byte(configContent), 0644)

	_, err = LoadRepoConfig(tmpDir)
	if err == nil {
		t.Error("expected error for duplicate paths")
	}
}

func TestLoadRepoConfig_EmptyScopeName(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "repo-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

	configContent := `{
  "scopes": [
    { "path": "src/", "scope": "" }
  ]
}`

	configPath := filepath.Join(tmpDir, RepoConfigFile)
	_ = os.WriteFile(configPath, []byte(configContent), 0644)

	_, err = LoadRepoConfig(tmpDir)
	if err == nil {
		t.Error("expected error for empty scope name")
	}
}

func TestResolveScope(t *testing.T) {
	config := &types.RepoConfig{
		Scopes: []types.ScopeConfig{
			{Path: "src/api/v2/", Scope: "api-v2"},
			{Path: "src/api/", Scope: "api"},
			{Path: "src/", Scope: "core"},
			{Path: "docs/", Scope: "docs"},
		},
	}
	defaultScope := "repo"
	config.DefaultScope = &defaultScope

	// Sort by specificity (longest first)
	sortScopesBySpecificity(config)

	tests := []struct {
		path     string
		expected string
	}{
		{"src/api/v2/handler.go", "api-v2"},
		{"src/api/client.go", "api"},
		{"src/main.go", "core"},
		{"docs/readme.md", "docs"},
		{"README.md", "repo"},     // Falls back to default
		{"other/file.go", "repo"}, // Falls back to default
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := ResolveScope(tt.path, config)
			if result != tt.expected {
				t.Errorf("ResolveScope(%q) = %q, expected %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestResolveScope_NoDefaultScope(t *testing.T) {
	config := &types.RepoConfig{
		Scopes: []types.ScopeConfig{
			{Path: "src/", Scope: "core"},
		},
	}

	// File not matching any scope, no default
	result := ResolveScope("other/file.go", config)
	if result != "" {
		t.Errorf("expected empty scope, got %q", result)
	}
}

func TestResolveScope_NilConfig(t *testing.T) {
	result := ResolveScope("any/file.go", nil)
	if result != "" {
		t.Errorf("expected empty scope for nil config, got %q", result)
	}
}

func TestResolveScope_EmptyScopes(t *testing.T) {
	config := &types.RepoConfig{
		Scopes: []types.ScopeConfig{},
	}
	defaultScope := "default"
	config.DefaultScope = &defaultScope

	result := ResolveScope("any/file.go", config)
	if result != "default" {
		t.Errorf("expected 'default' scope, got %q", result)
	}
}

func TestHasScopes(t *testing.T) {
	tests := []struct {
		name     string
		config   *types.RepoConfig
		expected bool
	}{
		{
			name:     "nil config",
			config:   nil,
			expected: false,
		},
		{
			name:     "empty scopes",
			config:   &types.RepoConfig{},
			expected: false,
		},
		{
			name: "has scopes",
			config: &types.RepoConfig{
				Scopes: []types.ScopeConfig{{Path: "src/", Scope: "core"}},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasScopes(tt.config)
			if result != tt.expected {
				t.Errorf("HasScopes() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestCreateDefaultRepoConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "repo-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

	err = CreateDefaultRepoConfig(tmpDir)
	if err != nil {
		t.Fatalf("CreateDefaultRepoConfig failed: %v", err)
	}

	// Check file was created
	configPath := filepath.Join(tmpDir, RepoConfigFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}

	// Load and verify it's valid
	config, err := LoadRepoConfig(tmpDir)
	if err != nil {
		t.Fatalf("failed to load created config: %v", err)
	}

	if len(config.Scopes) != 0 {
		t.Errorf("expected empty scopes, got %d", len(config.Scopes))
	}
}

func TestCreateDefaultRepoConfig_DoesNotOverwrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "repo-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

	// Create existing config
	existingContent := `{"scopes": [{"path": "existing/", "scope": "existing"}]}`
	configPath := filepath.Join(tmpDir, RepoConfigFile)
	_ = os.WriteFile(configPath, []byte(existingContent), 0644)

	// Try to create default
	err = CreateDefaultRepoConfig(tmpDir)
	if err != nil {
		t.Fatalf("CreateDefaultRepoConfig failed: %v", err)
	}

	// Verify not overwritten
	content, _ := os.ReadFile(configPath)
	if string(content) != existingContent {
		t.Error("config was overwritten")
	}
}
