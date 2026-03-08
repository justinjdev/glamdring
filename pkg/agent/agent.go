package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/justin/glamdring/pkg/api"
	"github.com/justin/glamdring/pkg/config"
	"github.com/justin/glamdring/pkg/hooks"
	"github.com/justin/glamdring/pkg/tools"
)

// alwaysAllowTools is the set of tool names that never require user permission.
var alwaysAllowTools = map[string]bool{
	"Read": true,
	"Glob": true,
	"Grep": true,
}

// maxToolResultSize is the maximum size of a tool result before truncation.
// Results exceeding this are truncated before being sent to the API to protect
// the context window.
const maxToolResultSize = 50_000

// truncateToolResult caps tool output at maxToolResultSize bytes to prevent
// blowing the context window with huge results.
func truncateToolResult(output string) string {
	if len(output) <= maxToolResultSize {
		return output
	}
	log.Printf("truncating tool result from %d to %d bytes", len(output), maxToolResultSize)
	truncated := output[:maxToolResultSize]
	// Trim any incomplete trailing UTF-8 sequence (at most 3 bytes).
	for i := 0; i < 3 && len(truncated) > 0; i++ {
		if utf8.ValidString(truncated) {
			break
		}
		truncated = truncated[:len(truncated)-1]
	}
	return truncated +
		"\n... (truncated, full output was " + strconv.Itoa(len(output)) + " bytes)"
}

// Run starts the agentic loop as a one-shot operation (no multi-turn memory).
// It returns a channel of Messages that the consumer reads to receive text
// deltas, tool calls, permission requests, errors, and the final done signal.
// The channel is closed when the loop terminates.
//
// For multi-turn conversations with memory, use NewSession and Session.Turn.
func Run(ctx context.Context, cfg Config) <-chan Message {
	s := NewSession(cfg)
	s.messages = []api.RequestMessage{{Role: "user", Content: cfg.Prompt}}
	out := make(chan Message, 64)
	go func() {
		defer close(out)
		s.runTurn(ctx, out)
	}()
	return out
}

// turnResult holds the collected state from processing a single streamed response.
type turnResult struct {
	contentBlocks            []api.ContentBlock
	toolCalls                []toolCall
	stopReason               string
	inputTokens              int
	outputTokens             int
	cacheCreationTokens      int
	cacheReadTokens          int
	lastRequestInputTokens   int // input tokens from message_start (context snapshot)
}

// toolCall represents a single tool_use block extracted from the assistant response.
type toolCall struct {
	id    string
	name  string
	input json.RawMessage
}

