package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justin/glamdring/pkg/agent"
	"github.com/justin/glamdring/pkg/index"
)

// BuiltinHandler processes a built-in slash command. It may modify the model
// and returns a tea.Cmd to execute (or nil).
type BuiltinHandler func(m *Model, args string) tea.Cmd

// builtinCommands maps command names to their handlers.
var builtinCommands = map[string]BuiltinHandler{
	"help":     cmdHelp,
	"quit":     cmdQuit,
	"clear":    cmdClear,
	"cost":     cmdCost,
	"config":   cmdConfig,
	"model":    cmdModel,
	"compact":  cmdCompact,
	"index":    cmdIndex,
	"thinking": cmdThinking,
	"yolo":     cmdYolo,
	"mcp":      cmdMCP,
	"export":   cmdExport,
	"copy":     cmdCopy,
}

// builtinDescriptions provides short help text for each built-in command.
var builtinDescriptions = map[string]string{
	"help":     "Show available commands",
	"quit":     "Exit glamdring",
	"clear":    "Clear output and reset counters",
	"cost":     "Show token usage and cost",
	"config":   "Show current configuration",
	"model":    "Show or change the model",
	"compact":  "Summarize and compress context",
	"index":    "Show index status or rebuild (shire)",
	"thinking": "Toggle thinking block display",
	"yolo":     "Toggle auto-approve (optionally scope: /yolo bash,write)",
	"mcp":      "Manage MCP servers and tools",
	"export":   "Export conversation (--html for HTML format)",
	"copy":     "Copy last response to clipboard",
}

// BuiltinNames returns a sorted list of built-in command names.
func BuiltinNames() []string {
	names := make([]string, 0, len(builtinCommands))
	for name := range builtinCommands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DispatchBuiltin checks if the command name matches a built-in and executes it.
// Returns the handler and true if found, nil and false otherwise.
func DispatchBuiltin(name string) (BuiltinHandler, bool) {
	h, ok := builtinCommands[name]
	return h, ok
}

// cmdHelp lists all available commands.
func cmdHelp(m *Model, args string) tea.Cmd {
	var b strings.Builder
	b.WriteString("Built-in commands:\n")
	names := make([]string, 0, len(builtinDescriptions))
	for name := range builtinDescriptions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		b.WriteString(fmt.Sprintf("  /%s — %s\n", name, builtinDescriptions[name]))
	}

	if m.cmdRegistry != nil {
		userNames := m.cmdRegistry.Names()
		if len(userNames) > 0 {
			b.WriteString("\nUser-defined commands:\n")
			for _, name := range userNames {
				b.WriteString(fmt.Sprintf("  /%s\n", name))
			}
		}
	}

	m.output.AppendSystem(strings.TrimRight(b.String(), "\n"))
	return nil
}

// cmdQuit exits the program.
func cmdQuit(m *Model, args string) tea.Cmd {
	return tea.Quit
}

// cmdClear resets the output viewport, token counters, and conversation history.
func cmdClear(m *Model, args string) tea.Cmd {
	m.output.Clear()
	m.statusbar.Reset()
	m.totalInputTokens = 0
	m.totalOutputTokens = 0
	m.turn = 0
	m.statusbar.Update(m.agentCfg.Model, 0, 0, 0)
	if m.session != nil {
		m.session.Reset()
	}
	return nil
}

// cmdCost displays cumulative token usage and estimated cost.
func cmdCost(m *Model, args string) tea.Cmd {
	cost := costForModel(m.agentCfg.Model, m.totalInputTokens, m.totalOutputTokens)

	text := fmt.Sprintf(
		"Token usage:\n  Input:  %s\n  Output: %s\n  Cost:   $%.4f\n  Turns:  %d",
		formatTokens(m.totalInputTokens),
		formatTokens(m.totalOutputTokens),
		cost,
		m.turn,
	)
	m.output.AppendSystem(text)
	return nil
}

