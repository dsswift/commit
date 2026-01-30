package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/dsswift/commit/internal/assert"
)

// Stager handles git staging operations.
type Stager struct {
	workDir string
}

// NewStager creates a new git stager for the given directory.
func NewStager(workDir string) *Stager {
	return &Stager{workDir: workDir}
}

// StageFiles adds specific files to the staging area.
func (s *Stager) StageFiles(files []string) error {
	// PRECONDITIONS
	assert.NotEmpty(files, "files cannot be empty")

	for _, f := range files {
		fullPath := s.fullPath(f)
		// File should exist (unless it's a deletion)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			// Check if it's a tracked deleted file
			if !s.isTrackedFile(f) {
				return fmt.Errorf("file does not exist and is not tracked: %s", f)
			}
		}
	}

	// EXECUTION
	args := append([]string{"add"}, files...)
	cmd := exec.Command("git", args...)
	cmd.Dir = s.workDir

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stage files: %s: %w", string(out), err)
	}

	// POSTCONDITIONS
	staged, err := s.StagedFiles()
	if err != nil {
		return fmt.Errorf("failed to verify staging: %w", err)
	}

	for _, f := range files {
		assert.Contains(staged, f, "file should be staged after add: %s", f)
	}

	return nil
}

// UnstageAll removes all files from the staging area (keeps changes in working directory).
func (s *Stager) UnstageAll() error {
	// Check if HEAD exists (has at least one commit)
	checkHead := exec.Command("git", "rev-parse", "HEAD")
	checkHead.Dir = s.workDir
	hasHead := checkHead.Run() == nil

	var cmd *exec.Cmd
	if hasHead {
		cmd = exec.Command("git", "reset", "HEAD")
	} else {
		// No commits yet - use rm --cached to unstage
		cmd = exec.Command("git", "rm", "--cached", "-r", "--ignore-unmatch", ".")
	}
	cmd.Dir = s.workDir

	if out, err := cmd.CombinedOutput(); err != nil {
		// Ignore common non-error messages
		outStr := string(out)
		if strings.Contains(outStr, "Unstaged changes after reset") ||
			strings.Contains(outStr, "nothing to commit") {
			return nil
		}
		return fmt.Errorf("failed to unstage files: %s: %w", outStr, err)
	}

	return nil
}

// UnstageFiles removes specific files from the staging area.
func (s *Stager) UnstageFiles(files []string) error {
	assert.NotEmpty(files, "files cannot be empty")

	args := append([]string{"reset", "HEAD", "--"}, files...)
	cmd := exec.Command("git", args...)
	cmd.Dir = s.workDir

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to unstage files: %s: %w", string(out), err)
	}

	return nil
}

// StagedFiles returns the list of currently staged files.
func (s *Stager) StagedFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = s.workDir

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get staged files: %w", err)
	}

	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

// StageAll stages all changes (modified, added, deleted, untracked).
func (s *Stager) StageAll() error {
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = s.workDir

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stage all files: %s: %w", string(out), err)
	}

	return nil
}

// isTrackedFile checks if a file is tracked by git.
func (s *Stager) isTrackedFile(file string) bool {
	cmd := exec.Command("git", "ls-files", file)
	cmd.Dir = s.workDir

	out, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(out)) != ""
}

// fullPath returns the full path of a file relative to the work directory.
func (s *Stager) fullPath(file string) string {
	if strings.HasPrefix(file, "/") {
		return file
	}
	return s.workDir + "/" + file
}

// HasStagedChanges returns true if there are any staged changes.
func (s *Stager) HasStagedChanges() (bool, error) {
	files, err := s.StagedFiles()
	if err != nil {
		return false, err
	}
	return len(files) > 0, nil
}
