package api

import (
	"encoding/json"
	"testing"
)

func TestThinkingConfigSerialization(t *testing.T) {
	cfg := ThinkingConfig{
		Type:         "enabled",
		BudgetTokens: 10000,
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded["type"] != "enabled" {
		t.Errorf("type = %v, want %q", decoded["type"], "enabled")
	}
	if decoded["budget_tokens"] != float64(10000) {
		t.Errorf("budget_tokens = %v, want %v", decoded["budget_tokens"], 10000)
	}
}

func TestThinkingConfigOmitsZeroBudget(t *testing.T) {
	cfg := ThinkingConfig{Type: "disabled"}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, exists := decoded["budget_tokens"]; exists {
		t.Error("budget_tokens should be omitted when zero")
	}
}

func TestContentBlockSignatureDeserialization(t *testing.T) {
	raw := `{
		"type": "thinking",
		"thinking": "some thoughts",
		"signature": "abc123sig"
	}`

	var block ContentBlock
	if err := json.Unmarshal([]byte(raw), &block); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if block.Type != "thinking" {
		t.Errorf("type = %q, want %q", block.Type, "thinking")
	}
	if block.Thinking != "some thoughts" {
		t.Errorf("thinking = %q, want %q", block.Thinking, "some thoughts")
	}
	if block.Signature != "abc123sig" {
		t.Errorf("signature = %q, want %q", block.Signature, "abc123sig")
	}
}

func TestContentBlockSignatureRoundTrip(t *testing.T) {
	block := ContentBlock{
		Type:      "thinking",
		Thinking:  "deep thoughts",
		Signature: "sig456",
	}

	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ContentBlock
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Signature != "sig456" {
		t.Errorf("signature = %q, want %q", decoded.Signature, "sig456")
	}
}

func TestAPIErrorFormat(t *testing.T) {
	err := &APIError{
		StatusCode: 429,
		Type:       "rate_limit_error",
		Message:    "too many requests",
	}
	want := "api error 429 (rate_limit_error): too many requests"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
