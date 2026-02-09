// Package llm provides LLM integration for commit analysis.
package llm

import (
	"context"
	"fmt"
	"strings"

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
1. docs: ONLY for documentation files (.md, .txt, .rst, README, CHANGELOG, LICENSE). Code files are NEVER docs.
2. feat: Changes that affect APPLICATION BEHAVIOR or user experience:
   - App code: new features, UI changes, CLI args, API endpoints
   - Terraform/IaC: new resources, new policies, changed configurations
   - HTML/templates: changes to markup affect what users see (NOT refactoring)
   - Anything that changes what gets deployed or how it behaves
3. fix: Corrects incorrect/broken behavior in the application
4. refactor: ONLY pure restructuring with IDENTICAL behavior - examples:
   - Moving code/resources between files
   - Extracting duplicated logic into a shared service class
   - Renaming variables/functions for clarity
   If the system does ANYTHING different after the change, it is NOT refactor.
5. chore: General-purpose type for non-application changes. Also the FALLBACK when no other type fits or when a preferred type is not allowed:
   - CI/CD pipeline changes, GitHub Actions, build scripts
   - Dependency updates, linting configs, dev tooling
   - Catch-all for maintenance work that does not fit other categories
6. Always bundle test files with their corresponding feature or fix - never separate tests from implementation
7. Only use "test" type for standalone tests with no corresponding implementation changes; if "test" is not allowed, use "chore"

TYPE SUBSTITUTION (when your preferred type is not in the allowed list):
The allowed types list is ABSOLUTE. If your natural choice is not in the list, substitute:
  refactor → chore (describe the restructuring in the message)
  style    → chore (describe the formatting in the message)
  perf     → feat  (describe the optimization in the message)
  test     → chore (describe the test changes in the message)
  any other → chore (chore is the general fallback)
When substituting, preserve intent in the commit message so the change is clear.

GROUPING:
8. Each commit should represent a single logical change
9. Group related file changes together

SCOPE:
10. The scope after → is the pre-computed MOST SPECIFIC scope for each file - use it exactly as shown
11. Do not substitute a more general scope even if it also matches the file path
12. If hasScopes is true, include scope in format "type(scope): message"
13. If hasScopes is false, use format "type: message"

MESSAGE FORMAT:
14. Use conventional commit format: "type(scope): message"
15. Message must be lowercase, imperative mood, no period at end
16. Message must not exceed the specified max length

OUTPUT FORMAT:
Return a JSON object with a "commits" array. Each commit has:
- type: commit type (ONLY use types from the allowed list)
- scope: scope name or null if no scope
- message: the commit message (without type/scope prefix)
- files: array of file paths included in this commit
- reasoning: brief explanation of why this grouping

Example responses:
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
}

{
  "commits": [
    {
      "type": "chore",
      "scope": "utils",
      "message": "reorganize helper functions for clarity",
      "files": ["src/utils/helpers.ts"],
      "reasoning": "Refactoring work - using chore since refactor not allowed"
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
- ALLOWED TYPES (use ONLY these, substituting per rules above): %s
- Max message length: %d characters
- Has scopes: %v
- Behavioral test: %s%s

Return JSON only, no markdown code blocks.`,
		formatFiles(req.Files),
		req.Diff,
		formatCommits(req.RecentCommits),
		formatTypes(req.Rules.Types),
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

func formatTypes(types []string) string {
	return strings.Join(types, " | ")
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
