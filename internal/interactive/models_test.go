package interactive

import (
	"testing"
	"time"
)

func TestOperation_String(t *testing.T) {
	tests := []struct {
		op       Operation
		expected string
	}{
		{OpPick, "pick"},
		{OpSquash, "squash"},
		{OpReword, "reword"},
		{OpDrop, "drop"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.op.String(); got != tt.expected {
				t.Errorf("Operation.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestOperation_ShortString(t *testing.T) {
	tests := []struct {
		op       Operation
		expected string
	}{
		{OpPick, "p"},
		{OpSquash, "s"},
		{OpReword, "r"},
		{OpDrop, "d"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.op.ShortString(); got != tt.expected {
				t.Errorf("Operation.ShortString() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestOperation_Next(t *testing.T) {
	tests := []struct {
		op       Operation
		expected Operation
	}{
		{OpPick, OpSquash},
		{OpSquash, OpReword},
		{OpReword, OpDrop},
		{OpDrop, OpPick},
	}

	for _, tt := range tests {
		t.Run(tt.op.String(), func(t *testing.T) {
			if got := tt.op.Next(); got != tt.expected {
				t.Errorf("Operation.Next() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestWizardStep_String(t *testing.T) {
	tests := []struct {
		step     WizardStep
		expected string
	}{
		{StepSelect, "Select Commit"},
		{StepPushWarning, "Push Warning"},
		{StepEdit, "Edit"},
		{StepSquashMessage, "Squash Message"},
		{StepConfirm, "Confirm"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.step.String(); got != tt.expected {
				t.Errorf("WizardStep.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestRebaseEntry_IsSquashParent(t *testing.T) {
	entries := []RebaseEntry{
		{Commit: RebaseCommit{ShortHash: "abc"}, Operation: OpPick},
		{Commit: RebaseCommit{ShortHash: "def"}, Operation: OpSquash},
		{Commit: RebaseCommit{ShortHash: "ghi"}, Operation: OpPick},
	}

	tests := []struct {
		index    int
		expected bool
	}{
		{0, true},  // First pick has squash child
		{1, false}, // Squash can't be a parent
		{2, false}, // Last pick has no children
	}

	for _, tt := range tests {
		entry := entries[tt.index]
		if got := entry.IsSquashParent(entries, tt.index); got != tt.expected {
			t.Errorf("IsSquashParent(index=%d) = %v, want %v", tt.index, got, tt.expected)
		}
	}
}

func TestRebaseEntry_SquashChildren(t *testing.T) {
	entries := []RebaseEntry{
		{Commit: RebaseCommit{ShortHash: "abc"}, Operation: OpPick},
		{Commit: RebaseCommit{ShortHash: "def"}, Operation: OpSquash},
		{Commit: RebaseCommit{ShortHash: "ghi"}, Operation: OpSquash},
		{Commit: RebaseCommit{ShortHash: "jkl"}, Operation: OpPick},
	}

	// First pick should have 2 squash children
	children := entries[0].SquashChildren(entries, 0)
	if len(children) != 2 {
		t.Errorf("expected 2 squash children, got %d", len(children))
	}
	if children[0] != 1 || children[1] != 2 {
		t.Errorf("expected children [1, 2], got %v", children)
	}

	// Last pick should have no children
	children = entries[3].SquashChildren(entries, 3)
	if len(children) != 0 {
		t.Errorf("expected 0 squash children, got %d", len(children))
	}

	// Squash entries should return nil
	children = entries[1].SquashChildren(entries, 1)
	if children != nil {
		t.Errorf("expected nil for squash entry, got %v", children)
	}
}

func TestFindSquashParent(t *testing.T) {
	entries := []RebaseEntry{
		{Commit: RebaseCommit{ShortHash: "abc"}, Operation: OpPick},
		{Commit: RebaseCommit{ShortHash: "def"}, Operation: OpSquash},
		{Commit: RebaseCommit{ShortHash: "ghi"}, Operation: OpSquash},
		{Commit: RebaseCommit{ShortHash: "jkl"}, Operation: OpPick},
	}

	tests := []struct {
		squashIndex int
		expected    int
	}{
		{1, 0},  // First squash -> first pick
		{2, 0},  // Second squash -> first pick
		{0, -1}, // Not a squash
		{3, -1}, // Not a squash
	}

	for _, tt := range tests {
		if got := FindSquashParent(entries, tt.squashIndex); got != tt.expected {
			t.Errorf("FindSquashParent(index=%d) = %d, want %d", tt.squashIndex, got, tt.expected)
		}
	}
}

func TestCountPushedCommits(t *testing.T) {
	commits := []RebaseCommit{
		{ShortHash: "abc", IsPushed: true},
		{ShortHash: "def", IsPushed: false},
		{ShortHash: "ghi", IsPushed: true},
	}

	if got := CountPushedCommits(commits); got != 2 {
		t.Errorf("CountPushedCommits() = %d, want 2", got)
	}

	// Empty slice
	if got := CountPushedCommits(nil); got != 0 {
		t.Errorf("CountPushedCommits(nil) = %d, want 0", got)
	}
}

func TestRebaseEntry_GetEffectiveMessage(t *testing.T) {
	t.Run("returns original message when not edited", func(t *testing.T) {
		entry := RebaseEntry{
			Commit: RebaseCommit{Message: "original message"},
		}
		if got := entry.GetEffectiveMessage(); got != "original message" {
			t.Errorf("GetEffectiveMessage() = %q, want %q", got, "original message")
		}
	})

	t.Run("returns new message when edited", func(t *testing.T) {
		entry := RebaseEntry{
			Commit:     RebaseCommit{Message: "original message"},
			NewMessage: "new message",
		}
		if got := entry.GetEffectiveMessage(); got != "new message" {
			t.Errorf("GetEffectiveMessage() = %q, want %q", got, "new message")
		}
	})
}

func TestRebaseCommit_Fields(t *testing.T) {
	now := time.Now()
	commit := RebaseCommit{
		Hash:      "abc123def456",
		ShortHash: "abc123",
		Message:   "test commit",
		Author:    "Test User",
		Date:      now,
		IsPushed:  true,
	}

	if commit.Hash != "abc123def456" {
		t.Errorf("Hash = %q, want %q", commit.Hash, "abc123def456")
	}
	if commit.ShortHash != "abc123" {
		t.Errorf("ShortHash = %q, want %q", commit.ShortHash, "abc123")
	}
	if commit.Message != "test commit" {
		t.Errorf("Message = %q, want %q", commit.Message, "test commit")
	}
	if commit.Author != "Test User" {
		t.Errorf("Author = %q, want %q", commit.Author, "Test User")
	}
	if !commit.Date.Equal(now) {
		t.Errorf("Date = %v, want %v", commit.Date, now)
	}
	if !commit.IsPushed {
		t.Error("IsPushed = false, want true")
	}
}
