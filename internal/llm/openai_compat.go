package llm

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/dsswift/commit/pkg/types"
)

// OpenAI-compatible API types shared by openai, grok, and azure_foundry (OpenAI mode).

type chatRequest struct {
	Model       string        `json:"model,omitempty"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Usage   chatUsage    `json:"usage"`
}

type chatChoice struct {
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// llmRequestParams bundles the common parameters needed by the shared helpers.
type llmRequestParams struct {
	httpClient *http.Client
	model      string
	url        string
	headers    map[string]string
	provider   string
}

// analyzeChatCompletion sends an analysis request using the OpenAI-compatible chat completions format
// and returns a parsed CommitPlan.
func analyzeChatCompletion(ctx context.Context, params llmRequestParams, req *types.AnalysisRequest) (*types.CommitPlan, error) {
	systemPrompt, userPrompt := BuildPrompt(req)

	requestBody := chatRequest{
		Model: params.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.3,
		MaxTokens:   8192,
	}

	resp, err := doRequest(&llmRequest{
		ctx:      ctx,
		client:   params.httpClient,
		method:   "POST",
		url:      params.url,
		headers:  params.headers,
		body:     requestBody,
		provider: params.provider,
	})
	if err != nil {
		return nil, err
	}

	var chatResp chatResponse
	if err := json.Unmarshal(resp.Body, &chatResp); err != nil {
		return nil, &ProviderError{Provider: params.provider, Message: "failed to parse response", Err: err}
	}

	if len(chatResp.Choices) == 0 {
		return nil, &ProviderError{Provider: params.provider, Message: "empty response from API"}
	}

	if chatResp.Choices[0].FinishReason == "length" {
		return nil, &ProviderError{Provider: params.provider, Message: "response truncated: exceeded max tokens limit"}
	}

	content := cleanContent(chatResp.Choices[0].Message.Content)

	var plan types.CommitPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, &ProviderError{Provider: params.provider, Message: "failed to parse commit plan", Err: err}
	}

	return &plan, nil
}

// analyzeDiffChatCompletion sends a diff analysis request using the OpenAI-compatible chat format.
func analyzeDiffChatCompletion(ctx context.Context, params llmRequestParams, system, user string) (string, error) {
	requestBody := chatRequest{
		Model: params.model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.3,
		MaxTokens:   8192,
	}

	resp, err := doRequest(&llmRequest{
		ctx:      ctx,
		client:   params.httpClient,
		method:   "POST",
		url:      params.url,
		headers:  params.headers,
		body:     requestBody,
		provider: params.provider,
	})
	if err != nil {
		return "", err
	}

	var chatResp chatResponse
	if err := json.Unmarshal(resp.Body, &chatResp); err != nil {
		return "", &ProviderError{Provider: params.provider, Message: "failed to parse response", Err: err}
	}

	if len(chatResp.Choices) == 0 {
		return "", &ProviderError{Provider: params.provider, Message: "empty response from API"}
	}

	if chatResp.Choices[0].FinishReason == "length" {
		return "", &ProviderError{Provider: params.provider, Message: "response truncated: exceeded max tokens limit"}
	}

	return chatResp.Choices[0].Message.Content, nil
}
