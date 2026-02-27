package mcp

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/justin/glamdring/pkg/config"
	"github.com/justin/glamdring/pkg/tools"
)

// Manager manages the lifecycle of multiple MCP server processes and exposes
// their tools through the tools.Tool interface.
type Manager struct {
	mu      sync.Mutex
	servers map[string]*serverEntry
}

type serverEntry struct {
	client *Client
	tools  []*MCPTool
}

// NewManager creates an empty MCP manager.
func NewManager() *Manager {
	return &Manager{
		servers: make(map[string]*serverEntry),
	}
}

// StartServer launches an MCP server, performs the initialize handshake, and
// discovers its tools.
func (m *Manager) StartServer(ctx context.Context, name string, cfg config.MCPServerConfig) error {
	client, err := NewClient(cfg.Command, cfg.Args)
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

	adapted := make([]*MCPTool, len(defs))
	for i, def := range defs {
		adapted[i] = NewMCPTool(client, name, def)
	}

	m.mu.Lock()
	m.servers[name] = &serverEntry{client: client, tools: adapted}
	m.mu.Unlock()

	// Monitor the server process and remove tools if it crashes.
	go m.monitor(name)

	log.Printf("mcp: started server %q with %d tools", name, len(adapted))
	return nil
}

// Tools returns all tools from all running MCP servers.
func (m *Manager) Tools() []tools.Tool {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []tools.Tool
	for _, entry := range m.servers {
		for _, t := range entry.tools {
			out = append(out, t)
		}
	}
	return out
}

// Close shuts down all MCP servers.
func (m *Manager) Close() {
	m.mu.Lock()
	servers := make(map[string]*serverEntry, len(m.servers))
	for k, v := range m.servers {
		servers[k] = v
	}
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
	m.mu.Unlock()
	if !ok {
		return
	}

	// Block until the client's reader loop signals the process is gone.
	<-entry.client.done

	m.mu.Lock()
	delete(m.servers, name)
	m.mu.Unlock()

	log.Printf("mcp: server %q exited unexpectedly, its tools are no longer available", name)
}
