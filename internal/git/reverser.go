package git

import (
	"fmt"
	"os/exec"
)

// Reverser handles reversing commits.
type Reverser struct {
	workDir string
}

// NewReverser creates a new git reverser for the given directory.
func NewReverser(workDir string) *Reverser {
	return &Reverser{workDir: workDir}
}

// Reverse undoes the HEAD commit, keeping changes in the working directory.
func (r *Reverser) Reverse(force bool) error {
	collector := NewCollector(r.workDir)

	// Check if we're on the initial commit
	if collector.IsInitialCommit() {
		return fmt.Errorf("cannot reverse: HEAD is the initial commit")
	}

	// Check if commit has been pushed
	pushed, err := collector.IsCommitPushed()
	if err != nil {
		return fmt.Errorf("failed to check if commit is pushed: %w", err)
	}

	if pushed && !force {
		return &PushedCommitError{}
	}

	// Perform soft reset (keeps changes staged)
	cmd := exec.Command("git", "reset", "--soft", "HEAD~1")
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

// WasPushed returns true if the HEAD commit was pushed before reverse.
func (r *Reverser) WasPushed() (bool, error) {
	collector := NewCollector(r.workDir)
	return collector.IsCommitPushed()
}

// PushedCommitError indicates attempting to reverse a pushed commit without force.
type PushedCommitError struct{}

func (e *PushedCommitError) Error() string {
	return "HEAD commit has been pushed to origin.\n" +
		"Reversing will require force-push to sync with remote.\n" +
		"Use --reverse --force to proceed."
}
