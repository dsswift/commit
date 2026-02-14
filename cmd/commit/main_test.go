package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"testing"
)

func TestReverseFlag_Set(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "bare true", input: "true", want: 1},
		{name: "bare false", input: "false", want: 0},
		{name: "explicit 5", input: "5", want: 5},
		{name: "explicit 1", input: "1", want: 1},
		{name: "zero", input: "0", wantErr: true},
		{name: "negative", input: "-1", wantErr: true},
		{name: "non-numeric", input: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f reverseFlag
			err := f.Set(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Set(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("Set(%q) unexpected error: %v", tt.input, err)
				return
			}

			if int(f) != tt.want {
				t.Errorf("Set(%q) = %d, want %d", tt.input, int(f), tt.want)
			}
		})
	}
}

func TestReverseFlag_IsBoolFlag(t *testing.T) {
	var f reverseFlag
	if !f.IsBoolFlag() {
		t.Error("IsBoolFlag() should return true")
	}
}

func TestReverseFlag_String(t *testing.T) {
	tests := []struct {
		name  string
		value reverseFlag
		want  string
	}{
		{name: "zero value", value: 0, want: "0"},
		{name: "one", value: 1, want: "1"},
		{name: "five", value: 5, want: "5"},
		{name: "large number", value: 100, want: "100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.value.String()
			if got != tt.want {
				t.Errorf("reverseFlag(%d).String() = %q, want %q", int(tt.value), got, tt.want)
			}
		})
	}
}

// captureStdout captures stdout output from a function call.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	fn()

	_ = w.Close() //nolint:errcheck // test pipe close
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestPrintVerbose(t *testing.T) {
	output := captureStdout(t, func() {
		printVerbose("test verbose message")
	})

	expected := "   │ test verbose message\n"
	if output != expected {
		t.Errorf("printVerbose output = %q, want %q", output, expected)
	}
}

func TestPrintWarning(t *testing.T) {
	output := captureStdout(t, func() {
		printWarning("test warning message")
	})

	// The warning emoji has a variation selector, so just check content is present
	if !containsStr(output, "test warning message") {
		t.Errorf("printWarning output should contain the message, got %q", output)
	}
	if !containsStr(output, "⚠") {
		t.Errorf("printWarning output should contain warning symbol, got %q", output)
	}
}

func TestPrintStep(t *testing.T) {
	output := captureStdout(t, func() {
		printStep("🔧", "loading config")
	})

	expected := "\n🔧 loading config\n"
	if output != expected {
		t.Errorf("printStep output = %q, want %q", output, expected)
	}
}

func TestPrintSuccess(t *testing.T) {
	output := captureStdout(t, func() {
		printSuccess("it worked")
	})

	expected := "   ✓ it worked\n"
	if output != expected {
		t.Errorf("printSuccess output = %q, want %q", output, expected)
	}
}

func TestPrintStepError(t *testing.T) {
	output := captureStdout(t, func() {
		printStepError("it broke")
	})

	expected := "   ✗ it broke\n"
	if output != expected {
		t.Errorf("printStepError output = %q, want %q", output, expected)
	}
}

func TestPrintProgress(t *testing.T) {
	output := captureStdout(t, func() {
		printProgress("processing...")
	})

	expected := "   ⋯ processing...\n"
	if output != expected {
		t.Errorf("printProgress output = %q, want %q", output, expected)
	}
}

func TestPrintError(t *testing.T) {
	output := captureStdout(t, func() {
		printError("operation failed", fmt.Errorf("some error"))
	})

	expected := "   ✗ operation failed: some error\n"
	if output != expected {
		t.Errorf("printError output = %q, want %q", output, expected)
	}
}

func TestPrintFinal(t *testing.T) {
	output := captureStdout(t, func() {
		printFinal("✅", "all done")
	})

	expected := "\n✅ all done\n"
	if output != expected {
		t.Errorf("printFinal output = %q, want %q", output, expected)
	}
}

func TestParseFlags_Defaults(t *testing.T) {
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	}()

	flag.CommandLine = flag.NewFlagSet("commit", flag.ContinueOnError)
	os.Args = []string{"commit"}

	f := parseFlags()

	if f.staged {
		t.Error("staged should default to false")
	}
	if f.dryRun {
		t.Error("dryRun should default to false")
	}
	if f.verbose {
		t.Error("verbose should default to false")
	}
	if f.reverse != 0 {
		t.Errorf("reverse should default to 0, got %d", f.reverse)
	}
	if f.force {
		t.Error("force should default to false")
	}
	if f.interactive {
		t.Error("interactive should default to false")
	}
	if f.version {
		t.Error("version should default to false")
	}
	if f.upgrade {
		t.Error("upgrade should default to false")
	}
	if f.single {
		t.Error("single should default to false")
	}
	if f.smart {
		t.Error("smart should default to false")
	}
	if f.diffFile != "" {
		t.Errorf("diffFile should default to empty, got %q", f.diffFile)
	}
	if f.diffFrom != "" {
		t.Errorf("diffFrom should default to empty, got %q", f.diffFrom)
	}
	if f.diffTo != "" {
		t.Errorf("diffTo should default to empty, got %q", f.diffTo)
	}
	if f.provider != "" {
		t.Errorf("provider should default to empty, got %q", f.provider)
	}
	if f.setConfig != "" {
		t.Errorf("setConfig should default to empty, got %q", f.setConfig)
	}
}

func TestParseFlags_WithFlags(t *testing.T) {
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	}()

	flag.CommandLine = flag.NewFlagSet("commit", flag.ContinueOnError)
	os.Args = []string{"commit", "-staged", "-dry-run", "-v", "--version"}

	f := parseFlags()

	if !f.staged {
		t.Error("staged should be true")
	}
	if !f.dryRun {
		t.Error("dryRun should be true")
	}
	if !f.verbose {
		t.Error("verbose should be true")
	}
	if !f.version {
		t.Error("version should be true")
	}
}
