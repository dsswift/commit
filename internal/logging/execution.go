// Package logging provides JSONL logging for the commit tool.
package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dsswift/commit/internal/config"
)

// ExecutionLogger logs events for a single execution.
type ExecutionLogger struct {
	executionID string
	file        *os.File
	startTime   time.Time
}

// LogEvent represents a single event in the execution log.
type LogEvent struct {
	Timestamp   time.Time `json:"ts"`
	Event       string    `json:"event"`
	ExecutionID string    `json:"execution_id,omitempty"`
	Data        any       `json:"data,omitempty"`
}

// NewExecutionLogger creates a new execution logger.
func NewExecutionLogger(executionID string) (*ExecutionLogger, error) {
	configPath, err := config.ConfigPath()
	if err != nil {
		return nil, err
	}

	logsDir := filepath.Join(configPath, "logs", "executions")
	if err := os.MkdirAll(logsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	filename := fmt.Sprintf("%s.jsonl", executionID)
	logPath := filepath.Join(logsDir, filename)

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &ExecutionLogger{
		executionID: executionID,
		file:        file,
		startTime:   time.Now(),
	}, nil
}

// Log writes an event to the execution log.
func (l *ExecutionLogger) Log(event string, data any) {
	if l.file == nil {
		return
	}

	logEvent := LogEvent{
		Timestamp: time.Now().UTC(),
		Event:     event,
		Data:      data,
	}

	jsonBytes, err := json.Marshal(logEvent)
	if err != nil {
		return
	}

	_, _ = l.file.Write(jsonBytes)
	_, _ = l.file.Write([]byte("\n"))
}

// LogStart logs the start of execution.
func (l *ExecutionLogger) LogStart(version string, args []string) {
	l.Log("start", map[string]any{
		"execution_id": l.executionID,
		"version":      version,
		"args":         args,
	})
}

// LogConfigLoaded logs successful config loading.
func (l *ExecutionLogger) LogConfigLoaded(provider string, hasRepoConfig bool, scopes []string) {
	l.Log("config_loaded", map[string]any{
		"provider":        provider,
		"has_repo_config": hasRepoConfig,
		"scopes":          scopes,
	})
}

// LogGitStatus logs the git status.
func (l *ExecutionLogger) LogGitStatus(output string) {
	l.Log("git_status", map[string]any{
		"output": output,
	})
}

// LogGitDiff logs git diff info (not the actual diff content for security).
func (l *ExecutionLogger) LogGitDiff(files []string, diffLength int) {
	l.Log("git_diff", map[string]any{
		"files":       files,
		"diff_length": diffLength,
	})
}

// LogGitLog logs recent commits.
func (l *ExecutionLogger) LogGitLog(recentCommits []string) {
	l.Log("git_log", map[string]any{
		"recent_commits": recentCommits,
	})
}

// LogContextBuilt logs the analysis context summary.
func (l *ExecutionLogger) LogContextBuilt(fileCount int, diffChars int, scopes []string) {
	l.Log("context_built", map[string]any{
		"file_count":       fileCount,
		"total_diff_chars": diffChars,
		"scopes_detected":  scopes,
	})
}

// LogLLMRequest logs the LLM request (without sensitive content).
func (l *ExecutionLogger) LogLLMRequest(provider, model string, promptLength int) {
	l.Log("llm_request", map[string]any{
		"provider":      provider,
		"model":         model,
		"prompt_length": promptLength,
	})
}

// LogLLMResponse logs the LLM response.
func (l *ExecutionLogger) LogLLMResponse(responseLength int, commitsPlanned int) {
	l.Log("llm_response", map[string]any{
		"response_length": responseLength,
		"commits_planned": commitsPlanned,
	})
}

// LogPlanValidated logs plan validation result.
func (l *ExecutionLogger) LogPlanValidated(valid bool, errors []string) {
	l.Log("plan_validated", map[string]any{
		"valid":  valid,
		"errors": errors,
	})
}

// LogCommitExecuted logs a successfully executed commit.
func (l *ExecutionLogger) LogCommitExecuted(hash, message string, files []string) {
	l.Log("commit_executed", map[string]any{
		"hash":    hash,
		"message": message,
		"files":   files,
	})
}

// LogDryRun logs dry run output.
func (l *ExecutionLogger) LogDryRun(commits []map[string]any) {
	l.Log("dry_run", map[string]any{
		"commits": commits,
	})
}

// LogError logs an error.
func (l *ExecutionLogger) LogError(err error) {
	l.Log("error", map[string]any{
		"message": err.Error(),
	})
}

// LogComplete logs execution completion.
func (l *ExecutionLogger) LogComplete(exitCode int, commitsCreated int) {
	l.Log("complete", map[string]any{
		"duration_ms":     time.Since(l.startTime).Milliseconds(),
		"exit_code":       exitCode,
		"commits_created": commitsCreated,
	})
}

// Close closes the log file.
func (l *ExecutionLogger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Path returns the path to the log file.
func (l *ExecutionLogger) Path() string {
	if l.file != nil {
		return l.file.Name()
	}
	return ""
}
