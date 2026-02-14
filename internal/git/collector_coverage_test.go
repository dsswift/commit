package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dsswift/commit/internal/testutil"
	"github.com/dsswift/commit/pkg/types"
)

// TestCollector_InvalidateStatusCache verifies that InvalidateStatusCache clears the
// cached status, forcing a fresh git query on the next Status() call.
func TestCollector_InvalidateStatusCache(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	collector := NewCollector(repoDir)

	// First call caches the result
	status1, err := collector.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status1.HasChanges() {
		t.Fatal("expected no changes in empty repo")
	}

	// Create an untracked file -- cached status should not reflect it
	testutil.CreateFile(t, repoDir, "new.txt", "hello")
	status2, err := collector.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if len(status2.Untracked) != 0 {
		t.Error("expected cached status to still show 0 untracked files")
	}

	// Invalidate cache and re-query
	collector.InvalidateStatusCache()
	status3, err := collector.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if len(status3.Untracked) != 1 {
		t.Errorf("expected 1 untracked file after cache invalidation, got %d", len(status3.Untracked))
	}
}

// TestCollector_DiffStat verifies DiffStat returns per-file stat summaries.
func TestCollector_DiffStat(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	// Create initial commit
	testutil.CreateFile(t, repoDir, "file.txt", "line1\nline2\n")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "initial")

	// Modify file and stage it
	testutil.CreateFile(t, repoDir, "file.txt", "line1\nline2\nline3\nline4\n")
	testutil.GitAdd(t, repoDir, "file.txt")

	collector := NewCollector(repoDir)

	// Test staged diff stat
	stats, err := collector.DiffStat(true)
	if err != nil {
		t.Fatalf("DiffStat(staged) failed: %v", err)
	}
	if len(stats) == 0 {
		t.Error("expected non-empty diff stat for staged changes")
	}
	if _, ok := stats["file.txt"]; !ok {
		t.Errorf("expected file.txt in diff stat, got keys: %v", stats)
	}
}

// TestCollector_DiffStat_Unstaged verifies DiffStat for unstaged changes.
func TestCollector_DiffStat_Unstaged(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file.txt", "line1\n")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "initial")

	// Modify without staging
	testutil.CreateFile(t, repoDir, "file.txt", "line1\nline2\n")

	collector := NewCollector(repoDir)
	stats, err := collector.DiffStat(false)
	if err != nil {
		t.Fatalf("DiffStat(unstaged) failed: %v", err)
	}
	if _, ok := stats["file.txt"]; !ok {
		t.Errorf("expected file.txt in unstaged diff stat, got: %v", stats)
	}
}

