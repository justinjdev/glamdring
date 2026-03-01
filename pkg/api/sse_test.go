package api

import (
	"fmt"
	"io"
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

func TestDecodeEventPing(t *testing.T) {
	ev, err := decodeEvent("ping", []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev != nil {
		t.Errorf("ping should return nil event, got %+v", ev)
	}
}

func TestDecodeEventUnknownType(t *testing.T) {
	ev, err := decodeEvent("some_future_event", []byte(`{"foo":"bar"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev != nil {
		t.Errorf("unknown event type should return nil event, got %+v", ev)
	}
}

func TestDecodeEventError(t *testing.T) {
	data := `{"type":"error","error":{"type":"overloaded_error","message":"server overloaded"}}`
	ev, err := decodeEvent("error", []byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != "error" {
		t.Errorf("Type = %q, want %q", ev.Type, "error")
	}
	if ev.Err == nil {
		t.Fatal("expected Err to be set")
	}
	if !strings.Contains(ev.Err.Error(), "overloaded_error") {
		t.Errorf("Err = %q, want to contain %q", ev.Err.Error(), "overloaded_error")
	}
	if !strings.Contains(ev.Err.Error(), "server overloaded") {
		t.Errorf("Err = %q, want to contain %q", ev.Err.Error(), "server overloaded")
	}
}

func TestDecodeEventInvalidJSON(t *testing.T) {
	types := []string{"message_start", "content_block_start", "content_block_delta",
		"content_block_stop", "message_delta", "error"}

	for _, et := range types {
		_, err := decodeEvent(et, []byte(`{invalid json`))
		if err == nil {
			t.Errorf("decodeEvent(%q, invalid JSON) should return error", et)
		}
	}
}

func TestDecodeEventMessageDelta(t *testing.T) {
	data := `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":42}}`
	ev, err := decodeEvent("message_delta", []byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want %q", ev.StopReason, "end_turn")
	}
	if ev.Usage == nil || ev.Usage.OutputTokens != 42 {
		t.Errorf("Usage.OutputTokens = %v, want 42", ev.Usage)
	}
}

func TestParseSSECommentAndEmptyLines(t *testing.T) {
	// SSE stream with comment lines (starting with ':') should be ignored.
	var b strings.Builder
	b.WriteString(": this is a comment\n")
	b.WriteString("\n")
	b.WriteString("event: message_start\n")
	b.WriteString(`data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":0}}}`)
	b.WriteString("\n\n")
	b.WriteString(": another comment\n")
	b.WriteString("event: message_stop\n")
	b.WriteString(`data: {"type":"message_stop"}`)
	b.WriteString("\n\n")

	done := make(chan struct{})
	events := parseSSE(strings.NewReader(b.String()), done)

	var types []string
	for ev := range events {
		types = append(types, ev.Type)
	}

	if len(types) != 2 {
		t.Errorf("got %d events, want 2: %v", len(types), types)
	}
}

func TestParseSSEPingIgnored(t *testing.T) {
	var b strings.Builder
	b.WriteString("event: ping\n")
	b.WriteString(`data: {"type":"ping"}`)
	b.WriteString("\n\n")
	b.WriteString("event: message_start\n")
	b.WriteString(`data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":0}}}`)
	b.WriteString("\n\n")
	b.WriteString("event: message_stop\n")
	b.WriteString(`data: {"type":"message_stop"}`)
	b.WriteString("\n\n")

	done := make(chan struct{})
	events := parseSSE(strings.NewReader(b.String()), done)

	var types []string
	for ev := range events {
		types = append(types, ev.Type)
	}

	// ping should be skipped.
	if len(types) != 2 {
		t.Errorf("got %d events, want 2 (ping should be ignored): %v", len(types), types)
	}
}

func TestParseSSEDecodeError(t *testing.T) {
	var b strings.Builder
	b.WriteString("event: message_start\n")
	b.WriteString(`data: {invalid json}`)
	b.WriteString("\n\n")

	done := make(chan struct{})
	events := parseSSE(strings.NewReader(b.String()), done)

	var gotError bool
	for ev := range events {
		if ev.Type == "error" {
			gotError = true
		}
	}
	if !gotError {
		t.Error("expected error event for invalid JSON")
	}
}

func TestParseSSEScannerError(t *testing.T) {
	// Create a reader that errors mid-read.
	r := &errorReader{data: "event: message_start\ndata: ", err: io.ErrUnexpectedEOF}
	done := make(chan struct{})
	events := parseSSE(r, done)

	var gotError bool
	for ev := range events {
		if ev.Type == "error" {
			gotError = true
		}
	}
	if !gotError {
		t.Error("expected error event for scanner error")
	}
}

// errorReader returns data then an error.
type errorReader struct {
	data    string
	err     error
	dataPos int
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	if r.dataPos < len(r.data) {
		n = copy(p, r.data[r.dataPos:])
		r.dataPos += n
		return n, nil
	}
	return 0, r.err
}

func TestSendDoneClosed(t *testing.T) {
	// Use an unbuffered channel so that the done branch is the only one ready.
	ch := make(chan StreamEvent)
	done := make(chan struct{})
	close(done)

	ok := send(ch, done, StreamEvent{Type: "test"})
	if ok {
		t.Error("send should return false when done is closed")
	}
}
