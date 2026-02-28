package tui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/justin/glamdring/pkg/api"
)

func TestExportMarkdown_SimpleConversation(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "user", Content: "Hello there"},
		{Role: "assistant", Content: "Hi! How can I help?"},
	}

	result := exportMarkdown(msgs)

	if !strings.Contains(result, "## User") {
		t.Error("expected User header")
	}
	if !strings.Contains(result, "## Assistant") {
		t.Error("expected Assistant header")
	}
	if !strings.Contains(result, "Hello there") {
		t.Error("expected user message text")
	}
	if !strings.Contains(result, "Hi! How can I help?") {
		t.Error("expected assistant message text")
	}
}

func TestExportMarkdown_ToolUse(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "assistant", Content: []api.ContentBlock{
			{Type: "text", Text: "Let me read that file."},
			{Type: "tool_use", Name: "Read", Input: json.RawMessage(`{"file_path": "/tmp/test.go"}`)},
		}},
		{Role: "user", Content: []api.ContentBlock{
			{Type: "tool_result", ToolUseID: "123", Content: "package main\n"},
		}},
	}

	result := exportMarkdown(msgs)

	if !strings.Contains(result, "**Tool: Read**") {
		t.Error("expected tool call header")
	}
	if !strings.Contains(result, "file_path") {
		t.Error("expected tool input JSON")
	}
	if !strings.Contains(result, "**Tool Result:**") {
		t.Error("expected tool result header")
	}
	if !strings.Contains(result, "package main") {
		t.Error("expected tool result content")
	}
}

func TestExportMarkdown_ThinkingBlock(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "assistant", Content: []api.ContentBlock{
			{Type: "thinking", Thinking: "Let me consider this..."},
			{Type: "text", Text: "Here is my answer."},
		}},
	}

	result := exportMarkdown(msgs)

	if !strings.Contains(result, "<details>") {
		t.Error("expected details tag for thinking")
	}
	if !strings.Contains(result, "Let me consider this...") {
		t.Error("expected thinking content")
	}
}

func TestExportMarkdown_ToolError(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "user", Content: []api.ContentBlock{
			{Type: "tool_result", ToolUseID: "abc", Content: "file not found", IsError: true},
		}},
	}

	result := exportMarkdown(msgs)

	if !strings.Contains(result, "**Tool Error:**") {
		t.Error("expected tool error header")
	}
}

func TestExportHTML_SelfContained(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "World"},
	}

	result := exportHTML(msgs)

	if !strings.Contains(result, "<!DOCTYPE html>") {
		t.Error("expected HTML doctype")
	}
	if !strings.Contains(result, "<style>") {
		t.Error("expected embedded CSS")
	}
	if !strings.Contains(result, "--bg: #1a1612") {
		t.Error("expected gruvbox-inspired theme colors")
	}
	if !strings.Contains(result, "Hello") {
		t.Error("expected user message")
	}
	if !strings.Contains(result, "World") {
		t.Error("expected assistant message")
	}
}

func TestExportHTML_EscapesHTML(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "user", Content: `<script>alert("xss")</script>`},
	}

	result := exportHTML(msgs)

	if strings.Contains(result, `<script>alert`) {
		t.Error("expected HTML to be escaped")
	}
	if !strings.Contains(result, "&lt;script&gt;") {
		t.Error("expected escaped script tag")
	}
}

func TestParseContentBlocks_String(t *testing.T) {
	blocks := parseContentBlocks("hello")
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Type != "text" || blocks[0].Text != "hello" {
		t.Error("expected text block with 'hello'")
	}
}

func TestParseContentBlocks_AnySlice(t *testing.T) {
	// Simulate JSON-deserialized content: []any of map[string]any.
	raw := []byte(`[{"type":"text","text":"hello"},{"type":"tool_use","name":"Bash"}]`)
	var anySlice []any
	if err := json.Unmarshal(raw, &anySlice); err != nil {
		t.Fatalf("failed to unmarshal test data: %v", err)
	}

	blocks := parseContentBlocks(anySlice)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Type != "text" || blocks[0].Text != "hello" {
		t.Errorf("expected text block with 'hello', got type=%q text=%q", blocks[0].Type, blocks[0].Text)
	}
	if blocks[1].Type != "tool_use" || blocks[1].Name != "Bash" {
		t.Errorf("expected tool_use block with name 'Bash', got type=%q name=%q", blocks[1].Type, blocks[1].Name)
	}
}

func TestParseContentBlocks_InvalidAny(t *testing.T) {
	// Pass something that cannot be marshaled into ContentBlocks.
	blocks := parseContentBlocks(12345)
	if len(blocks) != 1 || blocks[0].Text != "(unparseable content)" {
		t.Errorf("expected placeholder block, got %v", blocks)
	}
}

func TestParseContentBlocks_InvalidAnySlice(t *testing.T) {
	// []any with non-object elements that won't unmarshal to ContentBlock.
	badSlice := []any{"not", "objects"}
	blocks := parseContentBlocks(badSlice)
	// Should still produce blocks (strings marshal/unmarshal as empty structs).
	// The important thing is it doesn't panic.
	_ = blocks
}

func TestExportMarkdown_EmptyMessages(t *testing.T) {
	result := exportMarkdown([]api.RequestMessage{})
	// Should produce minimal output (just a trailing newline).
	if result != "\n" {
		t.Errorf("expected single newline for empty messages, got %q", result)
	}
}

func TestExportHTML_EmptyMessages(t *testing.T) {
	result := exportHTML([]api.RequestMessage{})
	// Should still produce valid HTML structure.
	if !strings.Contains(result, "<!DOCTYPE html>") {
		t.Error("expected HTML doctype even for empty messages")
	}
}

func TestParseContentBlocks_TypedSlice(t *testing.T) {
	input := []api.ContentBlock{
		{Type: "text", Text: "test"},
		{Type: "tool_use", Name: "Bash"},
	}
	blocks := parseContentBlocks(input)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Type != "text" {
		t.Error("expected first block to be text")
	}
	if blocks[1].Name != "Bash" {
		t.Error("expected second block to be Bash tool_use")
	}
}
