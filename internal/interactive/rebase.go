package interactive

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Rebaser handles git rebase execution.
type Rebaser struct {
	workDir string
}

// NewRebaser creates a new rebaser for the given directory.
func NewRebaser(workDir string) *Rebaser {
	return &Rebaser{workDir: workDir}
}

// Execute runs the interactive rebase with the given plan.
func (r *Rebaser) Execute(entries []RebaseEntry, baseCommit string) error {
	if len(entries) == 0 {
		return fmt.Errorf("no commits to rebase")
	}

	// Generate the todo list content
	todoContent := r.generateTodo(entries)

	// Create a temporary script that will be used as GIT_SEQUENCE_EDITOR
	scriptPath, cleanup, err := r.createTodoScript(todoContent)
	if err != nil {
		return fmt.Errorf("failed to create todo script: %w", err)
	}
	defer cleanup()

	// Create reword messages script if needed
	rewordScript, rewordCleanup, err := r.createRewordScript(entries)
	if err != nil {
		return fmt.Errorf("failed to create reword script: %w", err)
	}
	defer rewordCleanup()

	// Run git rebase
	var cmd *exec.Cmd
	if baseCommit == "" {
		// Root commit selected - use --root flag
		cmd = exec.Command("git", "rebase", "-i", "--root")
	} else {
		cmd = exec.Command("git", "rebase", "-i", baseCommit)
	}
	cmd.Dir = r.workDir
	cmd.Env = append(os.Environ(),
		"GIT_SEQUENCE_EDITOR="+scriptPath,
	)

	// If we have reword messages, set the editor
	if rewordScript != "" {
		cmd.Env = append(cmd.Env, "GIT_EDITOR="+rewordScript)
	}

	// Capture output for error messages
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rebase failed: %s\n%s", err, string(output))
	}

	return nil
}

// generateTodo creates the git rebase todo list content.
func (r *Rebaser) generateTodo(entries []RebaseEntry) string {
	var lines []string

	for _, entry := range entries {
		op := entry.Operation.String()
		hash := entry.Commit.ShortHash
		msg := entry.Commit.Message

		// For reword, we still use the original message in the todo
		// The actual message change happens via GIT_EDITOR
		lines = append(lines, fmt.Sprintf("%s %s %s", op, hash, msg))
	}

	return strings.Join(lines, "\n") + "\n"
}

// createTodoScript creates a script that replaces the todo file with our content.
func (r *Rebaser) createTodoScript(todoContent string) (string, func(), error) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "commit-rebase-*")
	if err != nil {
		return "", func() {}, err
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	// Write the todo content to a file
	todoPath := filepath.Join(tmpDir, "todo")
	if err := os.WriteFile(todoPath, []byte(todoContent), 0644); err != nil {
		cleanup()
		return "", func() {}, err
	}

	// Create a script that copies our todo to the target
	scriptContent := fmt.Sprintf(`#!/bin/sh
cat "%s" > "$1"
`, todoPath)

	scriptPath := filepath.Join(tmpDir, "sequence-editor.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		cleanup()
		return "", func() {}, err
	}

	return scriptPath, cleanup, nil
}

// createRewordScript creates a script that handles reword message editing.
func (r *Rebaser) createRewordScript(entries []RebaseEntry) (string, func(), error) {
	// Collect all entries that need reword with custom messages
	var rewordMsgs []struct {
		hash    string
		message string
	}

	for _, entry := range entries {
		if entry.NewMessage != "" {
			rewordMsgs = append(rewordMsgs, struct {
				hash    string
				message string
			}{
				hash:    entry.Commit.ShortHash,
				message: entry.NewMessage,
			})
		}
	}

	if len(rewordMsgs) == 0 {
		return "", func() {}, nil
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "commit-reword-*")
	if err != nil {
		return "", func() {}, err
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	// Write message files
	for i, rm := range rewordMsgs {
		msgPath := filepath.Join(tmpDir, fmt.Sprintf("msg_%d.txt", i))
		if err := os.WriteFile(msgPath, []byte(rm.message), 0644); err != nil {
			cleanup()
			return "", func() {}, err
		}
	}

	// Create a script that writes the appropriate message
	// The script checks which commit is being edited and writes the corresponding message
	var scriptLines []string
	scriptLines = append(scriptLines, "#!/bin/sh")
	scriptLines = append(scriptLines, "# Check current commit being rebased")
	scriptLines = append(scriptLines, "CURRENT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo '')")

	for i, rm := range rewordMsgs {
		msgPath := filepath.Join(tmpDir, fmt.Sprintf("msg_%d.txt", i))
		// Check if current commit starts with the short hash
		scriptLines = append(scriptLines, fmt.Sprintf(`if echo "$CURRENT_COMMIT" | grep -q "^%s"; then`, rm.hash))
		scriptLines = append(scriptLines, fmt.Sprintf(`  cat "%s" > "$1"`, msgPath))
		scriptLines = append(scriptLines, "  exit 0")
		scriptLines = append(scriptLines, "fi")
	}

	// If no match, leave the file as-is (for initial squash message combining)
	scriptLines = append(scriptLines, "exit 0")

	scriptContent := strings.Join(scriptLines, "\n") + "\n"
	scriptPath := filepath.Join(tmpDir, "editor.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		cleanup()
		return "", func() {}, err
	}

	return scriptPath, cleanup, nil
}

// GenerateTodo is exported for testing.
func (r *Rebaser) GenerateTodo(entries []RebaseEntry) string {
	return r.generateTodo(entries)
}
