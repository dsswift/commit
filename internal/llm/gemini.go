package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dsswift/commit/internal/assert"
	"github.com/dsswift/commit/pkg/types"
)

const (
	geminiAPIURL       = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"
	defaultGeminiModel = "gemini-1.5-pro"
)

// GeminiProvider implements the Provider interface for Google's Gemini.
type GeminiProvider struct {
	apiKey  string
	model   string
	client  *http.Client
	baseURL string
}

// NewGeminiProvider creates a new Gemini provider.
func NewGeminiProvider(apiKey, model string) (*GeminiProvider, error) {
	assert.NotEmptyString(apiKey, "Gemini API key is required")

	if model == "" {
		model = defaultGeminiModel
	}

	return &GeminiProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: geminiAPIURL,
		client:  newHTTPClient(60 * time.Second),
	}, nil
}

// Name returns the provider name.
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// Model returns the model being used.
func (p *GeminiProvider) Model() string {
	return p.model
}

// Analyze sends an analysis request to Gemini and returns a commit plan.
func (p *GeminiProvider) Analyze(ctx context.Context, req *types.AnalysisRequest) (*types.CommitPlan, error) {
	assert.NotNil(req, "analysis request cannot be nil")
	assert.NotEmpty(req.Files, "analysis request must have files")

	systemPrompt, userPrompt := BuildPrompt(req)

	// Gemini uses a different format - combine system and user prompts
	combinedPrompt := systemPrompt + "\n\n---\n\n" + userPrompt

	requestBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: combinedPrompt},
				},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			Temperature:     0.3,
			MaxOutputTokens: 8192,
		},
	}

	resp, err := doRequest(&llmRequest{
		ctx:      ctx,
		client:   p.client,
		method:   "POST",
		url:      p.apiURL(),
		headers:  p.headers(),
		body:     requestBody,
		provider: "gemini",
	})
	if err != nil {
		return nil, err
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(resp.Body, &geminiResp); err != nil {
		return nil, &ProviderError{Provider: "gemini", Message: "failed to parse response", Err: err}
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, &ProviderError{Provider: "gemini", Message: "empty response from API"}
	}

	if geminiResp.Candidates[0].FinishReason == "MAX_TOKENS" {
		return nil, &ProviderError{Provider: "gemini", Message: "response truncated: exceeded max tokens limit"}
	}

	content := cleanContent(geminiResp.Candidates[0].Content.Parts[0].Text)

	var plan types.CommitPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, &ProviderError{Provider: "gemini", Message: "failed to parse commit plan", Err: err}
	}

	return &plan, nil
}

// AnalyzeDiff sends a diff analysis request to Gemini and returns the analysis.
func (p *GeminiProvider) AnalyzeDiff(ctx context.Context, system, user string) (string, error) {
	combinedPrompt := system + "\n\n---\n\n" + user

	requestBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: combinedPrompt},
				},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			Temperature:     0.3,
			MaxOutputTokens: 8192,
		},
	}

	resp, err := doRequest(&llmRequest{
		ctx:      ctx,
		client:   p.client,
		method:   "POST",
		url:      p.apiURL(),
		headers:  p.headers(),
		body:     requestBody,
		provider: "gemini",
	})
	if err != nil {
		return "", err
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(resp.Body, &geminiResp); err != nil {
		return "", &ProviderError{Provider: "gemini", Message: "failed to parse response", Err: err}
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", &ProviderError{Provider: "gemini", Message: "empty response from API"}
	}

	if geminiResp.Candidates[0].FinishReason == "MAX_TOKENS" {
		return "", &ProviderError{Provider: "gemini", Message: "response truncated: exceeded max tokens limit"}
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

// apiURL returns the full API URL with model substituted.
func (p *GeminiProvider) apiURL() string {
	return fmt.Sprintf(p.baseURL, p.model)
}

func (p *GeminiProvider) headers() map[string]string {
	return map[string]string{
		"Content-Type":  "application/json",
		"x-goog-api-key": p.apiKey,
	}
}

type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}
