package llm

import (
	"testing"

	"github.com/dsswift/commit/pkg/types"
)

func TestBuildPrompt(t *testing.T) {
	req := &types.AnalysisRequest{
		Files: []types.FileChange{
			{Path: "src/api/handler.go", Status: "modified", Scope: "api", DiffSummary: "+10 -5"},
			{Path: "docs/readme.md", Status: "added", Scope: "", DiffSummary: "+50"},
		},
		Diff:          "diff content here",
		RecentCommits: []string{"feat: add auth", "fix: handle error"},
		HasScopes:     true,
		Rules: types.CommitRules{
			Types:            []string{"feat", "fix", "docs"},
			MaxMessageLength: 50,
			BehavioralTest:   "feat = behavior change",
		},
	}

	system, user := BuildPrompt(req)

	// System prompt checks
	if system == "" {
		t.Error("expected non-empty system prompt")
	}

	if !containsString(system, "git commit message generator") {
		t.Error("system prompt should mention purpose")
	}

	if !containsString(system, "conventional commit format") {
		t.Error("system prompt should mention conventional commits")
	}

	// User prompt checks
	if user == "" {
		t.Error("expected non-empty user prompt")
	}

	if !containsString(user, "src/api/handler.go") {
		t.Error("user prompt should contain file paths")
	}

	if !containsString(user, "diff content here") {
		t.Error("user prompt should contain diff")
	}

	if !containsString(user, "feat: add auth") {
		t.Error("user prompt should contain recent commits")
	}

	if !containsString(user, "Has scopes: true") {
		t.Error("user prompt should indicate hasScopes")
	}
}

func TestBuildPrompt_NoRecentCommits(t *testing.T) {
	req := &types.AnalysisRequest{
		Files: []types.FileChange{
			{Path: "file.go", Status: "modified"},
		},
		Diff:          "diff",
		RecentCommits: []string{},
		HasScopes:     false,
		Rules: types.CommitRules{
			Types:            []string{"feat", "fix"},
			MaxMessageLength: 50,
		},
	}

	_, user := BuildPrompt(req)

	if !containsString(user, "(no recent commits)") {
		t.Error("user prompt should indicate no recent commits")
	}
}

func TestBuildPrompt_SingleCommit(t *testing.T) {
	req := &types.AnalysisRequest{
		Files: []types.FileChange{
			{Path: "file1.go", Status: "modified"},
			{Path: "file2.go", Status: "added"},
		},
		Diff:         "diff",
		HasScopes:    false,
		SingleCommit: true,
		Rules: types.CommitRules{
			Types:            []string{"feat", "fix"},
			MaxMessageLength: 50,
		},
	}

	_, user := BuildPrompt(req)

	if !containsString(user, "ONE commit") {
		t.Error("user prompt should instruct single commit when SingleCommit is true")
	}
}

func TestBuildPrompt_MultipleCommits(t *testing.T) {
	req := &types.AnalysisRequest{
		Files: []types.FileChange{
			{Path: "file1.go", Status: "modified"},
		},
		Diff:         "diff",
		HasScopes:    false,
		SingleCommit: false,
		Rules: types.CommitRules{
			Types:            []string{"feat", "fix"},
			MaxMessageLength: 50,
		},
	}

	_, user := BuildPrompt(req)

	if containsString(user, "ONE commit") {
		t.Error("user prompt should NOT instruct single commit when SingleCommit is false")
	}
}

