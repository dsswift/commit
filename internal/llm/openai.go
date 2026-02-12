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
	openaiAPIURL       = "https://api.openai.com/v1/chat/completions"
	defaultOpenAIModel = "gpt-4-turbo-preview"
)

// OpenAIProvider implements the Provider interface for OpenAI.
type OpenAIProvider struct {
	apiKey  string
	model   string
	client  *http.Client
	baseURL string
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(apiKey, model string) (*OpenAIProvider, error) {
	assert.NotEmptyString(apiKey, "OpenAI API key is required")

	if model == "" {
		model = defaultOpenAIModel
	}

	return &OpenAIProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: openaiAPIURL,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// Model returns the model being used.
func (p *OpenAIProvider) Model() string {
	return p.model
}

// Analyze sends an analysis request to OpenAI and returns a commit plan.
func (p *OpenAIProvider) Analyze(ctx context.Context, req *types.AnalysisRequest) (*types.CommitPlan, error) {
	assert.NotNil(req, "analysis request cannot be nil")
	assert.NotEmpty(req.Files, "analysis request must have files")

	systemPrompt, userPrompt := BuildPrompt(req)

	requestBody := openaiRequest{
		Model: p.model,
		Messages: []openaiMessage{
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
		provider: "openai",
	})
	if err != nil {
		return nil, err
	}

	var openaiResp openaiResponse
	if err := json.Unmarshal(resp.Body, &openaiResp); err != nil {
		return nil, &ProviderError{Provider: "openai", Message: "failed to parse response", Err: err}
	}

	if len(openaiResp.Choices) == 0 {
		return nil, &ProviderError{Provider: "openai", Message: "empty response from API"}
	}

	if openaiResp.Choices[0].FinishReason == "length" {
		return nil, &ProviderError{Provider: "openai", Message: "response truncated: exceeded max tokens limit"}
	}

	content := cleanContent(openaiResp.Choices[0].Message.Content)

	var plan types.CommitPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, &ProviderError{Provider: "openai", Message: "failed to parse commit plan", Err: err}
	}

	return &plan, nil
}

// AnalyzeDiff sends a diff analysis request to OpenAI and returns the analysis.
func (p *OpenAIProvider) AnalyzeDiff(ctx context.Context, system, user string) (string, error) {
	requestBody := openaiRequest{
		Model: p.model,
		Messages: []openaiMessage{
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
		provider: "openai",
	})
	if err != nil {
		return "", err
	}

	var openaiResp openaiResponse
	if err := json.Unmarshal(resp.Body, &openaiResp); err != nil {
		return "", &ProviderError{Provider: "openai", Message: "failed to parse response", Err: err}
	}

	if len(openaiResp.Choices) == 0 {
		return "", &ProviderError{Provider: "openai", Message: "empty response from API"}
	}

	if openaiResp.Choices[0].FinishReason == "length" {
		return "", &ProviderError{Provider: "openai", Message: "response truncated: exceeded max tokens limit"}
	}

	return openaiResp.Choices[0].Message.Content, nil
}

func (p *OpenAIProvider) headers() map[string]string {
	return map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + p.apiKey,
	}
}

type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []openaiChoice `json:"choices"`
	Usage   openaiUsage    `json:"usage"`
}

type openaiChoice struct {
	Message      openaiMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
