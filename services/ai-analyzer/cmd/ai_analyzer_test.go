package main

import (
	"encoding/json"
	"testing"
)

func TestGetEnv_ReturnsValue(t *testing.T) {
	t.Setenv("MY_TEST_VAR", "testval")
	got := getEnv("MY_TEST_VAR", "default")
	if got != "testval" {
		t.Errorf("expected 'testval', got %q", got)
	}
}

func TestGetEnv_ReturnsFallback(t *testing.T) {
	got := getEnv("NONEXISTENT_VAR_XYZ_123", "fallback")
	if got != "fallback" {
		t.Errorf("expected 'fallback', got %q", got)
	}
}

func TestAnthropicRequest_Serialization(t *testing.T) {
	req := AnthropicRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 500,
		Messages: []Message{
			{Role: "user", Content: "test"},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded AnthropicRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Model != req.Model {
		t.Errorf("Model mismatch: got %s", decoded.Model)
	}
	if decoded.MaxTokens != req.MaxTokens {
		t.Errorf("MaxTokens mismatch: got %d", decoded.MaxTokens)
	}
	if len(decoded.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(decoded.Messages))
	}
	if decoded.Messages[0].Role != "user" {
		t.Errorf("expected role 'user', got %s", decoded.Messages[0].Role)
	}
}

func TestAnthropicResponse_EmptyContent(t *testing.T) {
	resp := AnthropicResponse{}
	if len(resp.Content) != 0 {
		t.Error("empty response should have no content")
	}
}

func TestDeduplicateAlerts_EmptyReturnsEmpty(t *testing.T) {
	analyzer := &AIAnalyzer{}
	result, err := analyzer.DeduplicateAlerts(nil, "acme", []map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}

func TestDeduplicateAlerts_SingleAlertPassthrough(t *testing.T) {
	analyzer := &AIAnalyzer{}
	alerts := []map[string]any{{"id": "a1", "message": "test"}}
	result, err := analyzer.DeduplicateAlerts(nil, "acme", alerts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("single alert should pass through unchanged, got %d", len(result))
	}
}
// env
// request
// dedup
// env
// request
// dedup
// env
// request
// dedup
// env test
// request serial
// response empty
// dedup empty
// dedup single
// env test
// request serial
// response empty
// dedup empty
// dedup single
// env test
