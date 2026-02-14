package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dsswift/commit/internal/config"
	"github.com/dsswift/commit/internal/llm"
	"github.com/dsswift/commit/pkg/types"
)

func TestE2E_SmartCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// Create a temp git repo
	tmpDir := t.TempDir()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")

	// Create initial commit
	initialFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(initialFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "initial commit")

	// Create changes to be committed
	handlerFile := filepath.Join(tmpDir, "handler.go")
	if err := os.WriteFile(handlerFile, []byte("package main\nfunc Handler() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	configFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte("key: value\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Set up mock LLM server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scope := "api"
		resp := chatCompletionResponse{
			Choices: []chatCompletionChoice{
				{
					Message: chatCompletionMessage{
						Content: mustJSON(t, types.CommitPlan{
							Commits: []types.PlannedCommit{
								{
									Type:    "feat",
									Scope:   &scope,
									Message: "add handler endpoint",
									Files:   []string{"handler.go"},
								},
								{
									Type:    "chore",
									Message: "add configuration",
									Files:   []string{"config.yaml"},
								},
							},
						}),
					},
					FinishReason: "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Override provider factory to use mock server
	origFactory := newProviderFunc
	newProviderFunc = func(config *types.UserConfig) (llm.Provider, error) {
		return &mockProvider{baseURL: mockServer.URL}, nil
	}
	defer func() { newProviderFunc = origFactory }()

	// Set up fake config so LoadUserConfig() succeeds
	fakeHome := t.TempDir()
	configDir := filepath.Join(fakeHome, ".commit-tool")
	if err := os.MkdirAll(filepath.Join(configDir, "logs", "executions"), 0700); err != nil {
		t.Fatal(err)
	}
	envContent := "COMMIT_PROVIDER=openai\nOPENAI_API_KEY=test-key\n"
	if err := os.WriteFile(filepath.Join(configDir, ".env"), []byte(envContent), 0600); err != nil {
		t.Fatal(err)
	}
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", fakeHome)       //nolint:errcheck // test setup
	defer os.Setenv("HOME", origHome) //nolint:errcheck // test cleanup

	// Change to temp dir
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)        //nolint:errcheck // test setup
	defer os.Chdir(origDir) //nolint:errcheck // test cleanup

	// Run execute
	result := execute(flags{}, nil)

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	if len(result.CommitsCreated) != 2 {
		t.Errorf("expected 2 commits created, got %d", len(result.CommitsCreated))
	}

	// Verify git log has the commits
	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = tmpDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}

	logOutput := string(out)
	if !containsStr(logOutput, "feat(api): add handler endpoint") {
		t.Errorf("expected feat commit in git log, got:\n%s", logOutput)
	}
	if !containsStr(logOutput, "chore: add configuration") {
		t.Errorf("expected chore commit in git log, got:\n%s", logOutput)
	}
}

func TestE2E_NoChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// Create a temp git repo with a clean working tree
	tmpDir := t.TempDir()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")

	// Create initial commit so repo is non-empty
	initialFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(initialFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "initial commit")

	// No additional changes -- working tree is clean

	// Set up fake config so LoadUserConfig() succeeds
	fakeHome := t.TempDir()
	configDir := filepath.Join(fakeHome, ".commit-tool")
	if err := os.MkdirAll(filepath.Join(configDir, "logs", "executions"), 0700); err != nil {
		t.Fatal(err)
	}
	envContent := "COMMIT_PROVIDER=openai\nOPENAI_API_KEY=test-key\n"
	if err := os.WriteFile(filepath.Join(configDir, ".env"), []byte(envContent), 0600); err != nil {
		t.Fatal(err)
	}
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", fakeHome)       //nolint:errcheck // test setup
	defer os.Setenv("HOME", origHome) //nolint:errcheck // test cleanup

	// Override provider factory (should not be called, but save/restore anyway)
	origFactory := newProviderFunc
	defer func() { newProviderFunc = origFactory }()

	// Change to temp dir
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)        //nolint:errcheck // test setup
	defer os.Chdir(origDir) //nolint:errcheck // test cleanup

	// Run execute on clean repo
	result := execute(flags{}, nil)

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0 for no changes, got %d", result.ExitCode)
	}

	if len(result.CommitsCreated) != 0 {
		t.Errorf("expected 0 commits created, got %d", len(result.CommitsCreated))
	}
}

