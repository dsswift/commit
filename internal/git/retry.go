package git

import (
	"strings"
	"time"
)

// lockRetryAttempts is the maximum number of retries when a git index.lock conflict is detected.
const lockRetryAttempts = 3

// lockBaseDelay is the initial backoff delay between retries.
const lockBaseDelay = 100 * time.Millisecond

// isIndexLockError returns true if the error output indicates a git index.lock conflict.
func isIndexLockError(output string) bool {
	return strings.Contains(output, "index.lock") && strings.Contains(output, "File exists")
}

// retryOnLock wraps a git command execution, retrying with exponential backoff
// if the failure is caused by a stale index.lock file. This handles a macOS race
// condition where the lock file persists briefly after a git process exits.
//
// The fn parameter should execute a git command and return its combined output
// and error, matching the signature of exec.Cmd.CombinedOutput.
func retryOnLock(fn func() ([]byte, error)) ([]byte, error) {
	var out []byte
	var err error

	for attempt := range lockRetryAttempts {
		out, err = fn()
		if err == nil {
			return out, nil
		}

		// Only retry on index.lock conflicts
		if !isIndexLockError(string(out)) {
			return out, err
		}

		// Don't sleep after the last attempt
		if attempt < lockRetryAttempts-1 {
			delay := lockBaseDelay << uint(attempt) // 100ms, 200ms, 400ms
			time.Sleep(delay)
		}
	}

	return out, err
}