// cmdConfig displays the current configuration.
func cmdConfig(m *Model, args string) tea.Cmd {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Model:     %s\n", m.agentCfg.Model))

	if m.agentCfg.MaxTurns != nil && *m.agentCfg.MaxTurns > 0 {
		b.WriteString(fmt.Sprintf("Max turns: %d\n", *m.agentCfg.MaxTurns))
	} else {
		b.WriteString("Max turns: unlimited\n")
	}

	if m.agentCfg.CWD != "" {
		b.WriteString(fmt.Sprintf("CWD:       %s\n", m.agentCfg.CWD))
	}

	if m.indexDB != nil {
		b.WriteString(fmt.Sprintf("Indexer:    %s (auto-rebuild: %v)\n", m.indexerCfg.IndexerCommand(), m.indexerCfg.IndexerAutoRebuild()))
	} else if enabled := m.indexerCfg.IndexerEnabled(); enabled != nil && !*enabled {
		b.WriteString("Indexer:    disabled\n")
	}

	if m.settings.MCPServers != nil && len(m.settings.MCPServers) > 0 {
		names := make([]string, 0, len(m.settings.MCPServers))
		for name := range m.settings.MCPServers {
			names = append(names, name)
		}
		sort.Strings(names)
		b.WriteString(fmt.Sprintf("MCP servers: %s", strings.Join(names, ", ")))
	}

	m.output.AppendSystem(strings.TrimRight(b.String(), "\n"))
	return nil
}

// cmdModel shows or changes the current model.
func cmdModel(m *Model, args string) tea.Cmd {
	if args == "" {
		m.output.AppendSystem(fmt.Sprintf("Current model: %s", m.agentCfg.Model))
		return nil
	}

	m.agentCfg.Model = args
	m.session = nil // force recreation with new model on next submit
	m.statusbar.Update(args, m.totalInputTokens, m.totalOutputTokens, m.turn)
	m.output.AppendSystem(fmt.Sprintf("Model changed to: %s", args))
	return nil
}

// cmdCompact sends a summarization prompt to the agent, then truncates context.
func cmdCompact(m *Model, args string) tea.Cmd {
	m.compacting = true
	m.state = StateRunning
	m.spinning = true

	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	turnCtx, cancel := context.WithCancel(ctx)
	m.cancelTurn = cancel

	if m.session == nil {
		m.session = agent.NewSession(m.agentCfg)
	}
	ch := m.session.Turn(turnCtx, compactPrompt)
	return tea.Batch(
		func() tea.Msg { return agentStartedMsg{ch: ch} },
		m.spinner.Tick,
	)
}

// cmdIndex shows index status or triggers a rebuild via shire.
func cmdIndex(m *Model, args string) tea.Cmd {
	switch strings.TrimSpace(args) {
	case "rebuild":
		return cmdIndexRebuild(m)
	default:
		return cmdIndexStatus(m)
	}
}

func cmdIndexStatus(m *Model) tea.Cmd {
	if m.indexDB == nil {
		m.output.AppendSystem("No shire index found. Run /index rebuild or shire build to create one.")
		return nil
	}
	status, err := m.indexDB.IndexStatus()
	if err != nil {
		m.output.AppendError(fmt.Sprintf("index status error: %s", err))
		return nil
	}
	var b strings.Builder
	b.WriteString("Shire index status:\n")
	if status.IndexedAt != nil {
		b.WriteString(fmt.Sprintf("  Built:    %s\n", *status.IndexedAt))
	}
	if status.GitCommit != nil {
		b.WriteString(fmt.Sprintf("  Commit:   %s\n", *status.GitCommit))
	}
	if status.PackageCount != nil {
		b.WriteString(fmt.Sprintf("  Packages: %s\n", *status.PackageCount))
	}
	if status.SymbolCount != nil {
		b.WriteString(fmt.Sprintf("  Symbols:  %s\n", *status.SymbolCount))
	}
	if status.FileCount != nil {
		b.WriteString(fmt.Sprintf("  Files:    %s\n", *status.FileCount))
	}
	if status.TotalDurationMs != nil {
		b.WriteString(fmt.Sprintf("  Duration: %sms", *status.TotalDurationMs))
	}
	m.output.AppendSystem(strings.TrimRight(b.String(), "\n"))
	return nil
}

func cmdIndexRebuild(m *Model) tea.Cmd {
	cmdName := m.indexerCfg.IndexerCommand()
	binPath, err := exec.LookPath(cmdName)
	if err != nil {
		m.output.AppendError(fmt.Sprintf("%s is not installed. Install with: brew tap justinjdev/shire && brew install shire", cmdName))
		return nil
	}

	cwd := m.agentCfg.CWD
	if cwd == "" {
		m.output.AppendError("no working directory set")
		return nil
	}

	m.output.AppendSystem(fmt.Sprintf("Rebuilding index via %s...", cmdName))

	cmd := exec.Command(binPath, "build", "--root", cwd)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	if err != nil {
		m.output.AppendError(fmt.Sprintf("%s build failed: %s\n%s", cmdName, err, string(output)))
		return nil
	}

	// Reopen the database.
	dbPath := filepath.Join(cwd, ".shire", "index.db")
	newDB, err := index.Open(dbPath)
	if err != nil {
		m.output.AppendError(fmt.Sprintf("failed to open rebuilt index: %s", err))
		return nil
	}

	// Close old DB if any, swap in new one.
	if m.indexDB != nil {
		m.indexDB.Close()
	}
	m.indexDB = newDB

	// Show updated status.
	return cmdIndexStatus(m)
}