// processTurn reads all stream events for a single API turn, emitting deltas
// on the output channel and collecting the full content blocks and tool calls.
func processTurn(ctx context.Context, events <-chan api.StreamEvent, out chan<- Message) (*turnResult, error) {
	result := &turnResult{}

	// Track content blocks being built during the stream.
	// The index in this slice corresponds to the content block index from the API.
	type blockBuilder struct {
		block    api.ContentBlock
		inputBuf strings.Builder // accumulates input_json_delta for tool_use blocks
	}
	var blocks []blockBuilder

	for ev := range events {
		// Check for context cancellation.
		if ctx.Err() != nil {
			return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
		}

		switch ev.Type {
		case "message_start":
			if ev.Message != nil {
				result.inputTokens += ev.Message.Usage.InputTokens
				result.outputTokens += ev.Message.Usage.OutputTokens
				result.cacheCreationTokens += ev.Message.Usage.CacheCreationInputTokens
				result.cacheReadTokens += ev.Message.Usage.CacheReadInputTokens
				result.lastRequestInputTokens = ev.Message.Usage.InputTokens +
				ev.Message.Usage.CacheCreationInputTokens +
				ev.Message.Usage.CacheReadInputTokens
			}

		case "content_block_start":
			if ev.ContentBlock == nil {
				continue
			}
			// Ensure the blocks slice is large enough.
			for len(blocks) <= ev.Index {
				blocks = append(blocks, blockBuilder{})
			}
			blocks[ev.Index] = blockBuilder{
				block: *ev.ContentBlock,
			}

		case "content_block_delta":
			if ev.Delta == nil {
				continue
			}
			switch ev.Delta.Type {
			case "text_delta":
				if ev.Index < len(blocks) {
					blocks[ev.Index].block.Text += ev.Delta.Text
				}
				emit(ctx, out, Message{Type: MessageTextDelta, Text: ev.Delta.Text})

			case "thinking_delta":
				if ev.Index < len(blocks) {
					blocks[ev.Index].block.Thinking += ev.Delta.Thinking
				}
				emit(ctx, out, Message{Type: MessageThinkingDelta, Text: ev.Delta.Thinking})

			case "signature_delta":
				if ev.Index < len(blocks) {
					blocks[ev.Index].block.Signature += ev.Delta.Signature
				}

			case "input_json_delta":
				if ev.Index < len(blocks) {
					blocks[ev.Index].inputBuf.WriteString(ev.Delta.PartialJSON)
				}
			}

		case "content_block_stop":
			if ev.Index < len(blocks) {
				b := &blocks[ev.Index]
				// If this was a tool_use block, finalize the input JSON.
				if b.block.Type == "tool_use" {
					inputJSON := b.inputBuf.String()
					if inputJSON == "" {
						inputJSON = "{}"
					}
					b.block.Input = json.RawMessage(inputJSON)

					result.toolCalls = append(result.toolCalls, toolCall{
						id:    b.block.ID,
						name:  b.block.Name,
						input: b.block.Input,
					})
				}
			}

		case "message_delta":
			if ev.StopReason != "" {
				result.stopReason = ev.StopReason
			}
			if ev.Usage != nil {
				result.outputTokens += ev.Usage.OutputTokens
				result.cacheCreationTokens += ev.Usage.CacheCreationInputTokens
				result.cacheReadTokens += ev.Usage.CacheReadInputTokens
			}

		case "message_stop":
			// Stream is done for this turn.

		case "error":
			if ev.Err != nil {
				return nil, fmt.Errorf("stream error: %w", ev.Err)
			}
		}
	}

	// Build the final content blocks from the builders.
	result.contentBlocks = make([]api.ContentBlock, len(blocks))
	for i, b := range blocks {
		result.contentBlocks[i] = b.block
	}

	return result, nil
}

