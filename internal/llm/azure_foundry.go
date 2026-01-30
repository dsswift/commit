package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dsswift/commit/internal/assert"
	"github.com/dsswift/commit/pkg/types"
)

const (
	defaultAzureAPIVersion = "2024-02-15-preview"
	defaultAzureTimeout    = 60 * time.Second
)

// AzureFoundryProvider implements the Provider interface for Azure AI Foundry.
type AzureFoundryProvider struct {
	endpoint   string
	apiKey     string
	deployment string
	model      string
	client     *http.Client
}

// NewAzureFoundryProvider creates a new Azure Foundry provider.
func NewAzureFoundryProvider(endpoint, apiKey, deployment, model string) (*AzureFoundryProvider, error) {
	assert.NotEmptyString(endpoint, "Azure Foundry endpoint is required")
	assert.NotEmptyString(apiKey, "Azure Foundry API key is required")
	assert.NotEmptyString(deployment, "Azure Foundry deployment name is required")

	// Normalize endpoint - remove trailing slash
	endpoint = strings.TrimSuffix(endpoint, "/")

	return &AzureFoundryProvider{
		endpoint:   endpoint,
		apiKey:     apiKey,
		deployment: deployment,
		model:      model,
		client: &http.Client{
			Timeout: defaultAzureTimeout,
		},
	}, nil
}

// Name returns the provider name.
func (p *AzureFoundryProvider) Name() string {
	return "azure-foundry"
}

// Model returns the model/deployment being used.
func (p *AzureFoundryProvider) Model() string {
	if p.model != "" {
		return p.model
	}
	return p.deployment
}

// Analyze sends an analysis request to Azure Foundry and returns a commit plan.
func (p *AzureFoundryProvider) Analyze(ctx context.Context, req *types.AnalysisRequest) (*types.CommitPlan, error) {
	// PRECONDITIONS
	assert.NotNil(req, "analysis request cannot be nil")
	assert.NotEmpty(req.Files, "analysis request must have files")

	systemPrompt, userPrompt := BuildPrompt(req)

	// Build the request body (OpenAI-compatible format)
	requestBody := azureChatRequest{
		Messages: []azureChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.3, // Lower temperature for more deterministic output
		MaxTokens:   2000,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, &ProviderError{Provider: "azure-foundry", Message: "failed to marshal request", Err: err}
	}

	// Build the URL
	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		p.endpoint, p.deployment, defaultAzureAPIVersion)

	// Create the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &ProviderError{Provider: "azure-foundry", Message: "failed to create request", Err: err}
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", p.apiKey)

	// Execute the request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &ProviderError{Provider: "azure-foundry", Message: "request failed", Err: err}
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ProviderError{Provider: "azure-foundry", Message: "failed to read response", Err: err}
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, &ProviderError{
			Provider: "azure-foundry",
			Message:  fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(respBody)),
		}
	}

	// Parse the response
	var chatResp azureChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, &ProviderError{Provider: "azure-foundry", Message: "failed to parse response", Err: err}
	}

	if len(chatResp.Choices) == 0 {
		return nil, &ProviderError{Provider: "azure-foundry", Message: "empty response from API"}
	}

	// Extract the content
	content := chatResp.Choices[0].Message.Content

	// Parse the commit plan from the response
	plan, err := parseCommitPlan(content)
	if err != nil {
		return nil, &ProviderError{Provider: "azure-foundry", Message: "failed to parse commit plan", Err: err}
	}

	// POSTCONDITIONS
	assert.NotNil(plan, "commit plan should not be nil")

	return plan, nil
}

// parseCommitPlan extracts a CommitPlan from the LLM response content.
func parseCommitPlan(content string) (*types.CommitPlan, error) {
	// Clean up the content - remove markdown code blocks if present
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var plan types.CommitPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w\nContent: %s", err, content)
	}

	return &plan, nil
}

// Azure API types (OpenAI-compatible)

type azureChatRequest struct {
	Messages    []azureChatMessage `json:"messages"`
	Temperature float64            `json:"temperature,omitempty"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
}

type azureChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type azureChatResponse struct {
	Choices []azureChatChoice `json:"choices"`
	Usage   azureUsage        `json:"usage"`
}

type azureChatChoice struct {
	Message      azureChatMessage `json:"message"`
	FinishReason string           `json:"finish_reason"`
}

type azureUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
