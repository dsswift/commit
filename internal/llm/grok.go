package llm

import (
	"context"
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
		client:  newHTTPClient(60 * time.Second),
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

	return analyzeChatCompletion(ctx, p.requestParams(), req)
}

// AnalyzeDiff sends a diff analysis request to Grok and returns the analysis.
func (p *GrokProvider) AnalyzeDiff(ctx context.Context, system, user string) (string, error) {
	return analyzeDiffChatCompletion(ctx, p.requestParams(), system, user)
}

func (p *GrokProvider) requestParams() llmRequestParams {
	return llmRequestParams{
		httpClient: p.client,
		model:      p.model,
		url:        p.baseURL,
		headers:    p.headers(),
		provider:   "grok",
	}
}

func (p *GrokProvider) headers() map[string]string {
	return map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + p.apiKey,
	}
}
