// Package planner handles commit plan validation and execution.
package planner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dsswift/commit/internal/assert"
	"github.com/dsswift/commit/internal/config"
	"github.com/dsswift/commit/pkg/types"
)

// Validator validates commit plans from the LLM.
type Validator struct {
	workDir    string
	repoConfig *types.RepoConfig
	knownFiles map[string]bool
	fileOrder  []string
}

// NewValidator creates a new validator.
func NewValidator(workDir string, repoConfig *types.RepoConfig, knownFiles []string) *Validator {
	fileMap := make(map[string]bool)
	order := make([]string, 0, len(knownFiles))
	for _, f := range knownFiles {
		if fileMap[f] {
			continue
		}
		fileMap[f] = true
		order = append(order, f)
	}

	return &Validator{
		workDir:    workDir,
		repoConfig: repoConfig,
		knownFiles: fileMap,
		fileOrder:  order,
	}
}

// ValidationError represents a plan validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error in %s: %s", e.Field, e.Message)
}

// ValidationResult contains the outcome of plan validation.
type ValidationResult struct {
	Valid bool
	// Recovered lists files the LLM omitted from the plan that were
	// reassigned to a commit so no change is left behind.
	Recovered []string
	// Split lists the planned commits that held files of more than one
	// scope and were split into one commit per scope.
	Split  []string
	Errors []ValidationError
}

// Validate checks if a commit plan is valid.
func (v *Validator) Validate(plan *types.CommitPlan) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if plan == nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "plan",
			Message: "plan is nil",
		})
		return result
	}

	if len(plan.Commits) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "commits",
			Message: "no commits in plan",
		})
		return result
	}

	seenFiles := make(map[string]bool)

	for i, commit := range plan.Commits {
		// Validate commit type
		if commit.Type == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("commits[%d].type", i),
				Message: "commit type is empty",
			})
		} else if !v.repoConfig.IsTypeAllowed(commit.Type) {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("commits[%d].type", i),
				Message: fmt.Sprintf("commit type %q not allowed (allowed: %v)", commit.Type, v.repoConfig.AllowedTypes()),
			})
		}

		// Validate message
		if commit.Message == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("commits[%d].message", i),
				Message: "commit message is empty",
			})
		} else if len(commit.Message) > 50 {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("commits[%d].message", i),
				Message: fmt.Sprintf("commit message exceeds 50 chars: %d chars", len(commit.Message)),
			})
		}

		// Validate files
		if len(commit.Files) == 0 {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("commits[%d].files", i),
				Message: "commit has no files",
			})
		}

		for j, file := range commit.Files {
			// Reject path traversal attempts
			if !isPathSafe(file) {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   fmt.Sprintf("commits[%d].files[%d]", i, j),
					Message: fmt.Sprintf("unsafe file path: %s", file),
				})
				continue
			}

			// Check if file is in known files list
			if !v.knownFiles[file] {
				// Also check if file exists on disk (might be untracked)
				fullPath := filepath.Join(v.workDir, file)
				if _, err := os.Stat(fullPath); os.IsNotExist(err) {
					result.Valid = false
					result.Errors = append(result.Errors, ValidationError{
						Field:   fmt.Sprintf("commits[%d].files[%d]", i, j),
						Message: fmt.Sprintf("file does not exist: %s", file),
					})
				}
			}

			// Check for duplicate files across commits
			if seenFiles[file] {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   fmt.Sprintf("commits[%d].files[%d]", i, j),
					Message: fmt.Sprintf("file appears in multiple commits: %s", file),
				})
			}
			seenFiles[file] = true
		}
	}

	return result
}

