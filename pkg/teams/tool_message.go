package teams

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/justin/glamdring/pkg/tools"
)

// SendMessageTool sends messages between team agents.
type SendMessageTool struct {
	Registry  *ManagerRegistry
	AgentName string // the sending agent's name
}

type sendMessageInput struct {
	TeamName  string `json:"team_name"`
	Type      string `json:"type"`
	Recipient string `json:"recipient"`
	Content   string `json:"content"`
	RequestID string `json:"request_id"`
	Approve   *bool  `json:"approve"`
	Force     *bool  `json:"force"`
}

func (SendMessageTool) Name() string { return "SendMessage" }

func (SendMessageTool) Description() string {
	return "Send a message to another team agent or broadcast to all team members."
}

func (SendMessageTool) Schema() json.RawMessage {
	schema := map[string]any{
		"type":     "object",
		"required": []string{"team_name", "type"},
		"properties": map[string]any{
			"team_name": map[string]any{
				"type":        "string",
				"description": "Name of the team",
			},
			"type": map[string]any{
				"type":        "string",
				"description": "Message type: message, broadcast, shutdown_request, shutdown_response, approval_response",
				"enum":        []string{"message", "broadcast", "shutdown_request", "shutdown_response", "approval_response"},
			},
			"recipient": map[string]any{
				"type":        "string",
				"description": "Target agent name (required for message, shutdown_request, approval_response)",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Message content (required for message, broadcast)",
			},
			"request_id": map[string]any{
				"type":        "string",
				"description": "ID of the request being responded to (for shutdown_response, approval_response)",
			},
			"approve": map[string]any{
				"type":        "boolean",
				"description": "Whether to approve or deny (for shutdown_response, approval_response)",
			},
			"force": map[string]any{
				"type":        "boolean",
				"description": "Force shutdown (kills agent without waiting for approval)",
			},
		},
	}
	b, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("BUG: failed to marshal schema: %v", err))
	}
	return json.RawMessage(b)
}

func (s SendMessageTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in sendMessageInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tools.Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}, nil
	}

	mgr, errResult := getManager(s.Registry, in.TeamName)
	if errResult != nil {
		return *errResult, nil
	}

	var msg AgentMessage
	msg.From = s.AgentName

	switch in.Type {
	case "message":
		if in.Recipient == "" {
			return tools.Result{Output: "recipient is required for message type", IsError: true}, nil
		}
		if in.Content == "" {
			return tools.Result{Output: "content is required for message type", IsError: true}, nil
		}
		msg.Kind = MessageKindDM
		msg.To = in.Recipient
		msg.Content = in.Content

	case "broadcast":
		if in.Content == "" {
			return tools.Result{Output: "content is required for broadcast type", IsError: true}, nil
		}
		msg.Kind = MessageKindBroadcast
		msg.Content = in.Content

	case "shutdown_request":
		if in.Recipient == "" {
			return tools.Result{Output: "recipient is required for shutdown_request type", IsError: true}, nil
		}
		msg.Kind = MessageKindShutdownRequest
		msg.To = in.Recipient
		msg.Content = in.Content
		msg.Force = in.Force != nil && *in.Force

	case "shutdown_response":
		if in.RequestID == "" {
			return tools.Result{Output: "request_id is required for shutdown_response type", IsError: true}, nil
		}
		if in.Approve == nil {
			return tools.Result{Output: "approve is required for shutdown_response type", IsError: true}, nil
		}
		msg.Kind = MessageKindShutdownResponse
		msg.RequestID = in.RequestID
		msg.Approve = in.Approve
		msg.To = in.Recipient

	case "approval_response":
		if in.RequestID == "" {
			return tools.Result{Output: "request_id is required for approval_response type", IsError: true}, nil
		}
		if in.Recipient == "" {
			return tools.Result{Output: "recipient is required for approval_response type", IsError: true}, nil
		}
		if in.Approve == nil {
			return tools.Result{Output: "approve is required for approval_response type", IsError: true}, nil
		}
		msg.Kind = MessageKindApprovalResponse
		msg.To = in.Recipient
		msg.RequestID = in.RequestID
		msg.Approve = in.Approve

	default:
		return tools.Result{
			Output:  fmt.Sprintf("invalid message type %q; must be one of: message, broadcast, shutdown_request, shutdown_response, approval_response", in.Type),
			IsError: true,
		}, nil
	}

	if err := mgr.Messages.Send(msg); err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to send message: %s", err), IsError: true}, nil
	}

	mgr.Checkins.Reset(s.AgentName)

	out, err := json.Marshal(map[string]string{
		"message": fmt.Sprintf("%s message sent", in.Type),
	})
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to marshal result: %s", err), IsError: true}, nil
	}
	return tools.Result{Output: string(out)}, nil
}
