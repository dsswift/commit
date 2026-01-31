// Package interactive provides a TUI for interactive git rebase operations.
package interactive

import (
	"time"
)

// WizardStep represents the current step in the interactive rebase wizard.
type WizardStep int

const (
	StepSelect WizardStep = iota
	StepPushWarning
	StepEdit
	StepSquashMessage
	StepConfirm
)

// String returns the display name for the step.
func (s WizardStep) String() string {
	switch s {
	case StepSelect:
		return "Select Commit"
	case StepPushWarning:
		return "Push Warning"
	case StepEdit:
		return "Edit"
	case StepSquashMessage:
		return "Squash Message"
	case StepConfirm:
		return "Confirm"
	default:
		return "Unknown"
	}
}

// Operation represents a rebase operation for a commit.
type Operation int

const (
	OpPick Operation = iota
	OpSquash
	OpReword
	OpDrop
)

// String returns the full name of the operation.
func (o Operation) String() string {
	switch o {
	case OpPick:
		return "pick"
	case OpSquash:
		return "squash"
	case OpReword:
		return "reword"
	case OpDrop:
		return "drop"
	default:
		return "unknown"
	}
}

// ShortString returns the single-character shortcut for the operation.
func (o Operation) ShortString() string {
	switch o {
	case OpPick:
		return "p"
	case OpSquash:
		return "s"
	case OpReword:
		return "r"
	case OpDrop:
		return "d"
	default:
		return "?"
	}
}

// Next returns the next operation in the cycle (pick → squash → reword → drop → pick).
func (o Operation) Next() Operation {
	return (o + 1) % 4
}

// RebaseCommit represents a commit that can be included in a rebase.
type RebaseCommit struct {
	Hash      string
	ShortHash string
	Message   string
	Author    string
	Date      time.Time
	IsPushed  bool
}

// RebaseEntry represents a commit with its rebase operation.
type RebaseEntry struct {
	Commit        RebaseCommit
	Operation     Operation
	NewMessage    string // For reword or squash parent
	MessageEdited bool   // True if user pressed 'e' to edit message
}

// IsSquashParent returns true if this entry has any squash children following it.
func (e *RebaseEntry) IsSquashParent(entries []RebaseEntry, index int) bool {
	if index >= len(entries)-1 {
		return false
	}
	// A pick entry is a squash parent if the next entry is a squash
	if e.Operation != OpPick {
		return false
	}
	return entries[index+1].Operation == OpSquash
}

// SquashChildren returns the indices of all squash entries that belong to this parent.
func (e *RebaseEntry) SquashChildren(entries []RebaseEntry, parentIndex int) []int {
	if e.Operation != OpPick {
		return nil
	}

	var children []int
	for i := parentIndex + 1; i < len(entries); i++ {
		if entries[i].Operation == OpSquash {
			children = append(children, i)
		} else {
			// Stop at the next non-squash entry
			break
		}
	}
	return children
}

// FindSquashParent returns the index of the pick entry that this squash belongs to.
// Returns -1 if not found or if entry is not a squash.
func FindSquashParent(entries []RebaseEntry, squashIndex int) int {
	if squashIndex >= len(entries) || entries[squashIndex].Operation != OpSquash {
		return -1
	}

	// Walk backwards to find the nearest pick
	for i := squashIndex - 1; i >= 0; i-- {
		if entries[i].Operation == OpPick {
			return i
		}
	}
	return -1
}

// CountPushedCommits returns the number of commits that have been pushed.
func CountPushedCommits(commits []RebaseCommit) int {
	count := 0
	for _, c := range commits {
		if c.IsPushed {
			count++
		}
	}
	return count
}

// GetEffectiveMessage returns the message to use for a commit.
// If NewMessage is set (edited), returns that; otherwise returns the original.
func (e *RebaseEntry) GetEffectiveMessage() string {
	if e.NewMessage != "" {
		return e.NewMessage
	}
	return e.Commit.Message
}
