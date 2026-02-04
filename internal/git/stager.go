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
// Directories are expanded to include all files within them.
func (s *Stager) StageFiles(files []string) error {
	// PRECONDITIONS
	assert.NotEmpty(files, "files cannot be empty")

	// Get staged renames to handle rename sources correctly
	stagedRenames, err := s.getStagedRenames()
	if err != nil {
		return fmt.Errorf("failed to check staged renames: %w", err)
	}

	// Track which files are rename sources (already staged via the rename)
	renameSourcesRequested := make(map[string]string) // old_path -> new_path

	// Expand directories to their contained files
	var filesToStage []string
	for _, f := range files {
		// Check if this file is the source of a staged rename
		if newPath, isRenameSource := stagedRenames[f]; isRenameSource {
			// Skip staging - the rename is already staged
			// Track it for postcondition verification
			renameSourcesRequested[f] = newPath
			continue
		}

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

	// If all paths were empty directories or rename sources, nothing more to stage
	if len(filesToStage) == 0 {
		// Still verify rename sources are staged (via their destinations)
		if len(renameSourcesRequested) > 0 {
			staged, err := s.StagedFiles()
			if err != nil {
				return fmt.Errorf("failed to verify staging: %w", err)
			}
			stagedSet := make(map[string]bool)
			for _, f := range staged {
				stagedSet[f] = true
			}
			for oldPath, newPath := range renameSourcesRequested {
				if !stagedSet[newPath] {
					return fmt.Errorf("rename destination not staged: %s -> %s", oldPath, newPath)
				}
			}
		}
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

	// Get renames that may have formed during staging
	// (when we stage a delete + add of similar files, git auto-detects the rename)
	postStageRenames, _ := s.getStagedRenames()

	// Build a set of rename sources (old paths that are now part of a rename)
	renameSources := make(map[string]bool)
	for oldPath := range postStageRenames {
		renameSources[oldPath] = true
	}

	// Verify regular files are staged
	for _, f := range filesToStage {
		if !stagedSet[f] {
			// Check if this file became the source of an auto-detected rename
			if renameSources[f] {
				// File was staged as part of a rename - the destination is in stagedSet
				continue
			}
			// Check if file is ignored by git
			if s.isIgnored(f) {
				return fmt.Errorf("file is ignored by .gitignore: %s", f)
			}
			// Diagnose why staging failed
			return fmt.Errorf("file could not be staged: %s\n%s", f, s.diagnoseFile(f))
		}
	}

	// Verify rename sources are staged (via their destinations)
	for oldPath, newPath := range renameSourcesRequested {
		if !stagedSet[newPath] {
			return fmt.Errorf("rename destination not staged: %s -> %s", oldPath, newPath)
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

// expandDirectory returns all untracked, non-ignored files within a directory.
// Uses git ls-files for efficient batch ignore checking instead of per-file subprocess calls.
// Returns paths relative to the work directory.
func (s *Stager) expandDirectory(dir string) ([]string, error) {
	// Use git ls-files to get untracked, non-ignored files
	// --other: show untracked files
	// --exclude-standard: apply .gitignore rules
	cmd := exec.Command("git", "ls-files", "--other", "--exclude-standard", dir)
	cmd.Dir = s.workDir

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list files in %s: %w", dir, err)
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

// HasStagedChanges returns true if there are any staged changes.
func (s *Stager) HasStagedChanges() (bool, error) {
	files, err := s.StagedFiles()
	if err != nil {
		return false, err
	}
	return len(files) > 0, nil
}

// diagnoseFile returns diagnostic information about why a file couldn't be staged.
func (s *Stager) diagnoseFile(file string) string {
	var diag []string

	// Check if file exists on disk
	fullPath := s.fullPath(file)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		diag = append(diag, "  - file does not exist on disk")
	} else {
		diag = append(diag, "  - file exists on disk")
	}

	// Check if tracked by git
	if s.isTrackedFile(file) {
		diag = append(diag, "  - file is tracked by git")
	} else {
		diag = append(diag, "  - file is NOT tracked by git")
	}

	// Check git status for this file
	cmd := exec.Command("git", "status", "--porcelain", "--", file)
	cmd.Dir = s.workDir
	if out, err := cmd.Output(); err == nil {
		status := strings.TrimSpace(string(out))
		if status == "" {
			diag = append(diag, "  - git status: no changes detected for this path")
		} else {
			diag = append(diag, fmt.Sprintf("  - git status: %s", status))
		}
	}

	// Check if it's involved in a rename
	renames, _ := s.getStagedRenames()
	for oldPath, newPath := range renames {
		if oldPath == file {
			diag = append(diag, fmt.Sprintf("  - this is the SOURCE of a staged rename -> %s", newPath))
			diag = append(diag, "  - hint: the LLM included the old filename; the rename is already staged")
		}
		if newPath == file {
			diag = append(diag, fmt.Sprintf("  - this is the DESTINATION of a staged rename <- %s", oldPath))
		}
	}

	// List what IS currently staged
	staged, _ := s.StagedFiles()
	if len(staged) > 0 {
		diag = append(diag, fmt.Sprintf("  - currently staged files (%d): %v", len(staged), staged))
	}

	return strings.Join(diag, "\n")
}

// getStagedRenames returns a map of old_path -> new_path for staged renames.
func (s *Stager) getStagedRenames() (map[string]string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = s.workDir

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git status: %w", err)
	}

	renames := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 3 {
			continue
		}
		// Porcelain format: XY filename
		// X = index status, R means rename staged
		indexStatus := line[0]
		if indexStatus == 'R' {
			// Format: "R  old_path -> new_path"
			filename := strings.TrimSpace(line[3:])
			if strings.Contains(filename, " -> ") {
				parts := strings.Split(filename, " -> ")
				if len(parts) == 2 {
					renames[parts[0]] = parts[1]
				}
			}
		}
	}

	return renames, nil
}
