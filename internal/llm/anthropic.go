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
	anthropicAPIURL     = "https://api.anthropic.com/v1/messages"
	anthropicAPIVersion = "2023-06-01"
	defaultAnthropicModel = "claude-3-5-sonnet-20241022"
)

// AnthropicProvider implements the Provider interface for Anthropic's Claude.
type AnthropicProvider struct {
	apiKey string
	model  string
	client *http.Client
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(apiKey, model string) (*AnthropicProvider, error) {
	assert.NotEmptyString(apiKey, "Anthropic API key is required")

	if model == "" {
		model = defaultAnthropicModel
	}

	return &AnthropicProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// Name returns the provider name.
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// Model returns the model being used.
func (p *AnthropicProvider) Model() string {
	return p.model
}

// Analyze sends an analysis request to Anthropic and returns a commit plan.
func (p *AnthropicProvider) Analyze(ctx context.Context, req *types.AnalysisRequest) (*types.CommitPlan, error) {
	assert.NotNil(req, "analysis request cannot be nil")
	assert.NotEmpty(req.Files, "analysis request must have files")

	systemPrompt, userPrompt := BuildPrompt(req)

	requestBody := anthropicRequest{
		Model:     p.model,
		MaxTokens: 2000,
		System:    systemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: userPrompt},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, &ProviderError{Provider: "anthropic", Message: "failed to marshal request", Err: err}
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", anthropicAPIURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &ProviderError{Provider: "anthropic", Message: "failed to create request", Err: err}
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &ProviderError{Provider: "anthropic", Message: "request failed", Err: err}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ProviderError{Provider: "anthropic", Message: "failed to read response", Err: err}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &ProviderError{
			Provider: "anthropic",
			Message:  fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(respBody)),
		}
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, &ProviderError{Provider: "anthropic", Message: "failed to parse response", Err: err}
	}

	if len(anthropicResp.Content) == 0 {
		return nil, &ProviderError{Provider: "anthropic", Message: "empty response from API"}
	}

	content := anthropicResp.Content[0].Text

	// Clean up and parse
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var plan types.CommitPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, &ProviderError{Provider: "anthropic", Message: "failed to parse commit plan", Err: err}
	}

	return &plan, nil
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
	Usage   anthropicUsage     `json:"usage"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
