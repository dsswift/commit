package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
// Directories are expanded to include all files within them.
func (s *Stager) StageFiles(files []string) error {
	// PRECONDITIONS
	assert.NotEmpty(files, "files cannot be empty")

	// Expand directories to their contained files
	var filesToStage []string
	for _, f := range files {
		fullPath := s.fullPath(f)
		info, err := os.Stat(fullPath)
		if os.IsNotExist(err) {
			// Check if it's a tracked deleted file
			if !s.isTrackedFile(f) {
				return fmt.Errorf("file does not exist and is not tracked: %s", f)
			}
			// Deleted tracked file - include it
			filesToStage = append(filesToStage, f)
		} else if err != nil {
			return fmt.Errorf("failed to stat file %s: %w", f, err)
		} else if info.IsDir() {
			// Expand directory to all files within it
			dirFiles, err := s.expandDirectory(f)
			if err != nil {
				return fmt.Errorf("failed to expand directory %s: %w", f, err)
			}
			filesToStage = append(filesToStage, dirFiles...)
		} else {
			filesToStage = append(filesToStage, f)
		}
	}

	// If all paths were empty directories, nothing to stage
	if len(filesToStage) == 0 {
		return nil
	}

	// EXECUTION
	args := append([]string{"add"}, filesToStage...)
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

	stagedSet := make(map[string]bool)
	for _, f := range staged {
		stagedSet[f] = true
	}

	for _, f := range filesToStage {
		if !stagedSet[f] {
			// Check if file is ignored by git
			if s.isIgnored(f) {
				return fmt.Errorf("file is ignored by .gitignore: %s", f)
			}
			return fmt.Errorf("file could not be staged (unknown reason): %s", f)
		}
	}

	return nil
}

// isIgnored checks if a file is ignored by git.
func (s *Stager) isIgnored(file string) bool {
	cmd := exec.Command("git", "check-ignore", "-q", file)
	cmd.Dir = s.workDir
	return cmd.Run() == nil
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

// expandDirectory returns all files within a directory (recursively).
// Returns paths relative to the work directory.
func (s *Stager) expandDirectory(dir string) ([]string, error) {
	var files []string
	fullDir := s.fullPath(dir)

	err := filepath.Walk(fullDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil // Skip directories themselves, we only want files
		}

		// Convert back to relative path
		relPath, err := filepath.Rel(s.workDir, path)
		if err != nil {
			return err
		}

		// Skip ignored files
		if s.isIgnored(relPath) {
			return nil
		}

		files = append(files, relPath)
		return nil
	})

	return files, err
}

// HasStagedChanges returns true if there are any staged changes.
func (s *Stager) HasStagedChanges() (bool, error) {
	files, err := s.StagedFiles()
	if err != nil {
		return false, err
	}
	return len(files) > 0, nil
}