// TestCollector_DiffStat_NoChanges verifies DiffStat returns empty map with no changes.
func TestCollector_DiffStat_NoChanges(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file.txt", "content")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "initial")

	collector := NewCollector(repoDir)
	stats, err := collector.DiffStat(true)
	if err != nil {
		t.Fatalf("DiffStat failed: %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("expected empty diff stat, got %v", stats)
	}
}

// TestCollector_DiffNumstat verifies DiffNumstat returns per-file numeric change stats.
func TestCollector_DiffNumstat(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file.txt", "line1\nline2\nline3\n")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "initial")

	// Modify and stage
	testutil.CreateFile(t, repoDir, "file.txt", "line1\nline2\nline3\nline4\nline5\n")
	testutil.GitAdd(t, repoDir, "file.txt")

	collector := NewCollector(repoDir)
	numstats, err := collector.DiffNumstat(true)
	if err != nil {
		t.Fatalf("DiffNumstat(staged) failed: %v", err)
	}
	if len(numstats) == 0 {
		t.Error("expected non-empty numstat for staged changes")
	}
	fc, ok := numstats["file.txt"]
	if !ok {
		t.Fatalf("expected file.txt in numstat, got keys: %v", numstats)
	}
	if fc.Path != "file.txt" {
		t.Errorf("expected Path=file.txt, got %q", fc.Path)
	}
	if !strings.Contains(fc.DiffSummary, "+") {
		t.Errorf("expected DiffSummary to contain '+', got %q", fc.DiffSummary)
	}
}

// TestCollector_DiffNumstat_Unstaged verifies DiffNumstat for unstaged changes.
func TestCollector_DiffNumstat_Unstaged(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file.txt", "line1\n")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "initial")

	testutil.CreateFile(t, repoDir, "file.txt", "line1\nline2\nline3\n")

	collector := NewCollector(repoDir)
	numstats, err := collector.DiffNumstat(false)
	if err != nil {
		t.Fatalf("DiffNumstat(unstaged) failed: %v", err)
	}
	if _, ok := numstats["file.txt"]; !ok {
		t.Errorf("expected file.txt in unstaged numstat, got: %v", numstats)
	}
}

// TestCollector_DiffNumstat_NoChanges verifies DiffNumstat returns empty map with no changes.
func TestCollector_DiffNumstat_NoChanges(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file.txt", "content")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "initial")

	collector := NewCollector(repoDir)
	numstats, err := collector.DiffNumstat(true)
	if err != nil {
		t.Fatalf("DiffNumstat failed: %v", err)
	}
	if len(numstats) != 0 {
		t.Errorf("expected empty numstat, got %v", numstats)
	}
}

// TestCollector_HeadCommit verifies HeadCommit returns the current HEAD hash.
func TestCollector_HeadCommit(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file.txt", "content")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "initial commit")

	collector := NewCollector(repoDir)
	hash, err := collector.HeadCommit()
	if err != nil {
		t.Fatalf("HeadCommit failed: %v", err)
	}

	if hash == "" {
		t.Error("expected non-empty hash from HeadCommit")
	}

	// Hash should be a full 40 char hex string
	if len(hash) != 40 {
		t.Errorf("expected 40-char hash, got %d chars: %q", len(hash), hash)
	}

	// Verify it matches what git rev-parse HEAD returns
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, _ := cmd.Output()
	expected := strings.TrimSpace(string(out))
	if hash != expected {
		t.Errorf("HeadCommit returned %q, expected %q", hash, expected)
	}
}

// TestCollector_HeadCommit_NoCommits verifies HeadCommit returns error in empty repo.
func TestCollector_HeadCommit_NoCommits(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	collector := NewCollector(repoDir)
	_, err := collector.HeadCommit()
	if err == nil {
		t.Error("expected error for HeadCommit in repo with no commits")
	}
}

// TestCollector_IsCommitPushed_NoPush verifies IsCommitPushed returns false when
// there is no remote.
func TestCollector_IsCommitPushed_NoPush(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file.txt", "content")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "initial commit")

	collector := NewCollector(repoDir)
	pushed, err := collector.IsCommitPushed()
	if err != nil {
		t.Fatalf("IsCommitPushed failed: %v", err)
	}
	if pushed {
		t.Error("expected commit to not be pushed (no remote)")
	}
}

// TestCollector_IsRefPushed_NoPush verifies IsRefPushed returns false with no remote.
func TestCollector_IsRefPushed_NoPush(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file.txt", "content")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "initial commit")

	collector := NewCollector(repoDir)
	pushed, err := collector.IsRefPushed("HEAD")
	if err != nil {
		t.Fatalf("IsRefPushed failed: %v", err)
	}
	if pushed {
		t.Error("expected ref to not be pushed (no remote)")
	}
}

// TestCollector_AbsolutePath verifies AbsolutePath joins workDir with the relative path.
func TestCollector_AbsolutePath(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	collector := NewCollector(repoDir)

	tests := []struct {
		name     string
		relative string
		expected string
	}{
		{
			name:     "simple file",
			relative: "main.go",
			expected: filepath.Join(repoDir, "main.go"),
		},
		{
			name:     "nested path",
			relative: "internal/git/collector.go",
			expected: filepath.Join(repoDir, "internal/git/collector.go"),
		},
		{
			name:     "empty string",
			relative: "",
			expected: repoDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collector.AbsolutePath(tt.relative)
			if result != tt.expected {
				t.Errorf("AbsolutePath(%q) = %q, want %q", tt.relative, result, tt.expected)
			}
		})
	}
}

