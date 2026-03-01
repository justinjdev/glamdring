package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/justin/glamdring/pkg/tools"
)

// AdvancePhaseTool advances the current workflow phase for an agent.
type AdvancePhaseTool struct {
	Registry   *ManagerRegistry
	AgentName  string
	PriorityCh <-chan any
	CancelFunc context.CancelFunc
}

type advancePhaseInput struct {
	TeamName string `json:"team_name"`
	Summary  string `json:"summary"`
}

func (AdvancePhaseTool) Name() string { return "AdvancePhase" }

func (AdvancePhaseTool) Description() string {
	return "Advance to the next workflow phase. Requires a summary of what was accomplished in the current phase. Gate enforcement may require leader approval or a condition check before advancing."
}

func (AdvancePhaseTool) Schema() json.RawMessage {
	schema := map[string]any{
		"type":     "object",
		"required": []string{"team_name", "summary"},
		"properties": map[string]any{
			"team_name": map[string]any{
				"type":        "string",
				"description": "Name of the team",
			},
			"summary": map[string]any{
				"type":        "string",
				"description": "Summary of what was accomplished in the current phase",
			},
		},
	}
	b, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("BUG: failed to marshal schema: %v", err))
	}
	return json.RawMessage(b)
}

// resolveLeader determines who should approve a leader gate.
// Priority: phase GateConfig["leader"] > TeamConfig.Leader > FirstMember().
func resolveLeader(phase *Phase, mgr *TeamManager) (string, error) {
	if phase.GateConfig != nil {
		if leader := phase.GateConfig["leader"]; leader != "" {
			return leader, nil
		}
	}
	if mgr.Config.Leader != "" {
		return mgr.Config.Leader, nil
	}
	if name, ok := mgr.Members.FirstMember(); ok {
		return name, nil
	}
	return "", fmt.Errorf("no leader configured for approval gate")
}

// maxConditionOutput is the maximum output captured from a condition command.
const maxConditionOutput = 4096

func (a AdvancePhaseTool) Execute(ctx context.Context, input json.RawMessage) (tools.Result, error) {
	var in advancePhaseInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tools.Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}, nil
	}

	if in.Summary == "" {
		return tools.Result{Output: "summary is required", IsError: true}, nil
	}

	mgr, errResult := getManager(a.Registry, in.TeamName)
	if errResult != nil {
		return *errResult, nil
	}

	prevPhase, _, _ := mgr.Phases.Current(a.AgentName)

	// Determine gate type for the current phase.
	gate := ""
	if prevPhase != nil {
		gate = prevPhase.Gate
	}

	switch gate {
	case "", "auto":
		return a.executeAutoGate(mgr, prevPhase, in.Summary)

	case "leader":
		return a.executeLeaderGate(ctx, mgr, prevPhase, in.Summary)

	case "condition":
		return a.executeConditionGate(ctx, mgr, prevPhase, in.Summary)

	default:
		log.Printf("warning: unknown gate type %q, treating as auto", gate)
		return a.executeAutoGate(mgr, prevPhase, in.Summary)
	}
}

func (a AdvancePhaseTool) executeAutoGate(mgr *TeamManager, prevPhase *Phase, summary string) (tools.Result, error) {
	log.Printf("phase advance [auto]: agent=%s summary=%q", a.AgentName, summary)
	return a.advanceAndReturn(mgr, prevPhase, nil)
}