func TestFormatTypes(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{[]string{"feat", "fix", "chore"}, "feat | fix | chore"},
		{[]string{"feat"}, "feat"},
		{[]string{}, ""},
	}
	for _, tt := range tests {
		result := formatTypes(tt.input)
		if result != tt.expected {
			t.Errorf("formatTypes(%v) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestBuildPrompt_TypeSubstitution(t *testing.T) {
	req := &types.AnalysisRequest{
		Files:     []types.FileChange{{Path: "file.go", Status: "modified"}},
		Diff:      "diff",
		HasScopes: false,
		Rules:     types.CommitRules{Types: []string{"feat", "fix", "chore"}, MaxMessageLength: 50},
	}
	system, user := BuildPrompt(req)

	if !containsString(system, "TYPE SUBSTITUTION") {
		t.Error("system prompt should contain TYPE SUBSTITUTION section")
	}
	if !containsString(system, "refactor") {
		t.Error("system prompt should mention refactor in substitution rules")
	}
	if !containsString(system, "chore") {
		t.Error("system prompt should mention chore as fallback")
	}
	if !containsString(user, "feat | fix | chore") {
		t.Error("user prompt should format types with pipe separators")
	}
}

func TestParseCommitPlan_ValidJSON(t *testing.T) {
	content := `{
		"commits": [
			{
				"type": "feat",
				"scope": "api",
				"message": "add new endpoint",
				"files": ["src/api/handler.go"],
				"reasoning": "New feature"
			}
		]
	}`

	plan, err := parseCommitPlan(content)
	if err != nil {
		t.Fatalf("parseCommitPlan failed: %v", err)
	}

	if len(plan.Commits) != 1 {
		t.Errorf("expected 1 commit, got %d", len(plan.Commits))
	}

	commit := plan.Commits[0]
	if commit.Type != "feat" {
		t.Errorf("expected type 'feat', got %q", commit.Type)
	}

	if commit.Scope == nil || *commit.Scope != "api" {
		t.Error("expected scope 'api'")
	}

	if commit.Message != "add new endpoint" {
		t.Errorf("expected message 'add new endpoint', got %q", commit.Message)
	}
}

func TestParseCommitPlan_WithMarkdown(t *testing.T) {
	content := "```json\n{\"commits\": []}\n```"

	plan, err := parseCommitPlan(content)
	if err != nil {
		t.Fatalf("parseCommitPlan failed: %v", err)
	}

	if plan == nil {
		t.Error("expected non-nil plan")
	}
}

func TestParseCommitPlan_InvalidJSON(t *testing.T) {
	content := "not valid json"

	_, err := parseCommitPlan(content)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseCommitPlan_NullScope(t *testing.T) {
	content := `{
		"commits": [
			{
				"type": "docs",
				"scope": null,
				"message": "update readme",
				"files": ["README.md"],
				"reasoning": "Documentation update"
			}
		]
	}`

	plan, err := parseCommitPlan(content)
	if err != nil {
		t.Fatalf("parseCommitPlan failed: %v", err)
	}

	if plan.Commits[0].Scope != nil {
		t.Error("expected nil scope")
	}
}

func TestNewProvider_Anthropic(t *testing.T) {
	config := &types.UserConfig{
		Provider:        "anthropic",
		AnthropicAPIKey: "test-key",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	if provider.Name() != "anthropic" {
		t.Errorf("expected name 'anthropic', got %q", provider.Name())
	}
}

func TestNewProvider_OpenAI(t *testing.T) {
	config := &types.UserConfig{
		Provider:     "openai",
		OpenAIAPIKey: "test-key",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	if provider.Name() != "openai" {
		t.Errorf("expected name 'openai', got %q", provider.Name())
	}
}

func TestNewProvider_Grok(t *testing.T) {
	config := &types.UserConfig{
		Provider:   "grok",
		GrokAPIKey: "test-key",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	if provider.Name() != "grok" {
		t.Errorf("expected name 'grok', got %q", provider.Name())
	}
}

func TestNewProvider_Gemini(t *testing.T) {
	config := &types.UserConfig{
		Provider:     "gemini",
		GeminiAPIKey: "test-key",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	if provider.Name() != "gemini" {
		t.Errorf("expected name 'gemini', got %q", provider.Name())
	}
}

func TestNewProvider_AzureFoundry(t *testing.T) {
	config := &types.UserConfig{
		Provider:               "azure-foundry",
		AzureFoundryEndpoint:   "https://test.openai.azure.com",
		AzureFoundryAPIKey:     "test-key",
		AzureFoundryDeployment: "gpt-4",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	if provider.Name() != "azure-foundry" {
		t.Errorf("expected name 'azure-foundry', got %q", provider.Name())
	}
}

func TestNewProvider_Unsupported(t *testing.T) {
	config := &types.UserConfig{
		Provider: "unsupported",
	}

	_, err := NewProvider(config)
	if err == nil {
		t.Error("expected error for unsupported provider")
	}
}

func TestProviderError(t *testing.T) {
	err := &ProviderError{
		Provider: "anthropic",
		Message:  "API call failed",
	}

	if err.Error() != "anthropic: API call failed" {
		t.Errorf("unexpected error message: %q", err.Error())
	}

	// With wrapped error
	wrapped := &ProviderError{
		Provider: "openai",
		Message:  "request failed",
		Err:      &testError{"connection refused"},
	}

	if !containsString(wrapped.Error(), "connection refused") {
		t.Errorf("expected wrapped error in message: %q", wrapped.Error())
	}

	if wrapped.Unwrap() == nil {
		t.Error("expected Unwrap to return wrapped error")
	}
}

func TestAnthropicProvider_Model(t *testing.T) {
	provider, _ := NewAnthropicProvider("key", "")
	if provider.Model() != defaultAnthropicModel {
		t.Errorf("expected default model, got %q", provider.Model())
	}

	provider, _ = NewAnthropicProvider("key", "custom-model")
	if provider.Model() != "custom-model" {
		t.Errorf("expected 'custom-model', got %q", provider.Model())
	}
}

func TestOpenAIProvider_Model(t *testing.T) {
	provider, _ := NewOpenAIProvider("key", "")
	if provider.Model() != defaultOpenAIModel {
		t.Errorf("expected default model, got %q", provider.Model())
	}
}

func TestGrokProvider_Model(t *testing.T) {
	provider, _ := NewGrokProvider("key", "")
	if provider.Model() != defaultGrokModel {
		t.Errorf("expected default model, got %q", provider.Model())
	}
}

func TestGeminiProvider_Model(t *testing.T) {
	provider, _ := NewGeminiProvider("key", "")
	if provider.Model() != defaultGeminiModel {
		t.Errorf("expected default model, got %q", provider.Model())
	}
}

func TestAzureFoundryProvider_Model(t *testing.T) {
	provider, _ := NewAzureFoundryProvider("https://test.com", "key", "my-deployment", "")
	if provider.Model() != "my-deployment" {
		t.Errorf("expected 'my-deployment', got %q", provider.Model())
	}

	provider, _ = NewAzureFoundryProvider("https://test.com", "key", "my-deployment", "custom-model")
	if provider.Model() != "custom-model" {
		t.Errorf("expected 'custom-model', got %q", provider.Model())
	}
}

// Helper types and functions

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
