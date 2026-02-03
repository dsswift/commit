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
	defaultAzureFoundryTimeout = 60 * time.Second
	azureAnthropicAPIVersion   = "2023-06-01"
	azureOpenAIAPIVersion      = "2024-02-15-preview"
)

// AzureFoundryProvider implements the Provider interface for Azure AI Foundry.
// Automatically detects whether to use Anthropic or OpenAI API format based on deployment name.
type AzureFoundryProvider struct {
	endpoint   string
	apiKey     string
	deployment string
	model      string
	client     *http.Client
	isAnthropic bool
}

// NewAzureFoundryProvider creates a new Azure Foundry provider.
func NewAzureFoundryProvider(endpoint, apiKey, deployment, model string) (*AzureFoundryProvider, error) {
	assert.NotEmptyString(endpoint, "Azure Foundry endpoint is required")
	assert.NotEmptyString(apiKey, "Azure Foundry API key is required")
	assert.NotEmptyString(deployment, "Azure Foundry deployment name is required")

	// Normalize endpoint - remove trailing slash
	endpoint = strings.TrimSuffix(endpoint, "/")

	// Detect if this is an Anthropic model based on deployment name
	isAnthropic := isAnthropicDeployment(deployment)

	return &AzureFoundryProvider{
		endpoint:    endpoint,
		apiKey:      apiKey,
		deployment:  deployment,
		model:       model,
		isAnthropic: isAnthropic,
		client: &http.Client{
			Timeout: defaultAzureFoundryTimeout,
		},
	}, nil
}

// isAnthropicDeployment checks if the deployment name indicates an Anthropic model.
func isAnthropicDeployment(deployment string) bool {
	lower := strings.ToLower(deployment)
	return strings.Contains(lower, "claude")
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
	assert.NotNil(req, "analysis request cannot be nil")
	assert.NotEmpty(req.Files, "analysis request must have files")

	systemPrompt, userPrompt := BuildPrompt(req)

	var content string
	var err error

	if p.isAnthropic {
		content, err = p.callAnthropicAPI(ctx, systemPrompt, userPrompt)
	} else {
		content, err = p.callOpenAIAPI(ctx, systemPrompt, userPrompt)
	}

	if err != nil {
		return nil, err
	}

	plan, err := parseCommitPlan(content)
	if err != nil {
		return nil, &ProviderError{Provider: "azure-foundry", Message: "failed to parse commit plan", Err: err}
	}

	assert.NotNil(plan, "commit plan should not be nil")
	return plan, nil
}

// AnalyzeDiff sends a diff analysis request to Azure Foundry and returns the analysis.
func (p *AzureFoundryProvider) AnalyzeDiff(ctx context.Context, system, user string) (string, error) {
	if p.isAnthropic {
		return p.callAnthropicAPI(ctx, system, user)
	}
	return p.callOpenAIAPI(ctx, system, user)
}

// callAnthropicAPI makes a request using the Anthropic Messages API format.
func (p *AzureFoundryProvider) callAnthropicAPI(ctx context.Context, system, user string) (string, error) {
	requestBody := anthropicAPIRequest{
		Model:     p.deployment,
		MaxTokens: 8192,
		System:    system,
		Messages: []anthropicAPIMessage{
			{Role: "user", Content: user},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return "", &ProviderError{Provider: "azure-foundry", Message: "failed to marshal request", Err: err}
	}

	url := fmt.Sprintf("%s/anthropic/v1/messages", p.endpoint)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", &ProviderError{Provider: "azure-foundry", Message: "failed to create request", Err: err}
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("anthropic-version", azureAnthropicAPIVersion)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", &ProviderError{Provider: "azure-foundry", Message: "request failed", Err: err}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", &ProviderError{Provider: "azure-foundry", Message: "failed to read response", Err: err}
	}

	if resp.StatusCode != http.StatusOK {
		return "", &ProviderError{
			Provider: "azure-foundry",
			Message:  fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(respBody)),
		}
	}

	var anthropicResp anthropicAPIResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return "", &ProviderError{Provider: "azure-foundry", Message: "failed to parse response", Err: err}
	}

	if len(anthropicResp.Content) == 0 {
		return "", &ProviderError{Provider: "azure-foundry", Message: "empty response from API"}
	}

	if anthropicResp.StopReason == "max_tokens" {
		return "", &ProviderError{Provider: "azure-foundry", Message: "response truncated: exceeded max tokens limit"}
	}

	return anthropicResp.Content[0].Text, nil
}

// callOpenAIAPI makes a request using the OpenAI-compatible API format.
func (p *AzureFoundryProvider) callOpenAIAPI(ctx context.Context, system, user string) (string, error) {
	requestBody := openAIAPIRequest{
		Messages: []openAIAPIMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.3,
		MaxTokens:   8192,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return "", &ProviderError{Provider: "azure-foundry", Message: "failed to marshal request", Err: err}
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		p.endpoint, p.deployment, azureOpenAIAPIVersion)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", &ProviderError{Provider: "azure-foundry", Message: "failed to create request", Err: err}
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", &ProviderError{Provider: "azure-foundry", Message: "request failed", Err: err}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", &ProviderError{Provider: "azure-foundry", Message: "failed to read response", Err: err}
	}

	if resp.StatusCode != http.StatusOK {
		return "", &ProviderError{
			Provider: "azure-foundry",
			Message:  fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(respBody)),
		}
	}

	var openAIResp openAIAPIResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return "", &ProviderError{Provider: "azure-foundry", Message: "failed to parse response", Err: err}
	}

	if len(openAIResp.Choices) == 0 {
		return "", &ProviderError{Provider: "azure-foundry", Message: "empty response from API"}
	}

	if openAIResp.Choices[0].FinishReason == "length" {
		return "", &ProviderError{Provider: "azure-foundry", Message: "response truncated: exceeded max tokens limit"}
	}

	return openAIResp.Choices[0].Message.Content, nil
}

// parseCommitPlan extracts a CommitPlan from the LLM response content.
func parseCommitPlan(content string) (*types.CommitPlan, error) {
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

// Anthropic API types

type anthropicAPIRequest struct {
	Model     string               `json:"model"`
	MaxTokens int                  `json:"max_tokens"`
	System    string               `json:"system,omitempty"`
	Messages  []anthropicAPIMessage `json:"messages"`
}

type anthropicAPIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicAPIResponse struct {
	Content    []anthropicAPIContent `json:"content"`
	Usage      anthropicAPIUsage     `json:"usage"`
	StopReason string                `json:"stop_reason"`
}

type anthropicAPIContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicAPIUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// OpenAI API types

type openAIAPIRequest struct {
	Messages    []openAIAPIMessage `json:"messages"`
	Temperature float64            `json:"temperature,omitempty"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
}

type openAIAPIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIAPIResponse struct {
	Choices []openAIAPIChoice `json:"choices"`
	Usage   openAIAPIUsage    `json:"usage"`
}

type openAIAPIChoice struct {
	Message      openAIAPIMessage `json:"message"`
	FinishReason string           `json:"finish_reason"`
}

type openAIAPIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
