package tui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/justin/glamdring/pkg/api"
)

// --- exportHTML coverage ---

func TestExportHTML_ToolUse(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "assistant", Content: []api.ContentBlock{
			{Type: "text", Text: "Let me read that file."},
			{Type: "tool_use", Name: "Read", Input: json.RawMessage(`{"file_path": "/tmp/test.go"}`)},
		}},
	}

	result := exportHTML(msgs)

	if !strings.Contains(result, "Tool: Read") {
		t.Error("expected tool name in HTML output")
	}
	if !strings.Contains(result, "file_path") {
		t.Error("expected tool input JSON in HTML output")
	}
}

func TestExportHTML_ToolResult(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "user", Content: []api.ContentBlock{
			{Type: "tool_result", ToolUseID: "123", Content: "package main\n"},
		}},
	}

	result := exportHTML(msgs)

	if !strings.Contains(result, "tool-result") {
		t.Error("expected tool-result class in HTML output")
	}
	if !strings.Contains(result, "package main") {
		t.Error("expected tool result content in HTML output")
	}
}

func TestExportHTML_ToolError(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "user", Content: []api.ContentBlock{
			{Type: "tool_result", ToolUseID: "abc", Content: "file not found", IsError: true},
		}},
	}

	result := exportHTML(msgs)

	if !strings.Contains(result, "tool-error") {
		t.Error("expected tool-error class in HTML output")
	}
}

func TestExportHTML_ThinkingBlock(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "assistant", Content: []api.ContentBlock{
			{Type: "thinking", Thinking: "Let me consider..."},
			{Type: "text", Text: "Here is my answer."},
		}},
	}

	result := exportHTML(msgs)

	if !strings.Contains(result, "thinking") {
		t.Error("expected thinking class in HTML output")
	}
	if !strings.Contains(result, "Let me consider...") {
		t.Error("expected thinking content in HTML output")
	}
}

func TestExportHTML_ImageBlock(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "user", Content: []api.ContentBlock{
			{Type: "image", Source: &api.ImageSource{
				Type:      "base64",
				MediaType: "image/png",
				Data:      "abc",
			}},
		}},
	}

	result := exportHTML(msgs)

	if !strings.Contains(result, "image-ref") {
		t.Error("expected image-ref class in HTML output")
	}
	if !strings.Contains(result, "image/png") {
		t.Error("expected media type in HTML output")
	}
}

func TestExportHTML_ToolUseInvalidJSON(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "assistant", Content: []api.ContentBlock{
			{Type: "tool_use", Name: "Read", Input: json.RawMessage(`not valid json`)},
		}},
	}

	result := exportHTML(msgs)

	if !strings.Contains(result, "(invalid JSON)") {
		t.Error("expected '(invalid JSON)' for invalid tool input")
	}
}

func TestExportMarkdown_ToolUseInvalidJSON(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "assistant", Content: []api.ContentBlock{
			{Type: "tool_use", Name: "Read", Input: json.RawMessage(`not valid json`)},
		}},
	}

	result := exportMarkdown(msgs)

	if !strings.Contains(result, "(invalid JSON)") {
		t.Error("expected '(invalid JSON)' for invalid tool input in markdown")
	}
}

func TestExportMarkdown_ImageBlock(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "user", Content: []api.ContentBlock{
			{Type: "image", Source: &api.ImageSource{
				Type:      "base64",
				MediaType: "image/png",
				Data:      "abc",
			}},
		}},
	}

	result := exportMarkdown(msgs)

	if !strings.Contains(result, "[Image: image/png]") {
		t.Error("expected image reference in markdown output")
	}
}

func TestExportHTML_EmptyTextFallback(t *testing.T) {
	// Test the fallback for empty text in ContentBlock but content is a string.
	msgs := []api.RequestMessage{
		{Role: "user", Content: "direct string content"},
	}

	result := exportHTML(msgs)

	if !strings.Contains(result, "direct string content") {
		t.Error("expected direct string content in HTML output")
	}
}

func TestExportHTML_ImageBlockNilSource(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "user", Content: []api.ContentBlock{
			{Type: "image", Source: nil},
		}},
	}

	result := exportHTML(msgs)

	// Should not panic, and should not have image-ref.
	if strings.Contains(result, "image-ref") {
		t.Error("expected no image-ref for nil source")
	}
	_ = result
}

func TestExportMarkdown_ImageBlockNilSource(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "user", Content: []api.ContentBlock{
			{Type: "image", Source: nil},
		}},
	}

	result := exportMarkdown(msgs)

	// Should not panic, and should not have [Image:].
	if strings.Contains(result, "[Image:") {
		t.Error("expected no image ref for nil source")
	}
	_ = result
}

func TestExportHTML_CapitalizesRole(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "user", Content: "test"},
	}

	result := exportHTML(msgs)

	if !strings.Contains(result, "User") {
		t.Error("expected capitalized 'User' role")
	}
}

func TestExportHTML_EmptyRole(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "", Content: "test"},
	}

	result := exportHTML(msgs)
	// Should not panic.
	_ = result
}

