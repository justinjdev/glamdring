package agent

import (
	"context"
	"fmt"

	"github.com/justin/glamdring/pkg/api"
	"github.com/justin/glamdring/pkg/tools"
)

// Session maintains conversation state across multiple turns, giving the agent
// multi-turn memory within a single user session.
type Session struct {
	cfg          Config
	client       *api.Client
	registry     *tools.Registry
	sessionAllow map[string]bool
	messages     []api.RequestMessage
	TotalInput         int
	TotalOutput        int
	TotalCacheCreation int
	TotalCacheRead     int
	Turns              int
}

// NewSession creates a new Session from the given Config.
func NewSession(cfg Config) *Session {
	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}

	client := api.NewClient(cfg.Creds, model)

	registry := tools.NewRegistry()
	for _, t := range cfg.Tools {
		registry.Register(t)
	}

	return &Session{
		cfg:          cfg,
		client:       client,
		registry:     registry,
		sessionAllow: make(map[string]bool),
	}
}

// Turn sends a user message and returns a channel of Messages for the response.
// The user message is appended to the conversation history, preserving context
// from prior turns.
func (s *Session) Turn(ctx context.Context, prompt string) <-chan Message {
	s.messages = append(s.messages, api.RequestMessage{
		Role:    "user",
		Content: prompt,
	})

	out := make(chan Message, 64)
	go func() {
		defer close(out)
		s.runTurn(ctx, out)
	}()
	return out
}

// Messages returns the current conversation history.
func (s *Session) Messages() []api.RequestMessage {
	return s.messages
}

// Reset clears the conversation history and session-level permissions,
// but keeps the client and registry intact.
func (s *Session) Reset() {
	s.messages = nil
	s.sessionAllow = make(map[string]bool)
}

// runTurn executes the agentic loop for one or more API turns until the model
// stops (end_turn, refusal) or hits a limit.
func (s *Session) runTurn(ctx context.Context, out chan<- Message) {
	turnInputBefore := s.TotalInput
	turnOutputBefore := s.TotalOutput
	turnCacheCreationBefore := s.TotalCacheCreation
	turnCacheReadBefore := s.TotalCacheRead

	for {
		if err := ctx.Err(); err != nil {
			emit(ctx, out, Message{Type: MessageError, Err: fmt.Errorf("context cancelled: %w", err)})
			return
		}

		req := &api.MessageRequest{
			MaxTokens: 16384,
			Messages:  s.messages,
			System:    s.cfg.SystemPrompt,
			Tools:     s.registry.Schemas(),
		}

		events, err := s.client.Stream(ctx, req)
		if err != nil {
			emit(ctx, out, Message{Type: MessageError, Err: fmt.Errorf("api stream: %w", err)})
			return
		}

		turnResult, err := processTurn(ctx, events, out)
		if err != nil {
			emit(ctx, out, Message{Type: MessageError, Err: err})
			return
		}

		s.TotalInput += turnResult.inputTokens
		s.TotalOutput += turnResult.outputTokens
		s.TotalCacheCreation += turnResult.cacheCreationTokens
		s.TotalCacheRead += turnResult.cacheReadTokens

		s.messages = append(s.messages, api.RequestMessage{
			Role:    "assistant",
			Content: turnResult.contentBlocks,
		})

		switch turnResult.stopReason {
		case "end_turn", "refusal":
			emit(ctx, out, Message{
				Type:                     MessageDone,
				InputTokens:              s.TotalInput - turnInputBefore,
				OutputTokens:             s.TotalOutput - turnOutputBefore,
				CacheCreationInputTokens: s.TotalCacheCreation - turnCacheCreationBefore,
				CacheReadInputTokens:     s.TotalCacheRead - turnCacheReadBefore,
			})
			return

		case "tool_use":
			toolResults, err := executeTools(ctx, out, s.registry, turnResult.toolCalls, s.sessionAllow, s.cfg.HookRunner)
			if err != nil {
				emit(ctx, out, Message{Type: MessageError, Err: err})
				return
			}

			resultBlocks := make([]api.ContentBlock, len(toolResults))
			for i, r := range toolResults {
				resultBlocks[i] = r
			}
			s.messages = append(s.messages, api.RequestMessage{
				Role:    "user",
				Content: resultBlocks,
			})

		case "max_tokens":
			// Model ran out of output tokens mid-response. Send a continuation
			// prompt so it can finish its thought.
			s.messages = append(s.messages, api.RequestMessage{
				Role:    "user",
				Content: "Continue from where you left off.",
			})

		default:
			// Unknown stop reason -- treat as done.
			emit(ctx, out, Message{
				Type:                     MessageDone,
				InputTokens:              s.TotalInput - turnInputBefore,
				OutputTokens:             s.TotalOutput - turnOutputBefore,
				CacheCreationInputTokens: s.TotalCacheCreation - turnCacheCreationBefore,
				CacheReadInputTokens:     s.TotalCacheRead - turnCacheReadBefore,
			})
			return
		}

		s.Turns++
		if s.cfg.MaxTurns != nil && *s.cfg.MaxTurns > 0 && s.Turns >= *s.cfg.MaxTurns {
			emit(ctx, out, Message{Type: MessageMaxTurnsReached})
			return
		}
	}
}
