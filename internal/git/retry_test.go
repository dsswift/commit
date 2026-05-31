package git

import (
	"errors"
	"testing"
	"time"
)

func TestRetryOnLock_SucceedsFirstTry(t *testing.T) {
	calls := 0
	out, err := retryOnLock(func() ([]byte, error) {
		calls++
		return []byte("ok"), nil
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if string(out) != "ok" {
		t.Fatalf("expected output 'ok', got: %s", string(out))
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got: %d", calls)
	}
}

func TestRetryOnLock_SucceedsAfterTransientLock(t *testing.T) {
	calls := 0
	lockOutput := []byte(`fatal: Unable to create '/tmp/.git/index.lock': File exists.

Another git process seems to be running in this repository.`)

	out, err := retryOnLock(func() ([]byte, error) {
		calls++
		if calls <= 2 {
			return lockOutput, errors.New("exit status 128")
		}
		return []byte("committed"), nil
	})

	if err != nil {
		t.Fatalf("expected no error after retry, got: %v", err)
	}
	if string(out) != "committed" {
		t.Fatalf("expected output 'committed', got: %s", string(out))
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got: %d", calls)
	}
}

func TestRetryOnLock_FailsAfterMaxRetries(t *testing.T) {
	calls := 0
	lockOutput := []byte(`fatal: Unable to create '/tmp/.git/index.lock': File exists.`)

	_, err := retryOnLock(func() ([]byte, error) {
		calls++
		return lockOutput, errors.New("exit status 128")
	})

	if err == nil {
		t.Fatal("expected error after max retries, got nil")
	}
	if calls != lockRetryAttempts {
		t.Fatalf("expected %d calls, got: %d", lockRetryAttempts, calls)
	}
}

func TestRetryOnLock_NonLockErrorsNotRetried(t *testing.T) {
	calls := 0
	out, err := retryOnLock(func() ([]byte, error) {
		calls++
		return []byte("fatal: not a git repository"), errors.New("exit status 128")
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if string(out) != "fatal: not a git repository" {
		t.Fatalf("expected original output, got: %s", string(out))
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry), got: %d", calls)
	}
}

func TestIsIndexLockError(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{
			name:   "lock error",
			output: `fatal: Unable to create '/repo/.git/index.lock': File exists.`,
			want:   true,
		},
		{
			name:   "no lock error",
			output: `fatal: not a git repository`,
			want:   false,
		},
		{
			name:   "partial match index.lock only",
			output: `some message about index.lock`,
			want:   false,
		},
		{
			name:   "partial match File exists only",
			output: `File exists but not about lock`,
			want:   false,
		},
		{
			name:   "empty",
			output: "",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isIndexLockError(tt.output)
			if got != tt.want {
				t.Errorf("isIndexLockError(%q) = %v, want %v", tt.output, got, tt.want)
			}
		})
	}
}

func TestRetryOnLock_BackoffTiming(t *testing.T) {
	calls := 0
	lockOutput := []byte(`fatal: Unable to create '/tmp/.git/index.lock': File exists.`)

	start := time.Now()
	_, _ = retryOnLock(func() ([]byte, error) {
		calls++
		return lockOutput, errors.New("exit status 128")
	})
	elapsed := time.Since(start)

	// Should have slept ~100ms + ~200ms = ~300ms total (no sleep after last attempt)
	// Use a generous lower bound to avoid flaky tests
	expectedMin := 200 * time.Millisecond
	if elapsed < expectedMin {
		t.Errorf("expected at least %v of backoff, got %v", expectedMin, elapsed)
	}
}
