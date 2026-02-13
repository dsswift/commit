package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dsswift/commit/pkg/types"
)

// validCommitPlanJSON is the canonical success response body used across all provider tests.
const validCommitPlanJSON = `{"commits":[{"type":"feat","scope":"api","message":"add endpoint","files":["src/api.go"],"reasoning":"new feature"}]}`

// analysisRequest returns a minimal AnalysisRequest suitable for Analyze tests.
func analysisRequest() *types.AnalysisRequest {
	return &types.AnalysisRequest{
		Files: []types.FileChange{{Path: "test.go", Status: "modified", DiffSummary: "+1 -1"}},
		Diff:  "diff content",
		Rules: types.CommitRules{Types: []string{"feat", "fix"}, MaxMessageLength: 50},
	}
}

// --- Anthropic response helpers ---

func anthropicSuccessBody(content string) string {
	resp := anthropicResponse{
		Content:    []anthropicContent{{Type: "text", Text: content}},
		StopReason: "end_turn",
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func anthropicTruncatedBody() string {
	resp := anthropicResponse{
		Content:    []anthropicContent{{Type: "text", Text: `{"commits":[]`}},
		StopReason: "max_tokens",
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func anthropicEmptyBody() string {
	resp := anthropicResponse{
		Content:    []anthropicContent{},
		StopReason: "end_turn",
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

// --- OpenAI/Grok response helpers (shared chat types) ---

func openaiSuccessBody(content string) string {
	resp := chatResponse{
		Choices: []chatChoice{{
			Message:      chatMessage{Role: "assistant", Content: content},
			FinishReason: "stop",
		}},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func openaiTruncatedBody() string {
	resp := chatResponse{
		Choices: []chatChoice{{
			Message:      chatMessage{Role: "assistant", Content: `{"commits":[]`},
			FinishReason: "length",
		}},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func openaiEmptyBody() string {
	resp := chatResponse{Choices: []chatChoice{}}
	b, _ := json.Marshal(resp)
	return string(b)
}

func grokSuccessBody(content string) string {
	return openaiSuccessBody(content)
}

func grokTruncatedBody() string {
	return openaiTruncatedBody()
}

func grokEmptyBody() string {
	return openaiEmptyBody()
}

// --- Gemini response helpers ---

func geminiSuccessBody(content string) string {
	resp := geminiResponse{
		Candidates: []geminiCandidate{{
			Content:      geminiContent{Parts: []geminiPart{{Text: content}}},
			FinishReason: "STOP",
		}},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func geminiTruncatedBody() string {
	resp := geminiResponse{
		Candidates: []geminiCandidate{{
			Content:      geminiContent{Parts: []geminiPart{{Text: `{"commits":[]`}}},
			FinishReason: "MAX_TOKENS",
		}},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func geminiEmptyBody() string {
	resp := geminiResponse{Candidates: []geminiCandidate{}}
	b, _ := json.Marshal(resp)
	return string(b)
}

// --- Provider factory helpers ---

func newTestAnthropic(serverURL string) *AnthropicProvider {
	p, _ := NewAnthropicProvider("test-key", "test-model")
	p.baseURL = serverURL
	return p
}

func newTestOpenAI(serverURL string) *OpenAIProvider {
	p, _ := NewOpenAIProvider("test-key", "test-model")
	p.baseURL = serverURL
	return p
}

func newTestGrok(serverURL string) *GrokProvider {
	p, _ := NewGrokProvider("test-key", "test-model")
	p.baseURL = serverURL
	return p
}

func newTestGemini(serverURL string) *GeminiProvider {
	p, _ := NewGeminiProvider("test-key", "test-model")
	p.baseURL = serverURL + "/%s"
	return p
}

// newTestServer creates an httptest server that responds with the given status and body.
func newTestServer(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

// =====================================================================
// Anthropic Analyze tests
// =====================================================================

func TestAnthropicProvider_Analyze_Success(t *testing.T) {
	server := newTestServer(http.StatusOK, anthropicSuccessBody(validCommitPlanJSON))
	defer server.Close()

	p := newTestAnthropic(server.URL)
	plan, err := p.Analyze(context.Background(), analysisRequest())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(plan.Commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(plan.Commits))
	}
	if plan.Commits[0].Type != "feat" {
		t.Errorf("expected type 'feat', got %q", plan.Commits[0].Type)
	}
	if plan.Commits[0].Message != "add endpoint" {
		t.Errorf("expected message 'add endpoint', got %q", plan.Commits[0].Message)
	}
}

func TestAnthropicProvider_Analyze_APIError(t *testing.T) {
	server := newTestServer(http.StatusInternalServerError, `{"error":"internal"}`)
	defer server.Close()

	p := newTestAnthropic(server.URL)
	_, err := p.Analyze(context.Background(), analysisRequest())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	pe, ok := err.(*ProviderError)
	if !ok {
		t.Fatalf("expected *ProviderError, got %T", err)
	}
	if pe.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", pe.Provider)
	}
	if !strings.Contains(pe.Error(), "500") {
		t.Errorf("expected error to contain status code, got: %s", pe.Error())
	}
}

func TestAnthropicProvider_Analyze_MalformedJSON(t *testing.T) {
	server := newTestServer(http.StatusOK, "not json at all")
	defer server.Close()

	p := newTestAnthropic(server.URL)
	_, err := p.Analyze(context.Background(), analysisRequest())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %s", err.Error())
	}
}

func TestAnthropicProvider_Analyze_Truncated(t *testing.T) {
	server := newTestServer(http.StatusOK, anthropicTruncatedBody())
	defer server.Close()

	p := newTestAnthropic(server.URL)
	_, err := p.Analyze(context.Background(), analysisRequest())
	if err == nil {
		t.Fatal("expected error for truncated response")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("expected 'truncated' in error, got: %s", err.Error())
	}
}

func TestAnthropicProvider_Analyze_EmptyResponse(t *testing.T) {
	server := newTestServer(http.StatusOK, anthropicEmptyBody())
	defer server.Close()

	p := newTestAnthropic(server.URL)
	_, err := p.Analyze(context.Background(), analysisRequest())
	if err == nil {
		t.Fatal("expected error for empty response")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected 'empty response' in error, got: %s", err.Error())
	}
}

// =====================================================================
// Anthropic AnalyzeDiff tests
// =====================================================================

func TestAnthropicProvider_AnalyzeDiff_Success(t *testing.T) {
	server := newTestServer(http.StatusOK, anthropicSuccessBody("analysis result text"))
	defer server.Close()

	p := newTestAnthropic(server.URL)
	result, err := p.AnalyzeDiff(context.Background(), "system prompt", "user prompt")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != "analysis result text" {
		t.Errorf("expected 'analysis result text', got %q", result)
	}
}

func TestAnthropicProvider_AnalyzeDiff_APIError(t *testing.T) {
	server := newTestServer(http.StatusBadRequest, `{"error":"bad request"}`)
	defer server.Close()

	p := newTestAnthropic(server.URL)
	_, err := p.AnalyzeDiff(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	pe, ok := err.(*ProviderError)
	if !ok {
		t.Fatalf("expected *ProviderError, got %T", err)
	}
	if pe.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", pe.Provider)
	}
}

func TestAnthropicProvider_AnalyzeDiff_MalformedJSON(t *testing.T) {
	server := newTestServer(http.StatusOK, "{invalid")
	defer server.Close()

	p := newTestAnthropic(server.URL)
	_, err := p.AnalyzeDiff(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %s", err.Error())
	}
}

func TestAnthropicProvider_AnalyzeDiff_Truncated(t *testing.T) {
	server := newTestServer(http.StatusOK, anthropicTruncatedBody())
	defer server.Close()

	p := newTestAnthropic(server.URL)
	_, err := p.AnalyzeDiff(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error for truncated response")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("expected 'truncated' in error, got: %s", err.Error())
	}
}

func TestAnthropicProvider_AnalyzeDiff_EmptyResponse(t *testing.T) {
	server := newTestServer(http.StatusOK, anthropicEmptyBody())
	defer server.Close()

	p := newTestAnthropic(server.URL)
	_, err := p.AnalyzeDiff(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error for empty response")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected 'empty response' in error, got: %s", err.Error())
	}
}

// =====================================================================
// OpenAI Analyze tests
// =====================================================================

func TestOpenAIProvider_Analyze_Success(t *testing.T) {
	server := newTestServer(http.StatusOK, openaiSuccessBody(validCommitPlanJSON))
	defer server.Close()

	p := newTestOpenAI(server.URL)
	plan, err := p.Analyze(context.Background(), analysisRequest())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(plan.Commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(plan.Commits))
	}
	if plan.Commits[0].Type != "feat" {
		t.Errorf("expected type 'feat', got %q", plan.Commits[0].Type)
	}
	if plan.Commits[0].Message != "add endpoint" {
		t.Errorf("expected message 'add endpoint', got %q", plan.Commits[0].Message)
	}
}

func TestOpenAIProvider_Analyze_APIError(t *testing.T) {
	server := newTestServer(http.StatusTooManyRequests, `{"error":"rate limit"}`)
	defer server.Close()

	p := newTestOpenAI(server.URL)
	_, err := p.Analyze(context.Background(), analysisRequest())
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
	pe, ok := err.(*ProviderError)
	if !ok {
		t.Fatalf("expected *ProviderError, got %T", err)
	}
	if pe.Provider != "openai" {
		t.Errorf("expected provider 'openai', got %q", pe.Provider)
	}
}

func TestOpenAIProvider_Analyze_MalformedJSON(t *testing.T) {
	server := newTestServer(http.StatusOK, "<<<not json>>>")
	defer server.Close()

	p := newTestOpenAI(server.URL)
	_, err := p.Analyze(context.Background(), analysisRequest())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %s", err.Error())
	}
}

func TestOpenAIProvider_Analyze_Truncated(t *testing.T) {
	server := newTestServer(http.StatusOK, openaiTruncatedBody())
	defer server.Close()

	p := newTestOpenAI(server.URL)
	_, err := p.Analyze(context.Background(), analysisRequest())
	if err == nil {
		t.Fatal("expected error for truncated response")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("expected 'truncated' in error, got: %s", err.Error())
	}
}

func TestOpenAIProvider_Analyze_EmptyResponse(t *testing.T) {
	server := newTestServer(http.StatusOK, openaiEmptyBody())
	defer server.Close()

	p := newTestOpenAI(server.URL)
	_, err := p.Analyze(context.Background(), analysisRequest())
	if err == nil {
		t.Fatal("expected error for empty response")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected 'empty response' in error, got: %s", err.Error())
	}
}

// =====================================================================
// OpenAI AnalyzeDiff tests
// =====================================================================

func TestOpenAIProvider_AnalyzeDiff_Success(t *testing.T) {
	server := newTestServer(http.StatusOK, openaiSuccessBody("diff analysis output"))
	defer server.Close()

	p := newTestOpenAI(server.URL)
	result, err := p.AnalyzeDiff(context.Background(), "system prompt", "user prompt")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != "diff analysis output" {
		t.Errorf("expected 'diff analysis output', got %q", result)
	}
}

func TestOpenAIProvider_AnalyzeDiff_APIError(t *testing.T) {
	server := newTestServer(http.StatusForbidden, `{"error":"forbidden"}`)
	defer server.Close()

	p := newTestOpenAI(server.URL)
	_, err := p.AnalyzeDiff(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	pe, ok := err.(*ProviderError)
	if !ok {
		t.Fatalf("expected *ProviderError, got %T", err)
	}
	if pe.Provider != "openai" {
		t.Errorf("expected provider 'openai', got %q", pe.Provider)
	}
}

func TestOpenAIProvider_AnalyzeDiff_MalformedJSON(t *testing.T) {
	server := newTestServer(http.StatusOK, "broken{json")
	defer server.Close()

	p := newTestOpenAI(server.URL)
	_, err := p.AnalyzeDiff(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %s", err.Error())
	}
}

func TestOpenAIProvider_AnalyzeDiff_Truncated(t *testing.T) {
	server := newTestServer(http.StatusOK, openaiTruncatedBody())
	defer server.Close()

	p := newTestOpenAI(server.URL)
	_, err := p.AnalyzeDiff(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error for truncated response")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("expected 'truncated' in error, got: %s", err.Error())
	}
}

func TestOpenAIProvider_AnalyzeDiff_EmptyResponse(t *testing.T) {
	server := newTestServer(http.StatusOK, openaiEmptyBody())
	defer server.Close()

	p := newTestOpenAI(server.URL)
	_, err := p.AnalyzeDiff(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error for empty response")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected 'empty response' in error, got: %s", err.Error())
	}
}

// =====================================================================
// Grok Analyze tests
// =====================================================================

func TestGrokProvider_Analyze_Success(t *testing.T) {
	server := newTestServer(http.StatusOK, grokSuccessBody(validCommitPlanJSON))
	defer server.Close()

	p := newTestGrok(server.URL)
	plan, err := p.Analyze(context.Background(), analysisRequest())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(plan.Commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(plan.Commits))
	}
	if plan.Commits[0].Type != "feat" {
		t.Errorf("expected type 'feat', got %q", plan.Commits[0].Type)
	}
	if plan.Commits[0].Message != "add endpoint" {
		t.Errorf("expected message 'add endpoint', got %q", plan.Commits[0].Message)
	}
}

func TestGrokProvider_Analyze_APIError(t *testing.T) {
	server := newTestServer(http.StatusServiceUnavailable, `{"error":"service unavailable"}`)
	defer server.Close()

	p := newTestGrok(server.URL)
	_, err := p.Analyze(context.Background(), analysisRequest())
	if err == nil {
		t.Fatal("expected error for 503 response")
	}
	pe, ok := err.(*ProviderError)
	if !ok {
		t.Fatalf("expected *ProviderError, got %T", err)
	}
	if pe.Provider != "grok" {
		t.Errorf("expected provider 'grok', got %q", pe.Provider)
	}
}

func TestGrokProvider_Analyze_MalformedJSON(t *testing.T) {
	server := newTestServer(http.StatusOK, "}{bad")
	defer server.Close()

	p := newTestGrok(server.URL)
	_, err := p.Analyze(context.Background(), analysisRequest())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %s", err.Error())
	}
}

func TestGrokProvider_Analyze_Truncated(t *testing.T) {
	server := newTestServer(http.StatusOK, grokTruncatedBody())
	defer server.Close()

	p := newTestGrok(server.URL)
	_, err := p.Analyze(context.Background(), analysisRequest())
	if err == nil {
		t.Fatal("expected error for truncated response")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("expected 'truncated' in error, got: %s", err.Error())
	}
}

func TestGrokProvider_Analyze_EmptyResponse(t *testing.T) {
	server := newTestServer(http.StatusOK, grokEmptyBody())
	defer server.Close()

	p := newTestGrok(server.URL)
	_, err := p.Analyze(context.Background(), analysisRequest())
	if err == nil {
		t.Fatal("expected error for empty response")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected 'empty response' in error, got: %s", err.Error())
	}
}

// =====================================================================
// Grok AnalyzeDiff tests
// =====================================================================

func TestGrokProvider_AnalyzeDiff_Success(t *testing.T) {
	server := newTestServer(http.StatusOK, grokSuccessBody("grok diff analysis"))
	defer server.Close()

	p := newTestGrok(server.URL)
	result, err := p.AnalyzeDiff(context.Background(), "system prompt", "user prompt")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != "grok diff analysis" {
		t.Errorf("expected 'grok diff analysis', got %q", result)
	}
}

func TestGrokProvider_AnalyzeDiff_APIError(t *testing.T) {
	server := newTestServer(http.StatusUnauthorized, `{"error":"unauthorized"}`)
	defer server.Close()

	p := newTestGrok(server.URL)
	_, err := p.AnalyzeDiff(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	pe, ok := err.(*ProviderError)
	if !ok {
		t.Fatalf("expected *ProviderError, got %T", err)
	}
	if pe.Provider != "grok" {
		t.Errorf("expected provider 'grok', got %q", pe.Provider)
	}
}

func TestGrokProvider_AnalyzeDiff_MalformedJSON(t *testing.T) {
	server := newTestServer(http.StatusOK, "not-valid-json")
	defer server.Close()

	p := newTestGrok(server.URL)
	_, err := p.AnalyzeDiff(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %s", err.Error())
	}
}

func TestGrokProvider_AnalyzeDiff_Truncated(t *testing.T) {
	server := newTestServer(http.StatusOK, grokTruncatedBody())
	defer server.Close()

	p := newTestGrok(server.URL)
	_, err := p.AnalyzeDiff(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error for truncated response")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("expected 'truncated' in error, got: %s", err.Error())
	}
}

func TestGrokProvider_AnalyzeDiff_EmptyResponse(t *testing.T) {
	server := newTestServer(http.StatusOK, grokEmptyBody())
	defer server.Close()

	p := newTestGrok(server.URL)
	_, err := p.AnalyzeDiff(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error for empty response")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected 'empty response' in error, got: %s", err.Error())
	}
}

// =====================================================================
// Gemini Analyze tests
// =====================================================================

func TestGeminiProvider_Analyze_Success(t *testing.T) {
	server := newTestServer(http.StatusOK, geminiSuccessBody(validCommitPlanJSON))
	defer server.Close()

	p := newTestGemini(server.URL)
	plan, err := p.Analyze(context.Background(), analysisRequest())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(plan.Commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(plan.Commits))
	}
	if plan.Commits[0].Type != "feat" {
		t.Errorf("expected type 'feat', got %q", plan.Commits[0].Type)
	}
	if plan.Commits[0].Message != "add endpoint" {
		t.Errorf("expected message 'add endpoint', got %q", plan.Commits[0].Message)
	}
}

func TestGeminiProvider_Analyze_APIError(t *testing.T) {
	server := newTestServer(http.StatusBadRequest, `{"error":{"message":"invalid request"}}`)
	defer server.Close()

	p := newTestGemini(server.URL)
	_, err := p.Analyze(context.Background(), analysisRequest())
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	pe, ok := err.(*ProviderError)
	if !ok {
		t.Fatalf("expected *ProviderError, got %T", err)
	}
	if pe.Provider != "gemini" {
		t.Errorf("expected provider 'gemini', got %q", pe.Provider)
	}
}

func TestGeminiProvider_Analyze_MalformedJSON(t *testing.T) {
	server := newTestServer(http.StatusOK, "totally broken json")
	defer server.Close()

	p := newTestGemini(server.URL)
	_, err := p.Analyze(context.Background(), analysisRequest())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %s", err.Error())
	}
}

func TestGeminiProvider_Analyze_Truncated(t *testing.T) {
	server := newTestServer(http.StatusOK, geminiTruncatedBody())
	defer server.Close()

	p := newTestGemini(server.URL)
	_, err := p.Analyze(context.Background(), analysisRequest())
	if err == nil {
		t.Fatal("expected error for truncated response")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("expected 'truncated' in error, got: %s", err.Error())
	}
}

func TestGeminiProvider_Analyze_EmptyResponse(t *testing.T) {
	server := newTestServer(http.StatusOK, geminiEmptyBody())
	defer server.Close()

	p := newTestGemini(server.URL)
	_, err := p.Analyze(context.Background(), analysisRequest())
	if err == nil {
		t.Fatal("expected error for empty response")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected 'empty response' in error, got: %s", err.Error())
	}
}

// =====================================================================
// Gemini AnalyzeDiff tests
// =====================================================================

func TestGeminiProvider_AnalyzeDiff_Success(t *testing.T) {
	server := newTestServer(http.StatusOK, geminiSuccessBody("gemini diff result"))
	defer server.Close()

	p := newTestGemini(server.URL)
	result, err := p.AnalyzeDiff(context.Background(), "system prompt", "user prompt")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != "gemini diff result" {
		t.Errorf("expected 'gemini diff result', got %q", result)
	}
}

func TestGeminiProvider_AnalyzeDiff_APIError(t *testing.T) {
	server := newTestServer(http.StatusInternalServerError, `{"error":"server error"}`)
	defer server.Close()

	p := newTestGemini(server.URL)
	_, err := p.AnalyzeDiff(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	pe, ok := err.(*ProviderError)
	if !ok {
		t.Fatalf("expected *ProviderError, got %T", err)
	}
	if pe.Provider != "gemini" {
		t.Errorf("expected provider 'gemini', got %q", pe.Provider)
	}
}

func TestGeminiProvider_AnalyzeDiff_MalformedJSON(t *testing.T) {
	server := newTestServer(http.StatusOK, "[[malformed]]")
	defer server.Close()

	p := newTestGemini(server.URL)
	_, err := p.AnalyzeDiff(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %s", err.Error())
	}
}

func TestGeminiProvider_AnalyzeDiff_Truncated(t *testing.T) {
	server := newTestServer(http.StatusOK, geminiTruncatedBody())
	defer server.Close()

	p := newTestGemini(server.URL)
	_, err := p.AnalyzeDiff(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error for truncated response")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("expected 'truncated' in error, got: %s", err.Error())
	}
}

func TestGeminiProvider_AnalyzeDiff_EmptyResponse(t *testing.T) {
	server := newTestServer(http.StatusOK, geminiEmptyBody())
	defer server.Close()

	p := newTestGemini(server.URL)
	_, err := p.AnalyzeDiff(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error for empty response")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected 'empty response' in error, got: %s", err.Error())
	}
}

// =====================================================================
// Cross-provider header verification tests
// =====================================================================

func TestAnthropicProvider_SendsCorrectHeaders(t *testing.T) {
	var capturedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(anthropicSuccessBody(validCommitPlanJSON)))
	}))
	defer server.Close()

	p := newTestAnthropic(server.URL)
	_, _ = p.Analyze(context.Background(), analysisRequest())

	if capturedHeaders.Get("x-api-key") != "test-key" {
		t.Errorf("expected x-api-key 'test-key', got %q", capturedHeaders.Get("x-api-key"))
	}
	if capturedHeaders.Get("anthropic-version") != anthropicAPIVersion {
		t.Errorf("expected anthropic-version %q, got %q", anthropicAPIVersion, capturedHeaders.Get("anthropic-version"))
	}
	if capturedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", capturedHeaders.Get("Content-Type"))
	}
}

