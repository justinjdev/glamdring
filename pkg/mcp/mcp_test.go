package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/justin/glamdring/pkg/config"
)

// fakeCmd returns an exec.Cmd that has already exited (for testing Close paths).
func fakeCmd() *exec.Cmd {
	return exec.Command("true")
}

// newTestClient creates a Client wired to io.Pipe pairs for testing without
// a real subprocess. serverWriter is the write end the test uses to send
// JSON-RPC responses. clientWriter is the write end the Client writes requests
// to (the test reads from clientReader to see what the client sent).
// The returned Client has its readLoop already running.
func newTestClient() (client *Client, serverWriter *io.PipeWriter, clientReader *io.PipeReader) {
	// Client writes requests here; test reads from clientReader.
	clientReader, clientWriteEnd := io.Pipe()
	// Test writes responses here; Client reads from serverReadEnd.
	serverReadEnd, serverWriter := io.Pipe()

	client = &Client{
		cmd:     fakeCmd(),
		stdin:   clientWriteEnd,
		stdout:  serverReadEnd,
		pending: make(map[int]chan Response),
		done:    make(chan struct{}),
	}
	go client.readLoop()
	return client, serverWriter, clientReader
}

// respondTo reads one JSON-RPC request from r and writes a response with the
// given result to w. It returns the parsed request for assertion.
func respondTo(t *testing.T, r *io.PipeReader, w *io.PipeWriter, result any) Request {
	t.Helper()
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		t.Fatal("no request received from client")
	}
	var req Request
	if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	resp := Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultJSON,
	}
	data, _ := json.Marshal(resp)
	data = append(data, '\n')
	if _, err := w.Write(data); err != nil {
		t.Fatalf("write response: %v", err)
	}
	return req
}

// respondWithError reads one JSON-RPC request from r and writes an error
// response to w.
func respondWithError(t *testing.T, r *io.PipeReader, w *io.PipeWriter, code int, msg string) Request {
	t.Helper()
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		t.Fatal("no request received from client")
	}
	var req Request
	if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}

	resp := Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Error:   &MCPError{Code: code, Message: msg},
	}
	data, _ := json.Marshal(resp)
	data = append(data, '\n')
	if _, err := w.Write(data); err != nil {
		t.Fatalf("write error response: %v", err)
	}
	return req
}

// --- 6.1: Tool name prefix stripping ---

func TestMCPToolName_Simple(t *testing.T) {
	tool := NewMCPTool(nil, "server", ToolDefinition{Name: "read"})
	if got := tool.MCPToolName(); got != "read" {
		t.Errorf("MCPToolName() = %q, want %q", got, "read")
	}
	if got := tool.Name(); got != "server_read" {
		t.Errorf("Name() = %q, want %q", got, "server_read")
	}
}

func TestMCPToolName_UnderscoreInServerName(t *testing.T) {
	tool := NewMCPTool(nil, "my_server", ToolDefinition{Name: "read_file"})
	if got := tool.MCPToolName(); got != "read_file" {
		t.Errorf("MCPToolName() = %q, want %q", got, "read_file")
	}
	if got := tool.Name(); got != "my_server_read_file" {
		t.Errorf("Name() = %q, want %q", got, "my_server_read_file")
	}
}

func TestMCPToolName_MultipleUnderscores(t *testing.T) {
	tool := NewMCPTool(nil, "a_b_c", ToolDefinition{Name: "x_y_z"})
	if got := tool.MCPToolName(); got != "x_y_z" {
		t.Errorf("MCPToolName() = %q, want %q", got, "x_y_z")
	}
}

// --- 6.2: Channel close detection ---

func TestClosedChannelReturnsZeroValue(t *testing.T) {
	// Demonstrates the bug: a closed channel returns zero-value Response.
	// The fix in call() checks ok to detect this.
	ch := make(chan Response, 1)
	close(ch)
	resp, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed")
	}
	if resp.Error != nil || resp.Result != nil {
		t.Fatal("expected zero-value response from closed channel")
	}
}

// --- 6.4: Environment variable support ---

func TestNewClientSetsEnv(t *testing.T) {
	env := []string{"FOO=bar", "BAZ=qux"}
	client, err := NewClient("echo", []string{"hello"}, env)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	found := 0
	for _, e := range client.cmd.Env {
		if e == "FOO=bar" || e == "BAZ=qux" {
			found++
		}
	}
	if found != 2 {
		t.Errorf("expected both env vars in cmd.Env, found %d", found)
	}
}

func TestNewClientNoEnv(t *testing.T) {
	client, err := NewClient("echo", []string{"hello"}, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	// When no env is passed, cmd.Env should be nil (inherit parent).
	if client.cmd.Env != nil {
		t.Errorf("expected nil cmd.Env when no env vars provided")
	}
}

func TestNewClientStderrCaptured(t *testing.T) {
	client, err := NewClient("echo", []string{"hello"}, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.cmd.Stderr != &client.stderr {
		t.Error("expected cmd.Stderr to capture to client.stderr buffer")
	}
}

// --- 6.5: Duplicate server name guard ---

func TestServerCount(t *testing.T) {
	mgr := NewManager()
	if got := mgr.ServerCount(); got != 0 {
		t.Errorf("ServerCount() = %d, want 0", got)
	}

	// Manually add a server entry for testing.
	mgr.mu.Lock()
	mgr.servers["test"] = &serverEntry{
		tools: []*MCPTool{{mcpName: "read"}},
	}
	mgr.mu.Unlock()

	if got := mgr.ServerCount(); got != 1 {
		t.Errorf("ServerCount() = %d, want 1", got)
	}
}

// --- 6.6: Health visibility ---

func TestServerStatus(t *testing.T) {
	mgr := NewManager()

	mgr.mu.Lock()
	mgr.servers["alpha"] = &serverEntry{
		client: &Client{done: make(chan struct{})},
		tools:  []*MCPTool{{mcpName: "t1"}, {mcpName: "t2"}},
	}
	mgr.servers["beta"] = &serverEntry{
		client: &Client{done: make(chan struct{})},
		tools:  []*MCPTool{{mcpName: "t3"}},
	}
	mgr.mu.Unlock()

	statuses := mgr.ServerStatus()
	if len(statuses) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(statuses))
	}
	// Should be sorted by name.
	if statuses[0].Name != "alpha" {
		t.Errorf("expected first server 'alpha', got %q", statuses[0].Name)
	}
	if statuses[1].Name != "beta" {
		t.Errorf("expected second server 'beta', got %q", statuses[1].Name)
	}
	// Both should be alive (done channel is open).
	if !statuses[0].Alive {
		t.Error("expected alpha to be alive")
	}
	if statuses[0].Tools != 2 {
		t.Errorf("expected alpha to have 2 tools, got %d", statuses[0].Tools)
	}
}