// --- parseContentBlocks ---

func TestParseContentBlocks_StringInput(t *testing.T) {
	blocks := parseContentBlocks("hello world")
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Type != "text" || blocks[0].Text != "hello world" {
		t.Errorf("expected text block with 'hello world', got %+v", blocks[0])
	}
}

func TestParseContentBlocks_ContentBlockSlice(t *testing.T) {
	input := []api.ContentBlock{{Type: "text", Text: "direct"}}
	blocks := parseContentBlocks(input)
	if len(blocks) != 1 || blocks[0].Text != "direct" {
		t.Errorf("expected direct passthrough, got %+v", blocks)
	}
}

func TestParseContentBlocks_AnySliceValid(t *testing.T) {
	input := []any{
		map[string]any{"type": "text", "text": "from any"},
	}
	blocks := parseContentBlocks(input)
	if len(blocks) != 1 || blocks[0].Text != "from any" {
		t.Errorf("expected parsed block, got %+v", blocks)
	}
}

func TestParseContentBlocks_UnparseableAnySlice(t *testing.T) {
	input := []any{"not a valid block"}
	blocks := parseContentBlocks(input)
	if len(blocks) != 1 || blocks[0].Text != "(unparseable content)" {
		t.Errorf("expected unparseable fallback, got %+v", blocks)
	}
}

func TestParseContentBlocks_DefaultCase_Int(t *testing.T) {
	// An int doesn't match string, []ContentBlock, or []any, and can be
	// marshaled to JSON but can't be unmarshaled to []ContentBlock.
	input := 42
	blocks := parseContentBlocks(input)
	if len(blocks) != 1 || blocks[0].Text != "(unparseable content)" {
		t.Errorf("expected unparseable fallback for int, got %+v", blocks)
	}
}

func TestParseContentBlocks_DefaultCase_MarshalError(t *testing.T) {
	// A channel can't be marshaled to JSON.
	ch := make(chan int)
	blocks := parseContentBlocks(ch)
	if len(blocks) != 1 || blocks[0].Text != "(unparseable content)" {
		t.Errorf("expected unparseable fallback for channel, got %+v", blocks)
	}
}

func TestParseContentBlocks_AnySlice_MarshalError(t *testing.T) {
	// An []any containing a channel can't be marshaled.
	input := []any{make(chan int)}
	blocks := parseContentBlocks(input)
	if len(blocks) != 1 || blocks[0].Text != "(unparseable content)" {
		t.Errorf("expected unparseable fallback for []any with channel, got %+v", blocks)
	}
}

// --- exportMarkdown full coverage ---

func TestExportMarkdown_ToolUseBlock(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "assistant", Content: []api.ContentBlock{
			{Type: "tool_use", Name: "Bash", Input: json.RawMessage(`{"command":"ls"}`)},
		}},
	}

	result := exportMarkdown(msgs)
	if !strings.Contains(result, "Tool: Bash") {
		t.Error("expected tool name in markdown output")
	}
	if !strings.Contains(result, "command") {
		t.Error("expected tool input in markdown output")
	}
}

func TestExportMarkdown_ToolResultBlock(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "user", Content: []api.ContentBlock{
			{Type: "tool_result", ToolUseID: "id1", Content: "tool output here"},
		}},
	}

	result := exportMarkdown(msgs)
	if !strings.Contains(result, "tool output here") {
		t.Error("expected tool result content in markdown output")
	}
}

func TestExportMarkdown_ThinkingContent(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "assistant", Content: []api.ContentBlock{
			{Type: "thinking", Thinking: "reasoning..."},
			{Type: "text", Text: "answer"},
		}},
	}

	result := exportMarkdown(msgs)
	if !strings.Contains(result, "reasoning...") {
		t.Error("expected thinking content in markdown")
	}
}

func TestExportMarkdown_ToolErrorBlock(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "user", Content: []api.ContentBlock{
			{Type: "tool_result", ToolUseID: "id1", Content: "error occurred", IsError: true},
		}},
	}

	result := exportMarkdown(msgs)
	if !strings.Contains(result, "Tool Error") {
		t.Error("expected 'Tool Error' label in markdown for tool error")
	}
}

func TestExportHTML_MultipleMessages(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: []api.ContentBlock{
			{Type: "text", Text: "Hi there!"},
		}},
		{Role: "user", Content: "Thanks"},
	}

	result := exportHTML(msgs)
	if !strings.Contains(result, "Hello") {
		t.Error("expected first user message")
	}
	if !strings.Contains(result, "Hi there!") {
		t.Error("expected assistant response")
	}
	if !strings.Contains(result, "Thanks") {
		t.Error("expected second user message")
	}
}

func TestExportMarkdown_StringContent(t *testing.T) {
	msgs := []api.RequestMessage{
		{Role: "user", Content: "direct string"},
	}

	result := exportMarkdown(msgs)
	if !strings.Contains(result, "direct string") {
		t.Error("expected direct string content in markdown")
	}
}