func TestOpenAIProvider_SendsCorrectHeaders(t *testing.T) {
	var capturedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(openaiSuccessBody(validCommitPlanJSON)))
	}))
	defer server.Close()

	p := newTestOpenAI(server.URL)
	_, _ = p.Analyze(context.Background(), analysisRequest())

	if capturedHeaders.Get("Authorization") != "Bearer test-key" {
		t.Errorf("expected Authorization 'Bearer test-key', got %q", capturedHeaders.Get("Authorization"))
	}
	if capturedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", capturedHeaders.Get("Content-Type"))
	}
}

func TestGrokProvider_SendsCorrectHeaders(t *testing.T) {
	var capturedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(grokSuccessBody(validCommitPlanJSON)))
	}))
	defer server.Close()

	p := newTestGrok(server.URL)
	_, _ = p.Analyze(context.Background(), analysisRequest())

	if capturedHeaders.Get("Authorization") != "Bearer test-key" {
		t.Errorf("expected Authorization 'Bearer test-key', got %q", capturedHeaders.Get("Authorization"))
	}
	if capturedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", capturedHeaders.Get("Content-Type"))
	}
}

func TestGeminiProvider_SendsCorrectHeaders(t *testing.T) {
	var capturedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(geminiSuccessBody(validCommitPlanJSON)))
	}))
	defer server.Close()

	p := newTestGemini(server.URL)
	_, _ = p.Analyze(context.Background(), analysisRequest())

	if capturedHeaders.Get("x-goog-api-key") != "test-key" {
		t.Errorf("expected x-goog-api-key 'test-key', got %q", capturedHeaders.Get("x-goog-api-key"))
	}
	if capturedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", capturedHeaders.Get("Content-Type"))
	}
}

