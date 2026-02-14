package interactive

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dsswift/commit/internal/testutil"
)

func TestSelectModel_SelectCommit_IncludesSelectedCommit(t *testing.T) {
	// Create a SelectModel with mock commits
	m := &SelectModel{
		commits: []RebaseCommit{
			{Hash: "hash0", ShortHash: "abc0000", Message: "commit 0"},
			{Hash: "hash1", ShortHash: "abc1111", Message: "commit 1"},
			{Hash: "hash2", ShortHash: "abc2222", Message: "commit 2"},
		},
		cursor: 0, // Select the most recent commit
	}

	cmd := m.selectCommit()
	msg := cmd().(SelectDoneMsg)

	// The selected commit (index 0) should be included in entries
	if len(msg.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(msg.Entries))
	}
	if msg.Entries[0].Commit.Hash != "hash0" {
		t.Errorf("entry hash = %s, want hash0", msg.Entries[0].Commit.Hash)
	}
	// Base should be the parent (index 1)
	if msg.BaseCommit != "hash1" {
		t.Errorf("BaseCommit = %s, want hash1", msg.BaseCommit)
	}
}

func TestSelectModel_SelectCommit_IncludesAllNewerCommits(t *testing.T) {
	m := &SelectModel{
		commits: []RebaseCommit{
			{Hash: "hash0", ShortHash: "abc0000", Message: "commit 0"},
			{Hash: "hash1", ShortHash: "abc1111", Message: "commit 1"},
			{Hash: "hash2", ShortHash: "abc2222", Message: "commit 2"},
			{Hash: "hash3", ShortHash: "abc3333", Message: "commit 3"},
		},
		cursor: 2, // Select commit at index 2
	}

	cmd := m.selectCommit()
	msg := cmd().(SelectDoneMsg)

	// Should include commits 0, 1, 2 (3 entries)
	if len(msg.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(msg.Entries))
	}
	// Base should be commit at index 3 (parent of selected)
	if msg.BaseCommit != "hash3" {
		t.Errorf("BaseCommit = %s, want hash3", msg.BaseCommit)
	}
}

func TestSelectModel_GetParentHash(t *testing.T) {
	// This test requires a real git repo
	repoDir := testutil.TestRepo(t)

	// Create two commits
	testutil.CreateFile(t, repoDir, "file.txt", "content1")
	testutil.GitAdd(t, repoDir, "file.txt")
	parentShortHash := testutil.GitCommit(t, repoDir, "parent commit")

	testutil.CreateFile(t, repoDir, "file.txt", "content2")
	testutil.GitAdd(t, repoDir, "file.txt")
	childShortHash := testutil.GitCommit(t, repoDir, "child commit")

	m := &SelectModel{}
	// Change to repo dir for git command
	oldDir, _ := os.Getwd()
	_ = os.Chdir(repoDir)
	defer func() { _ = os.Chdir(oldDir) }()

	got := m.getParentHash(childShortHash)
	// getParentHash returns a full hash; verify it matches the parent's short hash
	if !strings.HasPrefix(got, parentShortHash) {
		t.Errorf("getParentHash(%s) = %s, want prefix %s", childShortHash, got, parentShortHash)
	}
}

func TestSelectModel_GetParentHash_RootCommit(t *testing.T) {
	repoDir := testutil.TestRepo(t)

	// Create only one commit (root)
	testutil.CreateFile(t, repoDir, "file.txt", "content")
	testutil.GitAdd(t, repoDir, "file.txt")
	rootHash := testutil.GitCommit(t, repoDir, "root commit")

	m := &SelectModel{}
	oldDir, _ := os.Getwd()
	_ = os.Chdir(repoDir)
	defer func() { _ = os.Chdir(oldDir) }()

	got := m.getParentHash(rootHash)
	if got != "" {
		t.Errorf("getParentHash(root) = %s, want empty string", got)
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name     string
		offset   time.Duration
		expected string
	}{
		{"just now", 30 * time.Second, "just now"},
		{"1 minute ago", 1 * time.Minute, "1 minute ago"},
		{"N minutes ago", 15 * time.Minute, "15 minutes ago"},
		{"1 hour ago", 1 * time.Hour, "1 hour ago"},
		{"N hours ago", 5 * time.Hour, "5 hours ago"},
		{"yesterday", 24 * time.Hour, "yesterday"},
		{"N days ago", 3 * 24 * time.Hour, "3 days ago"},
		{"1 week ago", 7 * 24 * time.Hour, "1 week ago"},
		{"N weeks ago", 21 * 24 * time.Hour, "3 weeks ago"},
		{"1 month ago", 30 * 24 * time.Hour, "1 month ago"},
		{"N months ago", 90 * 24 * time.Hour, "3 months ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := time.Now().Add(-tt.offset)
			got := formatAge(input)
			if got != tt.expected {
				t.Errorf("formatAge(Now - %v) = %q, want %q", tt.offset, got, tt.expected)
			}
		})
	}
}
