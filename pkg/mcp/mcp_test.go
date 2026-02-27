package mcp

import (
	"testing"

	"github.com/justin/glamdring/pkg/config"
)

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

func TestNewClientStderrDiscarded(t *testing.T) {
	client, err := NewClient("echo", []string{"hello"}, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.cmd.Stderr == nil {
		t.Error("expected cmd.Stderr to be set (io.Discard)")
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
