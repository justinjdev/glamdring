package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/justin/glamdring/pkg/auth"
)

func TestStreamNoGoroutineLeak(t *testing.T) {
	server := newMockSSEServer(buildSSEResponse("hello", "end_turn"))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "test-model")
	client.SetEndpoint(server.URL)

	// Stabilize goroutine count.
	runtime.GC()
	time.Sleep(10 * time.Millisecond)
	before := runtime.NumGoroutine()

	ctx := context.Background()
	events, err := client.Stream(ctx, &MessageRequest{
		MaxTokens: 1024,
		Messages:  []RequestMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	// Drain all events.
	for range events {
	}

	// Close the server so its goroutines clean up before measuring.
	server.Close()

	// Wait for goroutines to clean up.
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	time.Sleep(10 * time.Millisecond)
	after := runtime.NumGoroutine()

	// Allow +1 tolerance for runtime background goroutines.
	if after > before+1 {
		t.Errorf("goroutine leak: before=%d, after=%d", before, after)
	}
}

func TestStreamContextCancellation(t *testing.T) {
	// Server that sends events slowly — we cancel mid-stream.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		// Send message_start.
		fmt.Fprint(w, "event: message_start\n")
		fmt.Fprint(w, `data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":0}}}`)
		fmt.Fprint(w, "\n\n")
		flusher.Flush()

		// Block until client disconnects.
		<-r.Context().Done()
	}))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "test-model")
	client.SetEndpoint(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	events, err := client.Stream(ctx, &MessageRequest{
		MaxTokens: 1024,
		Messages:  []RequestMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	// Read one event, then cancel.
	<-events
	cancel()

	// Channel should close promptly.
	timeout := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-events:
			if !ok {
				return // success — channel closed
			}
		case <-timeout:
			t.Fatal("events channel not closed after context cancellation")
		}
	}
}

// newMockSSEServer creates a test server that returns canned SSE responses.
// Each call to the server returns the next response in the sequence.
// If the sequence is exhausted, it returns the last response.
func newMockSSEServer(responses ...string) *httptest.Server {
	call := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := call
		if idx >= len(responses) {
			idx = len(responses) - 1
		}
		call++

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, responses[idx])
	}))
}

// buildSSEResponse constructs a complete SSE stream for a simple text response
// with the given text and stop reason.
func buildSSEResponse(text, stopReason string) string {
	var b strings.Builder

	// message_start
	b.WriteString("event: message_start\n")
	b.WriteString(`data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":0}}}`)
	b.WriteString("\n\n")

	// content_block_start
	b.WriteString("event: content_block_start\n")
	b.WriteString(`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
	b.WriteString("\n\n")

	// content_block_delta
	b.WriteString("event: content_block_delta\n")
	b.WriteString(fmt.Sprintf(`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"%s"}}`, text))
	b.WriteString("\n\n")

	// content_block_stop
	b.WriteString("event: content_block_stop\n")
	b.WriteString(`data: {"type":"content_block_stop","index":0}`)
	b.WriteString("\n\n")

	// message_delta
	b.WriteString("event: message_delta\n")
	b.WriteString(fmt.Sprintf(`data: {"type":"message_delta","delta":{"stop_reason":"%s"},"usage":{"output_tokens":50}}`, stopReason))
	b.WriteString("\n\n")

	// message_stop
	b.WriteString("event: message_stop\n")
	b.WriteString(`data: {"type":"message_stop"}`)
	b.WriteString("\n\n")

	return b.String()
}