func TestE2E_StagedOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// Create a temp git repo
	tmpDir := t.TempDir()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")

	// Create initial commit
	initialFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(initialFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "initial commit")

	// Create multiple files
	stagedFile := filepath.Join(tmpDir, "staged.go")
	if err := os.WriteFile(stagedFile, []byte("package main\nfunc Staged() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	unstagedFile := filepath.Join(tmpDir, "unstaged.go")
	if err := os.WriteFile(unstagedFile, []byte("package main\nfunc Unstaged() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Stage only one file
	runGit("add", "staged.go")

	// Set up mock LLM server -- returns a commit for the staged file only
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatCompletionResponse{
			Choices: []chatCompletionChoice{
				{
					Message: chatCompletionMessage{
						Content: mustJSON(t, types.CommitPlan{
							Commits: []types.PlannedCommit{
								{
									Type:    "feat",
									Message: "add staged function",
									Files:   []string{"staged.go"},
								},
							},
						}),
					},
					FinishReason: "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Override provider factory
	origFactory := newProviderFunc
	newProviderFunc = func(config *types.UserConfig) (llm.Provider, error) {
		return &mockProvider{baseURL: mockServer.URL}, nil
	}
	defer func() { newProviderFunc = origFactory }()

	// Set up fake config
	fakeHome := t.TempDir()
	configDir := filepath.Join(fakeHome, ".commit-tool")
	if err := os.MkdirAll(filepath.Join(configDir, "logs", "executions"), 0700); err != nil {
		t.Fatal(err)
	}
	envContent := "COMMIT_PROVIDER=openai\nOPENAI_API_KEY=test-key\n"
	if err := os.WriteFile(filepath.Join(configDir, ".env"), []byte(envContent), 0600); err != nil {
		t.Fatal(err)
	}
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", fakeHome)       //nolint:errcheck // test setup
	defer os.Setenv("HOME", origHome) //nolint:errcheck // test cleanup

	// Change to temp dir
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)        //nolint:errcheck // test setup
	defer os.Chdir(origDir) //nolint:errcheck // test cleanup

	// Run execute with --staged
	result := execute(flags{staged: true}, nil)

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	if len(result.CommitsCreated) != 1 {
		t.Fatalf("expected 1 commit created, got %d", len(result.CommitsCreated))
	}

	// Verify git log has the staged commit
	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = tmpDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}

	logOutput := string(out)
	if !containsStr(logOutput, "feat: add staged function") {
		t.Errorf("expected staged commit in git log, got:\n%s", logOutput)
	}

	// Verify unstaged file is still untracked (not committed)
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = tmpDir
	out, err = cmd.Output()
	if err != nil {
		t.Fatalf("git status failed: %v", err)
	}

	statusOutput := string(out)
	if !containsStr(statusOutput, "unstaged.go") {
		t.Errorf("expected unstaged.go to still be in working tree, got:\n%s", statusOutput)
	}
}

func TestE2E_DryRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// Create a temp git repo
	tmpDir := t.TempDir()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")

	// Create initial commit
	initialFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(initialFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "initial commit")

	// Capture initial commit count
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = tmpDir
	countBefore, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-list failed: %v", err)
	}

	// Create a change
	newFile := filepath.Join(tmpDir, "feature.go")
	if err := os.WriteFile(newFile, []byte("package main\nfunc Feature() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Set up mock LLM server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatCompletionResponse{
			Choices: []chatCompletionChoice{
				{
					Message: chatCompletionMessage{
						Content: mustJSON(t, types.CommitPlan{
							Commits: []types.PlannedCommit{
								{
									Type:    "feat",
									Message: "add feature function",
									Files:   []string{"feature.go"},
								},
							},
						}),
					},
					FinishReason: "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Override provider factory
	origFactory := newProviderFunc
	newProviderFunc = func(config *types.UserConfig) (llm.Provider, error) {
		return &mockProvider{baseURL: mockServer.URL}, nil
	}
	defer func() { newProviderFunc = origFactory }()

	// Set up fake config
	fakeHome := t.TempDir()
	configDir := filepath.Join(fakeHome, ".commit-tool")
	if err := os.MkdirAll(filepath.Join(configDir, "logs", "executions"), 0700); err != nil {
		t.Fatal(err)
	}
	envContent := "COMMIT_PROVIDER=openai\nOPENAI_API_KEY=test-key\n"
	if err := os.WriteFile(filepath.Join(configDir, ".env"), []byte(envContent), 0600); err != nil {
		t.Fatal(err)
	}
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", fakeHome)       //nolint:errcheck // test setup
	defer os.Setenv("HOME", origHome) //nolint:errcheck // test cleanup

	// Change to temp dir
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)        //nolint:errcheck // test setup
	defer os.Chdir(origDir) //nolint:errcheck // test cleanup

	// Run execute with --dry-run
	result := execute(flags{dryRun: true}, nil)

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	// Verify NO new commits were created in git log
	cmd = exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = tmpDir
	countAfter, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-list failed: %v", err)
	}

	if strings.TrimSpace(string(countBefore)) != strings.TrimSpace(string(countAfter)) {
		t.Errorf("dry-run should not create commits: had %s commits before, %s after",
			strings.TrimSpace(string(countBefore)), strings.TrimSpace(string(countAfter)))
	}

	// Verify the file is still untracked
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = tmpDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status failed: %v", err)
	}

	statusOutput := string(out)
	if !containsStr(statusOutput, "feature.go") {
		t.Errorf("expected feature.go to still be in working tree after dry-run, got:\n%s", statusOutput)
	}
}

func TestE2E_SingleCommitMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// Create a temp git repo
	tmpDir := t.TempDir()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")

	// Create initial commit
	initialFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(initialFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "initial commit")

	// Create multiple files
	file1 := filepath.Join(tmpDir, "handler.go")
	if err := os.WriteFile(file1, []byte("package main\nfunc Handler() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	file2 := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(file2, []byte("key: value\n"), 0644); err != nil {
		t.Fatal(err)
	}

	file3 := filepath.Join(tmpDir, "utils.go")
	if err := os.WriteFile(file3, []byte("package main\nfunc Util() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Set up mock LLM server -- returns a single commit with all files
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatCompletionResponse{
			Choices: []chatCompletionChoice{
				{
					Message: chatCompletionMessage{
						Content: mustJSON(t, types.CommitPlan{
							Commits: []types.PlannedCommit{
								{
									Type:    "feat",
									Message: "add handler, config, and utils",
									Files:   []string{"handler.go", "config.yaml", "utils.go"},
								},
							},
						}),
					},
					FinishReason: "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Override provider factory
	origFactory := newProviderFunc
	newProviderFunc = func(config *types.UserConfig) (llm.Provider, error) {
		return &mockProvider{baseURL: mockServer.URL}, nil
	}
	defer func() { newProviderFunc = origFactory }()

	// Set up fake config
	fakeHome := t.TempDir()
	configDir := filepath.Join(fakeHome, ".commit-tool")
	if err := os.MkdirAll(filepath.Join(configDir, "logs", "executions"), 0700); err != nil {
		t.Fatal(err)
	}
	envContent := "COMMIT_PROVIDER=openai\nOPENAI_API_KEY=test-key\n"
	if err := os.WriteFile(filepath.Join(configDir, ".env"), []byte(envContent), 0600); err != nil {
		t.Fatal(err)
	}
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", fakeHome)       //nolint:errcheck // test setup
	defer os.Setenv("HOME", origHome) //nolint:errcheck // test cleanup

	// Change to temp dir
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)        //nolint:errcheck // test setup
	defer os.Chdir(origDir) //nolint:errcheck // test cleanup

	// Run execute with --single
	result := execute(flags{single: true}, nil)

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	if len(result.CommitsCreated) != 1 {
		t.Fatalf("expected exactly 1 commit created in single mode, got %d", len(result.CommitsCreated))
	}

	// Verify git log shows exactly 1 new commit (2 total: initial + single)
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = tmpDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-list failed: %v", err)
	}

	commitCount := strings.TrimSpace(string(out))
	if commitCount != "2" {
		t.Errorf("expected 2 total commits (initial + single), got %s", commitCount)
	}

	// Verify the commit message
	cmd = exec.Command("git", "log", "-1", "--format=%s")
	cmd.Dir = tmpDir
	out, err = cmd.Output()
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}

	lastMsg := strings.TrimSpace(string(out))
	if lastMsg != "feat: add handler, config, and utils" {
		t.Errorf("expected single commit message, got: %s", lastMsg)
	}
}

func TestE2E_ConfigError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// Create a temp git repo
	tmpDir := t.TempDir()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")

	// Create initial commit
	initialFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(initialFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "initial commit")

	// Create a change so we get past the "nothing to commit" check
	newFile := filepath.Join(tmpDir, "change.go")
	if err := os.WriteFile(newFile, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Set HOME to a temp dir WITHOUT .commit-tool/.env
	fakeHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", fakeHome)       //nolint:errcheck // test setup
	defer os.Setenv("HOME", origHome) //nolint:errcheck // test cleanup

	// Override provider factory (save/restore but should not be called)
	origFactory := newProviderFunc
	defer func() { newProviderFunc = origFactory }()

	// Change to temp dir
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)        //nolint:errcheck // test setup
	defer os.Chdir(origDir) //nolint:errcheck // test cleanup

	// Run execute -- should fail with config error
	result := execute(flags{}, nil)

	if result.ExitCode != 1 {
		t.Errorf("expected exit code 1 for config error, got %d", result.ExitCode)
	}

	if len(result.CommitsCreated) != 0 {
		t.Errorf("expected 0 commits created on config error, got %d", len(result.CommitsCreated))
	}
}

func TestE2E_HandleSetConfig(t *testing.T) {
	// Set up fake HOME with existing config
	fakeHome := t.TempDir()
	configDir := filepath.Join(fakeHome, ".commit-tool")
	if err := os.MkdirAll(filepath.Join(configDir, "logs", "executions"), 0700); err != nil {
		t.Fatal(err)
	}
	envContent := "COMMIT_PROVIDER=openai\nOPENAI_API_KEY=test-key\n"
	if err := os.WriteFile(filepath.Join(configDir, ".env"), []byte(envContent), 0600); err != nil {
		t.Fatal(err)
	}
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", fakeHome)       //nolint:errcheck // test setup
	defer os.Setenv("HOME", origHome) //nolint:errcheck // test cleanup

	// Test valid set
	code := handleSetConfig("defaultMode=single")
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	// Verify config was written
	content, err := os.ReadFile(filepath.Join(configDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(string(content), "COMMIT_DEFAULT_MODE=single") {
		t.Errorf("expected config to contain COMMIT_DEFAULT_MODE=single, got:\n%s", content)
	}

	// Test invalid format
	code = handleSetConfig("noequals")
	if code != 1 {
		t.Errorf("expected exit code 1 for invalid format, got %d", code)
	}

	// Test invalid key
	code = handleSetConfig("unknownKey=value")
	if code != 1 {
		t.Errorf("expected exit code 1 for unknown key, got %d", code)
	}

	// Test invalid value
	code = handleSetConfig("defaultMode=invalid")
	if code != 1 {
		t.Errorf("expected exit code 1 for invalid value, got %d", code)
	}
}

func TestE2E_HandleReverse(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	tmpDir := t.TempDir()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")

	// Create initial commit
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "initial commit")

	// Create a second commit to reverse
	if err := os.WriteFile(filepath.Join(tmpDir, "feature.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "feature.go")
	runGit("commit", "-m", "feat: add feature")

	// Verify we have 2 commits
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = tmpDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "2" {
		t.Fatalf("expected 2 commits, got %s", strings.TrimSpace(string(out)))
	}

	// Reverse the last commit
	code := handleReverse(tmpDir, 1, false, false)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	// Should be back to 1 commit
	cmd = exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = tmpDir
	out, err = cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "1" {
		t.Errorf("expected 1 commit after reverse, got %s", strings.TrimSpace(string(out)))
	}

	// feature.go should be uncommitted now
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = tmpDir
	out, err = cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(string(out), "feature.go") {
		t.Errorf("expected feature.go in working tree after reverse, got:\n%s", string(out))
	}
}

func TestE2E_HandleConfigError_AllBranches(t *testing.T) {
	// Set up fake HOME so EnsureConfigDir/CreateDefaultConfig don't touch real HOME
	fakeHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", fakeHome)       //nolint:errcheck // test setup
	defer os.Setenv("HOME", origHome) //nolint:errcheck // test cleanup

	// Test all error type branches in handleConfigError
	// We just need to verify these don't panic — they print to stdout
	handleConfigError(&config.ConfigNotFoundError{Path: "/fake/path"})
	handleConfigError(&config.ProviderNotConfiguredError{})
	handleConfigError(&config.InvalidProviderError{Provider: "fake"})
	handleConfigError(&config.MissingAPIKeyError{Provider: "openai", EnvVar: "OPENAI_API_KEY"})
	handleConfigError(&config.InvalidDefaultModeError{Mode: "bad"})
	handleConfigError(fmt.Errorf("generic error"))
}

// Mock types for the test server response
type chatCompletionResponse struct {
	Choices []chatCompletionChoice `json:"choices"`
}

type chatCompletionChoice struct {
	Message      chatCompletionMessage `json:"message"`
	FinishReason string                `json:"finish_reason"`
}

type chatCompletionMessage struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content"`
}

// mockProvider implements llm.Provider using a test HTTP server.
type mockProvider struct {
	baseURL string
}

func (p *mockProvider) Name() string  { return "openai" }
func (p *mockProvider) Model() string { return "test-model" }

func (p *mockProvider) Analyze(ctx context.Context, req *types.AnalysisRequest) (*types.CommitPlan, error) {
	// Call the mock server
	httpResp, err := http.Post(p.baseURL+"/v1/chat/completions", "application/json", nil)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close() //nolint:errcheck // test mock

	var resp chatCompletionResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, err
	}

	content := resp.Choices[0].Message.Content
	var plan types.CommitPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, err
	}

	return &plan, nil
}

func (p *mockProvider) AnalyzeDiff(ctx context.Context, system, user string) (string, error) {
	return "mock analysis", nil
}

func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsIndex(s, substr))
}

func containsIndex(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
