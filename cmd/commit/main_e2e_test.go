package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Override provider factory to use mock server
	origFactory := newProviderFunc
	newProviderFunc = func(config *types.UserConfig) (llm.Provider, error) {
		return &mockProvider{baseURL: mockServer.URL}, nil
	}
	defer func() { newProviderFunc = origFactory }()

	// Change to temp dir
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

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
	defer httpResp.Body.Close()

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
