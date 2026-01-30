// Package assert provides Tiger Beetle-style assertions for rigorous contract testing.
// Assertions are ALWAYS enabled, even in production. They represent invariants that
// should never be violated.
package assert

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// AssertionError is returned when an assertion fails.
type AssertionError struct {
	Message string
	File    string
	Line    int
}

func (e *AssertionError) Error() string {
	return fmt.Sprintf("assertion failed at %s:%d: %s", e.File, e.Line, e.Message)
}

// fail creates an assertion error with caller information.
func fail(msg string, args ...any) {
	_, file, line, ok := runtime.Caller(2) // Skip fail() and the assert function
	if !ok {
		file = "unknown"
		line = 0
	}

	// Extract just the filename, not the full path
	if idx := strings.LastIndex(file, "/"); idx >= 0 {
		file = file[idx+1:]
	}

	message := fmt.Sprintf(msg, args...)
	panic(&AssertionError{
		Message: message,
		File:    file,
		Line:    line,
	})
}

// True asserts that the condition is true.
func True(condition bool, msg string, args ...any) {
	if !condition {
		fail(msg, args...)
	}
}

// False asserts that the condition is false.
func False(condition bool, msg string, args ...any) {
	if condition {
		fail(msg, args...)
	}
}

// NotNil asserts that the value is not nil.
func NotNil(value any, msg string, args ...any) {
	if value == nil {
		fail(msg, args...)
	}
}

// NotEmpty asserts that a slice is not empty.
func NotEmpty[T any](slice []T, msg string, args ...any) {
	if len(slice) == 0 {
		fail(msg, args...)
	}
}

// NotEmptyString asserts that a string is not empty.
func NotEmptyString(s string, msg string, args ...any) {
	if s == "" {
		fail(msg, args...)
	}
}

// FileExists asserts that a file exists at the given path.
func FileExists(path string, msg string, args ...any) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fail(msg, args...)
	}
}

// DirExists asserts that a directory exists at the given path.
func DirExists(path string, msg string, args ...any) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		fail(msg, args...)
	}
	if err == nil && !info.IsDir() {
		fail(msg, args...)
	}
}

// Contains asserts that a slice contains an item.
func Contains[T comparable](slice []T, item T, msg string, args ...any) {
	for _, v := range slice {
		if v == item {
			return
		}
	}
	fail(msg, args...)
}

// NotContains asserts that a slice does not contain an item.
func NotContains[T comparable](slice []T, item T, msg string, args ...any) {
	for _, v := range slice {
		if v == item {
			fail(msg, args...)
		}
	}
}

// Equal asserts that two values are equal.
func Equal[T comparable](expected, actual T, msg string, args ...any) {
	if expected != actual {
		fail(msg, args...)
	}
}

// NotEqual asserts that two values are not equal.
func NotEqual[T comparable](expected, actual T, msg string, args ...any) {
	if expected == actual {
		fail(msg, args...)
	}
}

// NoError asserts that an error is nil.
func NoError(err error, msg string, args ...any) {
	if err != nil {
		fail(msg+": %v", append(args, err)...)
	}
}

// Error asserts that an error is not nil.
func Error(err error, msg string, args ...any) {
	if err == nil {
		fail(msg, args...)
	}
}

// LenEquals asserts that a slice has the expected length.
func LenEquals[T any](slice []T, expected int, msg string, args ...any) {
	if len(slice) != expected {
		fail(msg, args...)
	}
}

// InRange asserts that a value is within a range [min, max].
func InRange(value, min, max int, msg string, args ...any) {
	if value < min || value > max {
		fail(msg, args...)
	}
}

// Positive asserts that a number is positive (> 0).
func Positive(value int, msg string, args ...any) {
	if value <= 0 {
		fail(msg, args...)
	}
}

// NonNegative asserts that a number is non-negative (>= 0).
func NonNegative(value int, msg string, args ...any) {
	if value < 0 {
		fail(msg, args...)
	}
}

// ValidCommitType asserts that a commit type is one of the allowed types.
func ValidCommitType(commitType string, allowedTypes []string, msg string, args ...any) {
	for _, t := range allowedTypes {
		if t == commitType {
			return
		}
	}
	fail(msg, args...)
}

// MaxLength asserts that a string does not exceed max length.
func MaxLength(s string, max int, msg string, args ...any) {
	if len(s) > max {
		fail(msg, args...)
	}
}