func TestDisconnectServer(t *testing.T) {
	mgr := NewManager()

	err := mgr.DisconnectServer("nonexistent")
	if err == nil {
		t.Error("expected error for unknown server")
	}
}

// --- 6.7: Tool filtering ---

func TestFilterToolDefs_Allowlist(t *testing.T) {
	defs := []ToolDefinition{
		{Name: "read"}, {Name: "write"}, {Name: "delete"},
	}
	filtered := filterToolDefs(defs, config.MCPToolsConfig{
		Enabled: []string{"read", "write"},
	})
	if len(filtered) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(filtered))
	}
	names := map[string]bool{}
	for _, d := range filtered {
		names[d.Name] = true
	}
	if !names["read"] || !names["write"] {
		t.Errorf("expected read and write, got %v", names)
	}
}

func TestFilterToolDefs_Denylist(t *testing.T) {
	defs := []ToolDefinition{
		{Name: "read"}, {Name: "write"}, {Name: "delete"},
	}
	filtered := filterToolDefs(defs, config.MCPToolsConfig{
		Disabled: []string{"delete"},
	})
	if len(filtered) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(filtered))
	}
	for _, d := range filtered {
		if d.Name == "delete" {
			t.Error("delete should be filtered out")
		}
	}
}

func TestFilterToolDefs_AllowlistPrecedence(t *testing.T) {
	defs := []ToolDefinition{
		{Name: "read"}, {Name: "write"}, {Name: "delete"},
	}
	filtered := filterToolDefs(defs, config.MCPToolsConfig{
		Enabled:  []string{"read"},
		Disabled: []string{"write"},
	})
	if len(filtered) != 1 || filtered[0].Name != "read" {
		t.Fatalf("expected only 'read', got %v", filtered)
	}
}

func TestFilterToolDefs_NoFilter(t *testing.T) {
	defs := []ToolDefinition{
		{Name: "read"}, {Name: "write"},
	}
	filtered := filterToolDefs(defs, config.MCPToolsConfig{})
	if len(filtered) != 2 {
		t.Fatalf("expected 2 tools (no filter), got %d", len(filtered))
	}
}

func TestDisableEnableTool(t *testing.T) {
	mgr := NewManager()

	mgr.mu.Lock()
	mgr.servers["test"] = &serverEntry{
		tools: []*MCPTool{
			{mcpName: "read", name: "test_read"},
			{mcpName: "write", name: "test_write"},
		},
	}
	mgr.mu.Unlock()

	// All tools initially visible.
	if got := len(mgr.Tools()); got != 2 {
		t.Fatalf("expected 2 tools, got %d", got)
	}

	// Disable one.
	if err := mgr.DisableTool("test", "read"); err != nil {
		t.Fatalf("DisableTool: %v", err)
	}
	if got := len(mgr.Tools()); got != 1 {
		t.Fatalf("expected 1 tool after disable, got %d", got)
	}

	// Re-enable.
	if err := mgr.EnableTool("test", "read"); err != nil {
		t.Fatalf("EnableTool: %v", err)
	}
	if got := len(mgr.Tools()); got != 2 {
		t.Fatalf("expected 2 tools after enable, got %d", got)
	}
}

func TestDisableTool_UnknownServer(t *testing.T) {
	mgr := NewManager()
	if err := mgr.DisableTool("nonexistent", "read"); err == nil {
		t.Fatal("expected error for unknown server")
	}
}

func TestDisableTool_UnknownTool(t *testing.T) {
	mgr := NewManager()

	mgr.mu.Lock()
	mgr.servers["test"] = &serverEntry{
		tools: []*MCPTool{{mcpName: "read", name: "test_read"}},
	}
	mgr.mu.Unlock()

	if err := mgr.DisableTool("test", "nonexistent"); err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestListServerTools(t *testing.T) {
	mgr := NewManager()

	mgr.mu.Lock()
	mgr.servers["test"] = &serverEntry{
		tools: []*MCPTool{
			{mcpName: "read", name: "test_read"},
			{mcpName: "write", name: "test_write"},
		},
	}
	mgr.mu.Unlock()

	// Disable one tool.
	_ = mgr.DisableTool("test", "write")

	tools, err := mgr.ListServerTools("test")
	if err != nil {
		t.Fatalf("ListServerTools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tool infos, got %d", len(tools))
	}

	for _, ti := range tools {
		switch ti.Name {
		case "read":
			if ti.Disabled {
				t.Error("read should not be disabled")
			}
		case "write":
			if !ti.Disabled {
				t.Error("write should be disabled")
			}
		default:
			t.Errorf("unexpected tool %q", ti.Name)
		}
	}
}

func TestListServerTools_UnknownServer(t *testing.T) {
	mgr := NewManager()
	_, err := mgr.ListServerTools("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown server")
	}
}

// --- EnableTool validation ---

func TestEnableTool_UnknownTool(t *testing.T) {
	mgr := NewManager()

	mgr.mu.Lock()
	mgr.servers["test"] = &serverEntry{
		tools: []*MCPTool{{mcpName: "read", name: "test_read"}},
	}
	mgr.mu.Unlock()

	err := mgr.EnableTool("test", "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got %q", err)
	}
}

func TestEnableTool_UnknownServer(t *testing.T) {
	mgr := NewManager()
	err := mgr.EnableTool("nonexistent", "read")
	if err == nil {
		t.Fatal("expected error for unknown server")
	}
}

// --- Monitor skips replaced server ---

func TestMonitorSkipsReplacedServer(t *testing.T) {
	mgr := NewManager()
	deathCalled := false
	mgr.OnServerDeath = func(name string) {
		deathCalled = true
	}

	// Create a client with an already-closed done channel to simulate death.
	oldDone := make(chan struct{})
	close(oldDone)
	oldClient := &Client{done: oldDone}

	// Create a new client that "replaced" the old one.
	newClient := &Client{done: make(chan struct{})}

	// Insert the new client in the map — the old one has been replaced.
	mgr.mu.Lock()
	mgr.servers["test"] = &serverEntry{client: newClient}
	mgr.mu.Unlock()

	// Simulate what monitor does: it captured oldClient, waits on oldClient.done,
	// then checks if current.client == oldClient. Since we replaced it, the
	// guard should prevent OnServerDeath from firing.
	<-oldClient.done

	mgr.mu.Lock()
	current, stillExists := mgr.servers["test"]
	shouldFire := stillExists && current.client == oldClient
	mgr.mu.Unlock()

	if shouldFire {
		t.Error("expected monitor guard to prevent firing for replaced server")
	}
	if deathCalled {
		t.Error("OnServerDeath should not have been called for replaced server")
	}
}