// cmdThinking toggles display of thinking blocks.
func cmdThinking(m *Model, args string) tea.Cmd {
	m.showThinking = !m.showThinking
	if m.showThinking {
		m.output.AppendSystem("Thinking display enabled.")
	} else {
		m.output.AppendSystem("Thinking display disabled.")
	}
	return nil
}

// cmdYolo toggles yolo mode or scopes it to specific tools.
func cmdYolo(m *Model, args string) tea.Cmd {
	if m.session == nil {
		m.session = agent.NewSession(m.agentCfg)
	}

	args = strings.TrimSpace(args)
	if args != "" {
		// Scoped yolo: /yolo bash,write
		toolNames := strings.Split(args, ",")
		for i := range toolNames {
			toolNames[i] = strings.TrimSpace(toolNames[i])
		}
		m.session.SetYoloScoped(toolNames)
		m.output.AppendSystem(fmt.Sprintf("Auto-approve enabled for: %s", strings.Join(toolNames, ", ")))
		return nil
	}

	// Toggle global yolo.
	m.session.ToggleYolo()
	m.statusbar.SetYolo(m.session.IsYolo())
	if m.session.IsYolo() {
		m.output.AppendSystem("YOLO mode enabled — all tools auto-approved.")
	} else {
		m.output.AppendSystem("YOLO mode disabled — tool permissions restored.")
	}
	return nil
}

// cmdMCP shows MCP server status or manages servers and tools.
// Subcommands:
//
//	/mcp               - list servers
//	/mcp restart <name> - restart a server
//	/mcp disconnect <name> - disconnect a server
//	/mcp tools <name>  - list tools on a server
//	/mcp enable <server> <tool> - re-enable a disabled tool
//	/mcp disable <server> <tool> - disable a tool
func cmdMCP(m *Model, args string) tea.Cmd {
	if m.mcpMgr == nil {
		m.output.AppendSystem("No MCP servers configured.")
		return nil
	}

	parts := strings.Fields(args)
	if len(parts) == 0 {
		return cmdMCPList(m)
	}

	switch parts[0] {
	case "restart":
		if len(parts) < 2 {
			m.output.AppendError("Usage: /mcp restart <server-name>")
			return nil
		}
		return cmdMCPRestart(m, parts[1])
	case "disconnect":
		if len(parts) < 2 {
			m.output.AppendError("Usage: /mcp disconnect <server-name>")
			return nil
		}
		return cmdMCPDisconnect(m, parts[1])
	case "tools":
		if len(parts) < 2 {
			m.output.AppendError("Usage: /mcp tools <server-name>")
			return nil
		}
		return cmdMCPTools(m, parts[1])
	case "enable":
		if len(parts) < 3 {
			m.output.AppendError("Usage: /mcp enable <server-name> <tool-name>")
			return nil
		}
		return cmdMCPEnableTool(m, parts[1], parts[2])
	case "disable":
		if len(parts) < 3 {
			m.output.AppendError("Usage: /mcp disable <server-name> <tool-name>")
			return nil
		}
		return cmdMCPDisableTool(m, parts[1], parts[2])
	default:
		m.output.AppendError(fmt.Sprintf("Unknown /mcp subcommand: %s", parts[0]))
		m.output.AppendSystem("Usage: /mcp [restart|disconnect|tools|enable|disable] ...")
		return nil
	}
}

func cmdMCPList(m *Model) tea.Cmd {
	servers := m.mcpMgr.ServerStatus()
	if len(servers) == 0 {
		m.output.AppendSystem("No MCP servers running.")
		return nil
	}

	var b strings.Builder
	b.WriteString("MCP servers:\n")
	for _, s := range servers {
		status := "alive"
		if !s.Alive {
			status = "dead"
		}
		b.WriteString(fmt.Sprintf("  %s — %s (%d tools)\n", s.Name, status, s.Tools))
	}
	m.output.AppendSystem(strings.TrimRight(b.String(), "\n"))
	return nil
}

func cmdMCPRestart(m *Model, name string) tea.Cmd {
	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if err := m.mcpMgr.RestartServer(ctx, name); err != nil {
		m.output.AppendError(fmt.Sprintf("Failed to restart %q: %s", name, err))
		return nil
	}
	m.output.AppendSystem(fmt.Sprintf("Restarted MCP server %q", name))
	m.statusbar.UpdateMCP(m.mcpConfiguredCount, m.mcpMgr.ServerCount())
	m.refreshMCPTools()
	return nil
}