// =====================================================================
// Request body verification tests
// =====================================================================

func TestAnthropicProvider_SendsCorrectRequestBody(t *testing.T) {
	var capturedBody anthropicRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(anthropicSuccessBody(validCommitPlanJSON)))
	}))
	defer server.Close()

	p := newTestAnthropic(server.URL)
	_, _ = p.Analyze(context.Background(), analysisRequest())

	if capturedBody.Model != "test-model" {
		t.Errorf("expected model 'test-model', got %q", capturedBody.Model)
	}
	if capturedBody.MaxTokens != 8192 {
		t.Errorf("expected max_tokens 8192, got %d", capturedBody.MaxTokens)
	}
	if len(capturedBody.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(capturedBody.Messages))
	}
	if capturedBody.Messages[0].Role != "user" {
		t.Errorf("expected role 'user', got %q", capturedBody.Messages[0].Role)
	}
	if capturedBody.System == "" {
		t.Error("expected non-empty system prompt")
	}
}

func TestOpenAIProvider_SendsCorrectRequestBody(t *testing.T) {
	var capturedBody chatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(openaiSuccessBody(validCommitPlanJSON)))
	}))
	defer server.Close()

	p := newTestOpenAI(server.URL)
	_, _ = p.Analyze(context.Background(), analysisRequest())

	if capturedBody.Model != "test-model" {
		t.Errorf("expected model 'test-model', got %q", capturedBody.Model)
	}
	if len(capturedBody.Messages) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(capturedBody.Messages))
	}
	if capturedBody.Messages[0].Role != "system" {
		t.Errorf("expected first message role 'system', got %q", capturedBody.Messages[0].Role)
	}
	if capturedBody.Messages[1].Role != "user" {
		t.Errorf("expected second message role 'user', got %q", capturedBody.Messages[1].Role)
	}
}