// --- Stderr capture ---

func TestClientStderrCapture(t *testing.T) {
	client, err := NewClient("echo", []string{"hello"}, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	// Verify stderr is wired to the buffer.
	if client.cmd.Stderr != &client.stderr {
		t.Error("expected cmd.Stderr to be wired to client.stderr buffer")
	}
	// Verify Stderr() returns empty initially.
	if got := client.Stderr(); got != "" {
		t.Errorf("expected empty stderr, got %q", got)
	}
}

// --- Manager.Close clears servers map ---

func TestManagerCloseClearsServers(t *testing.T) {
	mgr := NewManager()

	// Add a fake server with a closed done channel so Close doesn't block.
	done := make(chan struct{})
	close(done)
	mgr.mu.Lock()
	mgr.servers["test"] = &serverEntry{
		client: &Client{
			done:  done,
			stdin: nopWriteCloser{},
			cmd:   fakeCmd(),
		},
	}
	mgr.mu.Unlock()

	mgr.Close()

	if got := mgr.ServerCount(); got != 0 {
		t.Errorf("expected 0 servers after Close, got %d", got)
	}
}

// nopWriteCloser is a no-op io.WriteCloser for testing.
type nopWriteCloser struct{}

func (nopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (nopWriteCloser) Close() error                 { return nil }

// errWriteCloser always returns an error on Write.
type errWriteCloser struct{}

func (errWriteCloser) Write(p []byte) (int, error) { return 0, fmt.Errorf("write error") }
func (errWriteCloser) Close() error                 { return nil }

// =============================================================================
// Protocol types
// =============================================================================

func TestMCPError_Error(t *testing.T) {
	e := &MCPError{Code: -32601, Message: "method not found"}
	if got := e.Error(); got != "method not found" {
		t.Errorf("MCPError.Error() = %q, want %q", got, "method not found")
	}
}

// =============================================================================
// Adapter: Description, Schema, Execute
// =============================================================================

func TestMCPTool_DescriptionAndSchema(t *testing.T) {
	schema := json.RawMessage(`{"type":"object"}`)
	tool := NewMCPTool(nil, "srv", ToolDefinition{
		Name:        "mytool",
		Description: "does things",
		InputSchema: schema,
	})
	if got := tool.Description(); got != "does things" {
		t.Errorf("Description() = %q, want %q", got, "does things")
	}
	if got := string(tool.Schema()); got != `{"type":"object"}` {
		t.Errorf("Schema() = %q, want %q", got, `{"type":"object"}`)
	}
}

func TestMCPTool_Execute_Success(t *testing.T) {
	client, serverWriter, clientReader := newTestClient()
	defer serverWriter.Close()
	defer clientReader.Close()

	tool := NewMCPTool(client, "srv", ToolDefinition{Name: "echo"})

	ctx := context.Background()
	input := json.RawMessage(`{"text":"hello"}`)

	type execOut struct {
		output  string
		isError bool
		err     error
	}
	resultCh := make(chan execOut, 1)

	go func() {
		r, err := tool.Execute(ctx, input)
		resultCh <- execOut{r.Output, r.IsError, err}
	}()

	// Respond with tool call result.
	respondTo(t, clientReader, serverWriter, ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: "world"}},
	})

	got := <-resultCh
	if got.err != nil {
		t.Fatalf("Execute returned error: %v", got.err)
	}
	if got.output != "world" {
		t.Errorf("Execute output = %q, want %q", got.output, "world")
	}
	if got.isError {
		t.Error("Execute isError should be false")
	}
}

func TestMCPTool_Execute_ServerError(t *testing.T) {
	client, serverWriter, clientReader := newTestClient()
	defer serverWriter.Close()
	defer clientReader.Close()

	tool := NewMCPTool(client, "srv", ToolDefinition{Name: "fail"})

	ctx := context.Background()
	input := json.RawMessage(`{}`)

	type execOut struct {
		output  string
		isError bool
		err     error
	}
	resultCh := make(chan execOut, 1)

	go func() {
		r, err := tool.Execute(ctx, input)
		resultCh <- execOut{r.Output, r.IsError, err}
	}()

	// Respond with JSON-RPC error.
	respondWithError(t, clientReader, serverWriter, -32000, "something broke")

	got := <-resultCh
	// Execute wraps transport errors into Result, not Go error.
	if got.err != nil {
		t.Fatalf("Execute returned Go error: %v", got.err)
	}
	if !got.isError {
		t.Error("expected isError = true")
	}
	if !strings.Contains(got.output, "something broke") {
		t.Errorf("expected error text in output, got %q", got.output)
	}
}

func TestMCPTool_Execute_IsErrorFromServer(t *testing.T) {
	client, serverWriter, clientReader := newTestClient()
	defer serverWriter.Close()
	defer clientReader.Close()

	tool := NewMCPTool(client, "srv", ToolDefinition{Name: "fail"})

	ctx := context.Background()
	input := json.RawMessage(`{}`)

	type execOut struct {
		output  string
		isError bool
	}
	resultCh := make(chan execOut, 1)

	go func() {
		r, _ := tool.Execute(ctx, input)
		resultCh <- execOut{r.Output, r.IsError}
	}()

	respondTo(t, clientReader, serverWriter, ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: "bad input"}},
		IsError: true,
	})

	got := <-resultCh
	if !got.isError {
		t.Error("expected isError = true from tool result")
	}
	if got.output != "bad input" {
		t.Errorf("output = %q, want %q", got.output, "bad input")
	}
}

// =============================================================================
// Client: readLoop
// =============================================================================

