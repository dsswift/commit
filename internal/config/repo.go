package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dsswift/commit/internal/assert"
	"github.com/dsswift/commit/pkg/types"
)

const (
	// RepoConfigFile is the name of the repo-specific config file.
	RepoConfigFile = ".commit.json"
)

// LoadRepoConfig loads the repository configuration from .commit.json if it exists.
// Returns nil (not an error) if the file doesn't exist - it's optional.
func LoadRepoConfig(gitRoot string) (*types.RepoConfig, error) {
	assert.NotEmptyString(gitRoot, "git root path cannot be empty")

	configPath := filepath.Join(gitRoot, RepoConfigFile)

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Optional file - return default config
		return defaultRepoConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read repo config: %w", err)
	}

	var config types.RepoConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse repo config: %w", err)
	}

	// Validate and normalize scopes
	if err := validateScopes(&config); err != nil {
		return nil, err
	}

	// Sort scopes by path length (longest first) for proper matching
	sortScopesBySpecificity(&config)

	// Set default commit types if not specified
	if config.CommitTypes.Mode == "" {
		config.CommitTypes.Mode = "whitelist"
		config.CommitTypes.Types = types.DefaultCommitTypes()
	}

	return &config, nil
}

// defaultRepoConfig returns the default configuration when no .commit.json exists.
func defaultRepoConfig() *types.RepoConfig {
	return &types.RepoConfig{
		Scopes: nil,
		CommitTypes: types.CommitTypeConfig{
			Mode:  "whitelist",
			Types: types.DefaultCommitTypes(),
		},
	}
}

// validateScopes ensures scope configurations are valid.
func validateScopes(config *types.RepoConfig) error {
	seen := make(map[string]bool)

	for i, scope := range config.Scopes {
		// Normalize path separators
		config.Scopes[i].Path = filepath.ToSlash(scope.Path)

		// Ensure path ends with / for directory matching
		if !strings.HasSuffix(config.Scopes[i].Path, "/") {
			config.Scopes[i].Path += "/"
		}

		// Check for duplicate paths
		if seen[config.Scopes[i].Path] {
			return fmt.Errorf("duplicate scope path: %s", config.Scopes[i].Path)
		}
		seen[config.Scopes[i].Path] = true

		// Validate scope name
		if scope.Scope == "" {
			return fmt.Errorf("scope name cannot be empty for path: %s", scope.Path)
		}
	}

	return nil
}

// sortScopesBySpecificity sorts scopes by path length (longest first).
// This ensures more specific paths are matched before general ones.
func sortScopesBySpecificity(config *types.RepoConfig) {
	sort.Slice(config.Scopes, func(i, j int) bool {
		return len(config.Scopes[i].Path) > len(config.Scopes[j].Path)
	})
}

// ResolveScope determines the scope for a given file path.
// Uses longest-match-wins algorithm.
func ResolveScope(filePath string, config *types.RepoConfig) string {
	if config == nil || len(config.Scopes) == 0 {
		if config != nil && config.DefaultScope != nil {
			return *config.DefaultScope
		}
		return ""
	}

	// Normalize path
	normalizedPath := filepath.ToSlash(filePath)

	// Find longest matching scope (scopes are pre-sorted by length)
	for _, scope := range config.Scopes {
		if strings.HasPrefix(normalizedPath, scope.Path) {
			return scope.Scope
		}
	}

	// No match - return default scope if set
	if config.DefaultScope != nil {
		return *config.DefaultScope
	}

	return ""
}

// HasScopes returns true if the config has any scope definitions.
func HasScopes(config *types.RepoConfig) bool {
	return config != nil && len(config.Scopes) > 0
}

// CreateDefaultRepoConfig creates a template .commit.json file.
func CreateDefaultRepoConfig(path string) error {
	configPath := filepath.Join(path, RepoConfigFile)

	// Don't overwrite existing config
	if _, err := os.Stat(configPath); err == nil {
		return nil
	}

	config := types.RepoConfig{
		Scopes:       []types.ScopeConfig{},
		DefaultScope: nil,
		CommitTypes: types.CommitTypeConfig{
			Mode:  "whitelist",
			Types: types.DefaultCommitTypes(),
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
