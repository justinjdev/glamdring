package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// BuiltinHandler processes a built-in slash command. It may modify the model
// and returns a tea.Cmd to execute (or nil).
type BuiltinHandler func(m *Model, args string) tea.Cmd

// builtinCommands maps command names to their handlers.
var builtinCommands = map[string]BuiltinHandler{
	"help":    cmdHelp,
	"quit":    cmdQuit,
	"clear":   cmdClear,
	"cost":    cmdCost,
	"config":  cmdConfig,
	"model":   cmdModel,
	"compact": cmdCompact,
}

// builtinDescriptions provides short help text for each built-in command.
var builtinDescriptions = map[string]string{
	"help":    "Show available commands",
	"quit":    "Exit glamdring",
	"clear":   "Clear output and reset counters",
	"cost":    "Show token usage and cost",
	"config":  "Show current configuration",
	"model":   "Show or change the model",
	"compact": "Summarize and compress context",
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

// cmdClear resets the output viewport and token counters.
func cmdClear(m *Model, args string) tea.Cmd {
	m.output.Clear()
	m.statusbar.Reset()
	m.totalInputTokens = 0
	m.totalOutputTokens = 0
	m.turn = 0
	m.statusbar.Update(m.agentCfg.Model, 0, 0, 0)
	return nil
}

// cmdCost displays cumulative token usage and estimated cost.
func cmdCost(m *Model, args string) tea.Cmd {
	cost := float64(m.totalInputTokens)/1_000_000*opusInputCostPerMillion +
		float64(m.totalOutputTokens)/1_000_000*opusOutputCostPerMillion

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

	if m.agentCfg.MaxTurns > 0 {
		b.WriteString(fmt.Sprintf("Max turns: %d\n", m.agentCfg.MaxTurns))
	} else {
		b.WriteString("Max turns: unlimited\n")
	}

	if m.agentCfg.CWD != "" {
		b.WriteString(fmt.Sprintf("CWD:       %s\n", m.agentCfg.CWD))
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
	m.statusbar.Update(args, m.totalInputTokens, m.totalOutputTokens, m.turn)
	m.output.AppendSystem(fmt.Sprintf("Model changed to: %s", args))
	return nil
}

// cmdCompact sends a summarization prompt to the agent, then truncates context.
func cmdCompact(m *Model, args string) tea.Cmd {
	m.compacting = true
	m.state = StateRunning

	cfg := m.agentCfg
	cfg.Prompt = compactPrompt
	cfg.MaxTurns = 1

	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	return listenToAgent(ctx, cfg)
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