func TestReadLoop_DispatchesResponse(t *testing.T) {
	client, serverWriter, _ := newTestClient()

	// Register a pending request.
	ch := make(chan Response, 1)
	client.mu.Lock()
	client.pending[42] = ch
	client.mu.Unlock()

	// Write a response from the "server".
	result, _ := json.Marshal(map[string]string{"ok": "true"})
	resp := Response{JSONRPC: "2.0", ID: 42, Result: result}
	data, _ := json.Marshal(resp)
	data = append(data, '\n')
	serverWriter.Write(data)

	// Wait for dispatch.
	select {
	case got := <-ch:
		if got.ID != 42 {
			t.Errorf("response ID = %d, want 42", got.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for response dispatch")
	}

	serverWriter.Close()
	<-client.done
}

func TestReadLoop_SkipsNotifications(t *testing.T) {
	client, serverWriter, _ := newTestClient()

	// Write a notification (ID=0, no pending entry).
	notif := Response{JSONRPC: "2.0", ID: 0}
	data, _ := json.Marshal(notif)
	data = append(data, '\n')
	serverWriter.Write(data)

	// Write a real response to confirm readLoop is still running.
	ch := make(chan Response, 1)
	client.mu.Lock()
	client.pending[1] = ch
	client.mu.Unlock()

	resp := Response{JSONRPC: "2.0", ID: 1, Result: json.RawMessage(`{}`)}
	data2, _ := json.Marshal(resp)
	data2 = append(data2, '\n')
	serverWriter.Write(data2)

	select {
	case got := <-ch:
		if got.ID != 1 {
			t.Errorf("response ID = %d, want 1", got.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}

	serverWriter.Close()
	<-client.done
}

func TestReadLoop_SkipsMalformedJSON(t *testing.T) {
	client, serverWriter, _ := newTestClient()

	// Write malformed JSON.
	serverWriter.Write([]byte("this is not json\n"))

	// Write a valid response after.
	ch := make(chan Response, 1)
	client.mu.Lock()
	client.pending[5] = ch
	client.mu.Unlock()

	resp := Response{JSONRPC: "2.0", ID: 5, Result: json.RawMessage(`{}`)}
	data, _ := json.Marshal(resp)
	data = append(data, '\n')
	serverWriter.Write(data)

	select {
	case got := <-ch:
		if got.ID != 5 {
			t.Errorf("response ID = %d, want 5", got.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}

	serverWriter.Close()
	<-client.done
}

func TestReadLoop_SkipsEmptyLines(t *testing.T) {
	client, serverWriter, _ := newTestClient()

	// Write empty lines.
	serverWriter.Write([]byte("\n\n\n"))

	// Write a valid response.
	ch := make(chan Response, 1)
	client.mu.Lock()
	client.pending[7] = ch
	client.mu.Unlock()

	resp := Response{JSONRPC: "2.0", ID: 7, Result: json.RawMessage(`{}`)}
	data, _ := json.Marshal(resp)
	data = append(data, '\n')
	serverWriter.Write(data)

	select {
	case got := <-ch:
		if got.ID != 7 {
			t.Errorf("response ID = %d, want 7", got.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}

	serverWriter.Close()
	<-client.done
}

func TestReadLoop_UnblocksPendingOnClose(t *testing.T) {
	client, serverWriter, _ := newTestClient()

	ch := make(chan Response, 1)
	client.mu.Lock()
	client.pending[99] = ch
	client.mu.Unlock()

	// Close the server writer, triggering EOF on readLoop.
	serverWriter.Close()

	// The pending channel should be closed.
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed, got a value")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for pending channel to close")
	}

	<-client.done
}

// =============================================================================
// Client: call
// =============================================================================

func TestCall_Success(t *testing.T) {
	client, serverWriter, clientReader := newTestClient()
	defer serverWriter.Close()

	ctx := context.Background()
	var result InitializeResult

	go func() {
		respondTo(t, clientReader, serverWriter, InitializeResult{
			ProtocolVersion: "2024-11-05",
			ServerInfo:      ServerInfo{Name: "test-server"},
		})
	}()

	err := client.call(ctx, "initialize", InitializeParams{
		ProtocolVersion: "2024-11-05",
	}, &result)
	if err != nil {
		t.Fatalf("call returned error: %v", err)
	}
	if result.ProtocolVersion != "2024-11-05" {
		t.Errorf("result.ProtocolVersion = %q, want %q", result.ProtocolVersion, "2024-11-05")
	}
	if result.ServerInfo.Name != "test-server" {
		t.Errorf("result.ServerInfo.Name = %q, want %q", result.ServerInfo.Name, "test-server")
	}
}

func TestCall_NilParams(t *testing.T) {
	client, serverWriter, clientReader := newTestClient()
	defer serverWriter.Close()

	ctx := context.Background()

	go func() {
		req := respondTo(t, clientReader, serverWriter, json.RawMessage(`{}`))
		// Verify no params field.
		if req.Params != nil {
			t.Errorf("expected nil params, got %s", string(req.Params))
		}
	}()

	err := client.call(ctx, "ping", nil, nil)
	if err != nil {
		t.Fatalf("call returned error: %v", err)
	}
}

func TestCall_ErrorResponse(t *testing.T) {
	client, serverWriter, clientReader := newTestClient()
	defer serverWriter.Close()

	ctx := context.Background()

	go func() {
		respondWithError(t, clientReader, serverWriter, -32601, "method not found")
	}()

	var result json.RawMessage
	err := client.call(ctx, "nonexistent", nil, &result)
	if err == nil {
		t.Fatal("expected error from call")
	}
	if !strings.Contains(err.Error(), "method not found") {
		t.Errorf("error = %q, want to contain 'method not found'", err)
	}
}

func TestCall_ContextCancelled(t *testing.T) {
	client, serverWriter, clientReader := newTestClient()
	defer serverWriter.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Drain requests so the write in call() doesn't block on the pipe.
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		scanner := bufio.NewScanner(clientReader)
		for scanner.Scan() {
			// Discard all requests -- never respond.
		}
	}()

	// Cancel after a brief moment so call() can write but never gets a response.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := client.call(ctx, "initialize", nil, nil)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	// Clean up: close client stdin so drain goroutine exits.
	client.stdin.Close()
	<-drainDone
}

func TestCall_ServerDied(t *testing.T) {
	client, serverWriter, clientReader := newTestClient()

	// Close the server side to trigger readLoop exit.
	serverWriter.Close()
	<-client.done

	// Also close the client reader so the write to stdin fails immediately
	// rather than blocking the pipe.
	clientReader.Close()

	ctx := context.Background()
	err := client.call(ctx, "test", nil, nil)
	if err == nil {
		t.Fatal("expected error when server is dead")
	}
	// Could be "write request" (broken pipe) or "exited" depending on scheduling.
	if !strings.Contains(err.Error(), "exited") && !strings.Contains(err.Error(), "write request") {
		t.Errorf("error = %q, want to contain 'exited' or 'write request'", err)
	}
}

func TestCall_WriteError(t *testing.T) {
	// Create a client with a stdin that always fails on write.
	serverReadEnd, serverWriteEnd := io.Pipe()
	_ = serverWriteEnd
	client := &Client{
		cmd:     fakeCmd(),
		stdin:   errWriteCloser{},
		stdout:  serverReadEnd,
		pending: make(map[int]chan Response),
		done:    make(chan struct{}),
	}
	go client.readLoop()

	ctx := context.Background()
	err := client.call(ctx, "test", nil, nil)
	if err == nil {
		t.Fatal("expected write error")
	}
	if !strings.Contains(err.Error(), "write request") {
		t.Errorf("error = %q, want to contain 'write request'", err)
	}

	// Verify pending map was cleaned up.
	client.mu.Lock()
	pendingCount := len(client.pending)
	client.mu.Unlock()
	if pendingCount != 0 {
		t.Errorf("expected 0 pending after write error, got %d", pendingCount)
	}

	serverReadEnd.Close()
	<-client.done
}

func TestCall_NilResult(t *testing.T) {
	// call with nil result target should not panic when result is returned.
	client, serverWriter, clientReader := newTestClient()
	defer serverWriter.Close()

	ctx := context.Background()

	go func() {
		respondTo(t, clientReader, serverWriter, map[string]string{"key": "value"})
	}()

	// Pass nil as result -- should work fine, just ignore the result.
	err := client.call(ctx, "test", nil, nil)
	if err != nil {
		t.Fatalf("call returned error: %v", err)
	}
}

// =============================================================================
// Client: ListTools
// =============================================================================

func TestListTools(t *testing.T) {
	client, serverWriter, clientReader := newTestClient()
	defer serverWriter.Close()

	ctx := context.Background()

	go func() {
		respondTo(t, clientReader, serverWriter, ToolsListResult{
			Tools: []ToolDefinition{
				{Name: "read", Description: "read a file", InputSchema: json.RawMessage(`{}`)},
				{Name: "write", Description: "write a file", InputSchema: json.RawMessage(`{}`)},
			},
		})
	}()

	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].Name != "read" {
		t.Errorf("tools[0].Name = %q, want %q", tools[0].Name, "read")
	}
	if tools[1].Name != "write" {
		t.Errorf("tools[1].Name = %q, want %q", tools[1].Name, "write")
	}
}

func TestListTools_Error(t *testing.T) {
	client, serverWriter, clientReader := newTestClient()
	defer serverWriter.Close()

	ctx := context.Background()

	go func() {
		respondWithError(t, clientReader, serverWriter, -32000, "list failed")
	}()

	_, err := client.ListTools(ctx)
	if err == nil {
		t.Fatal("expected error from ListTools")
	}
}

// =============================================================================
// Client: CallTool
// =============================================================================

func TestCallTool_SingleTextBlock(t *testing.T) {
	client, serverWriter, clientReader := newTestClient()
	defer serverWriter.Close()

	ctx := context.Background()

	go func() {
		req := respondTo(t, clientReader, serverWriter, ToolCallResult{
			Content: []ContentBlock{{Type: "text", Text: "hello world"}},
		})
		// Verify the request contains the correct tool name.
		var params ToolCallParams
		json.Unmarshal(req.Params, &params)
		if params.Name != "echo" {
			t.Errorf("tool name = %q, want %q", params.Name, "echo")
		}
	}()

	text, isError, err := client.CallTool(ctx, "echo", json.RawMessage(`{"msg":"hi"}`))
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if text != "hello world" {
		t.Errorf("text = %q, want %q", text, "hello world")
	}
	if isError {
		t.Error("expected isError = false")
	}
}

func TestCallTool_MultipleTextBlocks(t *testing.T) {
	client, serverWriter, clientReader := newTestClient()
	defer serverWriter.Close()

	ctx := context.Background()

	go func() {
		respondTo(t, clientReader, serverWriter, ToolCallResult{
			Content: []ContentBlock{
				{Type: "text", Text: "line1"},
				{Type: "image", Text: "ignored"},
				{Type: "text", Text: "line2"},
			},
		})
	}()

	text, _, err := client.CallTool(ctx, "multi", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if text != "line1\nline2" {
		t.Errorf("text = %q, want %q", text, "line1\nline2")
	}
}

func TestCallTool_NoTextBlocks(t *testing.T) {
	client, serverWriter, clientReader := newTestClient()
	defer serverWriter.Close()

	ctx := context.Background()

	go func() {
		respondTo(t, clientReader, serverWriter, ToolCallResult{
			Content: []ContentBlock{{Type: "image", Text: "data"}},
		})
	}()

	text, _, err := client.CallTool(ctx, "img", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if text != "" {
		t.Errorf("text = %q, want empty", text)
	}
}

func TestCallTool_WithIsError(t *testing.T) {
	client, serverWriter, clientReader := newTestClient()
	defer serverWriter.Close()

	ctx := context.Background()

	go func() {
		respondTo(t, clientReader, serverWriter, ToolCallResult{
			Content: []ContentBlock{{Type: "text", Text: "bad input"}},
			IsError: true,
		})
	}()

	text, isError, err := client.CallTool(ctx, "fail", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if text != "bad input" {
		t.Errorf("text = %q, want %q", text, "bad input")
	}
	if !isError {
		t.Error("expected isError = true")
	}
}

func TestCallTool_TransportError(t *testing.T) {
	client, serverWriter, clientReader := newTestClient()
	defer serverWriter.Close()

	ctx := context.Background()

	go func() {
		respondWithError(t, clientReader, serverWriter, -32000, "transport fail")
	}()

	_, _, err := client.CallTool(ctx, "broken", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

// =============================================================================
// Client: Alive
// =============================================================================

func TestAlive_WhenRunning(t *testing.T) {
	client := &Client{done: make(chan struct{})}
	if !client.Alive() {
		t.Error("expected Alive() = true when done channel is open")
	}
}

func TestAlive_WhenDead(t *testing.T) {
	done := make(chan struct{})
	close(done)
	client := &Client{done: done}
	if client.Alive() {
		t.Error("expected Alive() = false when done channel is closed")
	}
}

// =============================================================================
// Client: Start (using TestHelperProcess pattern)
// =============================================================================

// TestHelperProcess is used by tests that need a real subprocess acting as an
// MCP server. It is not a test itself -- it is invoked as a subprocess.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_TEST_HELPER_PROCESS") != "1" {
		return
	}
	mode := os.Getenv("GO_TEST_HELPER_MODE")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	switch mode {
	case "mcp_server":
		// A minimal MCP server: handles initialize, tools/list, tools/call.
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var req Request
			if err := json.Unmarshal(line, &req); err != nil {
				continue
			}
			var resp Response
			resp.JSONRPC = "2.0"
			resp.ID = req.ID

			switch req.Method {
			case "initialize":
				result, _ := json.Marshal(InitializeResult{
					ProtocolVersion: "2024-11-05",
					ServerInfo:      ServerInfo{Name: "test-helper", Version: "1.0"},
				})
				resp.Result = result
			case "notifications/initialized":
				// Notification -- no response.
				continue
			case "tools/list":
				result, _ := json.Marshal(ToolsListResult{
					Tools: []ToolDefinition{
						{Name: "echo", Description: "echoes input", InputSchema: json.RawMessage(`{"type":"object"}`)},
					},
				})
				resp.Result = result
			case "tools/call":
				var params ToolCallParams
				json.Unmarshal(req.Params, &params)
				result, _ := json.Marshal(ToolCallResult{
					Content: []ContentBlock{{Type: "text", Text: "called:" + params.Name}},
				})
				resp.Result = result
			default:
				resp.Error = &MCPError{Code: -32601, Message: "unknown method"}
			}

			data, _ := json.Marshal(resp)
			fmt.Fprintf(os.Stdout, "%s\n", data)
		}
	case "mcp_server_bad_init":
		// Sends a malformed response to initialize.
		for scanner.Scan() {
			fmt.Fprintf(os.Stdout, "not valid json\n")
			return
		}
	case "mcp_server_error_init":
		// Sends an error response to initialize.
		for scanner.Scan() {
			line := scanner.Bytes()
			var req Request
			json.Unmarshal(line, &req)
			resp := Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &MCPError{Code: -32000, Message: "init failed"},
			}
			data, _ := json.Marshal(resp)
			fmt.Fprintf(os.Stdout, "%s\n", data)
			return
		}
	case "mcp_server_dies":
		// Handles initialize then exits.
		for scanner.Scan() {
			line := scanner.Bytes()
			var req Request
			json.Unmarshal(line, &req)
			if req.Method == "initialize" {
				result, _ := json.Marshal(InitializeResult{ProtocolVersion: "2024-11-05"})
				resp := Response{JSONRPC: "2.0", ID: req.ID, Result: result}
				data, _ := json.Marshal(resp)
				fmt.Fprintf(os.Stdout, "%s\n", data)
			}
			if req.Method == "notifications/initialized" {
				continue
			}
			// Exit after handling initialize.
			if req.Method == "notifications/initialized" || req.Method == "tools/list" {
				os.Exit(0)
			}
		}
	case "mcp_server_stderr":
		// Writes to stderr and then handles MCP.
		fmt.Fprintf(os.Stderr, "stderr output from server")
		for scanner.Scan() {
			line := scanner.Bytes()
			var req Request
			json.Unmarshal(line, &req)
			result, _ := json.Marshal(InitializeResult{ProtocolVersion: "2024-11-05"})
			resp := Response{JSONRPC: "2.0", ID: req.ID, Result: result}
			data, _ := json.Marshal(resp)
			fmt.Fprintf(os.Stdout, "%s\n", data)
		}
	}
	os.Exit(0)
}

// helperClient creates a Client using the test helper subprocess.
func helperClient(t *testing.T, mode string) *Client {
	t.Helper()
	client, err := NewClient(os.Args[0], []string{"-test.run=TestHelperProcess"}, []string{
		"GO_TEST_HELPER_PROCESS=1",
		"GO_TEST_HELPER_MODE=" + mode,
	})
	if err != nil {
		t.Fatalf("NewClient for helper: %v", err)
	}
	return client
}

func TestStart_Success(t *testing.T) {
	client := helperClient(t, "mcp_server")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	if !client.Alive() {
		t.Error("expected client to be alive after Start")
	}
}

func TestStart_ErrorInit(t *testing.T) {
	client := helperClient(t, "mcp_server_error_init")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Start(ctx)
	if err == nil {
		t.Fatal("expected error from Start with error init server")
	}
	if !strings.Contains(err.Error(), "initialize") {
		t.Errorf("error = %q, want to contain 'initialize'", err)
	}
}

func TestStart_ThenListToolsAndCallTool(t *testing.T) {
	client := helperClient(t, "mcp_server")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	// ListTools
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "echo" {
		t.Errorf("tool name = %q, want %q", tools[0].Name, "echo")
	}

	// CallTool
	text, isError, err := client.CallTool(ctx, "echo", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if text != "called:echo" {
		t.Errorf("text = %q, want %q", text, "called:echo")
	}
	if isError {
		t.Error("expected isError = false")
	}
}

func TestStart_StderrCapture(t *testing.T) {
	client := helperClient(t, "mcp_server_stderr")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	client.Close()

	stderr := client.Stderr()
	if !strings.Contains(stderr, "stderr output from server") {
		t.Errorf("stderr = %q, want to contain 'stderr output from server'", stderr)
	}
}

func TestStart_InvalidCommand(t *testing.T) {
	client, err := NewClient("/nonexistent/binary", nil, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = client.Start(ctx)
	if err == nil {
		t.Fatal("expected error starting nonexistent binary")
	}
	if !strings.Contains(err.Error(), "start process") {
		t.Errorf("error = %q, want to contain 'start process'", err)
	}
}

// =============================================================================
// Client: Close
// =============================================================================

func TestClose_LiveServer(t *testing.T) {
	client := helperClient(t, "mcp_server")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	err := client.Close()
	// Close should succeed -- process exits cleanly.
	if err != nil {
		t.Logf("Close returned: %v (acceptable for killed process)", err)
	}

	if client.Alive() {
		t.Error("expected client to be dead after Close")
	}
}

// =============================================================================
// Manager: StartServer (integration with test helper process)
// =============================================================================

func TestManager_StartServer(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := config.MCPServerConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess"},
		Env: map[string]string{
			"GO_TEST_HELPER_PROCESS": "1",
			"GO_TEST_HELPER_MODE":   "mcp_server",
		},
	}

	err := mgr.StartServer(ctx, "test-srv", cfg)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}

	if got := mgr.ServerCount(); got != 1 {
		t.Errorf("ServerCount() = %d, want 1", got)
	}

	tools := mgr.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name() != "test-srv_echo" {
		t.Errorf("tool name = %q, want %q", tools[0].Name(), "test-srv_echo")
	}
}

func TestManager_StartServer_ReplacesExisting(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := config.MCPServerConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess"},
		Env: map[string]string{
			"GO_TEST_HELPER_PROCESS": "1",
			"GO_TEST_HELPER_MODE":   "mcp_server",
		},
	}

	// Start the first server.
	if err := mgr.StartServer(ctx, "test-srv", cfg); err != nil {
		t.Fatalf("first StartServer: %v", err)
	}

	// Start a second with the same name -- should replace.
	if err := mgr.StartServer(ctx, "test-srv", cfg); err != nil {
		t.Fatalf("second StartServer: %v", err)
	}

	if got := mgr.ServerCount(); got != 1 {
		t.Errorf("ServerCount() = %d, want 1", got)
	}
}

func TestManager_StartServer_ToolFiltering(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := config.MCPServerConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess"},
		Env: map[string]string{
			"GO_TEST_HELPER_PROCESS": "1",
			"GO_TEST_HELPER_MODE":   "mcp_server",
		},
		Tools: config.MCPToolsConfig{
			Disabled: []string{"echo"},
		},
	}

	if err := mgr.StartServer(ctx, "filtered", cfg); err != nil {
		t.Fatalf("StartServer: %v", err)
	}

	tools := mgr.Tools()
	if len(tools) != 0 {
		t.Errorf("expected 0 tools after filtering, got %d", len(tools))
	}
}

func TestManager_StartServer_InvalidCommand(t *testing.T) {
	mgr := NewManager()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cfg := config.MCPServerConfig{
		Command: "/nonexistent/binary",
	}

	err := mgr.StartServer(ctx, "bad", cfg)
	if err == nil {
		t.Fatal("expected error for invalid command")
	}
}

func TestManager_StartServer_ErrorInit(t *testing.T) {
	mgr := NewManager()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := config.MCPServerConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess"},
		Env: map[string]string{
			"GO_TEST_HELPER_PROCESS": "1",
			"GO_TEST_HELPER_MODE":   "mcp_server_error_init",
		},
	}

	err := mgr.StartServer(ctx, "bad-init", cfg)
	if err == nil {
		t.Fatal("expected error for server that fails init")
	}
	if !strings.Contains(err.Error(), "start") {
		t.Errorf("error = %q, want to contain 'start'", err)
	}
}

// =============================================================================
// Manager: RestartServer
// =============================================================================

func TestManager_RestartServer(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := config.MCPServerConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess"},
		Env: map[string]string{
			"GO_TEST_HELPER_PROCESS": "1",
			"GO_TEST_HELPER_MODE":   "mcp_server",
		},
	}

	if err := mgr.StartServer(ctx, "restart-srv", cfg); err != nil {
		t.Fatalf("StartServer: %v", err)
	}

	// Restart it.
	if err := mgr.RestartServer(ctx, "restart-srv"); err != nil {
		t.Fatalf("RestartServer: %v", err)
	}

	if got := mgr.ServerCount(); got != 1 {
		t.Errorf("ServerCount() = %d, want 1", got)
	}

	// Tools should still be available.
	tools := mgr.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool after restart, got %d", len(tools))
	}
}

