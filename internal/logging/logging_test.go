package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateExecutionID(t *testing.T) {
	id1 := GenerateExecutionID()
	id2 := GenerateExecutionID()

	if id1 == "" {
		t.Error("expected non-empty execution ID")
	}

	if !strings.HasPrefix(id1, "exec_") {
		t.Errorf("expected ID to start with 'exec_', got %q", id1)
	}

	// IDs should be unique (different random suffix)
	if id1 == id2 {
		t.Error("expected unique IDs")
	}

	// ID should contain timestamp
	today := time.Now().Format("20060102")
	if !strings.Contains(id1, today) {
		t.Errorf("expected ID to contain today's date %s, got %q", today, id1)
	}
}

func TestExecutionLogger_Log(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "logging-test-*")
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	t.Setenv("HOME", tmpDir)

	// Create logger
	execID := "exec_test_123"
	logger, err := NewExecutionLogger(execID)
	if err != nil {
		t.Fatalf("NewExecutionLogger failed: %v", err)
	}
	defer logger.Close() //nolint:errcheck // test cleanup

	// Log some events
	logger.Log("test_event", map[string]string{"key": "value"})
	logger.LogStart("1.0.0", []string{"--dry-run"})
	logger.LogConfigLoaded("anthropic", true, []string{"api", "core"})

	// Close to flush
	_ = logger.Close()

	// Read and verify log file
	logPath := logger.Path()
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 log lines, got %d", len(lines))
	}

	// Verify first event
	var event LogEvent
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("failed to parse log event: %v", err)
	}

	if event.Event != "test_event" {
		t.Errorf("expected event 'test_event', got %q", event.Event)
	}
}

func TestExecutionLogger_Path(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "logging-test-*")
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	t.Setenv("HOME", tmpDir)

	execID := "exec_test_456"
	logger, err := NewExecutionLogger(execID)
	if err != nil {
		t.Fatalf("NewExecutionLogger failed: %v", err)
	}
	defer logger.Close() //nolint:errcheck // test cleanup

	path := logger.Path()

	if !strings.Contains(path, execID) {
		t.Errorf("expected path to contain execution ID, got %q", path)
	}

	if !strings.HasSuffix(path, ".jsonl") {
		t.Errorf("expected .jsonl extension, got %q", path)
	}
}

func TestWriteRegistryEntry(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "logging-test-*")
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	t.Setenv("HOME", tmpDir)

	entry := RegistryEntry{
		ExecutionID:    "exec_test_789",
		Timestamp:      time.Now().Format(time.RFC3339),
		Version:        "1.0.0",
		CWD:            "/test/path",
		Args:           []string{"--dry-run"},
		GitRoot:        "/test/path",
		DurationMS:     1234,
		ExitCode:       0,
		CommitsCreated: 3,
	}

	err := WriteRegistryEntry(entry)
	if err != nil {
		t.Fatalf("WriteRegistryEntry failed: %v", err)
	}

	// Verify entry was written
	registryPath := filepath.Join(tmpDir, ".commit-tool", "logs", "tool_executions.jsonl")
	content, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("failed to read registry: %v", err)
	}

	var readEntry RegistryEntry
	if err := json.Unmarshal(content[:len(content)-1], &readEntry); err != nil { // -1 to remove newline
		t.Fatalf("failed to parse registry entry: %v", err)
	}

	if readEntry.ExecutionID != entry.ExecutionID {
		t.Errorf("expected execution ID %q, got %q", entry.ExecutionID, readEntry.ExecutionID)
	}

	if readEntry.CommitsCreated != entry.CommitsCreated {
		t.Errorf("expected commits created %d, got %d", entry.CommitsCreated, readEntry.CommitsCreated)
	}
}

func TestGetRecentExecutions(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "logging-test-*")
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	t.Setenv("HOME", tmpDir)

	// Write multiple entries
	for i := 1; i <= 5; i++ {
		entry := RegistryEntry{
			ExecutionID:    "exec_" + string(rune('0'+i)),
			Timestamp:      time.Now().Format(time.RFC3339),
			Version:        "1.0.0",
			CommitsCreated: i,
		}
		_ = WriteRegistryEntry(entry)
	}

	// Get recent 3
	recent, err := GetRecentExecutions(3)
	if err != nil {
		t.Fatalf("GetRecentExecutions failed: %v", err)
	}

	if len(recent) != 3 {
		t.Errorf("expected 3 recent executions, got %d", len(recent))
	}

	// Should be the last 3 entries
	if recent[0].ExecutionID != "exec_3" {
		t.Errorf("expected first recent to be exec_3, got %q", recent[0].ExecutionID)
	}
}

func TestGetRecentExecutions_NoFile(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "logging-test-*")
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	t.Setenv("HOME", tmpDir)

	// Should not error for missing file
	recent, err := GetRecentExecutions(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(recent) != 0 {
		t.Errorf("expected empty result, got %v", recent)
	}
}