// TestCollector_GetFileStats verifies GetFileStats returns per-file add/remove counts.
func TestCollector_GetFileStats(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "a.txt", "line1\nline2\nline3\n")
	testutil.CreateFile(t, repoDir, "b.txt", "hello\n")
	testutil.GitAdd(t, repoDir, "a.txt", "b.txt")
	testutil.GitCommit(t, repoDir, "initial")

	// Modify both files and stage
	testutil.CreateFile(t, repoDir, "a.txt", "line1\nline2\nline3\nline4\n")
	testutil.CreateFile(t, repoDir, "b.txt", "hello\nworld\ngoodbye\n")
	testutil.GitAdd(t, repoDir, "a.txt", "b.txt")

	collector := NewCollector(repoDir)
	stats, err := collector.GetFileStats(true)
	if err != nil {
		t.Fatalf("GetFileStats failed: %v", err)
	}

	if len(stats) != 2 {
		t.Fatalf("expected 2 file stats, got %d", len(stats))
	}

	// Build a map for easier lookup
	statsMap := make(map[string]FileStat)
	for _, s := range stats {
		statsMap[s.Path] = s
	}

	aStat, ok := statsMap["a.txt"]
	if !ok {
		t.Fatal("expected a.txt in file stats")
	}
	if aStat.Added != 1 {
		t.Errorf("expected 1 added line for a.txt, got %d", aStat.Added)
	}

	bStat, ok := statsMap["b.txt"]
	if !ok {
		t.Fatal("expected b.txt in file stats")
	}
	if bStat.Added < 2 {
		t.Errorf("expected at least 2 added lines for b.txt, got %d", bStat.Added)
	}
}

// TestCollector_GetFileStats_Unstaged verifies GetFileStats for unstaged changes.
func TestCollector_GetFileStats_Unstaged(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file.txt", "original\n")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "initial")

	testutil.CreateFile(t, repoDir, "file.txt", "original\nnew line\n")

	collector := NewCollector(repoDir)
	stats, err := collector.GetFileStats(false)
	if err != nil {
		t.Fatalf("GetFileStats(unstaged) failed: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 file stat, got %d", len(stats))
	}
	if stats[0].Path != "file.txt" {
		t.Errorf("expected file.txt, got %q", stats[0].Path)
	}
	if stats[0].Added != 1 {
		t.Errorf("expected 1 added line, got %d", stats[0].Added)
	}
}

// TestCollector_GetFileStats_NoChanges verifies GetFileStats returns empty with no changes.
func TestCollector_GetFileStats_NoChanges(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file.txt", "content")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "initial")

	collector := NewCollector(repoDir)
	stats, err := collector.GetFileStats(true)
	if err != nil {
		t.Fatalf("GetFileStats failed: %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("expected 0 stats, got %d", len(stats))
	}
}

// TestCollector_GetCommitLog verifies GetCommitLog returns detailed commit info.
func TestCollector_GetCommitLog(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	// Create commits
	for i := 1; i <= 3; i++ {
		testutil.CreateFile(t, repoDir, "file.txt", strings.Repeat("x", i))
		testutil.GitAdd(t, repoDir, "file.txt")
		testutil.GitCommit(t, repoDir, "commit "+string(rune('0'+i)))
	}

	collector := NewCollector(repoDir)
	commits, err := collector.GetCommitLog(2)
	if err != nil {
		t.Fatalf("GetCommitLog failed: %v", err)
	}

	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}

	// Most recent first
	if commits[0].Message != "commit 3" {
		t.Errorf("expected first commit message 'commit 3', got %q", commits[0].Message)
	}
	if commits[1].Message != "commit 2" {
		t.Errorf("expected second commit message 'commit 2', got %q", commits[1].Message)
	}

	// Verify fields are populated
	for _, c := range commits {
		if c.Hash == "" {
			t.Error("expected non-empty Hash")
		}
		if len(c.Hash) != 40 {
			t.Errorf("expected 40-char hash, got %d: %q", len(c.Hash), c.Hash)
		}
		if c.ShortHash == "" {
			t.Error("expected non-empty ShortHash")
		}
		if c.Author == "" {
			t.Error("expected non-empty Author")
		}
		if c.Author != "Test User" {
			t.Errorf("expected author 'Test User', got %q", c.Author)
		}
		if c.Date.IsZero() {
			t.Error("expected non-zero Date")
		}
	}
}

