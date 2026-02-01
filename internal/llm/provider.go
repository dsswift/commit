// Package llm provides LLM integration for commit analysis.
package llm

import (
	"context"
	"fmt"

	"github.com/dsswift/commit/pkg/types"
)

// Provider is the interface for LLM providers.
type Provider interface {
	// Analyze sends an analysis request to the LLM and returns a commit plan.
	Analyze(ctx context.Context, req *types.AnalysisRequest) (*types.CommitPlan, error)

	// AnalyzeDiff sends a diff analysis request to the LLM and returns the analysis.
	AnalyzeDiff(ctx context.Context, system, user string) (string, error)

	// Name returns the provider name.
	Name() string

	// Model returns the model being used.
	Model() string
}

// NewProvider creates a provider based on the user configuration.
func NewProvider(config *types.UserConfig) (Provider, error) {
	switch config.Provider {
	case "azure-foundry":
		return NewAzureFoundryProvider(
			config.AzureFoundryEndpoint,
			config.AzureFoundryAPIKey,
			config.AzureFoundryDeployment,
			config.Model,
		)
	case "anthropic":
		return NewAnthropicProvider(config.AnthropicAPIKey, config.Model)
	case "openai":
		return NewOpenAIProvider(config.OpenAIAPIKey, config.Model)
	case "grok":
		return NewGrokProvider(config.GrokAPIKey, config.Model)
	case "gemini":
		return NewGeminiProvider(config.GeminiAPIKey, config.Model)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", config.Provider)
	}
}

// BuildPrompt creates the system and user prompts for commit analysis.
func BuildPrompt(req *types.AnalysisRequest) (system string, user string) {
	system = `You are a git commit message generator. Analyze the provided code changes and create semantic commits.

RULES:
1. Each commit should represent a single logical change
2. Use conventional commit format: type(scope): message
3. Message must be lowercase, imperative mood, no period at end
4. Message must not exceed the specified max length
5. Type must be one of the allowed types
6. If hasScopes is true, include scope in format type(scope): message
7. If hasScopes is false, use format type: message
8. Group related file changes together
9. feat = changes behavior/output, refactor = same behavior different structure

OUTPUT FORMAT:
Return a JSON object with a "commits" array. Each commit has:
- type: commit type (feat, fix, docs, etc.)
- scope: scope name or null if no scope
- message: the commit message (without type/scope prefix)
- files: array of file paths included in this commit
- reasoning: brief explanation of why this grouping

Example response:
{
  "commits": [
    {
      "type": "feat",
      "scope": "auth",
      "message": "add logout functionality",
      "files": ["src/auth/logout.ts"],
      "reasoning": "New file adding logout behavior"
    }
  ]
}`

	singleCommitRule := ""
	if req.SingleCommit {
		singleCommitRule = "\n- IMPORTANT: Create exactly ONE commit containing ALL files"
	}

	user = fmt.Sprintf(`Analyze these changes and create semantic commits:

FILES:
%s

DIFF:
%s

RECENT COMMITS (for style reference):
%s

RULES:
- Allowed types: %v
- Max message length: %d characters
- Has scopes: %v
- Behavioral test: %s%s

Return JSON only, no markdown code blocks.`,
		formatFiles(req.Files),
		req.Diff,
		formatCommits(req.RecentCommits),
		req.Rules.Types,
		req.Rules.MaxMessageLength,
		req.HasScopes,
		req.Rules.BehavioralTest,
		singleCommitRule,
	)

	return system, user
}

func formatFiles(files []types.FileChange) string {
	result := ""
	for _, f := range files {
		scope := f.Scope
		if scope == "" {
			scope = "(no scope)"
		}
		result += fmt.Sprintf("- %s [%s] %s → %s\n", f.Path, f.Status, f.DiffSummary, scope)
	}
	return result
}

func formatCommits(commits []string) string {
	if len(commits) == 0 {
		return "(no recent commits)"
	}
	result := ""
	for _, c := range commits {
		result += fmt.Sprintf("- %s\n", c)
	}
	return result
}

// ProviderError wraps errors from LLM providers.
type ProviderError struct {
	Provider string
	Message  string
	Err      error
}

func (e *ProviderError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Provider, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Provider, e.Message)
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}
