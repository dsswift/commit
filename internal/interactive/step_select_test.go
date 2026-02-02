package interactive

import (
	"os"
	"testing"
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
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create two commits
	createFile(t, repoDir, "file.txt", "content1")
	gitAdd(t, repoDir, "file.txt")
	parentHash := gitCommit(t, repoDir, "parent commit")

	createFile(t, repoDir, "file.txt", "content2")
	gitAdd(t, repoDir, "file.txt")
	childHash := gitCommit(t, repoDir, "child commit")

	m := &SelectModel{}
	// Change to repo dir for git command
	oldDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(oldDir)

	got := m.getParentHash(childHash)
	if got != parentHash {
		t.Errorf("getParentHash(%s) = %s, want %s", childHash, got, parentHash)
	}
}

func TestSelectModel_GetParentHash_RootCommit(t *testing.T) {
	repoDir, cleanup := testRepo(t)
	defer cleanup()

	// Create only one commit (root)
	createFile(t, repoDir, "file.txt", "content")
	gitAdd(t, repoDir, "file.txt")
	rootHash := gitCommit(t, repoDir, "root commit")

	m := &SelectModel{}
	oldDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(oldDir)

	got := m.getParentHash(rootHash)
	if got != "" {
		t.Errorf("getParentHash(root) = %s, want empty string", got)
	}
}
