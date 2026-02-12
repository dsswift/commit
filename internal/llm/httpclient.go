package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// llmRequest describes an HTTP request to an LLM provider.
type llmRequest struct {
	ctx      context.Context
	client   *http.Client
	method   string
	url      string
	headers  map[string]string
	body     interface{}
	provider string
}

// llmResponse contains the raw HTTP response from an LLM provider.
type llmResponse struct {
	StatusCode int
	Body       []byte
}

// doRequest marshals body, sends the HTTP request, reads the response, and checks status.
func doRequest(req *llmRequest) (*llmResponse, error) {
	bodyBytes, err := json.Marshal(req.body)
	if err != nil {
		return nil, &ProviderError{Provider: req.provider, Message: "failed to marshal request", Err: err}
	}

	httpReq, err := http.NewRequestWithContext(req.ctx, req.method, req.url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, &ProviderError{Provider: req.provider, Message: "failed to create request", Err: err}
	}

	for k, v := range req.headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := req.client.Do(httpReq)
	if err != nil {
		return nil, &ProviderError{Provider: req.provider, Message: "request failed", Err: err}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ProviderError{Provider: req.provider, Message: "failed to read response", Err: err}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &ProviderError{
			Provider: req.provider,
			Message:  fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(respBody)),
		}
	}

	return &llmResponse{
		StatusCode: resp.StatusCode,
		Body:       respBody,
	}, nil
}

// cleanContent strips markdown code fences from LLM response text.
func cleanContent(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	return content
}