func (a AdvancePhaseTool) executeLeaderGate(ctx context.Context, mgr *TeamManager, prevPhase *Phase, summary string) (tools.Result, error) {
	leader, err := resolveLeader(prevPhase, mgr)
	if err != nil {
		return tools.Result{Output: err.Error(), IsError: true}, nil
	}

	requestID := fmt.Sprintf("phase-%s-%d", a.AgentName, time.Now().UnixNano())

	// Send approval request to the leader.
	msg := AgentMessage{
		Kind:      MessageKindApprovalRequest,
		From:      a.AgentName,
		To:        leader,
		Content:   summary,
		RequestID: requestID,
	}
	if err := mgr.Messages.Send(msg); err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to send approval request: %s", err), IsError: true}, nil
	}

	if a.PriorityCh == nil {
		return tools.Result{Output: "leader gate requires priority channel for approval flow", IsError: true}, nil
	}

	// Block waiting for approval response.
	var buffered []any
	for {
		select {
		case raw, ok := <-a.PriorityCh:
			if !ok {
				return tools.Result{Output: "priority channel closed while waiting for approval", IsError: true}, nil
			}

			// Check for force shutdown.
			if isForceShutdownMsg(raw) {
				if a.CancelFunc != nil {
					a.CancelFunc()
				}
				return tools.Result{Output: "force shutdown received while waiting for approval", IsError: true}, nil
			}

			// Try to parse as approval response.
			resp, ok := parseApprovalResponse(raw, requestID)
			if !ok {
				buffered = append(buffered, raw)
				continue
			}

			if resp.Approve != nil && *resp.Approve {
				log.Printf("phase advance [leader]: agent=%s approved by=%s", a.AgentName, leader)
				return a.advanceAndReturn(mgr, prevPhase, buffered)
			}
			content := "approval rejected"
			if resp.Content != "" {
				content = resp.Content
			}
			return tools.Result{Output: fmt.Sprintf("phase advance rejected by %s: %s", leader, content), IsError: true}, nil

		case <-ctx.Done():
			return tools.Result{Output: "context cancelled while waiting for approval", IsError: true}, nil
		}
	}
}

func (a AdvancePhaseTool) executeConditionGate(ctx context.Context, mgr *TeamManager, prevPhase *Phase, _ string) (tools.Result, error) {
	command := ""
	if prevPhase.GateConfig != nil {
		command = prevPhase.GateConfig["command"]
	}
	if command == "" {
		return tools.Result{Output: "condition gate requires gate_config.command", IsError: true}, nil
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()

	// Truncate output to maxConditionOutput.
	output := buf.Bytes()
	if len(output) > maxConditionOutput {
		output = output[:maxConditionOutput]
	}

	if err != nil {
		return tools.Result{
			Output:  fmt.Sprintf("condition gate failed: %s\n%s", err, string(output)),
			IsError: true,
		}, nil
	}

	log.Printf("phase advance [condition]: agent=%s command=%q", a.AgentName, command)
	return a.advanceAndReturn(mgr, prevPhase, nil)
}

func (a AdvancePhaseTool) advanceAndReturn(mgr *TeamManager, prevPhase *Phase, buffered []any) (tools.Result, error) {
	phase, err := mgr.Phases.Advance(a.AgentName)
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to advance phase: %s", err), IsError: true}, nil
	}

	result := map[string]any{
		"phase_name": phase.Name,
		"tools":      phase.Tools,
		"model":      phase.Model,
	}
	if prevPhase != nil {
		result["previous_phase"] = prevPhase.Name
	}
	if len(buffered) > 0 {
		result["buffered_messages"] = len(buffered)
	}

	out, err := json.Marshal(result)
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to marshal phase result: %s", err), IsError: true}, nil
	}
	return tools.Result{Output: string(out)}, nil
}

// isForceShutdownMsg checks if a priority message is a force shutdown request.
func isForceShutdownMsg(msg any) bool {
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

// parseApprovalResponse checks if a priority message is an approval response
// matching the given request ID. Returns the parsed message and true if matched.
func parseApprovalResponse(msg any, requestID string) (AgentMessage, bool) {
	data, err := json.Marshal(msg)
	if err != nil {
		return AgentMessage{}, false
	}
	var parsed AgentMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		return AgentMessage{}, false
	}
	if parsed.Kind == MessageKindApprovalResponse && parsed.RequestID == requestID {
		return parsed, true
	}
	return AgentMessage{}, false
}