func TestCleanupOldLogs(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "logging-test-*")
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	t.Setenv("HOME", tmpDir)

	// Create executions directory
	execDir := filepath.Join(tmpDir, ".commit-tool", "logs", "executions")
	_ = os.MkdirAll(execDir, 0700)

	// Create old and new log files
	oldFile := filepath.Join(execDir, "old_exec.jsonl")
	newFile := filepath.Join(execDir, "new_exec.jsonl")

	_ = os.WriteFile(oldFile, []byte("old"), 0600)
	_ = os.WriteFile(newFile, []byte("new"), 0600)

	// Set old file's mod time to 60 days ago
	oldTime := time.Now().AddDate(0, 0, -60)
	_ = os.Chtimes(oldFile, oldTime, oldTime)

	// Run cleanup
	err := CleanupOldLogs()
	if err != nil {
		t.Fatalf("CleanupOldLogs failed: %v", err)
	}

	// Old file should be deleted
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("expected old file to be deleted")
	}

	// New file should remain
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		t.Error("expected new file to remain")
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected int
	}{
		{"empty", []byte(""), 0},
		{"single line", []byte("line1"), 1},
		{"two lines", []byte("line1\nline2"), 2},
		{"trailing newline", []byte("line1\nline2\n"), 2},
		{"multiple newlines", []byte("line1\n\nline3"), 2}, // Empty line is skipped
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := splitLines(tt.input)
			if len(lines) != tt.expected {
				t.Errorf("expected %d lines, got %d", tt.expected, len(lines))
			}
		})
	}
}

func TestRegistryRotation(t *testing.T) {
	// Test that shouldRotate returns correct values
	tmpDir, _ := os.MkdirTemp("", "logging-test-*")
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

	// Create small file (under threshold)
	smallFile := filepath.Join(tmpDir, "small.jsonl")
	_ = os.WriteFile(smallFile, []byte("small"), 0600)

	if shouldRotate(smallFile) {
		t.Error("should not rotate small file")
	}

	// Non-existent file
	if shouldRotate(filepath.Join(tmpDir, "nonexistent")) {
		t.Error("should not rotate non-existent file")
	}
}

func TestExecutionLogger_AllLogMethods(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "logging-test-*")
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	t.Setenv("HOME", tmpDir)

	logger, _ := NewExecutionLogger("exec_all_methods")
	defer logger.Close() //nolint:errcheck // test cleanup

	// Call all log methods to ensure they don't panic
	logger.LogStart("1.0.0", []string{})
	logger.LogConfigLoaded("anthropic", true, []string{"api"})
	logger.LogGitStatus("M file.go")
	logger.LogGitDiff([]string{"file.go"}, 100)
	logger.LogGitLog([]string{"commit 1", "commit 2"})
	logger.LogContextBuilt(5, 1000, []string{"api", "core"})
	logger.LogLLMRequest("anthropic", "claude-3-5-sonnet", 2000)
	logger.LogLLMResponse(500, 3)
	logger.LogPlanValidated(true, nil)
	logger.LogCommitExecuted("abc123", "feat: add feature", []string{"file.go"})
	logger.LogDryRun([]map[string]any{{"type": "feat"}})
	logger.LogError(&testError{"test error"})
	logger.LogComplete(0, 3)

	_ = logger.Close()

	// Verify file was created with content
	content, _ := os.ReadFile(logger.Path())
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	if len(lines) < 10 {
		t.Errorf("expected at least 10 log lines, got %d", len(lines))
	}
}

func TestRotateRegistry(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "logging-rotate-*")
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

	regPath := filepath.Join(tmpDir, "registry.jsonl")

	// Create registry file and two backups
	_ = os.WriteFile(regPath, []byte("current"), 0600)
	_ = os.WriteFile(regPath+".1", []byte("backup1"), 0600)
	_ = os.WriteFile(regPath+".2", []byte("backup2"), 0600)

	rotateRegistry(regPath)

	// Original file should be gone (renamed to .1)
	if _, err := os.Stat(regPath); !os.IsNotExist(err) {
		t.Error("expected original file to be renamed away")
	}

	// .1 should now contain "current" (was the original)
	data, err := os.ReadFile(regPath + ".1")
	if err != nil {
		t.Fatalf("failed to read .1 backup: %v", err)
	}
	if string(data) != "current" {
		t.Errorf(".1 backup content = %q, want %q", string(data), "current")
	}

	// .2 should now contain "backup1" (was .1)
	data, err = os.ReadFile(regPath + ".2")
	if err != nil {
		t.Fatalf("failed to read .2 backup: %v", err)
	}
	if string(data) != "backup1" {
		t.Errorf(".2 backup content = %q, want %q", string(data), "backup1")
	}
}

func TestShouldRotate(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "logging-rotate-*")
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

	// Small file should not need rotation
	smallFile := filepath.Join(tmpDir, "small.jsonl")
	_ = os.WriteFile(smallFile, []byte("small"), 0600)
	if shouldRotate(smallFile) {
		t.Error("shouldRotate returned true for small file")
	}

	// Non-existent file should not need rotation
	if shouldRotate(filepath.Join(tmpDir, "missing")) {
		t.Error("shouldRotate returned true for non-existent file")
	}

	// File > 10MB should need rotation
	largeFile := filepath.Join(tmpDir, "large.jsonl")
	f, err := os.Create(largeFile)
	if err != nil {
		t.Fatalf("failed to create large file: %v", err)
	}
	// Write 10MB + 1 byte
	buf := make([]byte, 1024*1024) // 1MB chunks
	for i := 0; i < 10; i++ {
		_, _ = f.Write(buf)
	}
	_, _ = f.Write([]byte("x")) // push over threshold
	_ = f.Close()               //nolint:errcheck // test file close

	if !shouldRotate(largeFile) {
		t.Error("shouldRotate returned false for file > 10MB")
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
