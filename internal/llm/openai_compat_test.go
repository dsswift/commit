package llm

import "testing"

func TestUsesCompletionTokens(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"gpt-4o", false},
		{"gpt-4.1", false},
		{"grok-4", false},
		{"claude-sonnet-4-5", false},
		{"", false},
		{"gpt-5", true},
		{"gpt-5.6-terra", true},
		{"gpt5-mini", true},
		{"o1", true},
		{"o3-mini", true},
		{"o4-mini", true},
		{"my-deployment gpt-5.6-terra", true},
		{"my-deployment gpt-4o", false},
	}

	for _, tt := range tests {
		if got := usesCompletionTokens(tt.model); got != tt.want {
			t.Errorf("usesCompletionTokens(%q) = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestNewChatRequest_LegacyModel(t *testing.T) {
	req := newChatRequest("gpt-4o", []chatMessage{{Role: "user", Content: "hi"}})

	if req.MaxTokens != defaultMaxTokens {
		t.Errorf("expected max_tokens %d, got %d", defaultMaxTokens, req.MaxTokens)
	}
	if req.MaxCompletionTokens != 0 {
		t.Errorf("expected max_completion_tokens unset, got %d", req.MaxCompletionTokens)
	}
	if req.Temperature != 0.3 {
		t.Errorf("expected temperature 0.3, got %v", req.Temperature)
	}
}

func TestNewChatRequest_ReasoningModel(t *testing.T) {
	req := newChatRequest("gpt-5.6-terra", []chatMessage{{Role: "user", Content: "hi"}})

	if req.MaxCompletionTokens != defaultMaxTokens {
		t.Errorf("expected max_completion_tokens %d, got %d", defaultMaxTokens, req.MaxCompletionTokens)
	}
	if req.MaxTokens != 0 {
		t.Errorf("expected max_tokens unset, got %d", req.MaxTokens)
	}
	if req.Temperature != 0 {
		t.Errorf("expected temperature omitted, got %v", req.Temperature)
	}
}
