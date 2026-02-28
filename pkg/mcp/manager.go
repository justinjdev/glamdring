package mcp

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"

	"github.com/justin/glamdring/pkg/config"
	"github.com/justin/glamdring/pkg/tools"
)

// Manager manages the lifecycle of multiple MCP server processes and exposes
// their tools through the tools.Tool interface.
type Manager struct {
	mu            sync.Mutex
	servers       map[string]*serverEntry
	disabledTools map[string]map[string]bool // server -> tool -> disabled (runtime)

	// OnServerDeath is called when a server process exits unexpectedly.
	// It is invoked from a monitor goroutine; implementations must be safe
	// to call from any goroutine.
	OnServerDeath func(name string)
}

type serverEntry struct {
	client *Client
	tools  []*MCPTool
	config config.MCPServerConfig
}

// NewManager creates an empty MCP manager.
func NewManager() *Manager {
	return &Manager{
		servers:       make(map[string]*serverEntry),
		disabledTools: make(map[string]map[string]bool),
	}
}

// StartServer launches an MCP server, performs the initialize handshake, and
// discovers its tools. If a server with the same name is already running, the
// old server is closed first.
func (m *Manager) StartServer(ctx context.Context, name string, cfg config.MCPServerConfig) error {
	// Close any existing server with the same name.
	// Delete the entry before unlocking so monitor sees !stillExists
	// and doesn't fire OnServerDeath for the replaced server.
	m.mu.Lock()
	if existing, ok := m.servers[name]; ok {
		delete(m.servers, name)
		m.mu.Unlock()
		log.Printf("mcp: replacing existing server %q", name)
		if err := existing.client.Close(); err != nil {
			log.Printf("mcp: warning closing old %q: %v", name, err)
		}
	} else {
		m.mu.Unlock()
	}

	client, err := NewClient(cfg.Command, cfg.Args, cfg.EnvSlice())
	if err != nil {
		return fmt.Errorf("mcp %s: create client: %w", name, err)
	}

	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("mcp %s: start: %w", name, err)
	}

	defs, err := client.ListTools(ctx)
	if err != nil {
		_ = client.Close()
		return fmt.Errorf("mcp %s: list tools: %w", name, err)
	}

	// Apply config-level tool filtering.
	defs = filterToolDefs(defs, cfg.Tools)

	adapted := make([]*MCPTool, len(defs))
	for i, def := range defs {
		adapted[i] = NewMCPTool(client, name, def)
	}

	m.mu.Lock()
	m.servers[name] = &serverEntry{client: client, tools: adapted, config: cfg}
	m.mu.Unlock()

	// Monitor the server process and remove tools if it crashes.
	go m.monitor(name)

	log.Printf("mcp: started server %q with %d tools", name, len(adapted))
	return nil
}

// Tools returns all tools from all running MCP servers, respecting runtime
// disable overrides.
func (m *Manager) Tools() []tools.Tool {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []tools.Tool
	for serverName, entry := range m.servers {
		disabled := m.disabledTools[serverName]
		for _, t := range entry.tools {
			if disabled != nil && disabled[t.mcpName] {
				continue
			}
			out = append(out, t)
		}
	}
	return out
}

// ServerCount returns the number of currently running MCP servers.
func (m *Manager) ServerCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.servers)
}

// MCPServerStatus holds status information about an MCP server.
type MCPServerStatus struct {
	Name  string
	Alive bool
	Tools int
}

