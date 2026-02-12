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
	grokAPIURL       = "https://api.x.ai/v1/chat/completions"
	defaultGrokModel = "grok-beta"
)

// GrokProvider implements the Provider interface for xAI's Grok.
type GrokProvider struct {
	apiKey  string
	model   string
	client  *http.Client
	baseURL string
}

// NewGrokProvider creates a new Grok provider.
func NewGrokProvider(apiKey, model string) (*GrokProvider, error) {
	assert.NotEmptyString(apiKey, "Grok API key is required")

	if model == "" {
		model = defaultGrokModel
	}

	return &GrokProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: grokAPIURL,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// Name returns the provider name.
func (p *GrokProvider) Name() string {
	return "grok"
}

// Model returns the model being used.
func (p *GrokProvider) Model() string {
	return p.model
}

// Analyze sends an analysis request to Grok and returns a commit plan.
// Grok uses an OpenAI-compatible API.
func (p *GrokProvider) Analyze(ctx context.Context, req *types.AnalysisRequest) (*types.CommitPlan, error) {
	assert.NotNil(req, "analysis request cannot be nil")
	assert.NotEmpty(req.Files, "analysis request must have files")

	systemPrompt, userPrompt := BuildPrompt(req)

	requestBody := grokRequest{
		Model: p.model,
		Messages: []grokMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.3,
		MaxTokens:   8192,
	}

	resp, err := doRequest(&llmRequest{
		ctx:      ctx,
		client:   p.client,
		method:   "POST",
		url:      p.baseURL,
		headers:  p.headers(),
		body:     requestBody,
		provider: "grok",
	})
	if err != nil {
		return nil, err
	}

	var grokResp grokResponse
	if err := json.Unmarshal(resp.Body, &grokResp); err != nil {
		return nil, &ProviderError{Provider: "grok", Message: "failed to parse response", Err: err}
	}

	if len(grokResp.Choices) == 0 {
		return nil, &ProviderError{Provider: "grok", Message: "empty response from API"}
	}

	if grokResp.Choices[0].FinishReason == "length" {
		return nil, &ProviderError{Provider: "grok", Message: "response truncated: exceeded max tokens limit"}
	}

	content := cleanContent(grokResp.Choices[0].Message.Content)

	var plan types.CommitPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, &ProviderError{Provider: "grok", Message: "failed to parse commit plan", Err: err}
	}

	return &plan, nil
}

// AnalyzeDiff sends a diff analysis request to Grok and returns the analysis.
func (p *GrokProvider) AnalyzeDiff(ctx context.Context, system, user string) (string, error) {
	requestBody := grokRequest{
		Model: p.model,
		Messages: []grokMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.3,
		MaxTokens:   8192,
	}

	resp, err := doRequest(&llmRequest{
		ctx:      ctx,
		client:   p.client,
		method:   "POST",
		url:      p.baseURL,
		headers:  p.headers(),
		body:     requestBody,
		provider: "grok",
	})
	if err != nil {
		return "", err
	}

	var grokResp grokResponse
	if err := json.Unmarshal(resp.Body, &grokResp); err != nil {
		return "", &ProviderError{Provider: "grok", Message: "failed to parse response", Err: err}
	}

	if len(grokResp.Choices) == 0 {
		return "", &ProviderError{Provider: "grok", Message: "empty response from API"}
	}

	if grokResp.Choices[0].FinishReason == "length" {
		return "", &ProviderError{Provider: "grok", Message: "response truncated: exceeded max tokens limit"}
	}

	return grokResp.Choices[0].Message.Content, nil
}

func (p *GrokProvider) headers() map[string]string {
	return map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + p.apiKey,
	}
}

type grokRequest struct {
	Model       string        `json:"model"`
	Messages    []grokMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type grokMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type grokResponse struct {
	Choices []grokChoice `json:"choices"`
	Usage   grokUsage    `json:"usage"`
}

type grokChoice struct {
	Message      grokMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type grokUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
