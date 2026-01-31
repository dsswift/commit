package interactive

import (
	"strings"
	"testing"
	"time"
)

func TestRebaser_GenerateTodo(t *testing.T) {
	rebaser := NewRebaser("/tmp")

	entries := []RebaseEntry{
		{
			Commit:    RebaseCommit{ShortHash: "abc1234", Message: "feat: add user authentication"},
			Operation: OpPick,
		},
		{
			Commit:    RebaseCommit{ShortHash: "def5678", Message: "fix: login validation"},
			Operation: OpPick,
		},
	}

	todo := rebaser.GenerateTodo(entries)

	expected := "pick abc1234 feat: add user authentication\npick def5678 fix: login validation\n"
	if todo != expected {
		t.Errorf("GenerateTodo() =\n%q\nwant\n%q", todo, expected)
	}
}

func TestRebaser_GenerateTodo_WithSquash(t *testing.T) {
	rebaser := NewRebaser("/tmp")

	entries := []RebaseEntry{
		{
			Commit:    RebaseCommit{ShortHash: "abc1234", Message: "feat: add user authentication"},
			Operation: OpPick,
		},
		{
			Commit:    RebaseCommit{ShortHash: "def5678", Message: "fix: login validation"},
			Operation: OpSquash,
		},
	}

	todo := rebaser.GenerateTodo(entries)

	if !strings.Contains(todo, "pick abc1234") {
		t.Error("expected todo to contain 'pick abc1234'")
	}
	if !strings.Contains(todo, "squash def5678") {
		t.Error("expected todo to contain 'squash def5678'")
	}
}

func TestRebaser_GenerateTodo_WithReword(t *testing.T) {
	rebaser := NewRebaser("/tmp")

	entries := []RebaseEntry{
		{
			Commit:    RebaseCommit{ShortHash: "abc1234", Message: "feat: add user authentication"},
			Operation: OpReword,
		},
	}

	todo := rebaser.GenerateTodo(entries)

	if !strings.Contains(todo, "reword abc1234") {
		t.Error("expected todo to contain 'reword abc1234'")
	}
}

func TestRebaser_GenerateTodo_WithDrop(t *testing.T) {
	rebaser := NewRebaser("/tmp")

	entries := []RebaseEntry{
		{
			Commit:    RebaseCommit{ShortHash: "abc1234", Message: "chore: debug logging"},
			Operation: OpDrop,
		},
	}

	todo := rebaser.GenerateTodo(entries)

	if !strings.Contains(todo, "drop abc1234") {
		t.Error("expected todo to contain 'drop abc1234'")
	}
}

func TestRebaser_GenerateTodo_ComplexPlan(t *testing.T) {
	rebaser := NewRebaser("/tmp")

	entries := []RebaseEntry{
		{
			Commit:    RebaseCommit{ShortHash: "abc1234", Message: "feat: add user authentication"},
			Operation: OpPick,
		},
		{
			Commit:    RebaseCommit{ShortHash: "def5678", Message: "fix: login validation"},
			Operation: OpSquash,
		},
		{
			Commit:    RebaseCommit{ShortHash: "ghi9012", Message: "fix: auth edge case"},
			Operation: OpSquash,
		},
		{
			Commit:    RebaseCommit{ShortHash: "jkl3456", Message: "feat: password reset"},
			Operation: OpReword,
		},
		{
			Commit:    RebaseCommit{ShortHash: "mno7890", Message: "chore: debug logging"},
			Operation: OpDrop,
		},
	}

	todo := rebaser.GenerateTodo(entries)

	// Verify order and operations
	lines := strings.Split(strings.TrimSpace(todo), "\n")
	if len(lines) != 5 {
		t.Errorf("expected 5 lines, got %d", len(lines))
	}

	expectedOps := []string{"pick", "squash", "squash", "reword", "drop"}
	for i, line := range lines {
		if !strings.HasPrefix(line, expectedOps[i]) {
			t.Errorf("line %d: expected to start with %q, got %q", i, expectedOps[i], line)
		}
	}
}

func TestRebaser_GenerateTodo_Empty(t *testing.T) {
	rebaser := NewRebaser("/tmp")

	todo := rebaser.GenerateTodo([]RebaseEntry{})

	if todo != "\n" {
		t.Errorf("GenerateTodo(empty) = %q, want empty line", todo)
	}
}

func TestRebaser_GenerateTodo_PreservesMessageContent(t *testing.T) {
	rebaser := NewRebaser("/tmp")

	// Message with special characters
	entries := []RebaseEntry{
		{
			Commit:    RebaseCommit{ShortHash: "abc1234", Message: "fix: handle edge case (issue #123)"},
			Operation: OpPick,
		},
	}

	todo := rebaser.GenerateTodo(entries)

	if !strings.Contains(todo, "fix: handle edge case (issue #123)") {
		t.Error("message content was not preserved")
	}
}

func TestRebaser_GenerateTodo_UsesOriginalMessage(t *testing.T) {
	rebaser := NewRebaser("/tmp")

	// Even if NewMessage is set, GenerateTodo uses original message
	// (NewMessage is applied via GIT_EDITOR during actual rebase)
	entries := []RebaseEntry{
		{
			Commit:        RebaseCommit{ShortHash: "abc1234", Message: "original message"},
			Operation:     OpReword,
			NewMessage:    "new message",
			MessageEdited: true,
		},
	}

	todo := rebaser.GenerateTodo(entries)

	// Todo should use original message
	if !strings.Contains(todo, "original message") {
		t.Error("expected todo to contain original message")
	}
}

// Helper function to create test commits
func makeTestCommit(hash, message string) RebaseCommit {
	return RebaseCommit{
		Hash:      hash + "000000",
		ShortHash: hash,
		Message:   message,
		Author:    "Test User",
		Date:      time.Now(),
		IsPushed:  false,
	}
}

func TestRebaser_NewRebaser(t *testing.T) {
	r := NewRebaser("/test/path")
	if r.workDir != "/test/path" {
		t.Errorf("workDir = %q, want %q", r.workDir, "/test/path")
	}
}