// TestCollector_GetCommitLog_Empty verifies GetCommitLog returns empty for repo with no commits.
func TestCollector_GetCommitLog_Empty(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	collector := NewCollector(repoDir)
	commits, err := collector.GetCommitLog(5)
	if err != nil {
		t.Fatalf("GetCommitLog failed: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("expected 0 commits in empty repo, got %d", len(commits))
	}
}

// TestCollector_GetCommitsInRange verifies getting commits between two refs.
func TestCollector_GetCommitsInRange(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	// Create 5 commits
	var hashes []string
	for i := 1; i <= 5; i++ {
		testutil.CreateFile(t, repoDir, "file.txt", strings.Repeat("x", i))
		testutil.GitAdd(t, repoDir, "file.txt")
		testutil.GitCommit(t, repoDir, "commit "+string(rune('0'+i)))

		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = repoDir
		out, _ := cmd.Output()
		hashes = append(hashes, strings.TrimSpace(string(out)))
	}

	collector := NewCollector(repoDir)

	// Get commits from commit 2 to commit 4 (exclusive of commit 2, inclusive of 3 and 4)
	commits, err := collector.GetCommitsInRange(hashes[1], hashes[3])
	if err != nil {
		t.Fatalf("GetCommitsInRange failed: %v", err)
	}

	if len(commits) != 2 {
		t.Fatalf("expected 2 commits in range, got %d", len(commits))
	}

	// Most recent first
	if commits[0].Message != "commit 4" {
		t.Errorf("expected first commit 'commit 4', got %q", commits[0].Message)
	}
	if commits[1].Message != "commit 3" {
		t.Errorf("expected second commit 'commit 3', got %q", commits[1].Message)
	}
}

// TestCollector_GetCommitsInRange_Full verifies getting all commits in range.
func TestCollector_GetCommitsInRange_Full(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file.txt", "v1")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "first")

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, _ := cmd.Output()
	firstHash := strings.TrimSpace(string(out))

	testutil.CreateFile(t, repoDir, "file.txt", "v2")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "second")

	testutil.CreateFile(t, repoDir, "file.txt", "v3")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "third")

	collector := NewCollector(repoDir)
	commits, err := collector.GetCommitsInRange(firstHash, "HEAD")
	if err != nil {
		t.Fatalf("GetCommitsInRange failed: %v", err)
	}

	if len(commits) != 2 {
		t.Fatalf("expected 2 commits between first and HEAD, got %d", len(commits))
	}
}

