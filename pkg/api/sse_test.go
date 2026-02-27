package api

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseSSELargePayload(t *testing.T) {
	// Create a text delta larger than the default 64KB scanner buffer.
	largeText := strings.Repeat("x", 100_000)

	var b strings.Builder
	b.WriteString("event: message_start\n")
	b.WriteString(`data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":0}}}`)
	b.WriteString("\n\n")

	b.WriteString("event: content_block_start\n")
	b.WriteString(`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
	b.WriteString("\n\n")

	b.WriteString("event: content_block_delta\n")
	b.WriteString(fmt.Sprintf(`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"%s"}}`, largeText))
	b.WriteString("\n\n")

	b.WriteString("event: content_block_stop\n")
	b.WriteString(`data: {"type":"content_block_stop","index":0}`)
	b.WriteString("\n\n")

	b.WriteString("event: message_delta\n")
	b.WriteString(`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":50}}`)
	b.WriteString("\n\n")

	b.WriteString("event: message_stop\n")
	b.WriteString(`data: {"type":"message_stop"}`)
	b.WriteString("\n\n")

	done := make(chan struct{})
	events := parseSSE(strings.NewReader(b.String()), done)

	var gotText string
	var gotError error
	for ev := range events {
		if ev.Type == "error" {
			gotError = ev.Err
		}
		if ev.Type == "content_block_delta" && ev.Delta != nil {
			gotText = ev.Delta.Text
		}
	}

	if gotError != nil {
		t.Fatalf("unexpected error parsing large payload: %v", gotError)
	}
	if len(gotText) != 100_000 {
		t.Errorf("text length = %d, want 100000", len(gotText))
	}
}