func cmdMCPDisconnect(m *Model, name string) tea.Cmd {
	if err := m.mcpMgr.DisconnectServer(name); err != nil {
		m.output.AppendError(fmt.Sprintf("Failed to disconnect %q: %s", name, err))
		return nil
	}
	if m.mcpConfiguredCount > 0 {
		m.mcpConfiguredCount--
	}
	m.output.AppendSystem(fmt.Sprintf("Disconnected MCP server %q", name))
	m.statusbar.UpdateMCP(m.mcpConfiguredCount, m.mcpMgr.ServerCount())
	m.refreshMCPTools()
	return nil
}

func cmdMCPTools(m *Model, serverName string) tea.Cmd {
	toolInfos, err := m.mcpMgr.ListServerTools(serverName)
	if err != nil {
		m.output.AppendError(err.Error())
		return nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Tools for %s:\n", serverName))
	for _, t := range toolInfos {
		status := "enabled"
		if t.Disabled {
			status = "disabled"
		}
		b.WriteString(fmt.Sprintf("  %s — %s\n", t.Name, status))
	}
	m.output.AppendSystem(strings.TrimRight(b.String(), "\n"))
	return nil
}

func cmdMCPEnableTool(m *Model, serverName, toolName string) tea.Cmd {
	if err := m.mcpMgr.EnableTool(serverName, toolName); err != nil {
		m.output.AppendError(err.Error())
		return nil
	}
	m.output.AppendSystem(fmt.Sprintf("Enabled tool %q on server %q", toolName, serverName))
	m.refreshMCPTools()
	return nil
}

func cmdMCPDisableTool(m *Model, serverName, toolName string) tea.Cmd {
	if err := m.mcpMgr.DisableTool(serverName, toolName); err != nil {
		m.output.AppendError(err.Error())
		return nil
	}
	m.output.AppendSystem(fmt.Sprintf("Disabled tool %q on server %q", toolName, serverName))
	m.refreshMCPTools()
	return nil
}

// cmdExport exports the conversation to a file. Supports --html flag for HTML format.
func cmdExport(m *Model, args string) tea.Cmd {
	if m.session == nil {
		m.output.AppendError("No conversation to export.")
		return nil
	}

	msgs := m.session.Messages()
	if len(msgs) == 0 {
		m.output.AppendError("No conversation to export.")
		return nil
	}

	fields := strings.Fields(args)
	useHTML := false
	var outPath string

	for _, f := range fields {
		if f == "--html" {
			useHTML = true
		} else {
			outPath = f
		}
	}

	if outPath == "" {
		ts := time.Now().Format("20060102-150405")
		ext := "md"
		if useHTML {
			ext = "html"
		}
		outPath = fmt.Sprintf("conversation-%s.%s", ts, ext)
	}

	var content string
	if useHTML {
		content = exportHTML(msgs)
	} else {
		content = exportMarkdown(msgs)
	}

	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		m.output.AppendError(fmt.Sprintf("Failed to write export: %s", err))
		return nil
	}

	m.output.AppendSystem(fmt.Sprintf("Conversation exported to %s", outPath))
	return nil
}

// cmdCopy copies the last assistant response to the system clipboard.
func cmdCopy(m *Model, args string) tea.Cmd {
	var text string
	for i := len(m.output.blocks) - 1; i >= 0; i-- {
		b := m.output.blocks[i]
		if b.kind == blockText && strings.TrimSpace(b.content) != "" {
			text = strings.TrimSpace(b.content)
			break
		}
	}

	if text == "" {
		m.output.AppendError("No response to copy.")
		return nil
	}

	WriteClipboardText(text)

	lines := strings.Count(text, "\n") + 1
	m.output.AppendSystem(fmt.Sprintf("Copied %d lines to clipboard.", lines))
	return nil
}

const compactPrompt = `Summarize our conversation so far into a compact context block. Be aggressive about compression — discard noise, keep only what matters for continuing work.

Output in this exact format:

## Compacted Context

### Task
[one-line description of what we're working on]

### Key Findings
- [decisions made]
- [constraints discovered]
- [patterns identified]

### Files
- [file:lines] — [what's relevant and why]

### Current State
- [what's been done]
- [what's working / broken]

### Next Steps
- [what needs to happen next]
- [open questions]

Rules:
- Discard raw search/grep output — keep only conclusions
- Discard full file contents — keep only relevant line ranges
- Discard verbose build/test output — keep only pass/fail
- Discard exploratory dead ends — keep only what was learned
- Keep it under 40 lines total`
