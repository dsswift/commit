package planner

import (
	"errors"
	"fmt"

	"github.com/dsswift/commit/internal/assert"
	"github.com/dsswift/commit/internal/git"
	"github.com/dsswift/commit/pkg/types"
)

// Executor executes a validated commit plan.
type Executor struct {
	workDir   string
	committer *git.Committer
	stager    *git.Stager
	dryRun    bool
}

// NewExecutor creates a new plan executor.
func NewExecutor(workDir string, dryRun bool) *Executor {
	return &Executor{
		workDir:   workDir,
		committer: git.NewCommitter(workDir),
		stager:    git.NewStager(workDir),
		dryRun:    dryRun,
	}
}

// ExecutionProgress is called for each commit being executed.
type ExecutionProgress func(current, total int, commit types.PlannedCommit)

// Execute runs the commit plan and returns the executed commits.
func (e *Executor) Execute(plan *types.CommitPlan, progress ExecutionProgress) ([]types.ExecutedCommit, error) {
	// PRECONDITIONS
	assert.NotNil(plan, "plan cannot be nil")
	assert.NotEmpty(plan.Commits, "plan must have commits")

	var executed []types.ExecutedCommit
	total := len(plan.Commits)

	for i, planned := range plan.Commits {
		// Report progress
		if progress != nil {
			progress(i+1, total, planned)
		}

		if e.dryRun {
			// In dry-run mode, just create a fake executed commit
			var fullMessage string
			if planned.Scope != nil && *planned.Scope != "" {
				fullMessage = fmt.Sprintf("%s(%s): %s", planned.Type, *planned.Scope, planned.Message)
			} else {
				fullMessage = fmt.Sprintf("%s: %s", planned.Type, planned.Message)
			}

			executed = append(executed, types.ExecutedCommit{
				Hash:    "(dry-run)",
				Type:    planned.Type,
				Scope:   planned.Scope,
				Message: fullMessage,
				Files:   planned.Files,
			})
			continue
		}

		// Execute the actual commit
		result, err := e.committer.ExecutePlannedCommit(planned)
		if err != nil {
			// Skip commits where all files were directories (nothing to stage)
			var noStagedErr *git.NoStagedFilesError
			if errors.As(err, &noStagedErr) {
				// Skip this commit silently - all paths were directories
				continue
			}
			return executed, &ExecutionError{
				CommitIndex: i,
				Planned:     planned,
				Err:         err,
			}
		}

		executed = append(executed, *result)
	}

	// POSTCONDITIONS - we may have fewer commits if some were skipped (directories only)
	if !e.dryRun && len(executed) == 0 {
		return executed, fmt.Errorf("no commits were executed (all planned commits contained only directories)")
	}

	return executed, nil
}

// ExecuteSingle executes a single commit from the plan.
func (e *Executor) ExecuteSingle(planned types.PlannedCommit) (*types.ExecutedCommit, error) {
	if e.dryRun {
		var fullMessage string
		if planned.Scope != nil && *planned.Scope != "" {
			fullMessage = fmt.Sprintf("%s(%s): %s", planned.Type, *planned.Scope, planned.Message)
		} else {
			fullMessage = fmt.Sprintf("%s: %s", planned.Type, planned.Message)
		}

		return &types.ExecutedCommit{
			Hash:    "(dry-run)",
			Type:    planned.Type,
			Scope:   planned.Scope,
			Message: fullMessage,
			Files:   planned.Files,
		}, nil
	}

	return e.committer.ExecutePlannedCommit(planned)
}

// PreviewPlan returns a human-readable preview of the plan.
func PreviewPlan(plan *types.CommitPlan) string {
	if plan == nil || len(plan.Commits) == 0 {
		return "No commits planned"
	}

	result := fmt.Sprintf("📋 %d commits planned:\n", len(plan.Commits))

	for i, commit := range plan.Commits {
		var msg string
		if commit.Scope != nil && *commit.Scope != "" {
			msg = fmt.Sprintf("%s(%s): %s", commit.Type, *commit.Scope, commit.Message)
		} else {
			msg = fmt.Sprintf("%s: %s", commit.Type, commit.Message)
		}

		result += fmt.Sprintf("\n  [%d/%d] %s\n", i+1, len(plan.Commits), msg)

		for _, file := range commit.Files {
			result += fmt.Sprintf("       └─ %s\n", file)
		}

		if commit.Reasoning != "" {
			result += fmt.Sprintf("       📝 %s\n", commit.Reasoning)
		}
	}

	return result
}

// ExecutionError represents a failure during plan execution.
type ExecutionError struct {
	CommitIndex int
	Planned     types.PlannedCommit
	Err         error
}

func (e *ExecutionError) Error() string {
	var msg string
	if e.Planned.Scope != nil && *e.Planned.Scope != "" {
		msg = fmt.Sprintf("%s(%s): %s", e.Planned.Type, *e.Planned.Scope, e.Planned.Message)
	} else {
		msg = fmt.Sprintf("%s: %s", e.Planned.Type, e.Planned.Message)
	}

	return fmt.Sprintf("failed to execute commit %d (%s): %v", e.CommitIndex+1, msg, e.Err)
}

func (e *ExecutionError) Unwrap() error {
	return e.Err
}
