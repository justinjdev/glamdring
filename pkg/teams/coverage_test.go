package teams

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// types.go -- Valid() false branches, Validate() untested branches
// ---------------------------------------------------------------------------

func TestMemberStatus_Valid_InvalidStatus(t *testing.T) {
	s := MemberStatus("bogus")
	if s.Valid() {
		t.Error("expected invalid status to return false")
	}
}

func TestTaskStatus_Valid_InvalidStatus(t *testing.T) {
	s := TaskStatus("bogus")
	if s.Valid() {
		t.Error("expected invalid status to return false")
	}
}

func TestMessageKind_Valid_InvalidKind(t *testing.T) {
	k := MessageKind("bogus")
	if k.Valid() {
		t.Error("expected invalid kind to return false")
	}
}

func TestAgentMessage_Validate_InvalidKind(t *testing.T) {
	msg := AgentMessage{Kind: "bogus", From: "a"}
	err := msg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid kind")
	}
	if !strings.Contains(err.Error(), "invalid message kind") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentMessage_Validate_EmptyFrom(t *testing.T) {
	msg := AgentMessage{Kind: MessageKindDM, From: ""}
	err := msg.Validate()
	if err == nil {
		t.Fatal("expected error for empty from")
	}
	if !strings.Contains(err.Error(), "from is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentMessage_Validate_DMEmptyTo(t *testing.T) {
	msg := AgentMessage{Kind: MessageKindDM, From: "alice", Content: "hi"}
	err := msg.Validate()
	if err == nil {
		t.Fatal("expected error for DM with empty To")
	}
	if !strings.Contains(err.Error(), "dm requires To") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentMessage_Validate_DMEmptyContent(t *testing.T) {
	msg := AgentMessage{Kind: MessageKindDM, From: "alice", To: "bob"}
	err := msg.Validate()
	if err == nil {
		t.Fatal("expected error for DM with empty Content")
	}
	if !strings.Contains(err.Error(), "dm requires Content") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentMessage_Validate_DMValid(t *testing.T) {
	msg := AgentMessage{Kind: MessageKindDM, From: "alice", To: "bob", Content: "hello"}
	if err := msg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgentMessage_Validate_BroadcastWithTo(t *testing.T) {
	msg := AgentMessage{Kind: MessageKindBroadcast, From: "alice", To: "bob"}
	err := msg.Validate()
	if err == nil {
		t.Fatal("expected error for broadcast with To set")
	}
	if !strings.Contains(err.Error(), "broadcast must not set To") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentMessage_Validate_BroadcastValid(t *testing.T) {
	msg := AgentMessage{Kind: MessageKindBroadcast, From: "alice", Content: "hi"}
	if err := msg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgentMessage_Validate_ShutdownRequestEmptyTo(t *testing.T) {
	msg := AgentMessage{Kind: MessageKindShutdownRequest, From: "alice"}
	err := msg.Validate()
	if err == nil {
		t.Fatal("expected error for shutdown_request with empty To")
	}
	if !strings.Contains(err.Error(), "shutdown_request requires To") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentMessage_Validate_ShutdownResponseEmptyRequestID(t *testing.T) {
	approve := true
	msg := AgentMessage{Kind: MessageKindShutdownResponse, From: "alice", Approve: &approve}
	err := msg.Validate()
	if err == nil {
		t.Fatal("expected error for shutdown_response with empty RequestID")
	}
	if !strings.Contains(err.Error(), "shutdown_response requires RequestID") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentMessage_Validate_ShutdownResponseNilApprove(t *testing.T) {
	msg := AgentMessage{Kind: MessageKindShutdownResponse, From: "alice", RequestID: "r1"}
	err := msg.Validate()
	if err == nil {
		t.Fatal("expected error for shutdown_response with nil Approve")
	}
	if !strings.Contains(err.Error(), "shutdown_response requires Approve") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentMessage_Validate_ShutdownResponseValid(t *testing.T) {
	approve := true
	msg := AgentMessage{Kind: MessageKindShutdownResponse, From: "alice", RequestID: "r1", Approve: &approve}
	if err := msg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgentMessage_Validate_ApprovalRequestEmptyTo(t *testing.T) {
	msg := AgentMessage{Kind: MessageKindApprovalRequest, From: "alice"}
	err := msg.Validate()
	if err == nil {
		t.Fatal("expected error for approval_request with empty To")
	}
	if !strings.Contains(err.Error(), "approval_request requires To") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentMessage_Validate_ApprovalResponseEmptyTo(t *testing.T) {
	approve := true
	msg := AgentMessage{Kind: MessageKindApprovalResponse, From: "alice", RequestID: "r1", Approve: &approve}
	err := msg.Validate()
	if err == nil {
		t.Fatal("expected error for approval_response with empty To")
	}
	if !strings.Contains(err.Error(), "approval_response requires To") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentMessage_Validate_ApprovalResponseEmptyRequestID(t *testing.T) {
	approve := true
	msg := AgentMessage{Kind: MessageKindApprovalResponse, From: "alice", To: "bob", Approve: &approve}
	err := msg.Validate()
	if err == nil {
		t.Fatal("expected error for approval_response with empty RequestID")
	}
	if !strings.Contains(err.Error(), "approval_response requires RequestID") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentMessage_Validate_ApprovalResponseNilApprove(t *testing.T) {
	msg := AgentMessage{Kind: MessageKindApprovalResponse, From: "alice", To: "bob", RequestID: "r1"}
	err := msg.Validate()
	if err == nil {
		t.Fatal("expected error for approval_response with nil Approve")
	}
	if !strings.Contains(err.Error(), "approval_response requires Approve") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentMessage_Validate_ApprovalResponseValid(t *testing.T) {
	approve := false
	msg := AgentMessage{Kind: MessageKindApprovalResponse, From: "alice", To: "bob", RequestID: "r1", Approve: &approve}
	if err := msg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// members.go -- Add with empty name, Add with invalid status, SetStatus invalid
// ---------------------------------------------------------------------------

func TestMemberRegistry_AddEmptyName(t *testing.T) {
	r := NewInMemoryMemberRegistry()
	err := r.Add(Member{Name: "", Status: MemberStatusIdle})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMemberRegistry_AddInvalidStatus(t *testing.T) {
	r := NewInMemoryMemberRegistry()
	err := r.Add(Member{Name: "alice", Status: "bogus"})
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
	if !strings.Contains(err.Error(), "invalid member status") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMemberRegistry_SetStatusInvalid(t *testing.T) {
	r := NewInMemoryMemberRegistry()
	r.Add(Member{Name: "alice", Status: MemberStatusIdle})
	err := r.SetStatus("alice", "bogus")
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
	if !strings.Contains(err.Error(), "invalid member status") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// manager.go -- NewTeamManager failure path (bad task dir)
// ---------------------------------------------------------------------------

func TestNewTeamManager_FailsWithBadDir(t *testing.T) {
	// Use a path that cannot be created (file as parent directory).
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "blocker")
	os.WriteFile(filePath, []byte("x"), 0644)

	_, err := NewTeamManager(TeamConfig{Name: "fail"}, filepath.Join(filePath, "tasks"))
	if err == nil {
		t.Fatal("expected error when task dir creation fails")
	}
	if !strings.Contains(err.Error(), "create task storage") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// tasks.go -- edge cases
// ---------------------------------------------------------------------------

func TestFileTaskStorage_UpdateScope(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)
	task, _ := s.Create(Task{Subject: "scoped", Status: TaskStatusPending})

	scope := &TaskScope{AllowPatterns: []string{"/src/**"}, DenyPatterns: []string{"/secret/*"}}
	updated, err := s.Update(task.ID, TaskUpdate{Scope: scope})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Scope == nil {
		t.Fatal("expected scope to be set")
	}
	if len(updated.Scope.AllowPatterns) != 1 || updated.Scope.AllowPatterns[0] != "/src/**" {
		t.Errorf("unexpected allow patterns: %v", updated.Scope.AllowPatterns)
	}
}

func TestFileTaskStorage_UpdateDescription(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)
	task, _ := s.Create(Task{Subject: "desc", Status: TaskStatusPending})

	desc := "new description"
	updated, err := s.Update(task.ID, TaskUpdate{Description: &desc})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Description != "new description" {
		t.Errorf("expected description 'new description', got %q", updated.Description)
	}
}

func TestFileTaskStorage_AppendUniqueNoDuplicates(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)
	task, _ := s.Create(Task{Subject: "dedup", Status: TaskStatusPending, Blocks: []string{"1", "2"}})

	// Add duplicates and new items.
	_, err := s.Update(task.ID, TaskUpdate{AddBlocks: []string{"2", "3"}})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := s.Get(task.ID)
	if len(got.Blocks) != 3 {
		t.Errorf("expected 3 blocks (deduped), got %d: %v", len(got.Blocks), got.Blocks)
	}
}

func TestFileTaskStorage_ListSortsByID(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	s.Create(Task{Subject: "third", Status: TaskStatusPending})
	s.Create(Task{Subject: "first", Status: TaskStatusPending})

	list := s.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(list))
	}
	// IDs should be "1" and "2", sorted numerically.
	if list[0].ID != "1" || list[1].ID != "2" {
		t.Errorf("expected IDs [1, 2], got [%s, %s]", list[0].ID, list[1].ID)
	}
}

func TestFileTaskStorage_ListWithNonJSONFiles(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)
	s.Create(Task{Subject: "real", Status: TaskStatusPending})

	// Write a non-JSON file.
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0644)

	list := s.List()
	if len(list) != 1 {
		t.Errorf("expected 1 task (ignoring .txt), got %d", len(list))
	}
}

func TestFileTaskStorage_ListWithCorruptFile(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)
	s.Create(Task{Subject: "good", Status: TaskStatusPending})

	// Write a corrupt JSON file.
	os.WriteFile(filepath.Join(dir, "99.json"), []byte("not valid json{"), 0644)

	list := s.List()
	// Should skip the corrupt file and still return the good task.
	if len(list) != 1 {
		t.Errorf("expected 1 task (skipping corrupt), got %d", len(list))
	}
}

func TestFileTaskStorage_NewFileTaskStorage_WithExistingNonNumericJSON(t *testing.T) {
	dir := t.TempDir()
	// Write a non-numeric JSON file name.
	os.WriteFile(filepath.Join(dir, "notes.json"), []byte(`{}`), 0644)

	s, err := NewFileTaskStorage(dir)
	if err != nil {
		t.Fatalf("NewFileTaskStorage: %v", err)
	}
	// nextN should still be 1, since "notes" is not numeric.
	task, err := s.Create(Task{Subject: "test", Status: TaskStatusPending})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task.ID != "1" {
		t.Errorf("expected ID 1, got %s", task.ID)
	}
}

func TestFileTaskStorage_CompletionClearsBlockedBy_WithNonJSONFiles(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	t1, _ := s.Create(Task{Subject: "blocker", Status: TaskStatusPending})
	s.Create(Task{Subject: "blocked", Status: TaskStatusPending, BlockedBy: []string{t1.ID}})

	// Write a non-JSON file to ensure clearBlockedByLocked skips it.
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0644)

	inProgress := TaskStatusInProgress
	s.Update(t1.ID, TaskUpdate{Status: &inProgress})
	completed := TaskStatusCompleted
	_, err := s.Update(t1.ID, TaskUpdate{Status: &completed})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
}

// ---------------------------------------------------------------------------
// mailbox.go -- broadcast priority drop, priority DM drop
// ---------------------------------------------------------------------------

func TestChannelTransport_BroadcastDropsWhenFull(t *testing.T) {
	tr := NewChannelTransport()
	tr.Subscribe("alice", 10)
	tr.Subscribe("bob", 1) // buffer size 1

	// Fill bob's regular channel.
	tr.Send(AgentMessage{Kind: MessageKindDM, From: "charlie", To: "bob", Content: "fill"})

	// Broadcast should drop for bob (full) but not error for non-priority.
	err := tr.Send(AgentMessage{Kind: MessageKindBroadcast, From: "alice", Content: "hello all"})
	// Non-priority broadcast drops are logged, not errored.
	if err != nil {
		t.Errorf("expected no error for dropped non-priority broadcast, got: %v", err)
	}
}

func TestChannelTransport_BroadcastPriorityDropError(t *testing.T) {
	tr := NewChannelTransport()
	tr.Subscribe("alice", 10)
	tr.Subscribe("bob", 1) // buffer size 1 for priority

	// Fill bob's priority channel with a direct priority message.
	tr.Send(AgentMessage{Kind: MessageKindApprovalRequest, From: "charlie", To: "bob", Content: "approve?"})

	// Send an ApprovalRequest as broadcast-like by sending to each agent.
	// Since ApprovalRequest requires To, we test the priority DM drop path instead
	// by sending another priority message.
	err := tr.Send(AgentMessage{Kind: MessageKindApprovalRequest, From: "alice", To: "bob", Content: "approve?"})
	if err == nil {
		t.Fatal("expected error for dropped priority DM")
	}
}

func TestChannelTransport_PriorityDMDropError(t *testing.T) {
	tr := NewChannelTransport()
	tr.Subscribe("bob", 1) // buffer size 1

	// Fill bob's priority channel.
	tr.Send(AgentMessage{Kind: MessageKindShutdownRequest, From: "alice", To: "bob", Content: "stop1"})

	// Second priority DM should be dropped.
	err := tr.Send(AgentMessage{Kind: MessageKindShutdownRequest, From: "alice", To: "bob", Content: "stop2"})
	if err == nil {
		t.Fatal("expected error for dropped priority DM")
	}
	if !strings.Contains(err.Error(), "dropped") {
		t.Errorf("expected 'dropped' in error, got: %v", err)
	}
}

func TestChannelTransport_SendInvalidMessage(t *testing.T) {
	tr := NewChannelTransport()
	// Invalid message (empty kind).
	err := tr.Send(AgentMessage{Kind: "bogus", From: "alice", To: "bob", Content: "hi"})
	if err == nil {
		t.Fatal("expected error for invalid message")
	}
	if !strings.Contains(err.Error(), "invalid message") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// phases.go -- Current with empty phases list
// ---------------------------------------------------------------------------

func TestPhaseTracker_CurrentEmptyPhases(t *testing.T) {
	pt := NewInMemoryPhaseTracker()
	pt.SetPhases("alice", []Phase{})

	_, _, err := pt.Current("alice")
	if err == nil {
		t.Fatal("expected error for empty phases list")
	}
	if !strings.Contains(err.Error(), "has no phases") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// decorators.go -- Name/Description/Schema coverage, invalid JSON, ScopedBash fallback
// ---------------------------------------------------------------------------

func TestScopedTool_Schema(t *testing.T) {
	inner := &stubTool{name: "Write"}
	scoped := NewScopedTool(inner, nil, nil)
	schema := scoped.Schema()
	if string(schema) != "{}" {
		t.Errorf("expected {}, got %s", schema)
	}
}

func TestScopedBash_NameDescriptionSchema(t *testing.T) {
	inner := &stubTool{name: "Bash"}
	scoped := NewScopedBash(inner, nil)

	if scoped.Name() != "Bash" {
		t.Errorf("expected name 'Bash', got %q", scoped.Name())
	}
	if scoped.Description() != "stub tool" {
		t.Errorf("expected description 'stub tool', got %q", scoped.Description())
	}
	schema := scoped.Schema()
	if string(schema) != "{}" {
		t.Errorf("expected {}, got %s", schema)
	}
}

func TestFileLockDecorator_NameDescriptionSchema(t *testing.T) {
	inner := &stubTool{name: "Write"}
	locks := newMockLockManager()
	dec := NewFileLockDecorator(inner, locks, "agent-a")

	if dec.Name() != "Write" {
		t.Errorf("expected name 'Write', got %q", dec.Name())
	}
	if dec.Description() != "stub tool" {
		t.Errorf("expected description 'stub tool', got %q", dec.Description())
	}
	schema := dec.Schema()
	if string(schema) != "{}" {
		t.Errorf("expected {}, got %s", schema)
	}
}

func TestCheckinGateDecorator_NameDescriptionSchema(t *testing.T) {
	inner := &stubTool{name: "Write"}
	tracker := newMockCheckinTracker()
	dec := NewCheckinGateDecorator(inner, tracker, "agent-a", 5)

	if dec.Name() != "Write" {
		t.Errorf("expected name 'Write', got %q", dec.Name())
	}
	if dec.Description() != "stub tool" {
		t.Errorf("expected description 'stub tool', got %q", dec.Description())
	}
	schema := dec.Schema()
	if string(schema) != "{}" {
		t.Errorf("expected {}, got %s", schema)
	}
}

func TestScopedTool_CheckPath_InvalidJSON(t *testing.T) {
	inner := &stubTool{name: "Write"}
	scoped := NewScopedTool(inner, []string{"/project/**"}, nil)

	input := json.RawMessage(`{invalid json}`)
	result, err := scoped.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for invalid JSON input")
	}
	if !strings.Contains(result.Output, "invalid input") {
		t.Errorf("unexpected error message: %s", result.Output)
	}
}

func TestScopedBash_CheckCommand_InvalidJSON(t *testing.T) {
	inner := &stubTool{name: "Bash"}
	scoped := NewScopedBash(inner, []string{"go "})

	input := json.RawMessage(`{invalid json}`)
	result, err := scoped.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for invalid JSON input")
	}
	if !strings.Contains(result.Output, "invalid input") {
		t.Errorf("unexpected error message: %s", result.Output)
	}
}

func TestScopedBash_ShellMetacharRejection(t *testing.T) {
	inner := &stubTool{name: "Bash"}
	scoped := NewScopedBash(inner, []string{"go "})

	metachars := []string{
		`go test; rm -rf /`,
		`go test && rm -rf /`,
		`go test || rm -rf /`,
		`go test | rm`,
		"go test `rm`",
		`go test $(rm)`,
		"go test\nrm",
	}

	for _, cmd := range metachars {
		input, _ := json.Marshal(map[string]string{"command": cmd})
		result, err := scoped.Execute(context.Background(), json.RawMessage(input))
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", cmd, err)
		}
		if !result.IsError {
			t.Errorf("expected error for command with metachar: %q", cmd)
		}
		if !strings.Contains(result.Output, "disallowed shell metacharacter") {
			t.Errorf("unexpected error message for %q: %s", cmd, result.Output)
		}
	}
}

func TestFileLockDecorator_CheckFileLock_InvalidJSON(t *testing.T) {
	locks := newMockLockManager()
	inner := &stubTool{name: "Write"}
	dec := NewFileLockDecorator(inner, locks, "agent-a")

	input := json.RawMessage(`{invalid json}`)
	result, err := dec.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for invalid JSON input")
	}
	if !strings.Contains(result.Output, "invalid input") {
		t.Errorf("unexpected error message: %s", result.Output)
	}
}

func TestFileLockDecorator_NoFilePath(t *testing.T) {
	locks := newMockLockManager()
	inner := &stubTool{name: "Write"}
	dec := NewFileLockDecorator(inner, locks, "agent-a")

	// Input without file_path should pass through without locking.
	input := json.RawMessage(`{"content": "hello"}`)
	result, err := dec.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success for input without file_path, got error: %s", result.Output)
	}
}

func TestScopedBash_StreamingFallbackToNonStreaming(t *testing.T) {
	inner := &stubTool{name: "Bash"} // does not implement StreamingTool
	scoped := NewScopedBash(inner, nil)

	input := json.RawMessage(`{"command": "echo hello"}`)
	result, err := scoped.ExecuteStreaming(context.Background(), input, func(s string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success, got error: %s", result.Output)
	}
}

// ---------------------------------------------------------------------------
// tool_message.go -- missing coverage branches
// ---------------------------------------------------------------------------

func TestSendMessageTool_InvalidJSON(t *testing.T) {
	reg := NewManagerRegistry()
	tool := SendMessageTool{Registry: reg, AgentName: "alice"}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{invalid}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(result.Output, "invalid input") {
		t.Errorf("unexpected error message: %s", result.Output)
	}
}

func TestSendMessageTool_EmptyTeamName(t *testing.T) {
	reg := NewManagerRegistry()
	tool := SendMessageTool{Registry: reg, AgentName: "alice"}

	input, _ := json.Marshal(map[string]string{"type": "message"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for empty team_name")
	}
}

func TestSendMessageTool_TeamNotFound(t *testing.T) {
	reg := NewManagerRegistry()
	tool := SendMessageTool{Registry: reg, AgentName: "alice"}

	input, _ := json.Marshal(map[string]string{"team_name": "nope", "type": "message"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for team not found")
	}
}

func TestSendMessageTool_DMEmptyContent(t *testing.T) {
	reg, _ := setupMessageTestTeam(t)
	tool := SendMessageTool{Registry: reg, AgentName: "alice"}

	input, _ := json.Marshal(map[string]string{
		"team_name": "msg-team",
		"type":      "message",
		"recipient": "bob",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for DM with empty content")
	}
	if !strings.Contains(result.Output, "content is required") {
		t.Errorf("unexpected error message: %s", result.Output)
	}
}

func TestSendMessageTool_ShutdownRequest(t *testing.T) {
	reg, mgr := setupMessageTestTeam(t)
	_, pri, _ := mgr.Messages.Subscribe("bob", 10)

	tool := SendMessageTool{Registry: reg, AgentName: "alice"}
	input, _ := json.Marshal(map[string]string{
		"team_name": "msg-team",
		"type":      "shutdown_request",
		"recipient": "bob",
		"content":   "time to stop",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	select {
	case msg := <-pri:
		if msg.Kind != MessageKindShutdownRequest {
			t.Errorf("expected shutdown_request, got %q", msg.Kind)
		}
	default:
		t.Fatal("expected message in priority channel")
	}
}

func TestSendMessageTool_ShutdownRequestMissingRecipient(t *testing.T) {
	reg, _ := setupMessageTestTeam(t)
	tool := SendMessageTool{Registry: reg, AgentName: "alice"}

	input, _ := json.Marshal(map[string]string{
		"team_name": "msg-team",
		"type":      "shutdown_request",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing recipient")
	}
}

func TestSendMessageTool_ShutdownResponseMissingRequestID(t *testing.T) {
	reg, _ := setupMessageTestTeam(t)
	tool := SendMessageTool{Registry: reg, AgentName: "alice"}

	approve := true
	input, _ := json.Marshal(map[string]any{
		"team_name": "msg-team",
		"type":      "shutdown_response",
		"approve":   approve,
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing request_id")
	}
}

func TestSendMessageTool_ShutdownResponseMissingApprove(t *testing.T) {
	reg, _ := setupMessageTestTeam(t)
	tool := SendMessageTool{Registry: reg, AgentName: "alice"}

	input, _ := json.Marshal(map[string]any{
		"team_name":  "msg-team",
		"type":       "shutdown_response",
		"request_id": "r1",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing approve")
	}
}

func TestSendMessageTool_ApprovalResponse(t *testing.T) {
	reg, mgr := setupMessageTestTeam(t)
	_, pri, _ := mgr.Messages.Subscribe("leader", 10)

	tool := SendMessageTool{Registry: reg, AgentName: "worker"}
	approve := false
	input, _ := json.Marshal(map[string]any{
		"team_name":  "msg-team",
		"type":       "approval_response",
		"recipient":  "leader",
		"request_id": "req-42",
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
	case msg := <-pri:
		if msg.Kind != MessageKindApprovalResponse {
			t.Errorf("expected approval_response, got %q", msg.Kind)
		}
		if msg.RequestID != "req-42" {
			t.Errorf("expected request_id 'req-42', got %q", msg.RequestID)
		}
		if msg.Approve == nil || *msg.Approve {
			t.Error("expected approve to be false")
		}
	default:
		t.Fatal("expected message in priority channel")
	}
}

func TestSendMessageTool_ApprovalResponseMissingRequestID(t *testing.T) {
	reg, _ := setupMessageTestTeam(t)
	tool := SendMessageTool{Registry: reg, AgentName: "alice"}

	approve := true
	input, _ := json.Marshal(map[string]any{
		"team_name": "msg-team",
		"type":      "approval_response",
		"recipient": "bob",
		"approve":   approve,
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing request_id")
	}
}

func TestSendMessageTool_ApprovalResponseMissingRecipient(t *testing.T) {
	reg, _ := setupMessageTestTeam(t)
	tool := SendMessageTool{Registry: reg, AgentName: "alice"}

	approve := true
	input, _ := json.Marshal(map[string]any{
		"team_name":  "msg-team",
		"type":       "approval_response",
		"request_id": "r1",
		"approve":    approve,
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing recipient")
	}
}

func TestSendMessageTool_ApprovalResponseMissingApprove(t *testing.T) {
	reg, _ := setupMessageTestTeam(t)
	tool := SendMessageTool{Registry: reg, AgentName: "alice"}

	input, _ := json.Marshal(map[string]any{
		"team_name":  "msg-team",
		"type":       "approval_response",
		"recipient":  "bob",
		"request_id": "r1",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing approve")
	}
}

func TestSendMessageTool_SendFails(t *testing.T) {
	reg, _ := setupMessageTestTeam(t)
	// Do NOT subscribe bob so the send fails.
	tool := SendMessageTool{Registry: reg, AgentName: "alice"}

	input, _ := json.Marshal(map[string]string{
		"team_name": "msg-team",
		"type":      "message",
		"recipient": "bob",
		"content":   "hello",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error when recipient is not subscribed")
	}
	if !strings.Contains(result.Output, "failed to send") {
		t.Errorf("unexpected error message: %s", result.Output)
	}
}

// ---------------------------------------------------------------------------
// tool_tasks.go -- missing coverage branches
// ---------------------------------------------------------------------------

func TestTaskCreateTool_InvalidJSON(t *testing.T) {
	reg := setupTaskTestTeam(t)
	tool := TaskCreateTool{Registry: reg}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{invalid}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTaskCreateTool_EmptyTeamName(t *testing.T) {
	reg := setupTaskTestTeam(t)
	tool := TaskCreateTool{Registry: reg}

	input, _ := json.Marshal(map[string]string{"subject": "test"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing team_name")
	}
}

func TestTaskListTool_InvalidJSON(t *testing.T) {
	reg := setupTaskTestTeam(t)
	tool := TaskListTool{Registry: reg}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{invalid}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTaskListTool_EmptyTeamName(t *testing.T) {
	reg := setupTaskTestTeam(t)
	tool := TaskListTool{Registry: reg}

	input, _ := json.Marshal(map[string]string{})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing team_name")
	}
}

func TestTaskListTool_TeamNotFound(t *testing.T) {
	reg := NewManagerRegistry()
	tool := TaskListTool{Registry: reg}

	input, _ := json.Marshal(map[string]string{"team_name": "nope"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for team not found")
	}
}

func TestTaskGetTool_InvalidJSON(t *testing.T) {
	reg := setupTaskTestTeam(t)
	tool := TaskGetTool{Registry: reg}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{invalid}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTaskGetTool_EmptyTeamName(t *testing.T) {
	reg := setupTaskTestTeam(t)
	tool := TaskGetTool{Registry: reg}

	input, _ := json.Marshal(map[string]string{"task_id": "1"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing team_name")
	}
}

func TestTaskGetTool_TeamNotFound(t *testing.T) {
	reg := NewManagerRegistry()
	tool := TaskGetTool{Registry: reg}

	input, _ := json.Marshal(map[string]string{"team_name": "nope", "task_id": "1"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for team not found")
	}
}

func TestTaskGetTool_EmptyTaskID(t *testing.T) {
	reg := setupTaskTestTeam(t)
	tool := TaskGetTool{Registry: reg}

	input, _ := json.Marshal(map[string]string{"team_name": "proj"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for empty task_id")
	}
}

func TestTaskUpdateTool_InvalidJSON(t *testing.T) {
	reg := setupTaskTestTeam(t)
	tool := TaskUpdateTool{Registry: reg, AgentName: "alice"}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{invalid}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTaskUpdateTool_EmptyTeamName(t *testing.T) {
	reg := setupTaskTestTeam(t)
	tool := TaskUpdateTool{Registry: reg, AgentName: "alice"}

	input, _ := json.Marshal(map[string]string{"task_id": "1"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing team_name")
	}
}

func TestTaskUpdateTool_TeamNotFound(t *testing.T) {
	reg := NewManagerRegistry()
	tool := TaskUpdateTool{Registry: reg, AgentName: "alice"}

	input, _ := json.Marshal(map[string]string{"team_name": "nope", "task_id": "1"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for team not found")
	}
}

func TestTaskUpdateTool_EmptyTaskID(t *testing.T) {
	reg := setupTaskTestTeam(t)
	tool := TaskUpdateTool{Registry: reg, AgentName: "alice"}

	input, _ := json.Marshal(map[string]string{"team_name": "proj"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for empty task_id")
	}
}

func TestTaskUpdateTool_InvalidStatus(t *testing.T) {
	reg := setupTaskTestTeam(t)
	createTool := TaskCreateTool{Registry: reg}
	updateTool := TaskUpdateTool{Registry: reg, AgentName: "alice"}

	createInput, _ := json.Marshal(map[string]string{"team_name": "proj", "subject": "test"})
	createResult, _ := createTool.Execute(context.Background(), createInput)
	var created map[string]string
	json.Unmarshal([]byte(createResult.Output), &created)

	updateInput, _ := json.Marshal(map[string]any{
		"team_name": "proj",
		"task_id":   created["task_id"],
		"status":    "bogus",
	})
	result, err := updateTool.Execute(context.Background(), updateInput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid status")
	}
	if !strings.Contains(result.Output, "invalid status") {
		t.Errorf("unexpected error message: %s", result.Output)
	}
}

func TestTaskUpdateTool_TaskNotFound(t *testing.T) {
	reg := setupTaskTestTeam(t)
	tool := TaskUpdateTool{Registry: reg, AgentName: "alice"}

	input, _ := json.Marshal(map[string]any{
		"team_name": "proj",
		"task_id":   "999",
		"status":    "in_progress",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for non-existent task")
	}
}

func TestTaskUpdateTool_NoAgentName_SkipsCheckinReset(t *testing.T) {
	reg := setupTaskTestTeam(t)
	createTool := TaskCreateTool{Registry: reg}
	updateTool := TaskUpdateTool{Registry: reg, AgentName: ""} // no agent name

	mgr := reg.Get("proj")
	mgr.Checkins.Increment("someone")

	createInput, _ := json.Marshal(map[string]string{"team_name": "proj", "subject": "test"})
	createResult, _ := createTool.Execute(context.Background(), createInput)
	var created map[string]string
	json.Unmarshal([]byte(createResult.Output), &created)

	updateInput, _ := json.Marshal(map[string]any{
		"team_name": "proj",
		"task_id":   created["task_id"],
		"status":    "in_progress",
	})
	result, err := updateTool.Execute(context.Background(), updateInput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}
	// "someone" checkin count should be unchanged since AgentName is empty.
	if mgr.Checkins.Count("someone") != 1 {
		t.Errorf("expected checkin count 1, got %d", mgr.Checkins.Count("someone"))
	}
}

func TestTaskUpdateTool_UpdateSubjectAndDescription(t *testing.T) {
	reg := setupTaskTestTeam(t)
	createTool := TaskCreateTool{Registry: reg}
	updateTool := TaskUpdateTool{Registry: reg, AgentName: "alice"}

	createInput, _ := json.Marshal(map[string]string{"team_name": "proj", "subject": "old"})
	createResult, _ := createTool.Execute(context.Background(), createInput)
	var created map[string]string
	json.Unmarshal([]byte(createResult.Output), &created)

	updateInput, _ := json.Marshal(map[string]any{
		"team_name":   "proj",
		"task_id":     created["task_id"],
		"subject":     "new subject",
		"description": "new description",
	})
	result, err := updateTool.Execute(context.Background(), updateInput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	var task Task
	json.Unmarshal([]byte(result.Output), &task)
	if task.Subject != "new subject" {
		t.Errorf("expected subject 'new subject', got %q", task.Subject)
	}
	if task.Description != "new description" {
		t.Errorf("expected description 'new description', got %q", task.Description)
	}
}

func TestTaskUpdateTool_AddBlocksAndBlockedBy(t *testing.T) {
	reg := setupTaskTestTeam(t)
	createTool := TaskCreateTool{Registry: reg}
	updateTool := TaskUpdateTool{Registry: reg, AgentName: "alice"}

	createInput, _ := json.Marshal(map[string]string{"team_name": "proj", "subject": "deps"})
	createResult, _ := createTool.Execute(context.Background(), createInput)
	var created map[string]string
	json.Unmarshal([]byte(createResult.Output), &created)

	updateInput, _ := json.Marshal(map[string]any{
		"team_name":    "proj",
		"task_id":      created["task_id"],
		"add_blocks":   []string{"10", "11"},
		"add_blocked_by": []string{"5"},
	})
	result, err := updateTool.Execute(context.Background(), updateInput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	var task Task
	json.Unmarshal([]byte(result.Output), &task)
	if len(task.Blocks) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(task.Blocks))
	}
	if len(task.BlockedBy) != 1 {
		t.Errorf("expected 1 blocked_by, got %d", len(task.BlockedBy))
	}
}

// ---------------------------------------------------------------------------
// tool_team.go -- missing coverage branches
// ---------------------------------------------------------------------------

func TestTeamCreateTool_InvalidJSON(t *testing.T) {
	reg := NewManagerRegistry()
	tool := TeamCreateTool{Registry: reg}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{invalid}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTeamCreateTool_InvalidTeamName(t *testing.T) {
	reg := NewManagerRegistry()
	tool := TeamCreateTool{Registry: reg}

	input, _ := json.Marshal(map[string]string{"team_name": "!!invalid!!"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid team_name")
	}
	if !strings.Contains(result.Output, "alphanumeric") {
		t.Errorf("unexpected error message: %s", result.Output)
	}
}

func TestTeamCreateTool_InvalidWorkflow(t *testing.T) {
	reg := NewManagerRegistry()
	tool := TeamCreateTool{Registry: reg}

	input, _ := json.Marshal(map[string]string{"team_name": "valid-name", "workflow": "bogus"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid workflow")
	}
}

func TestTeamCreateTool_WithTaskDirBase(t *testing.T) {
	reg := NewManagerRegistry()
	tool := TeamCreateTool{Registry: reg, TaskDirBase: t.TempDir()}

	input, _ := json.Marshal(map[string]string{"team_name": "custom-dir", "workflow": "rpiv"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	if reg.Get("custom-dir") == nil {
		t.Error("team should exist")
	}
}

func TestTeamDeleteTool_InvalidJSON(t *testing.T) {
	reg := NewManagerRegistry()
	tool := TeamDeleteTool{Registry: reg}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{invalid}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// tool_phase.go -- missing coverage branches
// ---------------------------------------------------------------------------

func TestAdvancePhaseTool_InvalidJSON(t *testing.T) {
	reg := NewManagerRegistry()
	tool := AdvancePhaseTool{Registry: reg, AgentName: "worker"}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{invalid}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestAdvancePhaseTool_EmptyTeamName(t *testing.T) {
	reg := NewManagerRegistry()
	tool := AdvancePhaseTool{Registry: reg, AgentName: "worker"}

	input, _ := json.Marshal(map[string]string{})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for empty team_name")
	}
}

// ---------------------------------------------------------------------------
// phaseregistry.go -- ExecuteStreaming allowed path, Get/Schemas nil path
// ---------------------------------------------------------------------------

func TestPhaseRegistry_ExecuteStreamingAllowed(t *testing.T) {
	reg := newTestRegistry("Read", "Write")
	tracker := newMockPhaseTracker()
	tracker.SetPhases("agent-a", []Phase{
		{Name: "implement", Tools: []string{"Read", "Write"}},
	})

	pr := NewPhaseRegistry(reg, tracker, "agent-a", []string{}, []string{})

	result, err := pr.ExecuteStreaming(context.Background(), "Write", json.RawMessage(`{}`), func(s string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success, got error: %s", result.Output)
	}
}

func TestPhaseRegistry_NilPhasesReturnsAll(t *testing.T) {
	reg := newTestRegistry("Read", "Write", "Bash")
	// Create a tracker with nil phases (simulating no workflow).
	tracker := &nilPhaseTracker{}

	pr := NewPhaseRegistry(reg, tracker, "agent-a", []string{}, []string{})

	// All tools should be available.
	schemas := pr.Schemas()
	if len(schemas) != 3 {
		t.Errorf("expected 3 schemas, got %d", len(schemas))
	}

	// Get should return tools.
	if pr.Get("Write") == nil {
		t.Error("Get(Write) should return the tool when no phases configured")
	}

	// Execute should work.
	result, err := pr.Execute(context.Background(), "Write", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Output)
	}

	// ExecuteStreaming should work.
	result, err = pr.ExecuteStreaming(context.Background(), "Write", json.RawMessage(`{}`), func(s string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Output)
	}
}

// nilPhaseTracker returns nil, 0, nil for Current -- simulating no phases configured.
type nilPhaseTracker struct{}

func (n *nilPhaseTracker) SetPhases(string, []Phase)              {}
func (n *nilPhaseTracker) Current(string) (*Phase, int, error)    { return nil, 0, nil }
func (n *nilPhaseTracker) Advance(string) (*Phase, error)         { return nil, nil }
func (n *nilPhaseTracker) AdvanceTo(string, string) (*Phase, error) { return nil, nil }
func (n *nilPhaseTracker) Remove(string)                          {}

// ---------------------------------------------------------------------------
// registry.go -- Create failure (NewTeamManager failure)
// ---------------------------------------------------------------------------

func TestManagerRegistry_CreateFailure(t *testing.T) {
	reg := NewManagerRegistry()
	// Use an invalid path to cause NewTeamManager to fail.
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "blocker")
	os.WriteFile(filePath, []byte("x"), 0644)

	_, err := reg.Create(TeamConfig{Name: "fail"}, filepath.Join(filePath, "tasks"))
	if err == nil {
		t.Fatal("expected error when task dir creation fails")
	}
}

// ---------------------------------------------------------------------------
// getManager helper -- empty team_name, team not found
// ---------------------------------------------------------------------------

func TestGetManager_EmptyTeamName(t *testing.T) {
	reg := NewManagerRegistry()
	_, errResult := getManager(reg, "")
	if errResult == nil {
		t.Fatal("expected error result for empty team_name")
	}
	if !strings.Contains(errResult.Output, "team_name is required") {
		t.Errorf("unexpected error: %s", errResult.Output)
	}
}

func TestGetManager_TeamNotFound(t *testing.T) {
	reg := NewManagerRegistry()
	_, errResult := getManager(reg, "nope")
	if errResult == nil {
		t.Fatal("expected error result for team not found")
	}
	if !strings.Contains(errResult.Output, "not found") {
		t.Errorf("unexpected error: %s", errResult.Output)
	}
}

func TestGetManager_Success(t *testing.T) {
	reg := NewManagerRegistry()
	reg.Create(TeamConfig{Name: "alpha"}, t.TempDir())
	mgr, errResult := getManager(reg, "alpha")
	if errResult != nil {
		t.Fatalf("unexpected error: %s", errResult.Output)
	}
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
}

// ---------------------------------------------------------------------------
// ScopedTool -- traversal bypass test
// ---------------------------------------------------------------------------

func TestScopedTool_TraversalBypassPrevented(t *testing.T) {
	inner := &stubTool{name: "Write"}
	scoped := NewScopedTool(inner, []string{"/project/src/*"}, nil)

	// Try path traversal.
	input := json.RawMessage(`{"file_path": "/project/src/../../etc/passwd"}`)
	result, err := scoped.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for path traversal attempt")
	}
}

// ---------------------------------------------------------------------------
// ScopedBash -- leading/trailing whitespace handling
// ---------------------------------------------------------------------------

func TestScopedBash_LeadingWhitespace(t *testing.T) {
	inner := &stubTool{name: "Bash"}
	scoped := NewScopedBash(inner, []string{"go "})

	input := json.RawMessage(`{"command": "  go test ./..."}`)
	result, err := scoped.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success after trimming whitespace, got error: %s", result.Output)
	}
}

// ---------------------------------------------------------------------------
// tasks.go -- appendUnique edge case: no existing items
// ---------------------------------------------------------------------------

func TestFileTaskStorage_AddBlocksToEmptySlice(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)
	task, _ := s.Create(Task{Subject: "empty blocks", Status: TaskStatusPending})

	_, err := s.Update(task.ID, TaskUpdate{AddBlocks: []string{"a", "b", "a"}})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := s.Get(task.ID)
	// "a" should appear only once.
	if len(got.Blocks) != 2 {
		t.Errorf("expected 2 blocks (deduped), got %d: %v", len(got.Blocks), got.Blocks)
	}
}

// ---------------------------------------------------------------------------
// Tool Schema() methods (covers the 0% branches via testing panic won't happen)
// ---------------------------------------------------------------------------

func TestToolSchemas_DoNotPanic(t *testing.T) {
	// SendMessageTool
	smt := SendMessageTool{}
	if smt.Name() != "SendMessage" {
		t.Errorf("unexpected name: %s", smt.Name())
	}
	if smt.Description() == "" {
		t.Error("expected non-empty description")
	}
	if len(smt.Schema()) == 0 {
		t.Error("expected non-empty schema")
	}

	// AdvancePhaseTool
	apt := AdvancePhaseTool{}
	if apt.Name() != "AdvancePhase" {
		t.Errorf("unexpected name: %s", apt.Name())
	}
	if apt.Description() == "" {
		t.Error("expected non-empty description")
	}
	if len(apt.Schema()) == 0 {
		t.Error("expected non-empty schema")
	}

	// TaskCreateTool
	tct := TaskCreateTool{}
	if tct.Name() != "TaskCreate" {
		t.Errorf("unexpected name: %s", tct.Name())
	}
	if tct.Description() == "" {
		t.Error("expected non-empty description")
	}
	if len(tct.Schema()) == 0 {
		t.Error("expected non-empty schema")
	}

	// TaskListTool
	tlt := TaskListTool{}
	if tlt.Name() != "TaskList" {
		t.Errorf("unexpected name: %s", tlt.Name())
	}
	if tlt.Description() == "" {
		t.Error("expected non-empty description")
	}
	if len(tlt.Schema()) == 0 {
		t.Error("expected non-empty schema")
	}

	// TaskGetTool
	tgt := TaskGetTool{}
	if tgt.Name() != "TaskGet" {
		t.Errorf("unexpected name: %s", tgt.Name())
	}
	if tgt.Description() == "" {
		t.Error("expected non-empty description")
	}
	if len(tgt.Schema()) == 0 {
		t.Error("expected non-empty schema")
	}

	// TaskUpdateTool
	tut := TaskUpdateTool{}
	if tut.Name() != "TaskUpdate" {
		t.Errorf("unexpected name: %s", tut.Name())
	}
	if tut.Description() == "" {
		t.Error("expected non-empty description")
	}
	if len(tut.Schema()) == 0 {
		t.Error("expected non-empty schema")
	}

	// TeamCreateTool
	tcrt := TeamCreateTool{}
	if tcrt.Name() != "TeamCreate" {
		t.Errorf("unexpected name: %s", tcrt.Name())
	}
	if tcrt.Description() == "" {
		t.Error("expected non-empty description")
	}
	if len(tcrt.Schema()) == 0 {
		t.Error("expected non-empty schema")
	}

	// TeamDeleteTool
	tdt := TeamDeleteTool{}
	if tdt.Name() != "TeamDelete" {
		t.Errorf("unexpected name: %s", tdt.Name())
	}
	if tdt.Description() == "" {
		t.Error("expected non-empty description")
	}
	if len(tdt.Schema()) == 0 {
		t.Error("expected non-empty schema")
	}
}

// ---------------------------------------------------------------------------
// tasks.go -- readTaskLocked non-NotExist error, unmarshal error
// ---------------------------------------------------------------------------

func TestFileTaskStorage_ReadTaskUnmarshalError(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	// Write a corrupt JSON file that exists but is not valid JSON.
	os.WriteFile(filepath.Join(dir, "42.json"), []byte("not json"), 0644)

	_, err := s.Get("42")
	if err == nil {
		t.Fatal("expected error for corrupt task file")
	}
	if !strings.Contains(err.Error(), "unmarshal task") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFileTaskStorage_DeleteNotFoundError(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	err := s.Delete("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent delete")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFileTaskStorage_UpdateCompletionClearBlockedByWithCorruptFile(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	t1, _ := s.Create(Task{Subject: "blocker", Status: TaskStatusPending})
	s.Create(Task{Subject: "blocked", Status: TaskStatusPending, BlockedBy: []string{t1.ID}})

	// Write a corrupt JSON file to test clearBlockedByLocked error handling.
	os.WriteFile(filepath.Join(dir, "99.json"), []byte("not json"), 0644)

	inProgress := TaskStatusInProgress
	s.Update(t1.ID, TaskUpdate{Status: &inProgress})
	completed := TaskStatusCompleted
	// This should succeed but log a warning about the corrupt file.
	_, err := s.Update(t1.ID, TaskUpdate{Status: &completed})
	if err != nil {
		// The error is wrapped: "task X completed but failed to unblock dependents: ..."
		if !strings.Contains(err.Error(), "failed to unblock dependents") {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func TestFileTaskStorage_ListWithRemovedDirectory(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	// Remove the directory entirely so List() hits the ReadDir error.
	os.RemoveAll(dir)

	list := s.List()
	if list != nil {
		t.Errorf("expected nil list for removed directory, got %v", list)
	}
}

func TestFileTaskStorage_NewFileTaskStorage_NonNumericJSONFiles(t *testing.T) {
	dir := t.TempDir()

	// Write files with various names.
	os.WriteFile(filepath.Join(dir, "abc.json"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(dir, "5.json"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0644)

	s, err := NewFileTaskStorage(dir)
	if err != nil {
		t.Fatalf("NewFileTaskStorage: %v", err)
	}

	// nextN should be 6 (from "5.json"), skipping "abc.json" and "readme.txt".
	task, err := s.Create(Task{Subject: "test", Status: TaskStatusPending})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task.ID != "6" {
		t.Errorf("expected ID 6 (after seeing 5.json), got %s", task.ID)
	}
}

// ---------------------------------------------------------------------------
// Team name validation tests
// ---------------------------------------------------------------------------

func TestTeamCreateTool_ValidTeamNames(t *testing.T) {
	cases := []struct {
		name    string
		valid   bool
	}{
		{"alpha", true},
		{"a-b-c", true},
		{"a_b_c", true},
		{"Alpha123", true},
		{"1starts-with-num", true},
		{"-starts-with-dash", false},
		{"_starts-with-underscore", false},
		{"has space", false},
		{"has.dot", false},
		{"", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := NewManagerRegistry()
			tool := TeamCreateTool{Registry: reg, TaskDirBase: t.TempDir()}
			input, _ := json.Marshal(map[string]string{"team_name": tc.name})
			result, err := tool.Execute(context.Background(), input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.valid && result.IsError {
				t.Errorf("expected valid team name %q, got error: %s", tc.name, result.Output)
			}
			if !tc.valid && !result.IsError {
				t.Errorf("expected invalid team name %q to be rejected", tc.name)
			}
		})
	}
}
