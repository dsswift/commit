package llm

import (
	"context"
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
		client:  newHTTPClient(60 * time.Second),
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

	return analyzeChatCompletion(ctx, p.requestParams(), req)
}

// AnalyzeDiff sends a diff analysis request to OpenAI and returns the analysis.
func (p *OpenAIProvider) AnalyzeDiff(ctx context.Context, system, user string) (string, error) {
	return analyzeDiffChatCompletion(ctx, p.requestParams(), system, user)
}

func (p *OpenAIProvider) requestParams() llmRequestParams {
	return llmRequestParams{
		httpClient: p.client,
		model:      p.model,
		url:        p.baseURL,
		headers:    p.headers(),
		provider:   "openai",
	}
}

func (p *OpenAIProvider) headers() map[string]string {
	return map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + p.apiKey,
	}
}
