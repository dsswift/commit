package assert

import (
	"os"
	"path/filepath"
	"testing"
)

// Helper to check if a function panics with AssertionError
func assertPanics(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("%s: expected panic, got none", name)
			return
		}
		if _, ok := r.(*AssertionError); !ok {
			t.Errorf("%s: expected AssertionError, got %T: %v", name, r, r)
		}
	}()
	f()
}

// Helper to check that a function does NOT panic
func assertNoPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s: unexpected panic: %v", name, r)
		}
	}()
	f()
}

func TestTrue(t *testing.T) {
	assertNoPanic(t, "True(true)", func() {
		True(true, "should not panic")
	})

	assertPanics(t, "True(false)", func() {
		True(false, "should panic")
	})
}

func TestFalse(t *testing.T) {
	assertNoPanic(t, "False(false)", func() {
		False(false, "should not panic")
	})

	assertPanics(t, "False(true)", func() {
		False(true, "should panic")
	})
}

func TestNotNil(t *testing.T) {
	assertNoPanic(t, "NotNil(non-nil)", func() {
		NotNil("value", "should not panic")
	})

	assertPanics(t, "NotNil(nil)", func() {
		NotNil(nil, "should panic")
	})
}

func TestNotEmpty(t *testing.T) {
	assertNoPanic(t, "NotEmpty(non-empty)", func() {
		NotEmpty([]string{"a", "b"}, "should not panic")
	})

	assertPanics(t, "NotEmpty(empty)", func() {
		NotEmpty([]string{}, "should panic")
	})

	assertPanics(t, "NotEmpty(nil slice)", func() {
		var nilSlice []int
		NotEmpty(nilSlice, "should panic")
	})
}

func TestNotEmptyString(t *testing.T) {
	assertNoPanic(t, "NotEmptyString(non-empty)", func() {
		NotEmptyString("hello", "should not panic")
	})

	assertPanics(t, "NotEmptyString(empty)", func() {
		NotEmptyString("", "should panic")
	})
}

