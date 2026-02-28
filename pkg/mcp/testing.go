package mcp

// AddTestServer adds a fake server entry to the manager for testing purposes.
// The server has no real process — it exists only to exercise tool listing,
// enable/disable, and status logic.
func (m *Manager) AddTestServer(name string, toolNames []string) {
	adapted := make([]*MCPTool, len(toolNames))
	for i, tn := range toolNames {
		adapted[i] = &MCPTool{
			name:    name + "_" + tn,
			mcpName: tn,
		}
	}

	done := make(chan struct{})
	m.mu.Lock()
	m.servers[name] = &serverEntry{
		client: &Client{done: done},
		tools:  adapted,
	}
	m.mu.Unlock()
}