// =====================================================================
// Context cancellation test
// =====================================================================

func TestProvider_Analyze_CancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow server; the context should cancel before this completes.
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	providers := []struct {
		name     string
		provider Provider
	}{
		{"anthropic", newTestAnthropic(server.URL)},
		{"openai", newTestOpenAI(server.URL)},
		{"grok", newTestGrok(server.URL)},
		{"gemini", newTestGemini(server.URL)},
	}

	for _, tc := range providers {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.provider.Analyze(ctx, analysisRequest())
			if err == nil {
				t.Fatal("expected error for cancelled context")
			}
		})
	}
}

// =====================================================================
// Analyze with markdown-wrapped JSON (code fence stripping)
// =====================================================================

func TestAnthropicProvider_Analyze_MarkdownWrappedJSON(t *testing.T) {
	wrapped := "```json\n" + validCommitPlanJSON + "\n```"
	server := newTestServer(http.StatusOK, anthropicSuccessBody(wrapped))
	defer server.Close()

	p := newTestAnthropic(server.URL)
	plan, err := p.Analyze(context.Background(), analysisRequest())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(plan.Commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(plan.Commits))
	}
}

func TestOpenAIProvider_Analyze_MarkdownWrappedJSON(t *testing.T) {
	wrapped := "```json\n" + validCommitPlanJSON + "\n```"
	server := newTestServer(http.StatusOK, openaiSuccessBody(wrapped))
	defer server.Close()

	p := newTestOpenAI(server.URL)
	plan, err := p.Analyze(context.Background(), analysisRequest())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(plan.Commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(plan.Commits))
	}
}

