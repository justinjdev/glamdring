package teams

import (
	"context"
	"encoding/json"
	"testing"
)

func setupMessageTestTeam(t *testing.T) (*ManagerRegistry, *TeamManager) {
	t.Helper()
	reg := NewManagerRegistry()
	dir := t.TempDir()
	mgr, err := reg.Create(TeamConfig{Name: "msg-team"}, dir)
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	return reg, mgr
}

func TestSendMessageTool_DM(t *testing.T) {
	reg, mgr := setupMessageTestTeam(t)

	// Subscribe bob so the message has a destination.
	regular, _, err := mgr.Messages.Subscribe("bob", 10)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	tool := SendMessageTool{Registry: reg, AgentName: "alice"}
	input, _ := json.Marshal(map[string]string{
		"team_name": "msg-team",
		"type":      "message",
		"recipient": "bob",
		"content":   "hello bob",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	// Verify the message arrived.
	select {
	case msg := <-regular:
		if msg.Kind != MessageKindDM {
			t.Errorf("expected DM kind, got %q", msg.Kind)
		}
		if msg.From != "alice" {
			t.Errorf("expected from 'alice', got %q", msg.From)
		}
		if msg.Content != "hello bob" {
			t.Errorf("expected content 'hello bob', got %q", msg.Content)
		}
	default:
		t.Fatal("expected message in bob's mailbox")
	}
}

func TestSendMessageTool_Broadcast(t *testing.T) {
	reg, mgr := setupMessageTestTeam(t)

	// Subscribe alice (sender) and bob (receiver).
	mgr.Messages.Subscribe("alice", 10)
	bobRegular, _, _ := mgr.Messages.Subscribe("bob", 10)

	tool := SendMessageTool{Registry: reg, AgentName: "alice"}
	input, _ := json.Marshal(map[string]string{
		"team_name": "msg-team",
		"type":      "broadcast",
		"content":   "team update",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	// Bob should have the broadcast.
	select {
	case msg := <-bobRegular:
		if msg.Kind != MessageKindBroadcast {
			t.Errorf("expected broadcast kind, got %q", msg.Kind)
		}
		if msg.Content != "team update" {
			t.Errorf("expected content 'team update', got %q", msg.Content)
		}
	default:
		t.Fatal("expected broadcast message for bob")
	}
}

func TestSendMessageTool_ResetsCheckin(t *testing.T) {
	reg, mgr := setupMessageTestTeam(t)

	mgr.Messages.Subscribe("bob", 10)

	// Increment alice's checkin counter.
	mgr.Checkins.Increment("alice")
	mgr.Checkins.Increment("alice")
	mgr.Checkins.Increment("alice")
	if mgr.Checkins.Count("alice") != 3 {
		t.Fatalf("expected checkin count 3, got %d", mgr.Checkins.Count("alice"))
	}

	tool := SendMessageTool{Registry: reg, AgentName: "alice"}
	input, _ := json.Marshal(map[string]string{
		"team_name": "msg-team",
		"type":      "message",
		"recipient": "bob",
		"content":   "hi",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	if mgr.Checkins.Count("alice") != 0 {
		t.Errorf("expected checkin count 0 after send, got %d", mgr.Checkins.Count("alice"))
	}
}

func TestSendMessageTool_DMRequiresRecipient(t *testing.T) {
	reg, _ := setupMessageTestTeam(t)

	tool := SendMessageTool{Registry: reg, AgentName: "alice"}
	input, _ := json.Marshal(map[string]string{
		"team_name": "msg-team",
		"type":      "message",
		"content":   "hello",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing recipient")
	}
}

func TestSendMessageTool_BroadcastRequiresContent(t *testing.T) {
	reg, _ := setupMessageTestTeam(t)

	tool := SendMessageTool{Registry: reg, AgentName: "alice"}
	input, _ := json.Marshal(map[string]string{
		"team_name": "msg-team",
		"type":      "broadcast",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing content")
	}
}

func TestSendMessageTool_InvalidType(t *testing.T) {
	reg, _ := setupMessageTestTeam(t)

	tool := SendMessageTool{Registry: reg, AgentName: "alice"}
	input, _ := json.Marshal(map[string]string{
		"team_name": "msg-team",
		"type":      "invalid",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid message type")
	}
}

func TestSendMessageTool_ShutdownResponse(t *testing.T) {
	reg, mgr := setupMessageTestTeam(t)

	// Subscribe leader to receive the response.
	_, priority, _ := mgr.Messages.Subscribe("leader", 10)

	tool := SendMessageTool{Registry: reg, AgentName: "worker"}
	approve := true
	input, _ := json.Marshal(map[string]any{
		"team_name":  "msg-team",
		"type":       "shutdown_response",
		"recipient":  "leader",
		"request_id": "req-1",
		"approve":    approve,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	select {
	case msg := <-priority:
		if msg.Kind != MessageKindShutdownResponse {
			t.Errorf("expected shutdown_response kind, got %q", msg.Kind)
		}
		if msg.RequestID != "req-1" {
			t.Errorf("expected request_id 'req-1', got %q", msg.RequestID)
		}
		if msg.Approve == nil || !*msg.Approve {
			t.Error("expected approve to be true")
		}
	default:
		t.Fatal("expected shutdown_response in priority channel")
	}
}
