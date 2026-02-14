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
	"time"

	"github.com/dsswift/commit/internal/assert"
	"github.com/dsswift/commit/pkg/types"
)

// Collector gathers git state information.
type Collector struct {
	workDir      string
	cachedStatus *types.GitStatus
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

// statusEntry holds parsed git status information for a single file.
type statusEntry struct {
	filename       string
	indexStatus    byte
	workTreeStatus byte
}

// Status returns the current git status. Results are cached after the first call.
func (c *Collector) Status() (*types.GitStatus, error) {
	if c.cachedStatus != nil {
		return c.cachedStatus, nil
	}

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = c.workDir

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git status: %w", err)
	}

	// First pass: collect all entries and filenames
	var entries []statusEntry
	var filenames []string
	scanner := bufio.NewScanner(bytes.NewReader(out))

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 3 {
			continue
		}

		indexStatus := line[0]
		workTreeStatus := line[1]
		filename := strings.TrimSpace(line[3:])

		if strings.Contains(filename, " -> ") {
			parts := strings.Split(filename, " -> ")
			if len(parts) == 2 {
				filename = parts[1]
			}
		}

		entries = append(entries, statusEntry{
			filename:       filename,
			indexStatus:    indexStatus,
			workTreeStatus: workTreeStatus,
		})
		filenames = append(filenames, filename)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Batch filter ignored files (single subprocess call instead of N calls)
	nonIgnored := c.filterIgnoredFiles(filenames)
	nonIgnoredSet := make(map[string]bool, len(nonIgnored))
	for _, f := range nonIgnored {
		nonIgnoredSet[f] = true
	}

	// Second pass: classify files that aren't ignored
	status := &types.GitStatus{}
	for _, entry := range entries {
		if !nonIgnoredSet[entry.filename] {
			continue // Skip ignored files
		}

		switch {
		case entry.indexStatus == 'M' || entry.workTreeStatus == 'M':
			status.Modified = append(status.Modified, entry.filename)
		case entry.indexStatus == 'A':
			status.Added = append(status.Added, entry.filename)
		case entry.indexStatus == 'D' || entry.workTreeStatus == 'D':
			status.Deleted = append(status.Deleted, entry.filename)
		case entry.indexStatus == 'R':
			status.Renamed = append(status.Renamed, entry.filename)
		case entry.indexStatus == '?' && entry.workTreeStatus == '?':
			status.Untracked = append(status.Untracked, entry.filename)
		}

		// Track staged files separately
		if entry.indexStatus != ' ' && entry.indexStatus != '?' {
			status.Staged = append(status.Staged, entry.filename)
		}
	}

	c.cachedStatus = status
	return status, nil
}

// InvalidateStatusCache clears the cached status, forcing the next Status() call to re-query git.
func (c *Collector) InvalidateStatusCache() {
	c.cachedStatus = nil
}

// IsIgnored checks if a file is ignored by .gitignore.
func (c *Collector) IsIgnored(file string) bool {
	cmd := exec.Command("git", "check-ignore", "-q", file)
	cmd.Dir = c.workDir
	return cmd.Run() == nil
}

// filterIgnoredFiles removes files that are ignored by .gitignore using batch check.
// This is much more efficient than per-file IsIgnored() calls for large file sets.
func (c *Collector) filterIgnoredFiles(files []string) []string {
	if len(files) == 0 {
		return files
	}

	// Use git check-ignore --stdin to batch check
	cmd := exec.Command("git", "check-ignore", "--stdin")
	cmd.Dir = c.workDir
	cmd.Stdin = strings.NewReader(strings.Join(files, "\n"))

	out, err := cmd.Output()
	if err != nil {
		// Exit code 1 means no files matched (none ignored) - that's fine
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return files
		}
		// On error, return original list (fail open)
		return files
	}

	// Build set of ignored files
	ignoredSet := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			ignoredSet[line] = true
		}
	}

	// Filter out ignored files
	var result []string
	for _, f := range files {
		if !ignoredSet[f] {
			result = append(result, f)
		}
	}

	return result
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
	return c.IsRefPushed("HEAD")
}

// IsRefPushed checks if the given ref exists on any remote branch.
func (c *Collector) IsRefPushed(ref string) (bool, error) {
	cmd := exec.Command("git", "branch", "-r", "--contains", ref)
	cmd.Dir = c.workDir

	out, err := cmd.Output()
	if err != nil {
		// Error likely means no remotes or commit not found on remotes
		return false, nil
	}

	// If output is non-empty, commit has been pushed
	return len(strings.TrimSpace(string(out))) > 0, nil
}

