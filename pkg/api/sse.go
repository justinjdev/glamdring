package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// parseSSE reads SSE events from r and sends typed StreamEvents on the returned channel.
// The channel is closed when the stream ends or an error occurs.
// The goroutine respects ctx cancellation via the done channel.
func parseSSE(r io.Reader, done <-chan struct{}) <-chan StreamEvent {
	ch := make(chan StreamEvent, 16)

	go func() {
		defer close(ch)

		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 1<<20), 10<<20) // 1MB initial, 10MB max
		var eventType string

		for scanner.Scan() {
			line := scanner.Text()

			// Empty line = end of event (but we dispatch on data: lines directly
			// since each SSE event has exactly one data line in the Messages API).
			if line == "" {
				eventType = ""
				continue
			}

			if strings.HasPrefix(line, "event: ") {
				eventType = strings.TrimPrefix(line, "event: ")
				continue
			}

			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				ev, err := decodeEvent(eventType, []byte(data))
				if err != nil {
					send(ch, done, StreamEvent{Type: "error", Err: fmt.Errorf("decode %s: %w", eventType, err)})
					return
				}
				if ev != nil {
					if !send(ch, done, *ev) {
						return
					}
				}
				// message_stop means the stream is complete.
				if eventType == "message_stop" {
					return
				}
				continue
			}

			// Ignore comment lines (starting with ':') and unknown lines.
		}

		if err := scanner.Err(); err != nil {
			send(ch, done, StreamEvent{Type: "error", Err: fmt.Errorf("sse read: %w", err)})
		}
	}()

	return ch
}

// send attempts to send an event on the channel, returning false if done is closed.
func send(ch chan<- StreamEvent, done <-chan struct{}, ev StreamEvent) bool {
	select {
	case <-done:
		return false
	case ch <- ev:
		return true
	}
}

// decodeEvent parses a JSON data payload into a typed StreamEvent.
func decodeEvent(eventType string, data []byte) (*StreamEvent, error) {
	switch eventType {
	case "message_start":
		var payload sseMessageStart
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &StreamEvent{
			Type:    "message_start",
			Message: &payload.Message,
		}, nil

	case "content_block_start":
		var payload sseContentBlockStart
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &StreamEvent{
			Type:         "content_block_start",
			Index:        payload.Index,
			ContentBlock: &payload.ContentBlock,
		}, nil

	case "content_block_delta":
		var payload sseContentBlockDelta
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &StreamEvent{
			Type:  "content_block_delta",
			Index: payload.Index,
			Delta: &payload.Delta,
		}, nil

	case "content_block_stop":
		var payload sseContentBlockStop
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &StreamEvent{
			Type:  "content_block_stop",
			Index: payload.Index,
		}, nil

	case "message_delta":
		var payload sseMessageDelta
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &StreamEvent{
			Type:       "message_delta",
			Delta:      &payload.Delta,
			StopReason: payload.Delta.StopReason,
			Usage:      &payload.Usage,
		}, nil

	case "message_stop":
		return &StreamEvent{
			Type: "message_stop",
		}, nil

	case "ping":
		// Keepalive, ignore.
		return nil, nil

	case "error":
		var payload sseError
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &StreamEvent{
			Type: "error",
			Err:  fmt.Errorf("%s: %s", payload.Error.Type, payload.Error.Message),
		}, nil

	default:
		// Unknown event type — skip silently for forward compatibility.
		return nil, nil
	}
}
