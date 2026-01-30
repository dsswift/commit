// Package types defines shared types for the commit tool.
package types

import "time"

// FileChange represents a single file change detected by git.
type FileChange struct {
	Path        string `json:"path"`
	Status      string `json:"status"` // "modified", "added", "deleted", "renamed"
	Scope       string `json:"scope,omitempty"`
	DiffSummary string `json:"diffSummary"` // e.g., "+45 -12"
}

// AnalysisRequest is the structured request sent to the LLM.
type AnalysisRequest struct {
	Files         []FileChange `json:"files"`
	Diff          string       `json:"diff"`
	RecentCommits []string     `json:"recentCommits"`
	HasScopes     bool         `json:"hasScopes"`
	Rules         CommitRules  `json:"rules"`
}

// CommitRules defines constraints for commit messages.
type CommitRules struct {
	Types            []string `json:"types"`
	MaxMessageLength int      `json:"maxMessageLength"`
	BehavioralTest   string   `json:"behavioralTest"`
}

// PlannedCommit represents a single commit planned by the LLM.
type PlannedCommit struct {
	Type      string   `json:"type"`
	Scope     *string  `json:"scope"` // nil if no scope
	Message   string   `json:"message"`
	Files     []string `json:"files"`
	Reasoning string   `json:"reasoning"`
}

// CommitPlan is the structured response from the LLM.
type CommitPlan struct {
	Commits []PlannedCommit `json:"commits"`
}

// ExecutedCommit represents a commit that was successfully created.
type ExecutedCommit struct {
	Hash    string   `json:"hash"`
	Type    string   `json:"type"`
	Scope   *string  `json:"scope,omitempty"`
	Message string   `json:"message"`
	Files   []string `json:"files"`
}

// UserConfig represents the user's global configuration from ~/.commit-tool/.env.
type UserConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model,omitempty"`
	DryRun   bool   `json:"dryRun,omitempty"`

	// API keys for different providers
	AnthropicAPIKey string `json:"-"` // Never log API keys
	OpenAIAPIKey    string `json:"-"`
	GrokAPIKey      string `json:"-"`
	GeminiAPIKey    string `json:"-"`

	// Azure Foundry settings
	AzureFoundryEndpoint   string `json:"-"`
	AzureFoundryAPIKey     string `json:"-"`
	AzureFoundryDeployment string `json:"azureFoundryDeployment,omitempty"`
}

// ScopeConfig defines a path-to-scope mapping.
type ScopeConfig struct {
	Path  string `json:"path"`
	Scope string `json:"scope"`
}

// CommitTypeConfig defines whitelist/blacklist for commit types.
type CommitTypeConfig struct {
	Mode  string   `json:"mode"` // "whitelist" or "blacklist"
	Types []string `json:"types"`
}

// RepoConfig represents the repository-specific configuration from .commit.json.
type RepoConfig struct {
	Scopes       []ScopeConfig    `json:"scopes"`
	DefaultScope *string          `json:"defaultScope,omitempty"`
	CommitTypes  CommitTypeConfig `json:"commitTypes,omitempty"`
}

// DefaultCommitTypes returns the standard set of allowed commit types.
func DefaultCommitTypes() []string {
	return []string{"feat", "fix", "docs", "refactor", "test", "chore", "perf", "style"}
}

// IsTypeAllowed checks if a commit type is allowed based on config.
func (c *RepoConfig) IsTypeAllowed(commitType string) bool {
	if c.CommitTypes.Mode == "" || len(c.CommitTypes.Types) == 0 {
		// Default: all standard types allowed
		for _, t := range DefaultCommitTypes() {
			if t == commitType {
				return true
			}
		}
		return false
	}

	found := false
	for _, t := range c.CommitTypes.Types {
		if t == commitType {
			found = true
			break
		}
	}

	if c.CommitTypes.Mode == "whitelist" {
		return found
	}
	// blacklist mode
	return !found
}

// AllowedTypes returns the list of allowed commit types.
func (c *RepoConfig) AllowedTypes() []string {
	if c.CommitTypes.Mode == "" || len(c.CommitTypes.Types) == 0 {
		return DefaultCommitTypes()
	}

	if c.CommitTypes.Mode == "whitelist" {
		return c.CommitTypes.Types
	}

	// blacklist mode: return all defaults except blacklisted
	allowed := []string{}
	for _, t := range DefaultCommitTypes() {
		blocked := false
		for _, bt := range c.CommitTypes.Types {
			if bt == t {
				blocked = true
				break
			}
		}
		if !blocked {
			allowed = append(allowed, t)
		}
	}
	return allowed
}

// ExecutionResult represents the outcome of a commit tool run.
type ExecutionResult struct {
	ExecutionID    string           `json:"execution_id"`
	StartTime      time.Time        `json:"start_time"`
	Duration       time.Duration    `json:"duration"`
	ExitCode       int              `json:"exit_code"`
	CommitsCreated []ExecutedCommit `json:"commits_created,omitempty"`
	Error          string           `json:"error,omitempty"`
}

// GitStatus represents the current state of the git repository.
type GitStatus struct {
	Modified  []string `json:"modified"`
	Added     []string `json:"added"`
	Deleted   []string `json:"deleted"`
	Renamed   []string `json:"renamed"`
	Untracked []string `json:"untracked"`
	Staged    []string `json:"staged"`
}

// HasChanges returns true if there are any changes.
func (s *GitStatus) HasChanges() bool {
	return len(s.Modified) > 0 || len(s.Added) > 0 || len(s.Deleted) > 0 ||
		len(s.Renamed) > 0 || len(s.Untracked) > 0
}

// AllFiles returns all files with changes (excluding staged for non-staged mode).
func (s *GitStatus) AllFiles() []string {
	seen := make(map[string]bool)
	var files []string

	for _, lists := range [][]string{s.Modified, s.Added, s.Deleted, s.Renamed, s.Untracked} {
		for _, f := range lists {
			if !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
	}

	return files
}
