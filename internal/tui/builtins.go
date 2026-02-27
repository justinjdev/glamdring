package tui

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justin/glamdring/pkg/index"
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
	"index":   cmdIndex,
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
	"index":   "Show index status or rebuild (shire)",
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
	shirePath, err := exec.LookPath("shire")
	if err != nil {
		m.output.AppendError("shire is not installed. Install with: brew tap justinjdev/shire && brew install shire")
		return nil
	}

	cwd := m.agentCfg.CWD
	if cwd == "" {
		m.output.AppendError("no working directory set")
		return nil
	}

	m.output.AppendSystem("Rebuilding shire index...")

	// Run shire build synchronously (it's fast for incremental builds).
	cmd := exec.Command(shirePath, "build", "--root", cwd)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	if err != nil {
		m.output.AppendError(fmt.Sprintf("shire build failed: %s\n%s", err, string(output)))
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