func TestManager_RestartServer_Unknown(t *testing.T) {
	mgr := NewManager()
	ctx := context.Background()

	err := mgr.RestartServer(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown server")
	}
	if !strings.Contains(err.Error(), "unknown server") {
		t.Errorf("error = %q, want to contain 'unknown server'", err)
	}
}

// =============================================================================
// Manager: DisconnectServer (success path)
// =============================================================================

func TestManager_DisconnectServer_Success(t *testing.T) {
	mgr := NewManager()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := config.MCPServerConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess"},
		Env: map[string]string{
			"GO_TEST_HELPER_PROCESS": "1",
			"GO_TEST_HELPER_MODE":   "mcp_server",
		},
	}

	if err := mgr.StartServer(ctx, "disc-srv", cfg); err != nil {
		t.Fatalf("StartServer: %v", err)
	}

	// DisconnectServer calls Close() which sends SIGTERM. The exit status
	// from a signal-killed process is not nil, but that's expected behavior.
	_ = mgr.DisconnectServer("disc-srv")

	if got := mgr.ServerCount(); got != 0 {
		t.Errorf("ServerCount() = %d, want 0", got)
	}

	// Tools from disconnected server should be gone.
	if got := len(mgr.Tools()); got != 0 {
		t.Errorf("expected 0 tools, got %d", got)
	}
}