// HasCommitDepth verifies that at least count commits exist (i.e. HEAD~count resolves).
func (c *Collector) HasCommitDepth(count int) error {
	assert.Positive(count, "commit depth must be positive")

	ref := fmt.Sprintf("HEAD~%d", count)
	cmd := exec.Command("git", "rev-parse", "--verify", ref)
	cmd.Dir = c.workDir

	if err := cmd.Run(); err != nil {
		actual := c.countCommits()
		return fmt.Errorf("cannot reverse %d commits: only %d commits exist", count, actual)
	}

	return nil
}

// countCommits returns the number of commits reachable from HEAD.
func (c *Collector) countCommits() int {
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = c.workDir

	out, err := cmd.Output()
	if err != nil {
		return 0
	}

	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}

	return n
}

// AbsolutePath converts a relative path to absolute within the repo.
func (c *Collector) AbsolutePath(relativePath string) string {
	return filepath.Join(c.workDir, relativePath)
}

// diffStatPattern is pre-compiled for performance (called per diff-stat invocation).
var diffStatPattern = regexp.MustCompile(`^\s*(.+?)\s*\|\s*(\d+)\s*(.*)$`)

// parseDiffStat parses git diff --stat output into a map of file -> summary.
func parseDiffStat(output string) map[string]string {
	result := make(map[string]string)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		matches := diffStatPattern.FindStringSubmatch(line)
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

// CommitInfo represents detailed information about a commit for interactive rebase.
type CommitInfo struct {
	Hash      string
	ShortHash string
	Message   string
	Author    string
	Date      time.Time
	IsPushed  bool
}

// GetCommitLog returns detailed commit information for interactive rebase.
// The commits are returned in reverse chronological order (most recent first).
func (c *Collector) GetCommitLog(count int) ([]CommitInfo, error) {
	assert.Positive(count, "commit count must be positive")

	// Use a custom format to get all needed fields
	// Format: hash|short_hash|author|date_unix|subject
	format := "%H|%h|%an|%at|%s"
	args := []string{"log", fmt.Sprintf("-%d", count), "--format=" + format}
	cmd := exec.Command("git", args...)
	cmd.Dir = c.workDir

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			return []CommitInfo{}, nil
		}
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}

	commits := c.parseCommitLog(out)
	c.batchResolvePushedStatus(commits)

	return commits, nil
}

// GetCommitsInRange returns commits between two refs (exclusive of 'from', inclusive of 'to').
func (c *Collector) GetCommitsInRange(from, to string) ([]CommitInfo, error) {
	format := "%H|%h|%an|%at|%s"
	args := []string{"log", "--format=" + format, from + ".." + to}
	cmd := exec.Command("git", args...)
	cmd.Dir = c.workDir

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get commits in range: %w", err)
	}

	commits := c.parseCommitLog(out)
	c.batchResolvePushedStatus(commits)

	return commits, nil
}

// parseCommitLog parses git log output into CommitInfo structs (without pushed status).
func (c *Collector) parseCommitLog(out []byte) []CommitInfo {
	var commits []CommitInfo
	scanner := bufio.NewScanner(bytes.NewReader(out))

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "|", 5)
		if len(parts) != 5 {
			continue
		}

		dateUnix, _ := strconv.ParseInt(parts[3], 10, 64)

		commits = append(commits, CommitInfo{
			Hash:      parts[0],
			ShortHash: parts[1],
			Author:    parts[2],
			Date:      time.Unix(dateUnix, 0),
			Message:   parts[4],
		})
	}

	return commits
}

// batchResolvePushedStatus determines pushed status for all commits using a single
// git log call to find local-only commits, then marks the rest as pushed.
func (c *Collector) batchResolvePushedStatus(commits []CommitInfo) {
	if len(commits) == 0 {
		return
	}

	localOnly := c.getLocalOnlyCommits()

	for i := range commits {
		commits[i].IsPushed = !localOnly[commits[i].Hash]
	}
}

// getLocalOnlyCommits returns a set of commit hashes that exist only locally
// (not reachable from any remote tracking branch).
func (c *Collector) getLocalOnlyCommits() map[string]bool {
	localOnly := make(map[string]bool)

	// Get all local-only commits in one call: commits on HEAD not reachable from any remote
	cmd := exec.Command("git", "log", "--format=%H", "--not", "--remotes")
	cmd.Dir = c.workDir

	out, err := cmd.Output()
	if err != nil {
		// If this fails (e.g., no remotes), treat all commits as local
		return localOnly
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		hash := strings.TrimSpace(scanner.Text())
		if hash != "" {
			localOnly[hash] = true
		}
	}

	return localOnly
}