func TestGrokProvider_Analyze_MarkdownWrappedJSON(t *testing.T) {
	wrapped := "```json\n" + validCommitPlanJSON + "\n```"
	server := newTestServer(http.StatusOK, grokSuccessBody(wrapped))
	defer server.Close()

	p := newTestGrok(server.URL)
	plan, err := p.Analyze(context.Background(), analysisRequest())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(plan.Commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(plan.Commits))
	}
}

func TestGeminiProvider_Analyze_MarkdownWrappedJSON(t *testing.T) {
	wrapped := "```json\n" + validCommitPlanJSON + "\n```"
	server := newTestServer(http.StatusOK, geminiSuccessBody(wrapped))
	defer server.Close()

	p := newTestGemini(server.URL)
	plan, err := p.Analyze(context.Background(), analysisRequest())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(plan.Commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(plan.Commits))
	}
}

// =====================================================================
// Gemini empty parts (valid candidates but no parts)
// =====================================================================

func TestGeminiProvider_Analyze_EmptyParts(t *testing.T) {
	resp := geminiResponse{
		Candidates: []geminiCandidate{{
			Content:      geminiContent{Parts: []geminiPart{}},
			FinishReason: "STOP",
		}},
	}
	b, _ := json.Marshal(resp)
	server := newTestServer(http.StatusOK, string(b))
	defer server.Close()

	p := newTestGemini(server.URL)
	_, err := p.Analyze(context.Background(), analysisRequest())
	if err == nil {
		t.Fatal("expected error for empty parts")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected 'empty response' in error, got: %s", err.Error())
	}
}