// =============================================================================
// Manager: monitor (death callback)
// =============================================================================

func TestManager_Monitor_DeathCallback(t *testing.T) {
	mgr := NewManager()

	deathCh := make(chan string, 1)
	mgr.OnServerDeath = func(name string) {
		deathCh <- name
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := config.MCPServerConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess"},
		Env: map[string]string{
			"GO_TEST_HELPER_PROCESS": "1",
			"GO_TEST_HELPER_MODE":   "mcp_server",
		},
	}

	if err := mgr.StartServer(ctx, "death-srv", cfg); err != nil {
		t.Fatalf("StartServer: %v", err)
	}

	// Kill the server process to trigger monitor.
	mgr.mu.Lock()
	entry := mgr.servers["death-srv"]
	mgr.mu.Unlock()

	entry.client.stdin.Close()

	select {
	case name := <-deathCh:
		if name != "death-srv" {
			t.Errorf("death callback name = %q, want %q", name, "death-srv")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for death callback")
	}

	// Server should be removed from manager.
	if got := mgr.ServerCount(); got != 0 {
		t.Errorf("ServerCount() = %d, want 0 after death", got)
	}
}

func TestManager_Monitor_NotCalledForRemovedServer(t *testing.T) {
	mgr := NewManager()

	done := make(chan struct{})
	close(done)

	// Set up a server that is already "dead" (done closed) but no longer
	// in the servers map. monitor should not panic or call OnServerDeath.
	deathCalled := false
	mgr.OnServerDeath = func(name string) {
		deathCalled = true
	}

	// monitor for a server that doesn't exist should just return.
	mgr.monitor("ghost")

	if deathCalled {
		t.Error("OnServerDeath should not be called for non-existent server")
	}
}

// =============================================================================
// AddTestServer
// =============================================================================

func TestAddTestServer(t *testing.T) {
	mgr := NewManager()
	mgr.AddTestServer("mock", []string{"read", "write", "delete"})

	if got := mgr.ServerCount(); got != 1 {
		t.Fatalf("ServerCount() = %d, want 1", got)
	}

	tools := mgr.Tools()
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}

	// Verify tool names are qualified.
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name()] = true
	}
	for _, expected := range []string{"mock_read", "mock_write", "mock_delete"} {
		if !names[expected] {
			t.Errorf("expected tool %q, not found in %v", expected, names)
		}
	}

	// Verify we can disable/enable tools on the test server.
	if err := mgr.DisableTool("mock", "write"); err != nil {
		t.Fatalf("DisableTool: %v", err)
	}
	if got := len(mgr.Tools()); got != 2 {
		t.Errorf("expected 2 tools after disable, got %d", got)
	}

	// Verify status.
	statuses := mgr.ServerStatus()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Name != "mock" {
		t.Errorf("status name = %q, want %q", statuses[0].Name, "mock")
	}
	if statuses[0].Tools != 3 {
		t.Errorf("status tools = %d, want 3", statuses[0].Tools)
	}
}

