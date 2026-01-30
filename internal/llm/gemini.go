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
	geminiAPIURL     = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"
	defaultGeminiModel = "gemini-1.5-pro"
)

// GeminiProvider implements the Provider interface for Google's Gemini.
type GeminiProvider struct {
	apiKey string
	model  string
	client *http.Client
}

// NewGeminiProvider creates a new Gemini provider.
func NewGeminiProvider(apiKey, model string) (*GeminiProvider, error) {
	assert.NotEmptyString(apiKey, "Gemini API key is required")

	if model == "" {
		model = defaultGeminiModel
	}

	return &GeminiProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
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
			MaxOutputTokens: 2000,
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, &ProviderError{Provider: "gemini", Message: "failed to marshal request", Err: err}
	}

	url := fmt.Sprintf(geminiAPIURL, p.model) + "?key=" + p.apiKey

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &ProviderError{Provider: "gemini", Message: "failed to create request", Err: err}
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &ProviderError{Provider: "gemini", Message: "request failed", Err: err}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ProviderError{Provider: "gemini", Message: "failed to read response", Err: err}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &ProviderError{
			Provider: "gemini",
			Message:  fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(respBody)),
		}
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, &ProviderError{Provider: "gemini", Message: "failed to parse response", Err: err}
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, &ProviderError{Provider: "gemini", Message: "empty response from API"}
	}

	content := geminiResp.Candidates[0].Content.Parts[0].Text

	// Clean up and parse
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var plan types.CommitPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, &ProviderError{Provider: "gemini", Message: "failed to parse commit plan", Err: err}
	}

	return &plan, nil
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
	Content geminiContent `json:"content"`
}