// ValidateAndFix attempts to fix minor validation issues.
// Returns the fixed plan and any remaining errors.
//
// Unless keepMixed is set, a commit holding files of more than one scope is
// split into one commit per scope: the release pipeline reads the version
// bump from each commit's scope, so a commit must never span two.
func (v *Validator) ValidateAndFix(plan *types.CommitPlan, keepMixed bool) (*types.CommitPlan, *ValidationResult) {
	if plan == nil {
		return nil, &ValidationResult{
			Valid:  false,
			Errors: []ValidationError{{Field: "plan", Message: "plan is nil"}},
		}
	}

	// Make a copy to avoid modifying the original
	fixedPlan := &types.CommitPlan{
		Commits: make([]types.PlannedCommit, len(plan.Commits)),
	}
	copy(fixedPlan.Commits, plan.Commits)

	// Fix truncatable issues
	for i := range fixedPlan.Commits {
		// Truncate overly long messages
		if len(fixedPlan.Commits[i].Message) > 50 {
			fixedPlan.Commits[i].Message = fixedPlan.Commits[i].Message[:47] + "..."
		}
	}

	// Merge commits that share files
	fixedPlan.Commits = v.mergeOverlappingCommits(fixedPlan.Commits)

	// Reassign any files the LLM left out
	recovered := v.ensureFullCoverage(fixedPlan)

	// One scope per commit
	var split []string
	if !keepMixed {
		fixedPlan.Commits, split = v.splitByScope(fixedPlan.Commits)
	}

	// Validate the fixed plan
	result := v.Validate(fixedPlan)
	result.Recovered = recovered
	result.Split = split

	return fixedPlan, result
}

// splitByScope splits every commit whose files resolve to more than one
// scope into one commit per scope, in the order the scopes first appear, each
// keeping the commit's type and message under its own scope. A commit whose
// files share one scope keeps that scope, so a scope the LLM got wrong is
// corrected too. Without configured scopes nothing changes.
func (v *Validator) splitByScope(commits []types.PlannedCommit) ([]types.PlannedCommit, []string) {
	if v.repoConfig == nil || len(v.repoConfig.Scopes) == 0 {
		return commits, nil
	}

	var out []types.PlannedCommit
	var split []string
	for _, commit := range commits {
		var order []string
		byScope := make(map[string][]string)
		for _, file := range commit.Files {
			scope := config.ResolveScope(file, v.repoConfig)
			if _, seen := byScope[scope]; !seen {
				order = append(order, scope)
			}
			byScope[scope] = append(byScope[scope], file)
		}

		if len(order) > 1 {
			split = append(split, fmt.Sprintf("%s: %s", commit.Message, strings.Join(order, ", ")))
		}
		for _, scope := range order {
			part := commit
			part.Files = byScope[scope]
			if scope == "" {
				part.Scope = nil
			} else {
				s := scope
				part.Scope = &s
			}
			out = append(out, part)
		}
	}

	return out, split
}

// ensureFullCoverage adds any known file missing from the plan to the commit
// whose files share the longest path prefix with it. LLMs routinely omit
// deletions, which would otherwise leave those changes uncommitted.
func (v *Validator) ensureFullCoverage(plan *types.CommitPlan) []string {
	if len(plan.Commits) == 0 {
		return nil
	}

	planned := make(map[string]bool)
	for _, commit := range plan.Commits {
		for _, file := range commit.Files {
			planned[file] = true
		}
	}

	var recovered []string
	for _, file := range v.fileOrder {
		if planned[file] {
			continue
		}
		target := v.bestCommitFor(file, plan.Commits)
		plan.Commits[target].Files = append(plan.Commits[target].Files, file)
		planned[file] = true
		recovered = append(recovered, file)
	}

	return recovered
}

// bestCommitFor returns the index of the commit that best matches a file,
// scored by the longest shared directory prefix with that commit's files.
func (v *Validator) bestCommitFor(file string, commits []types.PlannedCommit) int {
	best, bestScore := 0, -1

	for i, commit := range commits {
		for _, candidate := range commit.Files {
			if score := sharedDirDepth(file, candidate); score > bestScore {
				best, bestScore = i, score
			}
		}
	}

	return best
}

// sharedDirDepth counts the leading directory components two paths have in common.
func sharedDirDepth(a, b string) int {
	aParts := strings.Split(filepath.Dir(a), string(filepath.Separator))
	bParts := strings.Split(filepath.Dir(b), string(filepath.Separator))

	depth := 0
	for depth < len(aParts) && depth < len(bParts) && aParts[depth] == bParts[depth] {
		depth++
	}

	// filepath.Dir returns "." for root-level files, which shares nothing.
	if depth == 1 && aParts[0] == "." {
		return 0
	}

	return depth
}

