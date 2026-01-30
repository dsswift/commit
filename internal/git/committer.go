package git

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/dsswift/commit/internal/assert"
	"github.com/dsswift/commit/pkg/types"
)

// Committer handles git commit operations.
type Committer struct {
	workDir string
}

// NewCommitter creates a new git committer for the given directory.
func NewCommitter(workDir string) *Committer {
	return &Committer{workDir: workDir}
}

// Commit creates a new commit with the given message.
func (c *Committer) Commit(message string) (string, error) {
	// PRECONDITIONS
	assert.NotEmptyString(message, "commit message cannot be empty")
	assert.MaxLength(message, 200, "commit message too long: %d chars", len(message))

	// Verify there are staged changes
	stager := NewStager(c.workDir)
	hasStaged, err := stager.HasStagedChanges()
	if err != nil {
		return "", fmt.Errorf("failed to check staged changes: %w", err)
	}
	assert.True(hasStaged, "no staged changes to commit")

	// EXECUTION
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = c.workDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to commit: %s: %w", string(out), err)
	}

	// POSTCONDITIONS
	// Get the commit hash
	hash, err := c.getLastCommitHash()
	if err != nil {
		return "", fmt.Errorf("commit succeeded but failed to get hash: %w", err)
	}

	assert.NotEmptyString(hash, "commit hash should not be empty after commit")

	return hash, nil
}

// CommitWithScope creates a commit with type and optional scope.
func (c *Committer) CommitWithScope(commitType string, scope *string, message string) (string, error) {
	// PRECONDITIONS
	assert.NotEmptyString(commitType, "commit type cannot be empty")
	assert.NotEmptyString(message, "commit message cannot be empty")

	// Build the full commit message
	var fullMessage string
	if scope != nil && *scope != "" {
		fullMessage = fmt.Sprintf("%s(%s): %s", commitType, *scope, message)
	} else {
		fullMessage = fmt.Sprintf("%s: %s", commitType, message)
	}

	return c.Commit(fullMessage)
}

// getLastCommitHash returns the hash of the most recent commit.
func (c *Committer) getLastCommitHash() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = c.workDir

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// GetLastCommitMessage returns the message of the most recent commit.
func (c *Committer) GetLastCommitMessage() (string, error) {
	cmd := exec.Command("git", "log", "-1", "--pretty=%B")
	cmd.Dir = c.workDir

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get last commit message: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// VerifyCommit verifies that the last commit matches expected values.
func (c *Committer) VerifyCommit(expectedHash string, expectedFiles []string) error {
	// Verify hash
	hash, err := c.getLastCommitHash()
	if err != nil {
		return fmt.Errorf("failed to verify commit hash: %w", err)
	}

	if hash != expectedHash {
		return fmt.Errorf("commit hash mismatch: expected %s, got %s", expectedHash, hash)
	}

	// Verify files in commit
	cmd := exec.Command("git", "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD")
	cmd.Dir = c.workDir

	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get commit files: %w", err)
	}

	committedFiles := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			committedFiles[line] = true
		}
	}

	for _, expected := range expectedFiles {
		if !committedFiles[expected] {
			return fmt.Errorf("expected file not in commit: %s", expected)
		}
	}

	return nil
}

// ExecutePlannedCommit executes a single planned commit.
func (c *Committer) ExecutePlannedCommit(planned types.PlannedCommit) (*types.ExecutedCommit, error) {
	// PRECONDITIONS
	assert.NotEmpty(planned.Files, "commit must have files")
	assert.NotEmptyString(planned.Type, "commit must have type")
	assert.NotEmptyString(planned.Message, "commit must have message")

	// Stage only the files for this commit
	stager := NewStager(c.workDir)

	// First, unstage everything to start clean
	if err := stager.UnstageAll(); err != nil {
		return nil, fmt.Errorf("failed to unstage files: %w", err)
	}

	// Stage the specific files for this commit
	if err := stager.StageFiles(planned.Files); err != nil {
		return nil, fmt.Errorf("failed to stage files: %w", err)
	}

	// Create the commit
	hash, err := c.CommitWithScope(planned.Type, planned.Scope, planned.Message)
	if err != nil {
		return nil, fmt.Errorf("failed to create commit: %w", err)
	}

	// Build the full message for the result
	var fullMessage string
	if planned.Scope != nil && *planned.Scope != "" {
		fullMessage = fmt.Sprintf("%s(%s): %s", planned.Type, *planned.Scope, planned.Message)
	} else {
		fullMessage = fmt.Sprintf("%s: %s", planned.Type, planned.Message)
	}

	return &types.ExecutedCommit{
		Hash:    hash,
		Type:    planned.Type,
		Scope:   planned.Scope,
		Message: fullMessage,
		Files:   planned.Files,
	}, nil
}
