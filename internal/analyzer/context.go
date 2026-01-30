// Package analyzer builds the context for LLM analysis.
package analyzer

import (
	"fmt"

	"github.com/dsswift/commit/internal/assert"
	"github.com/dsswift/commit/internal/config"
	"github.com/dsswift/commit/internal/git"
	"github.com/dsswift/commit/pkg/types"
)

const (
	// MaxDiffChars is the maximum number of characters to include in the diff.
	MaxDiffChars = 4000
	// RecentCommitCount is the number of recent commits to include for style reference.
	RecentCommitCount = 10
)

// ContextBuilder builds the analysis request for the LLM.
type ContextBuilder struct {
	collector  *git.Collector
	repoConfig *types.RepoConfig
	workDir    string
}

// NewContextBuilder creates a new context builder.
func NewContextBuilder(workDir string, repoConfig *types.RepoConfig) *ContextBuilder {
	return &ContextBuilder{
		collector:  git.NewCollector(workDir),
		repoConfig: repoConfig,
		workDir:    workDir,
	}
}

// Build creates an AnalysisRequest from the current git state.
func (b *ContextBuilder) Build(stagedOnly bool) (*types.AnalysisRequest, error) {
	// Get git status
	status, err := b.collector.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get git status: %w", err)
	}

	// Determine which files to analyze
	var files []string
	if stagedOnly {
		files = status.Staged
	} else {
		files = status.AllFiles()
	}

	if len(files) == 0 {
		return nil, &NoChangesError{}
	}

	// Build file changes with scope resolution
	fileChanges, err := b.buildFileChanges(files, stagedOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to build file changes: %w", err)
	}

	// Get the diff
	diff, err := b.collector.Diff(stagedOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}

	// Truncate diff if too large
	truncatedDiff := git.TruncateDiff(diff, MaxDiffChars)

	// Get recent commits for style reference
	recentCommits, err := b.collector.RecentCommits(RecentCommitCount)
	if err != nil {
		// Non-fatal - proceed without recent commits
		recentCommits = []string{}
	}

	// Build the request
	request := &types.AnalysisRequest{
		Files:         fileChanges,
		Diff:          truncatedDiff,
		RecentCommits: recentCommits,
		HasScopes:     config.HasScopes(b.repoConfig),
		Rules: types.CommitRules{
			Types:            b.repoConfig.AllowedTypes(),
			MaxMessageLength: 50,
			BehavioralTest:   "feat = behavior change, refactor = same behavior different structure",
		},
	}

	// POSTCONDITIONS
	assert.NotEmpty(request.Files, "analysis request must have files")
	assert.NotEmpty(request.Rules.Types, "analysis request must have allowed types")

	return request, nil
}

// buildFileChanges creates FileChange objects from file paths.
func (b *ContextBuilder) buildFileChanges(files []string, stagedOnly bool) ([]types.FileChange, error) {
	// Get diff stats for all files
	numstat, err := b.collector.DiffNumstat(stagedOnly)
	if err != nil {
		return nil, err
	}

	// Get status to determine change type
	status, err := b.collector.Status()
	if err != nil {
		return nil, err
	}

	// Build lookup maps for status
	statusMap := make(map[string]string)
	for _, f := range status.Modified {
		statusMap[f] = "modified"
	}
	for _, f := range status.Added {
		statusMap[f] = "added"
	}
	for _, f := range status.Deleted {
		statusMap[f] = "deleted"
	}
	for _, f := range status.Renamed {
		statusMap[f] = "renamed"
	}
	for _, f := range status.Untracked {
		statusMap[f] = "added" // Untracked files being committed are "added"
	}

	var changes []types.FileChange
	for _, file := range files {
		change := types.FileChange{
			Path:   file,
			Status: statusMap[file],
			Scope:  config.ResolveScope(file, b.repoConfig),
		}

		// Add diff summary if available
		if stat, ok := numstat[file]; ok {
			change.DiffSummary = stat.DiffSummary
		}

		changes = append(changes, change)
	}

	return changes, nil
}

// BuildForFiles creates an AnalysisRequest for specific files.
func (b *ContextBuilder) BuildForFiles(files []string) (*types.AnalysisRequest, error) {
	assert.NotEmpty(files, "files cannot be empty")

	// Build file changes with scope resolution
	var fileChanges []types.FileChange
	for _, file := range files {
		change := types.FileChange{
			Path:  file,
			Scope: config.ResolveScope(file, b.repoConfig),
		}
		fileChanges = append(fileChanges, change)
	}

	// Get the diff for specific files
	diff, err := b.collector.Diff(false, files...)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}

	truncatedDiff := git.TruncateDiff(diff, MaxDiffChars)

	// Get recent commits for style reference
	recentCommits, err := b.collector.RecentCommits(RecentCommitCount)
	if err != nil {
		recentCommits = []string{}
	}

	return &types.AnalysisRequest{
		Files:         fileChanges,
		Diff:          truncatedDiff,
		RecentCommits: recentCommits,
		HasScopes:     config.HasScopes(b.repoConfig),
		Rules: types.CommitRules{
			Types:            b.repoConfig.AllowedTypes(),
			MaxMessageLength: 50,
			BehavioralTest:   "feat = behavior change, refactor = same behavior different structure",
		},
	}, nil
}

// NoChangesError indicates there are no changes to analyze.
type NoChangesError struct{}

func (e *NoChangesError) Error() string {
	return "nothing to commit - working tree is clean"
}

// Summary returns a human-readable summary of the analysis request.
func Summary(req *types.AnalysisRequest) string {
	scopeSet := make(map[string]bool)
	for _, f := range req.Files {
		if f.Scope != "" {
			scopeSet[f.Scope] = true
		}
	}

	scopes := make([]string, 0, len(scopeSet))
	for s := range scopeSet {
		scopes = append(scopes, s)
	}

	return fmt.Sprintf("%d files, %d chars diff, %d scopes detected",
		len(req.Files), len(req.Diff), len(scopes))
}