// isForceShutdown checks if a priority message is a force shutdown request.
func isForceShutdown(msg any) bool {
	data, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	var parsed struct {
		Kind  string `json:"kind"`
		Force bool   `json:"force"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return false
	}
	return parsed.Kind == "shutdown_request" && parsed.Force
}

// executeTools runs each tool call, handling permissions, and returns the
// tool_result content blocks for the next API request. If priorityCh is
// non-nil, high-priority inter-agent messages are checked between tool
// executions and injected as additional result blocks.
func executeTools(
	ctx context.Context,
	out chan<- Message,
	provider tools.ToolProvider,
	calls []toolCall,
	sessionAllow map[string]bool,
	hookRunner *hooks.HookRunner,
	permissions *config.PermissionConfig,
	teamScope *config.TeamScope,
	priorityCh <-chan any,
	cancelFunc context.CancelFunc,
) ([]api.ContentBlock, error) {
	results := make([]api.ContentBlock, 0, len(calls))

	for i, tc := range calls {
		// Check context. Fill in error results for remaining calls so the
		// message history stays valid (every tool_use needs a tool_result).
		if ctx.Err() != nil {
			for _, remaining := range calls[i:] {
				results = append(results, api.ContentBlock{
					Type:      "tool_result",
					ToolUseID: remaining.id,
					Content:   fmt.Sprintf("tool execution cancelled: %s", ctx.Err()),
					IsError:   true,
				})
			}
			return results, nil
		}

		// Parse the tool input for the message.
		var inputMap map[string]any
		if err := json.Unmarshal(tc.input, &inputMap); err != nil {
			inputMap = map[string]any{"raw": string(tc.input)}
		}

		// Emit the tool call message.
		emit(ctx, out, Message{
			Type:      MessageToolCall,
			ToolName:  tc.name,
			ToolID:    tc.id,
			ToolInput: inputMap,
		})

		// Run PreToolUse hooks. A failure blocks the tool.
		if hookRunner != nil {
			if err := hookRunner.Run(ctx, hooks.PreToolUse, tc.name, tc.input); err != nil {
				errMsg := fmt.Sprintf("blocked by hook: %s", err.Error())
				results = append(results, api.ContentBlock{
					Type:      "tool_result",
					ToolUseID: tc.id,
					Content:   errMsg,
					IsError:   true,
				})
				emit(ctx, out, Message{
					Type:        MessageToolResult,
					ToolName:    tc.name,
					ToolID:      tc.id,
					ToolOutput:  errMsg,
					ToolIsError: true,
				})
				continue
			}
		}

		// Check team scope restrictions (before general permissions).
		if teamScope != nil {
			if config.EvaluateTeamScope(teamScope, tc.name, inputMap) == config.PermissionResultDeny {
				errMsg := "blocked by team scope restriction"
				results = append(results, api.ContentBlock{
					Type:      "tool_result",
					ToolUseID: tc.id,
					Content:   errMsg,
					IsError:   true,
				})
				emit(ctx, out, Message{
					Type:        MessageToolResult,
					ToolName:    tc.name,
					ToolID:      tc.id,
					ToolOutput:  errMsg,
					ToolIsError: true,
				})
				continue
			}
		}

		// Check configured permission rules (deny -> allow -> default).
		skipPrompt := false
		if permissions != nil {
			switch permissions.Evaluate(tc.name, inputMap) {
			case config.PermissionResultDeny:
				log.Printf("permission denied by rule for tool %s", tc.name)
				errMsg := "blocked by permission rule"
				results = append(results, api.ContentBlock{
					Type:      "tool_result",
					ToolUseID: tc.id,
					Content:   errMsg,
					IsError:   true,
				})
				emit(ctx, out, Message{
					Type:        MessageToolResult,
					ToolName:    tc.name,
					ToolID:      tc.id,
					ToolOutput:  errMsg,
					ToolIsError: true,
				})
				continue
			case config.PermissionResultAllow:
				skipPrompt = true
			}
		}

		// Check session/default permissions.
		if !skipPrompt && !isAllowed(tc.name, sessionAllow) {
			summary := permissionSummary(tc.name, inputMap)
			permCh := make(chan PermissionAnswer, 1)

			emit(ctx, out, Message{
				Type:               MessagePermissionRequest,
				ToolName:           tc.name,
				ToolID:             tc.id,
				ToolInput:          inputMap,
				PermissionSummary:  summary,
				PermissionResponse: permCh,
			})

			// Block until we get a permission response or context is cancelled.
			var answer PermissionAnswer
			select {
			case answer = <-permCh:
			case <-ctx.Done():
				// Fill in error results for this and all remaining calls,
				// preserving any results already collected.
				for _, remaining := range calls[i:] {
					results = append(results, api.ContentBlock{
						Type:      "tool_result",
						ToolUseID: remaining.id,
						Content:   fmt.Sprintf("tool execution cancelled: %s", ctx.Err()),
						IsError:   true,
					})
				}
				return results, nil
			}

			switch answer {
			case PermissionDeny:
				results = append(results, api.ContentBlock{
					Type:      "tool_result",
					ToolUseID: tc.id,
					Content:   "Permission denied by user.",
					IsError:   true,
				})
				emit(ctx, out, Message{
					Type:        MessageToolResult,
					ToolName:    tc.name,
					ToolID:      tc.id,
					ToolOutput:  "Permission denied by user.",
					ToolIsError: true,
				})
				continue
			case PermissionAlwaysApprove:
				sessionAllow[tc.name] = true
			case PermissionApprove:
				// One-time approval, proceed.
			}
		}

		// Execute the tool, streaming output if supported.
		onOutput := func(text string) {
			emit(ctx, out, Message{
				Type:     MessageToolOutputDelta,
				ToolName: tc.name,
				ToolID:   tc.id,
				Text:     text,
			})
		}
		toolResult, err := provider.ExecuteStreaming(ctx, tc.name, tc.input, onOutput)
		if err != nil {
			// Execution error (not a tool-level error) — treat as error result.
			errMsg := fmt.Sprintf("tool execution error: %s", err.Error())
			results = append(results, api.ContentBlock{
				Type:      "tool_result",
				ToolUseID: tc.id,
				Content:   errMsg,
				IsError:   true,
			})
			emit(ctx, out, Message{
				Type:        MessageToolResult,
				ToolName:    tc.name,
				ToolID:      tc.id,
				ToolOutput:  errMsg,
				ToolIsError: true,
			})
			continue
		}

		// Truncate large outputs to protect the context window.
		output := truncateToolResult(toolResult.Output)

		results = append(results, api.ContentBlock{
			Type:      "tool_result",
			ToolUseID: tc.id,
			Content:   output,
			IsError:   toolResult.IsError,
		})
		emit(ctx, out, Message{
			Type:        MessageToolResult,
			ToolName:    tc.name,
			ToolID:      tc.id,
			ToolOutput:  toolResult.Output, // TUI gets full output
			ToolIsError: toolResult.IsError,
		})

		// Run PostToolUse hooks (failures are warnings, not blocking).
		if hookRunner != nil {
			if hookErr := hookRunner.Run(ctx, hooks.PostToolUse, tc.name, tc.input); hookErr != nil {
				log.Printf("warning: PostToolUse hook failed for %s: %v", tc.name, hookErr)
			}
		}

		// Drain all pending priority inter-agent messages between tool executions.
		if priorityCh != nil {
		drain:
			for {
				select {
				case msg, ok := <-priorityCh:
					if !ok {
						priorityCh = nil
						break drain
					}
					if isForceShutdown(msg) && cancelFunc != nil {
						results[len(results)-1].Content += "\n\n[Force shutdown received -- terminating]"
						// Fill in error results for remaining calls so
						// every tool_use has a matching tool_result.
						for _, remaining := range calls[i+1:] {
							results = append(results, api.ContentBlock{
								Type:      "tool_result",
								ToolUseID: remaining.id,
								Content:   "tool execution cancelled: force shutdown",
								IsError:   true,
							})
						}
						cancelFunc()
						return results, nil
					}
					text := formatPriorityMessage(msg)
					results[len(results)-1].Content += "\n\n" + text
				default:
					break drain
				}
			}
		}
	}

	return results, nil
}

// formatPriorityMessage converts a priority team message to a tool result string.
func formatPriorityMessage(msg any) string {
	switch v := msg.(type) {
	case string:
		return "[Team Message] " + v
	case fmt.Stringer:
		return "[Team Message] " + v.String()
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("[Team Message] %v", v)
		}
		return "[Team Message] " + string(data)
	}
}

// isAllowed checks whether a tool can execute without user permission.
func isAllowed(name string, sessionAllow map[string]bool) bool {
	if alwaysAllowTools[name] {
		return true
	}
	return sessionAllow[name]
}

// permissionSummary generates a human-readable summary of what the tool will do.
func permissionSummary(name string, input map[string]any) string {
	switch name {
	case "Bash":
		if cmd, ok := input["command"].(string); ok {
			// Truncate long commands for display.
			if len(cmd) > 80 {
				cmd = cmd[:77] + "..."
			}
			return fmt.Sprintf("Run: %s", cmd)
		}
		return fmt.Sprintf("Run: %s", name)

	case "Write":
		if path, ok := input["file_path"].(string); ok {
			return fmt.Sprintf("Write to %s", path)
		}
		return "Write to file"

	case "Edit":
		if path, ok := input["file_path"].(string); ok {
			return fmt.Sprintf("Edit %s", path)
		}
		return "Edit file"

	default:
		return fmt.Sprintf("Execute tool: %s", name)
	}
}

// emit sends a message on the output channel, respecting context cancellation.
func emit(ctx context.Context, out chan<- Message, msg Message) {
	select {
	case out <- msg:
	case <-ctx.Done():
	}
}