func TestFileExists(t *testing.T) {
	// Create a temp file
	tmpFile, err := os.CreateTemp("", "test-assert-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	_ = tmpFile.Close()
	defer os.Remove(tmpFile.Name()) //nolint:errcheck // test cleanup

	assertNoPanic(t, "FileExists(existing)", func() {
		FileExists(tmpFile.Name(), "should not panic")
	})

	assertPanics(t, "FileExists(non-existing)", func() {
		FileExists("/nonexistent/path/to/file", "should panic")
	})
}

func TestDirExists(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "test-assert-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup

	assertNoPanic(t, "DirExists(existing)", func() {
		DirExists(tmpDir, "should not panic")
	})

	assertPanics(t, "DirExists(non-existing)", func() {
		DirExists("/nonexistent/path/to/dir", "should panic")
	})

	// Test that a file is not treated as a directory
	tmpFile := filepath.Join(tmpDir, "file.txt")
	_ = os.WriteFile(tmpFile, []byte("test"), 0644)

	assertPanics(t, "DirExists(file)", func() {
		DirExists(tmpFile, "should panic for file")
	})
}

func TestContains(t *testing.T) {
	slice := []string{"a", "b", "c"}

	assertNoPanic(t, "Contains(found)", func() {
		Contains(slice, "b", "should not panic")
	})

	assertPanics(t, "Contains(not found)", func() {
		Contains(slice, "d", "should panic")
	})

	// Test with int slice
	intSlice := []int{1, 2, 3}
	assertNoPanic(t, "Contains(int found)", func() {
		Contains(intSlice, 2, "should not panic")
	})
}

func TestNotContains(t *testing.T) {
	slice := []string{"a", "b", "c"}

	assertNoPanic(t, "NotContains(not found)", func() {
		NotContains(slice, "d", "should not panic")
	})

	assertPanics(t, "NotContains(found)", func() {
		NotContains(slice, "b", "should panic")
	})
}

func TestEqual(t *testing.T) {
	assertNoPanic(t, "Equal(same)", func() {
		Equal(42, 42, "should not panic")
	})

	assertPanics(t, "Equal(different)", func() {
		Equal(42, 43, "should panic")
	})

	// Test with strings
	assertNoPanic(t, "Equal(same string)", func() {
		Equal("hello", "hello", "should not panic")
	})
}

func TestNotEqual(t *testing.T) {
	assertNoPanic(t, "NotEqual(different)", func() {
		NotEqual(42, 43, "should not panic")
	})

	assertPanics(t, "NotEqual(same)", func() {
		NotEqual(42, 42, "should panic")
	})
}

func TestNoError(t *testing.T) {
	assertNoPanic(t, "NoError(nil)", func() {
		NoError(nil, "should not panic")
	})

	assertPanics(t, "NoError(error)", func() {
		NoError(os.ErrNotExist, "should panic")
	})
}

func TestError(t *testing.T) {
	assertNoPanic(t, "Error(error)", func() {
		Error(os.ErrNotExist, "should not panic")
	})

	assertPanics(t, "Error(nil)", func() {
		Error(nil, "should panic")
	})
}

func TestLenEquals(t *testing.T) {
	slice := []string{"a", "b", "c"}

	assertNoPanic(t, "LenEquals(correct)", func() {
		LenEquals(slice, 3, "should not panic")
	})

	assertPanics(t, "LenEquals(wrong)", func() {
		LenEquals(slice, 5, "should panic")
	})
}

func TestInRange(t *testing.T) {
	assertNoPanic(t, "InRange(in range)", func() {
		InRange(5, 1, 10, "should not panic")
	})

	assertNoPanic(t, "InRange(at min)", func() {
		InRange(1, 1, 10, "should not panic")
	})

	assertNoPanic(t, "InRange(at max)", func() {
		InRange(10, 1, 10, "should not panic")
	})

	assertPanics(t, "InRange(below min)", func() {
		InRange(0, 1, 10, "should panic")
	})

	assertPanics(t, "InRange(above max)", func() {
		InRange(11, 1, 10, "should panic")
	})
}

func TestPositive(t *testing.T) {
	assertNoPanic(t, "Positive(positive)", func() {
		Positive(5, "should not panic")
	})

	assertPanics(t, "Positive(zero)", func() {
		Positive(0, "should panic")
	})

	assertPanics(t, "Positive(negative)", func() {
		Positive(-1, "should panic")
	})
}

func TestNonNegative(t *testing.T) {
	assertNoPanic(t, "NonNegative(positive)", func() {
		NonNegative(5, "should not panic")
	})

	assertNoPanic(t, "NonNegative(zero)", func() {
		NonNegative(0, "should not panic")
	})

	assertPanics(t, "NonNegative(negative)", func() {
		NonNegative(-1, "should panic")
	})
}

func TestValidCommitType(t *testing.T) {
	allowed := []string{"feat", "fix", "docs"}

	assertNoPanic(t, "ValidCommitType(valid)", func() {
		ValidCommitType("feat", allowed, "should not panic")
	})

	assertPanics(t, "ValidCommitType(invalid)", func() {
		ValidCommitType("refactor", allowed, "should panic")
	})
}

func TestMaxLength(t *testing.T) {
	assertNoPanic(t, "MaxLength(under)", func() {
		MaxLength("hello", 10, "should not panic")
	})

	assertNoPanic(t, "MaxLength(exact)", func() {
		MaxLength("hello", 5, "should not panic")
	})

	assertPanics(t, "MaxLength(over)", func() {
		MaxLength("hello world", 5, "should panic")
	})
}

func TestAssertionError_Error(t *testing.T) {
	err := &AssertionError{
		Message: "test message",
		File:    "test.go",
		Line:    42,
	}

	expected := "assertion failed at test.go:42: test message"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}
