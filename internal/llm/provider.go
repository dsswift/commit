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

TYPE SELECTION:
1. Type MUST be from the allowed types list - never use any other type
2. docs type: ONLY for actual documentation files (.md, .txt, .rst, README, CHANGELOG, LICENSE, etc.). Changes to code files (.go, .py, .js, .ts, .java, .cs, .rs, etc.) are NEVER docs - even if the change is to comments, strings, or embedded text. For code file changes, use: feat (behavior change), fix (bug fix), chore (maintenance/config), or refactor (restructure)
3. feat vs refactor: ANY user-perceivable change is feat (UI changes, new CLI args/aliases, button colors, autocomplete behavior, etc.). fix = correcting incorrect behavior. refactor = 100% non-functional with zero behavior change - only internal code structure changes invisible to users
4. Always bundle test files with their corresponding feature or fix - never separate tests from implementation
5. Only use "test" type for standalone tests with no corresponding implementation changes; if "test" is not allowed, use "chore"

GROUPING:
6. Each commit should represent a single logical change
7. Group related file changes together

SCOPE:
8. The scope after → is the pre-computed MOST SPECIFIC scope for each file - use it exactly as shown
9. Do not substitute a more general scope even if it also matches the file path
10. If hasScopes is true, include scope in format "type(scope): message"
11. If hasScopes is false, use format "type: message"

MESSAGE FORMAT:
12. Use conventional commit format: "type(scope): message"
13. Message must be lowercase, imperative mood, no period at end
14. Message must not exceed the specified max length

OUTPUT FORMAT:
Return a JSON object with a "commits" array. Each commit has:
- type: commit type (ONLY use types from the allowed list)
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

FILES (path [status] diff_summary → assigned_scope):
%s

DIFF:
%s

RECENT COMMITS (for style reference):
%s

RULES:
- Allowed types (ONLY use these, no other types): %v
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
