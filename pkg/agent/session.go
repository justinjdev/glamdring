package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/justin/glamdring/pkg/api"
	"github.com/justin/glamdring/pkg/config"
	"github.com/justin/glamdring/pkg/tools"
)

// Session maintains conversation state across multiple turns, giving the agent
// multi-turn memory within a single user session.
type Session struct {
	cfg          Config
	client       *api.Client
	registry     *tools.Registry
	provider     tools.ToolProvider // used for schemas and dispatch
	sessionAllow map[string]bool
	yolo         bool
	messages     []api.RequestMessage
	priorityCh   <-chan any
	regularCh    <-chan any
	teamScope    *config.TeamScope
	TotalInput         int
	TotalOutput        int
	TotalCacheCreation       int
	TotalCacheRead           int
	Turns                    int
	lastRequestInputTokens   int
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

	// Use a custom ToolProvider if configured, otherwise use the registry.
	var provider tools.ToolProvider
	if cfg.ToolProvider != nil {
		provider = cfg.ToolProvider
	} else {
		provider = registry
	}

	s := &Session{
		cfg:          cfg,
		client:       client,
		registry:     registry,
		provider:     provider,
		sessionAllow: make(map[string]bool),
		priorityCh:   cfg.PriorityMessages,
		regularCh:    cfg.RegularMessages,
		teamScope:    cfg.TeamScope,
	}
	if cfg.Yolo {
		s.SetYolo(true)
	}
	return s
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
	s.yolo = false
}

// ToggleYolo flips yolo mode on or off. When enabled, all registered tools
// are added to the session allow list. When disabled, the allow list is cleared.
func (s *Session) ToggleYolo() {
	s.SetYolo(!s.yolo)
}

// SetYolo sets yolo mode to the given state. When on, all registered tools
// are added to the session allow list. When off, the allow list is cleared.
func (s *Session) SetYolo(on bool) {
	s.yolo = on
	if on {
		// Use the base registry (not provider) since provider may filter tools.
		for _, t := range s.registry.All() {
			s.sessionAllow[t.Name()] = true
		}
	} else {
		s.sessionAllow = make(map[string]bool)
	}
}

// IsYolo returns whether yolo mode is active.
func (s *Session) IsYolo() bool {
	return s.yolo
}

// SetYoloScoped adds specific tools to the session allow list without
// enabling full yolo mode.
func (s *Session) SetYoloScoped(toolNames []string) {
	for _, name := range toolNames {
		s.sessionAllow[name] = true
	}
}

// SetModel changes the model for subsequent API requests. Used by team agents
// when advancing workflow phases.
func (s *Session) SetModel(model string) {
	s.client.SetModel(model)
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

		// Drain regular messages and inject as user-role messages before the
		// next API call. This delivers inter-agent messages between turns.
		s.drainRegularMessages()

		req := &api.MessageRequest{
			MaxTokens: 16384,
			Messages:  s.messages,
			System:    s.cfg.SystemPrompt,
			Tools:     s.provider.Schemas(),
		}

		events, err := s.client.Stream(ctx, req)
		if err != nil {
			// If a phase fallback model is available and this is a retryable
			// API error (429/5xx), switch to the fallback and retry once.
			if fallbackModel := s.tryFallbackModel(err); fallbackModel != "" {
				log.Printf("switching to fallback model %s after error: %v", fallbackModel, err)
				s.client.SetModel(fallbackModel)
				events, err = s.client.Stream(ctx, req)
			}
			if err != nil {
				emit(ctx, out, Message{Type: MessageError, Err: fmt.Errorf("api stream: %w", err)})
				return
			}
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
		s.lastRequestInputTokens = turnResult.lastRequestInputTokens

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
				LastRequestInputTokens:   s.lastRequestInputTokens,
			})
			return

		case "tool_use":
			toolResults, err := executeTools(ctx, out, s.provider, turnResult.toolCalls, s.sessionAllow, s.cfg.HookRunner, s.cfg.Permissions, s.teamScope, s.priorityCh)
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

			// After tool execution, sync the model if the phase changed
			// (e.g., AdvancePhase was called).
			s.syncPhaseModel()

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
				LastRequestInputTokens:   s.lastRequestInputTokens,
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

// drainRegularMessages reads all pending messages from the regular channel
// and injects them as a user-role message before the next API call.
func (s *Session) drainRegularMessages() {
	if s.regularCh == nil {
		return
	}
	var msgs []string
	for {
		select {
		case msg, ok := <-s.regularCh:
			if !ok {
				s.regularCh = nil
				goto done
			}
			if text := formatTeamMessage(msg); text != "" {
				msgs = append(msgs, text)
			}
		default:
			goto done
		}
	}
done:
	if len(msgs) > 0 {
		combined := strings.Join(msgs, "\n\n")
		s.messages = append(s.messages, api.RequestMessage{
			Role:    "user",
			Content: combined,
		})
	}
}

// syncPhaseModel checks if the ToolProvider is phase-aware and updates the
// client model to match the current phase. Called after tool execution to
// handle AdvancePhase transitions.
func (s *Session) syncPhaseModel() {
	if pmp, ok := s.provider.(tools.PhaseModelProvider); ok {
		if model, _ := pmp.CurrentPhaseModel(); model != "" && model != s.client.Model() {
			log.Printf("phase model changed to %s", model)
			s.client.SetModel(model)
		}
	}
}

// tryFallbackModel checks if the error is a retryable API error (429/5xx) and
// returns the phase fallback model if available. Returns "" if no fallback applies.
func (s *Session) tryFallbackModel(err error) string {
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		return ""
	}
	if apiErr.StatusCode != 429 && apiErr.StatusCode < 500 {
		return ""
	}
	pmp, ok := s.provider.(tools.PhaseModelProvider)
	if !ok {
		return ""
	}
	_, fallback := pmp.CurrentPhaseModel()
	return fallback
}

// formatTeamMessage converts an opaque team message to a string for injection.
func formatTeamMessage(msg any) string {
	switch v := msg.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(data)
	}
}
