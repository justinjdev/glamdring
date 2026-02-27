package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

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