// ServerStatus returns status info for all running servers, sorted by name.
func (m *Manager) ServerStatus() []MCPServerStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]MCPServerStatus, 0, len(m.servers))
	for name, entry := range m.servers {
		out = append(out, MCPServerStatus{
			Name:  name,
			Alive: entry.client.Alive(),
			Tools: len(entry.tools),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// RestartServer closes and re-launches a server by name using its stored config.
func (m *Manager) RestartServer(ctx context.Context, name string) error {
	m.mu.Lock()
	entry, ok := m.servers[name]
	var cfg config.MCPServerConfig
	if ok {
		cfg = entry.config
	}
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("unknown server %q", name)
	}

	if err := entry.client.Close(); err != nil {
		log.Printf("mcp: warning closing %q for restart: %v", name, err)
	}
	return m.StartServer(ctx, name, cfg)
}

// DisconnectServer stops an MCP server and removes it from the manager.
func (m *Manager) DisconnectServer(name string) error {
	m.mu.Lock()
	entry, ok := m.servers[name]
	if ok {
		delete(m.servers, name)
		delete(m.disabledTools, name)
	}
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("unknown server %q", name)
	}
	return entry.client.Close()
}

// ToolInfo describes a single tool on a server.
type ToolInfo struct {
	Name     string
	Disabled bool
}

// ListServerTools returns all tools for a server with their enabled/disabled state.
func (m *Manager) ListServerTools(serverName string) ([]ToolInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.servers[serverName]
	if !ok {
		return nil, fmt.Errorf("unknown server %q", serverName)
	}

	disabled := m.disabledTools[serverName]
	out := make([]ToolInfo, len(entry.tools))
	for i, t := range entry.tools {
		out[i] = ToolInfo{
			Name:     t.mcpName,
			Disabled: disabled != nil && disabled[t.mcpName],
		}
	}
	return out, nil
}

// DisableTool disables a specific tool on a server at runtime (session-only).
func (m *Manager) DisableTool(serverName, toolName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.servers[serverName]
	if !ok {
		return fmt.Errorf("unknown server %q", serverName)
	}

	found := false
	for _, t := range entry.tools {
		if t.mcpName == toolName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("tool %q not found on server %q", toolName, serverName)
	}

	if m.disabledTools[serverName] == nil {
		m.disabledTools[serverName] = make(map[string]bool)
	}
	m.disabledTools[serverName][toolName] = true
	return nil
}

// EnableTool re-enables a previously disabled tool (session-only).
func (m *Manager) EnableTool(serverName, toolName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.servers[serverName]
	if !ok {
		return fmt.Errorf("unknown server %q", serverName)
	}

	found := false
	for _, t := range entry.tools {
		if t.mcpName == toolName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("tool %q not found on server %q", toolName, serverName)
	}

	if m.disabledTools[serverName] != nil {
		delete(m.disabledTools[serverName], toolName)
	}
	return nil
}

// Close shuts down all MCP servers.
func (m *Manager) Close() {
	m.mu.Lock()
	servers := make(map[string]*serverEntry, len(m.servers))
	for k, v := range m.servers {
		servers[k] = v
	}
	m.servers = make(map[string]*serverEntry)
	m.mu.Unlock()

	for name, entry := range servers {
		if err := entry.client.Close(); err != nil {
			log.Printf("mcp: error closing server %q: %v", name, err)
		}
	}
}

// monitor waits for a server's reader loop to exit (indicating process death)
// and removes the server's tools.
func (m *Manager) monitor(name string) {
	m.mu.Lock()
	entry, ok := m.servers[name]
	if !ok {
		m.mu.Unlock()
		return
	}
	client := entry.client
	m.mu.Unlock()

	// Block until the client's reader loop signals the process is gone.
	<-client.done

	m.mu.Lock()
	// Only remove if the server entry still has our client — a restart may
	// have replaced it with a new entry while we were waiting.
	current, stillExists := m.servers[name]
	var cb func(string)
	if stillExists && current.client == client {
		delete(m.servers, name)
		cb = m.OnServerDeath
	}
	m.mu.Unlock()

	if cb != nil {
		log.Printf("mcp: server %q exited unexpectedly, its tools are no longer available", name)
		cb(name)
	}
}

// filterToolDefs applies allowlist/denylist filtering to tool definitions.
// Allowlist (Enabled) takes precedence if both are set.
func filterToolDefs(defs []ToolDefinition, tc config.MCPToolsConfig) []ToolDefinition {
	if len(tc.Enabled) > 0 {
		allowed := make(map[string]bool, len(tc.Enabled))
		for _, name := range tc.Enabled {
			allowed[name] = true
		}
		var filtered []ToolDefinition
		for _, def := range defs {
			if allowed[def.Name] {
				filtered = append(filtered, def)
			}
		}
		return filtered
	}

	if len(tc.Disabled) > 0 {
		blocked := make(map[string]bool, len(tc.Disabled))
		for _, name := range tc.Disabled {
			blocked[name] = true
		}
		var filtered []ToolDefinition
		for _, def := range defs {
			if !blocked[def.Name] {
				filtered = append(filtered, def)
			}
		}
		return filtered
	}

	return defs
}
