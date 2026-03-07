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
// Priority: phase GateConfig["leader"] > TeamConfig.Leader > alphabetically
// first registered member.
func resolveLeader(phase *Phase, mgr *TeamManager) (string, error) {
	if phase.GateConfig != nil {
		if leader := phase.GateConfig["leader"]; leader != "" {
			return leader, nil
		}
	}
	if mgr.Config.Leader != "" {
		return mgr.Config.Leader, nil
	}
	if name, ok := firstMember(mgr.Members); ok {
		return name, nil
	}
	return "", fmt.Errorf("no leader configured for approval gate")
}

// firstMember returns the name of the alphabetically first member, or false
// if the registry is empty.
func firstMember(reg MemberRegistry) (string, bool) {
	members := reg.List()
	if len(members) == 0 {
		return "", false
	}
	return members[0].Name, true
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

	prevPhase, _, err := mgr.Phases.Current(a.AgentName)
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to get current phase: %s", err), IsError: true}, nil
	}

	// Determine gate type for the current phase.
	var gate GateType
	if prevPhase != nil {
		gate = prevPhase.Gate
	}

	switch gate {
	case "", GateAuto:
		return a.executeAutoGate(mgr, prevPhase, in.Summary)

	case GateLeader:
		return a.executeLeaderGate(ctx, mgr, prevPhase, in.Summary)

	case GateCondition:
		return a.executeConditionGate(ctx, mgr, prevPhase, in.Summary)

	default:
		return tools.Result{
			Output:  fmt.Sprintf("unknown gate type %q; valid types: auto, leader, condition", gate),
			IsError: true,
		}, nil
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

	// Block waiting for approval response. Non-matching messages are buffered
	// and re-sent to the agent after the gate resolves.
	var buffered []AgentMessage
	for {
		select {
		case raw, ok := <-a.PriorityCh:
			if !ok {
				return tools.Result{Output: "priority channel closed while waiting for approval", IsError: true}, nil
			}

			am, isAgentMsg := raw.(AgentMessage)
			if !isAgentMsg {
				log.Printf("warning: non-AgentMessage on priority channel (type=%T), discarding", raw)
				continue
			}

			// Check for force shutdown.
			if am.Kind == MessageKindShutdownRequest && am.Force {
				if a.CancelFunc != nil {
					a.CancelFunc()
				}
				return tools.Result{Output: "force shutdown received while waiting for approval", IsError: true}, nil
			}

			// Check for matching approval response.
			if am.Kind == MessageKindApprovalResponse && am.RequestID == requestID {
				if am.Approve != nil && *am.Approve {
					log.Printf("phase advance [leader]: agent=%s approved by=%s", a.AgentName, leader)
					return a.advanceAndReturn(mgr, prevPhase, buffered)
				}
				content := "approval rejected"
				if am.Content != "" {
					content = am.Content
				}
				return tools.Result{Output: fmt.Sprintf("phase advance rejected by %s: %s", leader, content), IsError: true}, nil
			}

			buffered = append(buffered, am)

		case <-ctx.Done():
			return tools.Result{Output: "context cancelled while waiting for approval", IsError: true}, nil
		}
	}
}

func (a AdvancePhaseTool) executeConditionGate(ctx context.Context, mgr *TeamManager, prevPhase *Phase, _ string) (tools.Result, error) {
	if prevPhase == nil {
		return tools.Result{Output: "condition gate: no current phase", IsError: true}, nil
	}
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

func (a AdvancePhaseTool) advanceAndReturn(mgr *TeamManager, prevPhase *Phase, buffered []AgentMessage) (tools.Result, error) {
	// Re-send buffered messages so the agent does not lose them.
	for _, msg := range buffered {
		if err := mgr.Messages.Send(msg); err != nil {
			log.Printf("warning: failed to re-send buffered message from %s: %v", msg.From, err)
		}
	}

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