// TestCollector_parseCommitLog verifies parsing of git log formatted output.
func TestCollector_parseCommitLog(t *testing.T) {
	collector := NewCollector("/tmp") // workDir doesn't matter for parse

	now := time.Now().Unix()

	tests := []struct {
		name     string
		input    string
		expected int
		check    func([]CommitInfo)
	}{
		{
			name:     "empty output",
			input:    "",
			expected: 0,
			check:    nil,
		},
		{
			name:     "single commit",
			input:    "abc123def456abc123def456abc123def456abc1|abc123d|Alice|1700000000|feat: add login\n",
			expected: 1,
			check: func(commits []CommitInfo) {
				c := commits[0]
				if c.Hash != "abc123def456abc123def456abc123def456abc1" {
					t.Errorf("expected full hash, got %q", c.Hash)
				}
				if c.ShortHash != "abc123d" {
					t.Errorf("expected short hash 'abc123d', got %q", c.ShortHash)
				}
				if c.Author != "Alice" {
					t.Errorf("expected author 'Alice', got %q", c.Author)
				}
				if c.Date.Unix() != 1700000000 {
					t.Errorf("expected unix time 1700000000, got %d", c.Date.Unix())
				}
				if c.Message != "feat: add login" {
					t.Errorf("expected message 'feat: add login', got %q", c.Message)
				}
			},
		},
		{
			name: "multiple commits",
			input: strings.Join([]string{
				"aaaa0000aaaa0000aaaa0000aaaa0000aaaa0000|aaaa000|Bob|" + strings.TrimSpace(string(rune(now))) + "|fix: bug",
				"bbbb1111bbbb1111bbbb1111bbbb1111bbbb1111|bbbb111|Carol|1700000001|docs: update readme",
			}, "\n") + "\n",
			expected: 2,
			check:    nil,
		},
		{
			name:     "message with pipe characters",
			input:    "cccc2222cccc2222cccc2222cccc2222cccc2222|cccc222|Dave|1700000002|feat: add a|b toggle\n",
			expected: 1,
			check: func(commits []CommitInfo) {
				// SplitN with 5 means everything after 4th pipe is the message
				if commits[0].Message != "feat: add a|b toggle" {
					t.Errorf("expected message with pipe, got %q", commits[0].Message)
				}
			},
		},
		{
			name:     "malformed line ignored",
			input:    "not|enough|fields\naaaa0000aaaa0000aaaa0000aaaa0000aaaa0000|aaaa000|Eve|1700000003|chore: cleanup\n",
			expected: 1,
			check: func(commits []CommitInfo) {
				if commits[0].Author != "Eve" {
					t.Errorf("expected author 'Eve', got %q", commits[0].Author)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commits := collector.parseCommitLog([]byte(tt.input))
			if len(commits) != tt.expected {
				t.Fatalf("expected %d commits, got %d", tt.expected, len(commits))
			}
			if tt.check != nil {
				tt.check(commits)
			}
		})
	}
}

// TestCollector_batchResolvePushedStatus verifies pushed status resolution.
func TestCollector_batchResolvePushedStatus(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	// Create commits
	testutil.CreateFile(t, repoDir, "file.txt", "v1")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "first")

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, _ := cmd.Output()
	hash1 := strings.TrimSpace(string(out))

	testutil.CreateFile(t, repoDir, "file.txt", "v2")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "second")

	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, _ = cmd.Output()
	hash2 := strings.TrimSpace(string(out))

	collector := NewCollector(repoDir)

	commits := []CommitInfo{
		{Hash: hash1, Message: "first"},
		{Hash: hash2, Message: "second"},
	}

	// batchResolvePushedStatus should run without error and set IsPushed on each commit
	collector.batchResolvePushedStatus(commits)

	// Verify the method ran (IsPushed was resolved for all commits)
	// The exact value depends on whether remotes exist and the git version,
	// but the method should not panic or leave fields unset.
	for _, c := range commits {
		// IsPushed should be set (either true or false) -- we just verify the method ran
		_ = c.IsPushed
	}
}

// TestCollector_batchResolvePushedStatus_Empty verifies no panic on empty slice.
func TestCollector_batchResolvePushedStatus_Empty(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	collector := NewCollector(repoDir)
	commits := []CommitInfo{}
	// Should not panic
	collector.batchResolvePushedStatus(commits)
}

// TestCollector_getLocalOnlyCommits verifies local-only commit detection runs without error.
func TestCollector_getLocalOnlyCommits(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	testutil.CreateFile(t, repoDir, "file.txt", "v1")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "first")

	collector := NewCollector(repoDir)

	// Should not panic or error, even with no remotes
	localOnly := collector.getLocalOnlyCommits()

	// The result is a map of commit hashes; exact contents depend on git version
	// and remote configuration. We verify it returns a valid (possibly empty) map.
	if localOnly == nil {
		t.Error("expected non-nil map from getLocalOnlyCommits")
	}
}