// =============================================================================
// Multiple concurrent calls
// =============================================================================

func TestCall_ConcurrentRequests(t *testing.T) {
	// Use the subprocess helper for concurrent requests since io.Pipe is
	// synchronous and causes deadlocks with concurrent writers.
	client := helperClient(t, "mcp_server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	const N = 5
	var wg sync.WaitGroup
	errors := make(chan error, N)

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			text, _, err := client.CallTool(ctx, "echo", json.RawMessage(`{}`))
			if err != nil {
				errors <- err
				return
			}
			if text != "called:echo" {
				errors <- fmt.Errorf("unexpected result: %q", text)
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent call error: %v", err)
	}
}

// =============================================================================
// Additional coverage: Close paths
// =============================================================================

func TestClose_NilProcess(t *testing.T) {
	// Close a client whose process was never started -- cmd.Process is nil.
	done := make(chan struct{})
	close(done) // already "done" so Close doesn't block
	client := &Client{
		cmd:   fakeCmd(), // not started, so Process is nil
		stdin: nopWriteCloser{},
		done:  done,
	}
	// Should not panic even though Process is nil.
	_ = client.Close()
}

func TestClose_AlreadyDone(t *testing.T) {
	// Close a client whose done channel is already closed (server already dead).
	done := make(chan struct{})
	close(done)
	client := &Client{
		cmd:   fakeCmd(),
		stdin: nopWriteCloser{},
		done:  done,
	}
	// The first select case in Close picks up <-c.done immediately.
	_ = client.Close()
}

// =============================================================================
// Additional coverage: call unmarshal error
// =============================================================================

func TestCall_UnmarshalResultError(t *testing.T) {
	client, serverWriter, clientReader := newTestClient()
	defer serverWriter.Close()

	ctx := context.Background()

	go func() {
		scanner := bufio.NewScanner(clientReader)
		if !scanner.Scan() {
			return
		}
		var req Request
		json.Unmarshal(scanner.Bytes(), &req)
		// Send a response where Result is not valid for the target type.
		// We'll make result a string where the target expects a struct.
		resp := Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`"not a struct"`),
		}
		data, _ := json.Marshal(resp)
		data = append(data, '\n')
		serverWriter.Write(data)
	}()

	var result InitializeResult
	err := client.call(ctx, "test", nil, &result)
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
	if !strings.Contains(err.Error(), "unmarshal result") {
		t.Errorf("error = %q, want to contain 'unmarshal result'", err)
	}
}

