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

	"github.com/dsswift/commit/internal/httpclient"
)

// newHTTPClient creates an HTTP client using the shared transport with the given timeout.
func newHTTPClient(timeout time.Duration) *http.Client {
	return httpclient.NewClient(timeout)
}

// maxRetries is the total number of attempts (1 initial + 2 retries).
const maxRetries = 3

// retryableStatusCode returns true for HTTP status codes that warrant a retry.
func retryableStatusCode(code int) bool {
	return code == http.StatusTooManyRequests ||
		code == http.StatusBadGateway ||
		code == http.StatusServiceUnavailable ||
		code == http.StatusGatewayTimeout
}

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

// doRequest marshals body, sends the HTTP request with retry, reads the response, and checks status.
func doRequest(req *llmRequest) (*llmResponse, error) {
	bodyBytes, err := json.Marshal(req.body)
	if err != nil {
		return nil, &ProviderError{Provider: req.provider, Message: "failed to marshal request", Err: err}
	}

	var lastErr error
	backoff := 500 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Wait with exponential backoff, respecting context cancellation
			select {
			case <-req.ctx.Done():
				return nil, &ProviderError{Provider: req.provider, Message: "request cancelled", Err: req.ctx.Err()}
			case <-time.After(backoff):
			}
			backoff *= 2
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
			// Network errors are retryable
			lastErr = &ProviderError{Provider: req.provider, Message: "request failed", Err: err}
			// But context cancellation is not retryable
			if req.ctx.Err() != nil {
				return nil, lastErr
			}
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck // HTTP response body
		if err != nil {
			return nil, &ProviderError{Provider: req.provider, Message: "failed to read response", Err: err}
		}

		if resp.StatusCode == http.StatusOK {
			return &llmResponse{
				StatusCode: resp.StatusCode,
				Body:       respBody,
			}, nil
		}

		// Sanitize error body to prevent credential leakage
		errorBody := string(respBody)
		if len(errorBody) > 500 {
			errorBody = errorBody[:500] + "... (truncated)"
		}

		lastErr = &ProviderError{
			Provider: req.provider,
			Message:  fmt.Sprintf("API error (status %d): %s", resp.StatusCode, errorBody),
		}

		// Only retry on retryable status codes
		if !retryableStatusCode(resp.StatusCode) {
			return nil, lastErr
		}
	}

	return nil, lastErr
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
