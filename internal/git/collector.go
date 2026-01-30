// Package git provides git operations for the commit tool.
package git

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/dsswift/commit/internal/assert"
	"github.com/dsswift/commit/pkg/types"
)

// Collector gathers git state information.
type Collector struct {
	workDir string
}

// NewCollector creates a new git collector for the given directory.
func NewCollector(workDir string) *Collector {
	return &Collector{workDir: workDir}
}

// FindGitRoot finds the root directory of the git repository.
func FindGitRoot(startDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = startDir

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// IsGitRepo checks if the current directory is inside a git repository.
func IsGitRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	return cmd.Run() == nil
}

// Status returns the current git status.
func (c *Collector) Status() (*types.GitStatus, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = c.workDir

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git status: %w", err)
	}

	status := &types.GitStatus{}
	scanner := bufio.NewScanner(bytes.NewReader(out))

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 3 {
			continue
		}

		// Porcelain format: XY filename
		// X = index status, Y = work tree status
		indexStatus := line[0]
		workTreeStatus := line[1]
		filename := strings.TrimSpace(line[3:])

		// Handle renamed files (format: "R  old -> new")
		if strings.Contains(filename, " -> ") {
			parts := strings.Split(filename, " -> ")
			if len(parts) == 2 {
				filename = parts[1]
			}
		}

		// Classify the file based on status codes
		switch {
		case indexStatus == 'M' || workTreeStatus == 'M':
			status.Modified = append(status.Modified, filename)
		case indexStatus == 'A':
			status.Added = append(status.Added, filename)
		case indexStatus == 'D' || workTreeStatus == 'D':
			status.Deleted = append(status.Deleted, filename)
		case indexStatus == 'R':
			status.Renamed = append(status.Renamed, filename)
		case indexStatus == '?' && workTreeStatus == '?':
			status.Untracked = append(status.Untracked, filename)
		}

		// Track staged files separately
		if indexStatus != ' ' && indexStatus != '?' {
			status.Staged = append(status.Staged, filename)
		}
	}

	return status, scanner.Err()
}

// Diff returns the diff for the specified files or all changes.
func (c *Collector) Diff(stagedOnly bool, files ...string) (string, error) {
	args := []string{"diff"}

	if stagedOnly {
		args = append(args, "--staged")
	} else {
		args = append(args, "HEAD")
	}

	// Add file paths if specified
	if len(files) > 0 {
		args = append(args, "--")
		args = append(args, files...)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = c.workDir

	out, err := cmd.Output()
	if err != nil {
		// Exit code 1 is normal when there are no changes
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", fmt.Errorf("failed to get diff: %w", err)
	}

	return string(out), nil
}

// DiffStat returns a summary of changes (lines added/removed) for each file.
func (c *Collector) DiffStat(stagedOnly bool) (map[string]string, error) {
	args := []string{"diff", "--stat"}

	if stagedOnly {
		args = append(args, "--staged")
	} else {
		args = append(args, "HEAD")
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = c.workDir

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("failed to get diff stat: %w", err)
	}

	return parseDiffStat(string(out)), nil
}

// DiffNumstat returns numeric stats (added/removed lines) per file.
func (c *Collector) DiffNumstat(stagedOnly bool) (map[string]types.FileChange, error) {
	args := []string{"diff", "--numstat"}

	if stagedOnly {
		args = append(args, "--staged")
	} else {
		args = append(args, "HEAD")
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = c.workDir

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return map[string]types.FileChange{}, nil
		}
		return nil, fmt.Errorf("failed to get diff numstat: %w", err)
	}

	return parseNumstat(string(out)), nil
}

// RecentCommits returns recent commit messages.
func (c *Collector) RecentCommits(count int) ([]string, error) {
	assert.Positive(count, "commit count must be positive")

	args := []string{"log", "--oneline", fmt.Sprintf("-%d", count)}
	cmd := exec.Command("git", args...)
	cmd.Dir = c.workDir

	out, err := cmd.Output()
	if err != nil {
		// New repo with no commits
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to get recent commits: %w", err)
	}

	var commits []string
	scanner := bufio.NewScanner(bytes.NewReader(out))

	for scanner.Scan() {
		line := scanner.Text()
		// Extract just the message part (skip the hash)
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			commits = append(commits, parts[1])
		}
	}

	return commits, scanner.Err()
}