func TestGeminiProvider_AnalyzeDiff_EmptyParts(t *testing.T) {
	resp := geminiResponse{
		Candidates: []geminiCandidate{{
			Content:      geminiContent{Parts: []geminiPart{}},
			FinishReason: "STOP",
		}},
	}
	b, _ := json.Marshal(resp)
	server := newTestServer(http.StatusOK, string(b))
	defer server.Close()

	p := newTestGemini(server.URL)
	_, err := p.AnalyzeDiff(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error for empty parts")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected 'empty response' in error, got: %s", err.Error())
	}
}

// =====================================================================
// Analyze validates commit plan fields
// =====================================================================

func TestAllProviders_Analyze_ValidatesCommitPlanFields(t *testing.T) {
	// Verify that the parsed CommitPlan contains all expected fields from the JSON
	providers := []struct {
		name   string
		body   string
		create func(url string) Provider
	}{
		{"anthropic", anthropicSuccessBody(validCommitPlanJSON), func(u string) Provider { return newTestAnthropic(u) }},
		{"openai", openaiSuccessBody(validCommitPlanJSON), func(u string) Provider { return newTestOpenAI(u) }},
		{"grok", grokSuccessBody(validCommitPlanJSON), func(u string) Provider { return newTestGrok(u) }},
		{"gemini", geminiSuccessBody(validCommitPlanJSON), func(u string) Provider { return newTestGemini(u) }},
	}

	for _, tc := range providers {
		t.Run(tc.name, func(t *testing.T) {
			server := newTestServer(http.StatusOK, tc.body)
			defer server.Close()

			p := tc.create(server.URL)
			plan, err := p.Analyze(context.Background(), analysisRequest())
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			c := plan.Commits[0]
			if c.Type != "feat" {
				t.Errorf("expected type 'feat', got %q", c.Type)
			}
			if c.Scope == nil || *c.Scope != "api" {
				t.Errorf("expected scope 'api', got %v", c.Scope)
			}
			if c.Message != "add endpoint" {
				t.Errorf("expected message 'add endpoint', got %q", c.Message)
			}
			if len(c.Files) != 1 || c.Files[0] != "src/api.go" {
				t.Errorf("expected files [src/api.go], got %v", c.Files)
			}
			if c.Reasoning != "new feature" {
				t.Errorf("expected reasoning 'new feature', got %q", c.Reasoning)
			}
		})
	}
}
