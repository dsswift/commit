// Package analyzer provides change analysis for git repositories.
package analyzer

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// DiffRequest contains parameters for diff analysis.
type DiffRequest struct {
	FilePath string
	FromRef  string
	ToRef    string
	GitRoot  string
}

// DiffResult contains the result of a diff analysis.
type DiffResult struct {
	FilePath    string
	FromRef     string
	ToRef       string
	Diff        string
	NumStats    string
	LinesAdded  int
	LinesRemove int
}

// BuildDiffRequest creates a diff request for analysis.
func BuildDiffRequest(gitRoot, filePath, fromRef, toRef string) *DiffRequest {
	// Default refs
	if fromRef == "" && toRef == "" {
		// Compare working copy to HEAD
		toRef = "HEAD"
	} else if fromRef == "" {
		fromRef = "HEAD"
	}

	return &DiffRequest{
		FilePath: filePath,
		FromRef:  fromRef,
		ToRef:    toRef,
		GitRoot:  gitRoot,
	}
}

// GetDiff retrieves the diff for the requested file and refs.
func GetDiff(req *DiffRequest) (*DiffResult, error) {
	result := &DiffResult{
		FilePath: req.FilePath,
		FromRef:  req.FromRef,
		ToRef:    req.ToRef,
	}

	// Build diff command
	var diffArgs []string
	if req.FromRef == "" && req.ToRef == "HEAD" {
		// Uncommitted changes
		diffArgs = []string{"diff", "HEAD", "--", req.FilePath}
	} else if req.ToRef == "" {
		// From ref to working copy
		diffArgs = []string{"diff", req.FromRef, "--", req.FilePath}
	} else {
		// Between two refs
		diffArgs = []string{"diff", req.FromRef, req.ToRef, "--", req.FilePath}
	}

	cmd := exec.Command("git", diffArgs...)
	cmd.Dir = req.GitRoot
	output, err := cmd.Output()
	if err != nil {
		// Check if file exists in the refs
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("diff failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("diff failed: %w", err)
	}

	result.Diff = string(output)

	// Get numstat
	var numstatArgs []string
	if req.FromRef == "" && req.ToRef == "HEAD" {
		numstatArgs = []string{"diff", "HEAD", "--numstat", "--", req.FilePath}
	} else if req.ToRef == "" {
		numstatArgs = []string{"diff", req.FromRef, "--numstat", "--", req.FilePath}
	} else {
		numstatArgs = []string{"diff", req.FromRef, req.ToRef, "--numstat", "--", req.FilePath}
	}

	numstatCmd := exec.Command("git", numstatArgs...)
	numstatCmd.Dir = req.GitRoot
	numstatOutput, _ := numstatCmd.Output()

	if len(numstatOutput) > 0 {
		parts := strings.Fields(string(numstatOutput))
		if len(parts) >= 2 {
			_, _ = fmt.Sscanf(parts[0], "%d", &result.LinesAdded)
			_, _ = fmt.Sscanf(parts[1], "%d", &result.LinesRemove)
			result.NumStats = fmt.Sprintf("+%d -%d", result.LinesAdded, result.LinesRemove)
		}
	}

	return result, nil
}

// BuildDiffPrompt creates the LLM prompt for diff analysis.
func BuildDiffPrompt(result *DiffResult) (system, user string) {
	system = `You are a code change analyst. Your job is to analyze git diffs and explain what changed in clear, human-readable language.

Guidelines:
- Focus on the semantic meaning of changes, not just syntax
- Group related changes together
- Identify the type of change (bug fix, new feature, refactoring, etc.)
- Note any potential issues or improvements
- Use clear, concise language
- Format your response with markdown for readability`

	var refRange string
	if result.FromRef == "" && result.ToRef == "HEAD" {
		refRange = "uncommitted changes"
	} else if result.ToRef == "" {
		refRange = fmt.Sprintf("from %s to working copy", result.FromRef)
	} else {
		refRange = fmt.Sprintf("from %s to %s", result.FromRef, result.ToRef)
	}

	user = fmt.Sprintf(`Analyze the following changes to %s (%s):

Stats: %s

Diff:
%s

Provide a clear explanation of what changed and why these changes matter.`,
		result.FilePath,
		refRange,
		result.NumStats,
		result.Diff,
	)

	return system, user
}

// DiffAnalyzer handles diff analysis with LLM.
type DiffAnalyzer struct {
	gitRoot string
}

// NewDiffAnalyzer creates a new diff analyzer.
func NewDiffAnalyzer(gitRoot string) *DiffAnalyzer {
	return &DiffAnalyzer{gitRoot: gitRoot}
}

// Analyze performs diff analysis using the provided LLM provider.
func (a *DiffAnalyzer) Analyze(ctx context.Context, filePath, fromRef, toRef string, provider DiffProvider) (string, error) {
	// Build request
	req := BuildDiffRequest(a.gitRoot, filePath, fromRef, toRef)

	// Get diff
	result, err := GetDiff(req)
	if err != nil {
		return "", err
	}

	// Check if there are changes
	if result.Diff == "" {
		return "No changes detected in the specified file and range.", nil
	}

	// Build prompt
	system, user := BuildDiffPrompt(result)

	// Call LLM
	analysis, err := provider.AnalyzeDiff(ctx, system, user)
	if err != nil {
		return "", err
	}

	return analysis, nil
}

// DiffProvider interface for LLM providers that can analyze diffs.
type DiffProvider interface {
	AnalyzeDiff(ctx context.Context, system, user string) (string, error)
}