// CurrentBranch returns the name of the current branch.
func (c *Collector) CurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = c.workDir

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// HeadCommit returns the hash of the HEAD commit.
func (c *Collector) HeadCommit() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = c.workDir

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD commit: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// IsInitialCommit returns true if HEAD is the first commit.
func (c *Collector) IsInitialCommit() bool {
	cmd := exec.Command("git", "rev-parse", "HEAD~1")
	cmd.Dir = c.workDir
	return cmd.Run() != nil
}

// IsCommitPushed checks if the HEAD commit exists on any remote branch.
func (c *Collector) IsCommitPushed() (bool, error) {
	cmd := exec.Command("git", "branch", "-r", "--contains", "HEAD")
	cmd.Dir = c.workDir

	out, err := cmd.Output()
	if err != nil {
		// Error likely means no remotes or commit not found on remotes
		return false, nil
	}

	// If output is non-empty, commit has been pushed
	return len(strings.TrimSpace(string(out))) > 0, nil
}

// AbsolutePath converts a relative path to absolute within the repo.
func (c *Collector) AbsolutePath(relativePath string) string {
	return filepath.Join(c.workDir, relativePath)
}

// parseDiffStat parses git diff --stat output into a map of file -> summary.
func parseDiffStat(output string) map[string]string {
	result := make(map[string]string)

	// Pattern: filename | count +++ ---
	statPattern := regexp.MustCompile(`^\s*(.+?)\s*\|\s*(\d+)\s*(.*)$`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		matches := statPattern.FindStringSubmatch(line)
		if len(matches) == 4 {
			filename := strings.TrimSpace(matches[1])
			result[filename] = matches[2] + " " + matches[3]
		}
	}

	return result
}

// parseNumstat parses git diff --numstat output.
func parseNumstat(output string) map[string]types.FileChange {
	result := make(map[string]types.FileChange)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			added := parts[0]
			removed := parts[1]
			filename := parts[2]

			// Handle binary files (shown as -)
			if added == "-" {
				added = "binary"
			}
			if removed == "-" {
				removed = "binary"
			}

			summary := fmt.Sprintf("+%s -%s", added, removed)

			result[filename] = types.FileChange{
				Path:        filename,
				DiffSummary: summary,
			}
		}
	}

	return result
}

// TruncateDiff truncates a diff to the specified maximum characters.
func TruncateDiff(diff string, maxChars int) string {
	assert.Positive(maxChars, "maxChars must be positive")

	if len(diff) <= maxChars {
		return diff
	}

	// Try to truncate at a line boundary
	truncated := diff[:maxChars]
	lastNewline := strings.LastIndex(truncated, "\n")
	if lastNewline > maxChars/2 {
		truncated = truncated[:lastNewline]
	}

	return truncated + "\n\n... (truncated)"
}

// FileStat represents stats for a single file.
type FileStat struct {
	Path    string
	Added   int
	Removed int
}

// GetFileStats returns detailed stats for changed files.
func (c *Collector) GetFileStats(stagedOnly bool) ([]FileStat, error) {
	args := []string{"diff", "--numstat"}

	if stagedOnly {
		args = append(args, "--staged")
	} else {
		args = append(args, "HEAD")
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = c.workDir

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return []FileStat{}, nil
		}
		return nil, fmt.Errorf("failed to get file stats: %w", err)
	}

	var stats []FileStat
	scanner := bufio.NewScanner(bytes.NewReader(out))

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			added, _ := strconv.Atoi(parts[0])
			removed, _ := strconv.Atoi(parts[1])
			filename := parts[2]

			stats = append(stats, FileStat{
				Path:    filename,
				Added:   added,
				Removed: removed,
			})
		}
	}

	return stats, scanner.Err()
}