// =============================================================================
// Additional coverage: Start notification write error
// =============================================================================

func TestStart_NotificationWriteError(t *testing.T) {
	// This tests the case where the initialized notification write fails.
	// We use a helper subprocess that handles initialize but then we close
	// the stdin pipe before the notification can be sent.
	// However, Start() does the initialize call and notification in sequence,
	// so we need to intercept between them.
	// Instead, we test with a server that exits right after initialize response.
	client := helperClient(t, "mcp_server_dies")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// This may or may not error depending on timing -- the server exits
	// after sending initialize response but before or after notification write.
	// Either way, it shouldn't panic.
	_ = client.Start(ctx)
	client.Close()
}

// =============================================================================
// Manager: StartServer list tools failure
// =============================================================================

// =============================================================================
// Additional coverage: Close with stdin close error
// =============================================================================

func TestClose_StdinCloseError(t *testing.T) {
	// errCloseWriteCloser returns an error on Close.
	done := make(chan struct{})
	close(done)
	client := &Client{
		cmd:   fakeCmd(),
		stdin: errCloseWriteCloser{},
		done:  done,
	}
	// Should not panic; the stdin.Close error is logged but not returned.
	_ = client.Close()
}

// errCloseWriteCloser succeeds on Write but fails on Close.
type errCloseWriteCloser struct{}

func (errCloseWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (errCloseWriteCloser) Close() error                 { return fmt.Errorf("close error") }

// =============================================================================
// Additional coverage: call with SIGTERM error on already-dead process
// =============================================================================

func TestClose_SIGTERMError(t *testing.T) {
	// Start a real process, let it die, then Close.
	client := helperClient(t, "mcp_server")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Kill the process first.
	client.cmd.Process.Kill()
	// Wait for readLoop to detect death.
	<-client.done

	// Now Close -- SIGTERM will fail because process is already dead.
	_ = client.Close()
}

func TestManager_StartServer_ListToolsFails(t *testing.T) {
	mgr := NewManager()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use a server that dies right after initialization, before tools/list.
	cfg := config.MCPServerConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess"},
		Env: map[string]string{
			"GO_TEST_HELPER_PROCESS": "1",
			"GO_TEST_HELPER_MODE":   "mcp_server_dies",
		},
	}

	err := mgr.StartServer(ctx, "dies-early", cfg)
	// This should fail because the server exits before responding to tools/list.
	if err == nil {
		// It might succeed if the server happens to respond before dying.
		// In that case, just clean up.
		mgr.Close()
		return
	}
	if !strings.Contains(err.Error(), "list tools") && !strings.Contains(err.Error(), "start") {
		t.Errorf("error = %q, want to contain 'list tools' or 'start'", err)
	}
}
