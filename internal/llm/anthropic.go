package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/dsswift/commit/internal/assert"
	"github.com/dsswift/commit/pkg/types"
)

const (
	anthropicAPIURL       = "https://api.anthropic.com/v1/messages"
	anthropicAPIVersion   = "2023-06-01"
	defaultAnthropicModel = "claude-3-5-sonnet-20241022"
)

// AnthropicProvider implements the Provider interface for Anthropic's Claude.
type AnthropicProvider struct {
	apiKey  string
	model   string
	client  *http.Client
	baseURL string
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(apiKey, model string) (*AnthropicProvider, error) {
	assert.NotEmptyString(apiKey, "Anthropic API key is required")

	if model == "" {
		model = defaultAnthropicModel
	}

	return &AnthropicProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: anthropicAPIURL,
		client:  newHTTPClient(60 * time.Second),
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
		MaxTokens: 8192,
		System:    systemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: userPrompt},
		},
	}

	resp, err := doRequest(&llmRequest{
		ctx:      ctx,
		client:   p.client,
		method:   "POST",
		url:      p.baseURL,
		headers:  p.headers(),
		body:     requestBody,
		provider: "anthropic",
	})
	if err != nil {
		return nil, err
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(resp.Body, &anthropicResp); err != nil {
		return nil, &ProviderError{Provider: "anthropic", Message: "failed to parse response", Err: err}
	}

	if len(anthropicResp.Content) == 0 {
		return nil, &ProviderError{Provider: "anthropic", Message: "empty response from API"}
	}

	if anthropicResp.StopReason == "max_tokens" {
		return nil, &ProviderError{Provider: "anthropic", Message: "response truncated: exceeded max tokens limit"}
	}

	content := cleanContent(anthropicResp.Content[0].Text)

	var plan types.CommitPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, &ProviderError{Provider: "anthropic", Message: "failed to parse commit plan", Err: err}
	}

	return &plan, nil
}

// AnalyzeDiff sends a diff analysis request to Anthropic and returns the analysis.
func (p *AnthropicProvider) AnalyzeDiff(ctx context.Context, system, user string) (string, error) {
	requestBody := anthropicRequest{
		Model:     p.model,
		MaxTokens: 8192,
		System:    system,
		Messages: []anthropicMessage{
			{Role: "user", Content: user},
		},
	}

	resp, err := doRequest(&llmRequest{
		ctx:      ctx,
		client:   p.client,
		method:   "POST",
		url:      p.baseURL,
		headers:  p.headers(),
		body:     requestBody,
		provider: "anthropic",
	})
	if err != nil {
		return "", err
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(resp.Body, &anthropicResp); err != nil {
		return "", &ProviderError{Provider: "anthropic", Message: "failed to parse response", Err: err}
	}

	if len(anthropicResp.Content) == 0 {
		return "", &ProviderError{Provider: "anthropic", Message: "empty response from API"}
	}

	if anthropicResp.StopReason == "max_tokens" {
		return "", &ProviderError{Provider: "anthropic", Message: "response truncated: exceeded max tokens limit"}
	}

	return anthropicResp.Content[0].Text, nil
}

func (p *AnthropicProvider) headers() map[string]string {
	return map[string]string{
		"Content-Type":      "application/json",
		"x-api-key":         p.apiKey,
		"anthropic-version": anthropicAPIVersion,
	}
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
	Content    []anthropicContent `json:"content"`
	Usage      anthropicUsage     `json:"usage"`
	StopReason string             `json:"stop_reason"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
