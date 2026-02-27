package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/justin/glamdring/pkg/api"
	"github.com/justin/glamdring/pkg/hooks"
	"github.com/justin/glamdring/pkg/tools"
)

// alwaysAllowTools is the set of tool names that never require user permission.
var alwaysAllowTools = map[string]bool{
	"Read": true,
	"Glob": true,
	"Grep": true,
}

// Run starts the agentic loop. It returns a channel of Messages that the
// consumer reads to receive text deltas, tool calls, permission requests,
// errors, and the final done signal. The channel is closed when the loop
// terminates.
func Run(ctx context.Context, cfg Config) <-chan Message {
	out := make(chan Message, 64)

	go func() {
		defer close(out)
		run(ctx, cfg, out)
	}()

	return out
}

func run(ctx context.Context, cfg Config, out chan<- Message) {
	// Create API client.
	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}
	client := api.NewClient(cfg.APIKey, model)

	// Create tool registry and register tools.
	registry := tools.NewRegistry()
	for _, t := range cfg.Tools {
		registry.Register(t)
	}

	// Session-level set of tools that have been always-approved by the user.
	sessionAllow := make(map[string]bool)

	// Build initial conversation messages.
	messages := []api.RequestMessage{
		{Role: "user", Content: cfg.Prompt},
	}

	// Cumulative token usage across turns.
	var totalInput, totalOutput int

	turns := 0
	for {
		// Check context before starting a new turn.
		if err := ctx.Err(); err != nil {
			emit(ctx, out, Message{Type: MessageError, Err: fmt.Errorf("context cancelled: %w", err)})
			return
		}

		// Build the API request.
		req := &api.MessageRequest{
			MaxTokens: 16384,
			Messages:  messages,
			System:    cfg.SystemPrompt,
			Tools:     registry.Schemas(),
		}

		// Stream the response.
		events, err := client.Stream(ctx, req)
		if err != nil {
			emit(ctx, out, Message{Type: MessageError, Err: fmt.Errorf("api stream: %w", err)})
			return
		}

		// Process the stream for this turn.
		turnResult, err := processTurn(ctx, events, out)
		if err != nil {
			emit(ctx, out, Message{Type: MessageError, Err: err})
			return
		}

		// Accumulate token usage.
		totalInput += turnResult.inputTokens
		totalOutput += turnResult.outputTokens

		// Append the assistant response to the conversation.
		messages = append(messages, api.RequestMessage{
			Role:    "assistant",
			Content: turnResult.contentBlocks,
		})

		// If stop reason is end_turn or refusal, we're done.
		if turnResult.stopReason == "end_turn" || turnResult.stopReason == "refusal" {
			emit(ctx, out, Message{
				Type:         MessageDone,
				InputTokens:  totalInput,
				OutputTokens: totalOutput,
			})
			return
		}

		// If stop reason is tool_use, execute the tools.
		if turnResult.stopReason == "tool_use" {
			toolResults, err := executeTools(ctx, out, registry, turnResult.toolCalls, sessionAllow, cfg.HookRunner)
			if err != nil {
				emit(ctx, out, Message{Type: MessageError, Err: err})
				return
			}

			// Append tool results as a user message.
			resultBlocks := make([]api.ContentBlock, len(toolResults))
			for i, r := range toolResults {
				resultBlocks[i] = r
			}
			messages = append(messages, api.RequestMessage{
				Role:    "user",
				Content: resultBlocks,
			})
		}

		// Increment turn counter and check limit.
		turns++
		if cfg.MaxTurns > 0 && turns >= cfg.MaxTurns {
			emit(ctx, out, Message{Type: MessageMaxTurnsReached})
			return
		}
	}
}

// turnResult holds the collected state from processing a single streamed response.
type turnResult struct {
	contentBlocks []api.ContentBlock
	toolCalls     []toolCall
	stopReason    string
	inputTokens   int
	outputTokens  int
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

// executeTools runs each tool call, handling permissions, and returns the
// tool_result content blocks for the next API request.
func executeTools(
	ctx context.Context,
	out chan<- Message,
	registry *tools.Registry,
	calls []toolCall,
	sessionAllow map[string]bool,
	hookRunner *hooks.HookRunner,
) ([]api.ContentBlock, error) {
	results := make([]api.ContentBlock, 0, len(calls))

	for _, tc := range calls {
		// Check context.
		if ctx.Err() != nil {
			return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
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

		// Check permissions.
		if !isAllowed(tc.name, sessionAllow) {
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
				return nil, fmt.Errorf("context cancelled waiting for permission: %w", ctx.Err())
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

		// Execute the tool.
		toolResult, err := registry.Execute(ctx, tc.name, tc.input)
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

		results = append(results, api.ContentBlock{
			Type:      "tool_result",
			ToolUseID: tc.id,
			Content:   toolResult.Output,
			IsError:   toolResult.IsError,
		})
		emit(ctx, out, Message{
			Type:        MessageToolResult,
			ToolName:    tc.name,
			ToolID:      tc.id,
			ToolOutput:  toolResult.Output,
			ToolIsError: toolResult.IsError,
		})

		// Run PostToolUse hooks (failures are warnings, not blocking).
		if hookRunner != nil {
			_ = hookRunner.Run(ctx, hooks.PostToolUse, tc.name, tc.input)
		}
	}

	return results, nil
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
