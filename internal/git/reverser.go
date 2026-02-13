package git

import (
	"fmt"
	"os/exec"

	"github.com/dsswift/commit/internal/assert"
)

// Reverser handles reversing commits.
type Reverser struct {
	workDir string
}

// NewReverser creates a new git reverser for the given directory.
func NewReverser(workDir string) *Reverser {
	return &Reverser{workDir: workDir}
}

// Reverse undoes the last count commits, keeping changes in the working directory.
func (r *Reverser) Reverse(count int, force bool) error {
	assert.Positive(count, "reverse count must be positive")

	collector := NewCollector(r.workDir)

	// Check that enough commits exist to reverse
	if err := collector.HasCommitDepth(count); err != nil {
		return err
	}

	// Check if the oldest commit being reversed has been pushed
	pushed, err := r.WasPushed(count)
	if err != nil {
		return fmt.Errorf("failed to check if commit is pushed: %w", err)
	}

	if pushed && !force {
		return &PushedCommitError{Count: count}
	}

	// Perform soft reset (keeps changes staged)
	ref := fmt.Sprintf("HEAD~%d", count)
	cmd := exec.Command("git", "reset", "--soft", ref)
	cmd.Dir = r.workDir

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to soft reset: %s: %w", string(out), err)
	}

	// Unstage all files (put in working directory)
	stager := NewStager(r.workDir)
	if err := stager.UnstageAll(); err != nil {
		return fmt.Errorf("failed to unstage files: %w", err)
	}

	return nil
}

// WasPushed returns true if any of the last count commits have been pushed.
// Checks the oldest commit in the range (HEAD~(count-1)) since if that one
// is pushed, all newer ones must also be.
func (r *Reverser) WasPushed(count int) (bool, error) {
	assert.Positive(count, "reverse count must be positive")

	collector := NewCollector(r.workDir)
	ref := fmt.Sprintf("HEAD~%d", count-1)
	return collector.IsRefPushed(ref)
}

// PushedCommitError indicates attempting to reverse a pushed commit without force.
type PushedCommitError struct {
	Count int
}

func (e *PushedCommitError) Error() string {
	if e.Count > 1 {
		return fmt.Sprintf("One or more of the last %d commits have been pushed to origin.\n", e.Count) +
			"Reversing will require force-push to sync with remote.\n" +
			"Use --reverse --force to proceed."
	}
	return "HEAD commit has been pushed to origin.\n" +
		"Reversing will require force-push to sync with remote.\n" +
		"Use --reverse --force to proceed."
}
