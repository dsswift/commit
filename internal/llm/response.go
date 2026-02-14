package llm

import (
	"encoding/json"

	"github.com/dsswift/commit/pkg/types"
)

// processAnalyzeResponse validates and parses an LLM response into a CommitPlan.
// It checks for empty content, truncation, strips markdown fences, and unmarshals JSON.
func processAnalyzeResponse(provider, content string, truncated bool) (*types.CommitPlan, error) {
	if content == "" {
		return nil, &ProviderError{Provider: provider, Message: "empty response from API"}
	}

	if truncated {
		return nil, &ProviderError{Provider: provider, Message: "response truncated: exceeded max tokens limit"}
	}

	content = cleanContent(content)

	var plan types.CommitPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, &ProviderError{Provider: provider, Message: "failed to parse commit plan", Err: err}
	}

	return &plan, nil
}

// processTextResponse validates an LLM text response.
// It checks for empty content and truncation.
func processTextResponse(provider, content string, truncated bool) (string, error) {
	if content == "" {
		return "", &ProviderError{Provider: provider, Message: "empty response from API"}
	}

	if truncated {
		return "", &ProviderError{Provider: provider, Message: "response truncated: exceeded max tokens limit"}
	}

	return content, nil
}