// mergeOverlappingCommits merges commits that share files into single commits.
// When the LLM incorrectly puts the same file in multiple commits, this fixes it.
func (v *Validator) mergeOverlappingCommits(commits []types.PlannedCommit) []types.PlannedCommit {
	if len(commits) <= 1 {
		return commits
	}

	// Build a map of file -> commit indices that contain it
	fileToCommits := make(map[string][]int)
	for i, commit := range commits {
		for _, file := range commit.Files {
			fileToCommits[file] = append(fileToCommits[file], i)
		}
	}

	// Find commits that need to be merged (share at least one file)
	// Use union-find to group connected commits
	parent := make([]int, len(commits))
	for i := range parent {
		parent[i] = i
	}

	var find func(i int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}

	union := func(i, j int) {
		pi, pj := find(i), find(j)
		if pi != pj {
			parent[pi] = pj
		}
	}

	// Union commits that share files
	for _, indices := range fileToCommits {
		if len(indices) > 1 {
			for i := 1; i < len(indices); i++ {
				union(indices[0], indices[i])
			}
		}
	}

	// Group commits by their root
	groups := make(map[int][]int)
	for i := range commits {
		root := find(i)
		groups[root] = append(groups[root], i)
	}

	// Build merged commits
	var result []types.PlannedCommit
	for _, indices := range groups {
		if len(indices) == 1 {
			// No merge needed
			result = append(result, commits[indices[0]])
		} else {
			// Merge commits: take first commit's type/scope, combine messages and files
			merged := commits[indices[0]]

			// Collect all unique files
			fileSet := make(map[string]bool)
			for _, file := range merged.Files {
				fileSet[file] = true
			}

			// Merge in other commits
			for _, idx := range indices[1:] {
				other := commits[idx]
				for _, file := range other.Files {
					fileSet[file] = true
				}
				// If messages differ, could append, but for now just keep first
			}

			// Rebuild files slice
			merged.Files = nil
			for file := range fileSet {
				merged.Files = append(merged.Files, file)
			}

			result = append(result, merged)
		}
	}

	return result
}

// isPathSafe rejects absolute paths and paths containing ".." after cleaning.
func isPathSafe(file string) bool {
	if filepath.IsAbs(file) {
		return false
	}
	cleaned := filepath.Clean(file)
	// Check for directory traversal
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return false
	}
	// Also check for embedded traversal (e.g., "foo/../../etc/passwd")
	for _, part := range strings.Split(cleaned, string(filepath.Separator)) {
		if part == ".." {
			return false
		}
	}
	return true
}

// SensitiveFiles is a list of file patterns that should never be committed.
var SensitiveFiles = []string{
	"appsettings.json",
	"appsettings.*.json",
	"local.settings.json",
	".env",
	".env.*",
	"credentials.json",
	"secrets.json",
	"*.pem",
	"*.key",
	"*.p12",
	"*.pfx",
}

// FilterSensitiveFiles removes sensitive files from the plan.
func FilterSensitiveFiles(plan *types.CommitPlan) (filtered []string) {
	assert.NotNil(plan, "plan cannot be nil")

	for i := range plan.Commits {
		var safeFiles []string
		for _, file := range plan.Commits[i].Files {
			if isSensitiveFile(file) {
				filtered = append(filtered, file)
			} else {
				safeFiles = append(safeFiles, file)
			}
		}
		plan.Commits[i].Files = safeFiles
	}

	// Remove commits with no remaining files
	var nonEmptyCommits []types.PlannedCommit
	for _, commit := range plan.Commits {
		if len(commit.Files) > 0 {
			nonEmptyCommits = append(nonEmptyCommits, commit)
		}
	}
	plan.Commits = nonEmptyCommits

	return filtered
}

// isSensitiveFile checks if a file matches sensitive patterns.
func isSensitiveFile(file string) bool {
	base := filepath.Base(file)

	for _, pattern := range SensitiveFiles {
		matched, _ := filepath.Match(pattern, base)
		if matched {
			return true
		}
	}

	return false
}
