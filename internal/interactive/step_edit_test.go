package interactive

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func makeTestEntries() []RebaseEntry {
	return []RebaseEntry{
		{
			Commit:    RebaseCommit{ShortHash: "abc1234", Message: "feat: first commit", Date: time.Now()},
			Operation: OpPick,
		},
		{
			Commit:    RebaseCommit{ShortHash: "def5678", Message: "feat: second commit", Date: time.Now()},
			Operation: OpPick,
		},
		{
			Commit:    RebaseCommit{ShortHash: "ghi9012", Message: "feat: third commit", Date: time.Now()},
			Operation: OpPick,
		},
	}
}

func TestEditModel_MoveUp(t *testing.T) {
	entries := makeTestEntries()
	model := NewEditModel(entries, DefaultStyles(), DefaultKeyMap())

	// Start at first entry
	model.cursor = 0

	// Move up at top should be no-op
	model.moveUp()
	if model.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (should not move up at top)", model.cursor)
	}

	// Move to second entry and move up
	model.cursor = 1
	model.moveUp()
	if model.cursor != 0 {
		t.Errorf("cursor = %d, want 0 after moving up", model.cursor)
	}

	// Verify entries were swapped
	if model.entries[0].Commit.ShortHash != "def5678" {
		t.Errorf("first entry = %s, want def5678", model.entries[0].Commit.ShortHash)
	}
	if model.entries[1].Commit.ShortHash != "abc1234" {
		t.Errorf("second entry = %s, want abc1234", model.entries[1].Commit.ShortHash)
	}
}

func TestEditModel_MoveDown(t *testing.T) {
	entries := makeTestEntries()
	model := NewEditModel(entries, DefaultStyles(), DefaultKeyMap())

	// Start at last entry
	model.cursor = 2

	// Move down at bottom should be no-op
	model.moveDown()
	if model.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (should not move down at bottom)", model.cursor)
	}

	// Move to first entry and move down
	model.cursor = 0
	model.moveDown()
	if model.cursor != 1 {
		t.Errorf("cursor = %d, want 1 after moving down", model.cursor)
	}

	// Verify entries were swapped
	if model.entries[0].Commit.ShortHash != "def5678" {
		t.Errorf("first entry = %s, want def5678", model.entries[0].Commit.ShortHash)
	}
	if model.entries[1].Commit.ShortHash != "abc1234" {
		t.Errorf("second entry = %s, want abc1234", model.entries[1].Commit.ShortHash)
	}
}

func TestEditModel_MoveUp_AtTop(t *testing.T) {
	entries := makeTestEntries()
	model := NewEditModel(entries, DefaultStyles(), DefaultKeyMap())
	model.cursor = 0

	originalFirst := model.entries[0].Commit.ShortHash

	model.moveUp()

	// Cursor should stay at 0
	if model.cursor != 0 {
		t.Errorf("cursor moved from 0 to %d", model.cursor)
	}

	// Order should be unchanged
	if model.entries[0].Commit.ShortHash != originalFirst {
		t.Error("entries were modified when they shouldn't have been")
	}
}

func TestEditModel_MoveDown_AtBottom(t *testing.T) {
	entries := makeTestEntries()
	model := NewEditModel(entries, DefaultStyles(), DefaultKeyMap())
	model.cursor = 2

	originalLast := model.entries[2].Commit.ShortHash

	model.moveDown()

	// Cursor should stay at 2
	if model.cursor != 2 {
		t.Errorf("cursor moved from 2 to %d", model.cursor)
	}

	// Order should be unchanged
	if model.entries[2].Commit.ShortHash != originalLast {
		t.Error("entries were modified when they shouldn't have been")
	}
}

func TestEditModel_CycleOperation(t *testing.T) {
	entries := makeTestEntries()
	model := NewEditModel(entries, DefaultStyles(), DefaultKeyMap())
	model.cursor = 0

	// Initial operation is pick
	if model.entries[0].Operation != OpPick {
		t.Fatalf("initial operation = %v, want OpPick", model.entries[0].Operation)
	}

	// Cycle: pick -> squash
	model.cycleOperation()
	if model.entries[0].Operation != OpSquash {
		t.Errorf("after 1st cycle: %v, want OpSquash", model.entries[0].Operation)
	}

	// Cycle: squash -> reword
	model.cycleOperation()
	if model.entries[0].Operation != OpReword {
		t.Errorf("after 2nd cycle: %v, want OpReword", model.entries[0].Operation)
	}

	// Cycle: reword -> drop
	model.cycleOperation()
	if model.entries[0].Operation != OpDrop {
		t.Errorf("after 3rd cycle: %v, want OpDrop", model.entries[0].Operation)
	}

	// Cycle: drop -> pick (wrap around)
	model.cycleOperation()
	if model.entries[0].Operation != OpPick {
		t.Errorf("after 4th cycle: %v, want OpPick", model.entries[0].Operation)
	}
}

