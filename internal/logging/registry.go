package logging

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dsswift/commit/internal/config"
)

const (
	registryFile    = "tool_executions.jsonl"
	maxRegistrySize = 10 * 1024 * 1024 // 10MB
	retentionDays   = 30
)

// RegistryEntry represents a single execution in the registry.
type RegistryEntry struct {
	ExecutionID    string `json:"execution_id"`
	Timestamp      string `json:"timestamp"`
	Version        string `json:"version"`
	CWD            string `json:"cwd"`
	Args           []string `json:"args"`
	GitRoot        string `json:"git_root"`
	DurationMS     int64  `json:"duration_ms"`
	ExitCode       int    `json:"exit_code"`
	CommitsCreated int    `json:"commits_created"`
}

// GenerateExecutionID creates a unique execution ID.
func GenerateExecutionID() string {
	now := time.Now()

	// Generate random suffix
	randBytes := make([]byte, 3)
	rand.Read(randBytes)
	randSuffix := hex.EncodeToString(randBytes)

	return fmt.Sprintf("exec_%s_%s", now.Format("20060102_150405"), randSuffix)
}

// WriteRegistryEntry appends an entry to the tool_executions.jsonl file.
func WriteRegistryEntry(entry RegistryEntry) error {
	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}

	logsDir := filepath.Join(configPath, "logs")
	if err := os.MkdirAll(logsDir, 0700); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	registryPath := filepath.Join(logsDir, registryFile)

	// Check if rotation is needed
	if shouldRotate(registryPath) {
		rotateRegistry(registryPath)
	}

	file, err := os.OpenFile(registryPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to open registry file: %w", err)
	}
	defer file.Close()

	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %w", err)
	}

	_, err = file.Write(append(jsonBytes, '\n'))
	return err
}

// shouldRotate checks if the registry file needs rotation.
func shouldRotate(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() > maxRegistrySize
}

// rotateRegistry rotates the registry file.
func rotateRegistry(path string) {
	// Remove old backups
	os.Remove(path + ".2")

	// Rotate existing backups
	os.Rename(path+".1", path+".2")
	os.Rename(path, path+".1")
}

// CleanupOldLogs removes execution logs older than retention period.
func CleanupOldLogs() error {
	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}

	executionsDir := filepath.Join(configPath, "logs", "executions")

	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	entries, err := os.ReadDir(executionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(executionsDir, entry.Name()))
		}
	}

	return nil
}

// GetRecentExecutions returns the most recent N executions from the registry.
func GetRecentExecutions(count int) ([]RegistryEntry, error) {
	configPath, err := config.ConfigPath()
	if err != nil {
		return nil, err
	}

	registryPath := filepath.Join(configPath, "logs", registryFile)

	data, err := os.ReadFile(registryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var entries []RegistryEntry
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}

		var entry RegistryEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	// Return the last N entries
	if len(entries) > count {
		entries = entries[len(entries)-count:]
	}

	return entries, nil
}

// splitLines splits byte data into lines.
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0

	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}

	if start < len(data) {
		lines = append(lines, data[start:])
	}

	return lines
}