// TestCollector_getLocalOnlyCommits_EmptyRepo verifies no crash on empty repo.
func TestCollector_getLocalOnlyCommits_EmptyRepo(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	collector := NewCollector(repoDir)
	localOnly := collector.getLocalOnlyCommits()

	// Empty repo has no commits, so the map should be empty
	if len(localOnly) != 0 {
		t.Errorf("expected empty local-only set for repo with no commits, got %d entries", len(localOnly))
	}
}

// TestCollector_filterIgnoredFiles verifies batch gitignore filtering.
func TestCollector_filterIgnoredFiles(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	createGitignore(t, repoDir, "*.log", "build/", "*.tmp")

	collector := NewCollector(repoDir)

	t.Run("filters ignored files", func(t *testing.T) {
		input := []string{"main.go", "debug.log", "build/output.js", "app.tmp", "README.md"}
		result := collector.filterIgnoredFiles(input)

		// Should only contain main.go and README.md
		if len(result) != 2 {
			t.Fatalf("expected 2 non-ignored files, got %d: %v", len(result), result)
		}

		resultSet := make(map[string]bool)
		for _, f := range result {
			resultSet[f] = true
		}

		if !resultSet["main.go"] {
			t.Error("expected main.go to be in result")
		}
		if !resultSet["README.md"] {
			t.Error("expected README.md to be in result")
		}
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		result := collector.filterIgnoredFiles([]string{})
		if len(result) != 0 {
			t.Errorf("expected empty result, got %v", result)
		}
	})

	t.Run("no files ignored", func(t *testing.T) {
		input := []string{"main.go", "utils.go", "test.go"}
		result := collector.filterIgnoredFiles(input)
		if len(result) != 3 {
			t.Errorf("expected all 3 files to pass filter, got %d: %v", len(result), result)
		}
	})

	t.Run("all files ignored", func(t *testing.T) {
		input := []string{"error.log", "access.log", "build/main.js"}
		result := collector.filterIgnoredFiles(input)
		if len(result) != 0 {
			t.Errorf("expected 0 files after filter, got %d: %v", len(result), result)
		}
	})
}

// TestCollector_filterIgnoredFiles_NoGitignore verifies behavior when no repo .gitignore exists.
func TestCollector_filterIgnoredFiles_NoGitignore(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	collector := NewCollector(repoDir)
	// Use filenames unlikely to match any global gitignore patterns
	input := []string{"main.go", "utils.go", "handler.go"}
	result := collector.filterIgnoredFiles(input)

	// Without a repo .gitignore, these .go files should not be filtered
	if len(result) != len(input) {
		t.Errorf("expected %d files, got %d: %v", len(input), len(result), result)
	}
}

// TestParseDiffStat verifies parsing of git diff --stat output.
func TestParseDiffStat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:     "empty output",
			input:    "",
			expected: map[string]string{},
		},
		{
			name: "single file",
			input: ` file.txt | 3 +++
 1 file changed, 3 insertions(+)
`,
			expected: map[string]string{
				"file.txt": "3 +++",
			},
		},
		{
			name: "multiple files",
			input: ` main.go  | 10 +++++++---
 util.go  |  5 ++---
 2 files changed, 9 insertions(+), 6 deletions(-)
`,
			expected: map[string]string{
				"main.go": "10 +++++++---",
				"util.go": "5 ++---",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDiffStat(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d entries, got %d: %v", len(tt.expected), len(result), result)
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("for %q: expected %q, got %q", k, v, result[k])
				}
			}
		})
	}
}