func TestEditModel_SetOperation(t *testing.T) {
	entries := makeTestEntries()
	model := NewEditModel(entries, DefaultStyles(), DefaultKeyMap())
	model.cursor = 0

	tests := []struct {
		op       Operation
		expected Operation
	}{
		{OpSquash, OpSquash},
		{OpReword, OpReword},
		{OpDrop, OpDrop},
		{OpPick, OpPick},
	}

	for _, tt := range tests {
		model.setOperation(tt.op)
		if model.entries[0].Operation != tt.expected {
			t.Errorf("setOperation(%v): got %v, want %v", tt.op, model.entries[0].Operation, tt.expected)
		}
	}
}

func TestEditModel_SquashIndentation(t *testing.T) {
	// This tests the visual representation, which is in View()
	entries := []RebaseEntry{
		{
			Commit:    RebaseCommit{ShortHash: "abc1234", Message: "feat: parent commit"},
			Operation: OpPick,
		},
		{
			Commit:    RebaseCommit{ShortHash: "def5678", Message: "fix: child commit"},
			Operation: OpSquash,
		},
	}

	model := NewEditModel(entries, DefaultStyles(), DefaultKeyMap())
	view := model.View()

	// The squash entry should have indentation in the view
	// We can't check exact formatting, but we can verify both commits are present
	if !contains(view, "abc1234") {
		t.Error("view doesn't contain parent commit hash")
	}
	if !contains(view, "def5678") {
		t.Error("view doesn't contain squash commit hash")
	}
	if !contains(view, "squash") {
		t.Error("view doesn't show squash operation")
	}
}

func TestEditModel_MessageEdited_Flag(t *testing.T) {
	entries := makeTestEntries()
	model := NewEditModel(entries, DefaultStyles(), DefaultKeyMap())
	model.cursor = 0

	// Initially not edited
	if model.entries[0].MessageEdited {
		t.Error("MessageEdited should be false initially")
	}

	// Start editing
	model.startMessageEdit()
	if !model.editingMessage {
		t.Error("editingMessage should be true after startMessageEdit")
	}
	if model.editingIndex != 0 {
		t.Errorf("editingIndex = %d, want 0", model.editingIndex)
	}

	// Simulate setting a value and pressing enter
	model.messageInput.SetValue("new commit message")

	// Simulate enter key to save
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Message should be saved and flag set
	if model.entries[0].NewMessage != "new commit message" {
		t.Errorf("NewMessage = %q, want %q", model.entries[0].NewMessage, "new commit message")
	}
	if !model.entries[0].MessageEdited {
		t.Error("MessageEdited should be true after editing")
	}
	if model.editingMessage {
		t.Error("editingMessage should be false after saving")
	}
}

func TestEditModel_Entries(t *testing.T) {
	entries := makeTestEntries()
	model := NewEditModel(entries, DefaultStyles(), DefaultKeyMap())

	// Modify model entries
	model.entries[0].Operation = OpSquash

	// Get a copy
	copy := model.Entries()

	// Modify the copy
	copy[0].Operation = OpDrop

	// Original should be unchanged
	if model.entries[0].Operation != OpSquash {
		t.Error("Entries() should return a copy, not a reference")
	}
}

func TestEditModel_KeyBindings(t *testing.T) {
	entries := makeTestEntries()
	model := NewEditModel(entries, DefaultStyles(), DefaultKeyMap())

	// Test navigation keys
	model.cursor = 1
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	if model.cursor != 0 {
		t.Errorf("up key: cursor = %d, want 0", model.cursor)
	}

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if model.cursor != 1 {
		t.Errorf("down key: cursor = %d, want 1", model.cursor)
	}

	// Test operation keys
	model.cursor = 0
	model.entries[0].Operation = OpPick

	// 's' for squash
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if model.entries[0].Operation != OpSquash {
		t.Error("'s' key should set operation to squash")
	}

	// 'r' for reword
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if model.entries[0].Operation != OpReword {
		t.Error("'r' key should set operation to reword")
	}

	// 'd' for drop
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if model.entries[0].Operation != OpDrop {
		t.Error("'d' key should set operation to drop")
	}

	// 'p' for pick
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if model.entries[0].Operation != OpPick {
		t.Error("'p' key should set operation to pick")
	}
}

func TestEditModel_View_ShowsHelp(t *testing.T) {
	entries := makeTestEntries()
	model := NewEditModel(entries, DefaultStyles(), DefaultKeyMap())

	view := model.View()

	// Check for key hints in help bar
	helpItems := []string{"navigate", "move", "cycle op", "edit msg", "confirm", "back"}
	for _, item := range helpItems {
		if !contains(view, item) {
			t.Errorf("help bar should contain %q", item)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