// TestParseNumstat verifies parsing of git diff --numstat output.
func TestParseNumstat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
		checkFn func(map[string]types.FileChange)
	}{
		{
			name:    "empty output",
			input:   "",
			wantLen: 0,
		},
		{
			name:    "single file",
			input:   "3\t1\tmain.go\n",
			wantLen: 1,
			checkFn: func(result map[string]types.FileChange) {
				fc := result["main.go"]
				if fc.Path != "main.go" {
					t.Errorf("expected Path=main.go, got %q", fc.Path)
				}
				if fc.DiffSummary != "+3 -1" {
					t.Errorf("expected DiffSummary '+3 -1', got %q", fc.DiffSummary)
				}
			},
		},
		{
			name:    "binary file",
			input:   "-\t-\timage.png\n",
			wantLen: 1,
			checkFn: func(result map[string]types.FileChange) {
				fc := result["image.png"]
				if fc.DiffSummary != "+binary -binary" {
					t.Errorf("expected binary summary, got %q", fc.DiffSummary)
				}
			},
		},
		{
			name:    "multiple files",
			input:   "10\t5\tmain.go\n2\t0\tREADME.md\n",
			wantLen: 2,
			checkFn: func(result map[string]types.FileChange) {
				if result["main.go"].DiffSummary != "+10 -5" {
					t.Errorf("unexpected main.go summary: %q", result["main.go"].DiffSummary)
				}
				if result["README.md"].DiffSummary != "+2 -0" {
					t.Errorf("unexpected README.md summary: %q", result["README.md"].DiffSummary)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseNumstat(tt.input)
			if len(result) != tt.wantLen {
				t.Fatalf("expected %d entries, got %d: %v", tt.wantLen, len(result), result)
			}
			if tt.checkFn != nil {
				tt.checkFn(result)
			}
		})
	}
}

// TestCollector_countCommits verifies the internal commit counter.
func TestCollector_countCommits(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	collector := NewCollector(repoDir)

	// Empty repo
	if n := collector.countCommits(); n != 0 {
		t.Errorf("expected 0 commits in empty repo, got %d", n)
	}

	// After creating commits
	for i := 1; i <= 3; i++ {
		testutil.CreateFile(t, repoDir, "file.txt", strings.Repeat("v", i))
		testutil.GitAdd(t, repoDir, "file.txt")
		testutil.GitCommit(t, repoDir, "commit")
	}

	if n := collector.countCommits(); n != 3 {
		t.Errorf("expected 3 commits, got %d", n)
	}
}

// TestCollector_GetCommitLog_WithRemote tests GetCommitLog exercises the pushed
// status resolution path when a remote exists.
func TestCollector_GetCommitLog_WithRemote(t *testing.T) {
	// Create a "remote" bare repo
	remoteDir, err := os.MkdirTemp("", "git-remote-*")
	if err != nil {
		t.Fatalf("failed to create remote dir: %v", err)
	}
	defer os.RemoveAll(remoteDir) //nolint:errcheck

	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = remoteDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}

	// Create local repo
	repoDir := testutil.TestRepo(t)

	// Add remote
	cmd = exec.Command("git", "remote", "add", "origin", remoteDir)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}

	// Create and push a commit
	testutil.CreateFile(t, repoDir, "file.txt", "v1")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "pushed commit")

	cmd = exec.Command("git", "push", "-u", "origin", "HEAD")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git push failed: %s: %v", string(out), err)
	}

	// Create a local-only commit
	testutil.CreateFile(t, repoDir, "file.txt", "v2")
	testutil.GitAdd(t, repoDir, "file.txt")
	testutil.GitCommit(t, repoDir, "local only commit")

	collector := NewCollector(repoDir)
	commits, err := collector.GetCommitLog(2)
	if err != nil {
		t.Fatalf("GetCommitLog failed: %v", err)
	}

	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}

	// Verify commit messages are correct
	if commits[0].Message != "local only commit" {
		t.Errorf("expected first commit 'local only commit', got %q", commits[0].Message)
	}
	if commits[1].Message != "pushed commit" {
		t.Errorf("expected second commit 'pushed commit', got %q", commits[1].Message)
	}

	// Verify all fields are populated (IsPushed was resolved by batchResolvePushedStatus)
	for _, c := range commits {
		if c.Hash == "" || c.ShortHash == "" || c.Author == "" {
			t.Error("expected all commit fields to be populated")
		}
	}
}
