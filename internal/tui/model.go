package tui

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justin/glamdring/pkg/agent"
	"github.com/justin/glamdring/pkg/commands"
	"github.com/justin/glamdring/pkg/config"
	"github.com/justin/glamdring/pkg/hooks"
	"github.com/justin/glamdring/pkg/index"
	"github.com/justin/glamdring/pkg/mcp"
	"github.com/justin/glamdring/pkg/tools"
)

// State represents the current UI mode.
type State int

const (
	StateInput      State = iota // user can type
	StateRunning                 // agent is working
	StatePermission              // waiting for permission response
	StateCheckpoint              // checkpoint found, awaiting user decision
)

// AgentMsg wraps an agent.Message for delivery through the bubbletea message system.
type AgentMsg agent.Message

// inputHeight is the default number of visible rows for the input textarea.
const inputHeight = 3

// Model is the root bubbletea model for glamdring's TUI.
type Model struct {
	input     InputModel
	output    OutputModel
	statusbar StatusBar
	styles    Styles
	state     State

	// permission holds the current permission request when in StatePermission.
	permission *agent.Message

	width  int
	height int

	// agent wiring
	ctx      context.Context
	agentCfg agent.Config
	session  *agent.Session
	agentCh  <-chan agent.Message

	// slash command expansion
	cmdRegistry *commands.Registry

	// settings holds the resolved config for /config display.
	settings config.Settings

	// cumulative token tracking
	totalInputTokens  int
	totalOutputTokens int
	turn              int

	// compacting is true when /compact is running (agent summarizing).
	compacting bool

	// checkpointContent holds the checkpoint file content while in StateCheckpoint.
	checkpointContent string

	// indexDB is the shire index database, if available.
	indexDB *index.DB

	// indexerCfg holds indexer settings (command name, auto-rebuild).
	indexerCfg config.IndexerConfig

	// turnModifiedFiles tracks whether the current agent turn used file-modifying tools.
	turnModifiedFiles bool

	// cancelTurn cancels the context for the current agent turn.
	cancelTurn context.CancelFunc

	// lastCtrlC records the time of the last Ctrl+C press for double-tap quit.
	lastCtrlC time.Time

	// spinner and spinning track the thinking/typing indicator.
	spinner  spinner.Model
	spinning bool

	// showThinking controls whether thinking blocks are displayed.
	showThinking bool

	// lastContextThreshold tracks the last fired context threshold (0, 60, or 80).
	// Used to avoid firing the same threshold multiple times.
	lastContextThreshold int

	// mcpMgr manages MCP server lifecycles, used by /mcp command.
	mcpMgr *mcp.Manager

	// mcpConfiguredCount tracks the total configured MCP servers (including dead)
	// for the status bar denominator.
	mcpConfiguredCount int

	// baseTools holds non-MCP tools so we can rebuild the full tool list
	// when MCP servers change (restart, disconnect, enable/disable).
	baseTools []tools.Tool
}

// New creates the root TUI model without agent wiring.
func New() Model {
	styles := DefaultStyles()
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorAmber)
	return Model{
		input:     NewInputModel(styles),
		output:    NewOutputModel(styles, 80, 20),
		statusbar: NewStatusBar(styles),
		styles:    styles,
		state:     StateInput,
		spinner:   s,
	}
}

// NewWithAgent creates the root TUI model wired to an agent config.
func NewWithAgent(ctx context.Context, cfg agent.Config) Model {
	m := New()
	m.ctx = ctx
	m.agentCfg = cfg
	return m
}

// SetCommandRegistry sets the slash command registry for expansion and tab completion.
func (m *Model) SetCommandRegistry(r *commands.Registry) {
	m.cmdRegistry = r
	// Merge built-in command names with user-defined for tab completion.
	names := BuiltinNames()
	names = append(names, r.Names()...)
	m.input.SetAvailableCommands(names)
}

// SetIndexDB stores the shire index database for /index command access.
func (m *Model) SetIndexDB(db *index.DB) {
	m.indexDB = db
}

// SetIndexerConfig stores the indexer configuration.
func (m *Model) SetIndexerConfig(cfg config.IndexerConfig) {
	m.indexerCfg = cfg
}

// SetSettings stores the resolved settings for /config display.
func (m *Model) SetSettings(s config.Settings) {
	m.settings = s
}

// SetMCPManager stores the MCP manager for /mcp command and status bar updates.
func (m *Model) SetMCPManager(mgr *mcp.Manager) {
	m.mcpMgr = mgr
}

// SetMCPConfiguredCount sets the total configured MCP server count.
func (m *Model) SetMCPConfiguredCount(n int) {
	m.mcpConfiguredCount = n
}

// SetBaseTools stores the non-MCP tools for use by refreshMCPTools.
func (m *Model) SetBaseTools(t []tools.Tool) {
	m.baseTools = t
}

// InitMCPStatus initializes the MCP status bar counts. Call this
// synchronously before tea.NewProgram to ensure the counts are captured.
func (m *Model) InitMCPStatus() {
	if m.mcpMgr != nil {
		m.statusbar.UpdateMCP(m.mcpConfiguredCount, m.mcpMgr.ServerCount())
	}
}

// MCPServerDiedMsg signals that an MCP server has exited unexpectedly.
type MCPServerDiedMsg struct {
	Name string
}

// refreshMCPTools rebuilds the agent tool list with current MCP tools and
// resets the session so the next turn picks up the changes.
// Tool order matches main.go: base -> index -> MCP.
func (m *Model) refreshMCPTools() {
	var newTools []tools.Tool
	newTools = append(newTools, m.baseTools...)
	if m.indexDB != nil {
		newTools = append(newTools, index.Tools(m.indexDB)...)
	}
	if m.mcpMgr != nil {
		newTools = append(newTools, m.mcpMgr.Tools()...)
	}
	m.agentCfg.Tools = newTools
	if m.session != nil {
		m.session = nil
		m.output.AppendSystem("(session reset — tool configuration changed)")
	}
}

// Init initializes the TUI.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.input.Init(),
		m.output.Init(),
		m.startupCmd(),
		m.spinner.Tick,
	)
}

// startupCmd fires SessionStart hooks and checks for a checkpoint file.
func (m Model) startupCmd() tea.Cmd {
	return func() tea.Msg {
		// Fire SessionStart hooks.
		if m.agentCfg.HookRunner != nil {
			ctx := m.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			if err := m.agentCfg.HookRunner.Run(ctx, hooks.SessionStart, "N/A", nil); err != nil {
				log.Printf("warning: SessionStart hooks: %v", err)
			}
		}

		// Check for checkpoint file.
		if m.agentCfg.CWD != "" {
			path := filepath.Join(m.agentCfg.CWD, "tmp", "checkpoint.md")
			data, err := os.ReadFile(path)
			if err == nil && len(data) > 0 {
				return checkpointFoundMsg{content: string(data)}
			}
		}

		return nil
	}
}

// Update handles all incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layoutComponents()
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case SubmitMsg:
		return m.handleSubmit(msg)

	case agentStartedMsg:
		m.agentCh = msg.ch
		return m, waitForAgent(msg.ch)

	case AgentMsg:
		var cmd tea.Cmd
		m, cmd = m.handleAgentMsg(msg)
		// Keep draining the agent channel for more messages.
		if m.agentCh != nil && m.state != StatePermission {
			return m, tea.Batch(cmd, waitForAgent(m.agentCh))
		}
		return m, cmd

	case agentDoneMsg:
		m.agentCh = nil
		if m.state != StateInput {
			m.state = StateInput
			return m, m.input.Focus()
		}
		return m, nil

	case checkpointFoundMsg:
		m.output.AppendSystem("Found checkpoint from previous session:")
		m.output.AppendSystem(msg.content)
		m.checkpointContent = msg.content
		m.state = StateCheckpoint
		return m, nil

	case MCPServerDiedMsg:
		m.output.AppendError(fmt.Sprintf("MCP server %q died unexpectedly", msg.Name))
		if m.mcpMgr != nil {
			m.statusbar.UpdateMCP(m.mcpConfiguredCount, m.mcpMgr.ServerCount())
		}
		m.refreshMCPTools()
		return m, nil

	case indexRebuildDoneMsg:
		if msg.err != nil {
			log.Printf("index rebuild: %v", msg.err)
			return m, nil
		}
		if msg.db != nil {
			if m.indexDB != nil {
				m.indexDB.Close()
			}
			m.indexDB = msg.db
		}
		return m, nil

	case spinner.TickMsg:
		if m.spinning {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	// Pass through to focused component.
	switch m.state {
	case StateInput:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	case StateRunning, StatePermission:
		var cmd tea.Cmd
		m.output, cmd = m.output.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// handleKeyMsg routes key events based on current state.
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keybindings
	switch msg.String() {
	case "ctrl+c":
		if m.state == StateRunning || m.state == StatePermission {
			// Cancel the current agent turn.
			if m.cancelTurn != nil {
				m.cancelTurn()
				m.cancelTurn = nil
			}
			m.spinning = false
			m.permission = nil
			m.agentCh = nil
			m.state = StateInput
			m.output.AppendSystem("(interrupted)")
			return m, m.input.Focus()
		}
		// Double Ctrl+C within 1 second quits.
		if time.Since(m.lastCtrlC) < time.Second {
			return m, tea.Quit
		}
		m.lastCtrlC = time.Now()
		m.output.AppendSystem("(press Ctrl+C again to quit)")
		return m, nil

	case "shift+tab":
		if m.session != nil {
			m.session.ToggleYolo()
			m.statusbar.SetYolo(m.session.IsYolo())
			if m.session.IsYolo() {
				m.output.AppendSystem("YOLO mode enabled — all tools auto-approved.")
			} else {
				m.output.AppendSystem("YOLO mode disabled — tool permissions restored.")
			}
		}
		return m, nil

	case "esc":
		if m.state == StatePermission {
			// Deny permission on Escape — resume draining the agent channel.
			m.denyPermission()
			if m.agentCh != nil {
				return m, waitForAgent(m.agentCh)
			}
			return m, nil
		}
		if m.state == StateCheckpoint {
			m.checkpointContent = ""
			m.state = StateInput
			return m, m.input.Focus()
		}
	}

	switch m.state {
	case StateInput:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd

	case StatePermission:
		return m.handlePermissionKey(msg)

	case StateCheckpoint:
		return m.handleCheckpointKey(msg)

	case StateRunning:
		// Toggle expand/collapse on tool result blocks.
		if msg.String() == "e" {
			m.output.ToggleLastToolResult()
			return m, nil
		}
		// Allow scrolling the viewport while agent is working.
		var cmd tea.Cmd
		m.output, cmd = m.output.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleSubmit processes user input submission.
func (m Model) handleSubmit(msg SubmitMsg) (tea.Model, tea.Cmd) {
	if msg.Text == "" {
		return m, nil
	}

	// Check for built-in commands first (before displaying the message).
	if IsSlashCommand(msg.Text) {
		cmdName := CommandName(msg.Text)
		args := CommandArgs(msg.Text)

		if handler, ok := DispatchBuiltin(cmdName); ok {
			m.input.Reset()
			cmd := handler(&m, args)
			if m.state != StateRunning {
				// Normal built-in — stay in input mode.
				m.state = StateInput
				return m, tea.Batch(cmd, m.input.Focus())
			}
			// Handler started the agent (e.g., /compact).
			m.input.Blur()
			return m, cmd
		}
	}

	m.input.history.Add(msg.Text)
	m.output.AppendUserMessage(msg.Text)
	m.input.Reset()
	m.input.Blur()

	// Expand user-defined slash commands before sending to the agent.
	prompt := msg.Text
	if IsSlashCommand(prompt) && m.cmdRegistry != nil {
		cmdName := CommandName(prompt)
		args := CommandArgs(prompt)
		expanded, err := m.cmdRegistry.Expand(cmdName, args)
		if err != nil {
			m.output.AppendError(fmt.Sprintf("unknown command: /%s", cmdName))
			m.state = StateInput
			return m, m.input.Focus()
		}
		prompt = expanded
	}

	m.turn++
	m.turnModifiedFiles = false
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
	ch := m.session.Turn(turnCtx, prompt)
	return m, tea.Batch(
		func() tea.Msg { return agentStartedMsg{ch: ch} },
		m.spinner.Tick,
	)
}

// agentStartedMsg carries the agent output channel.
type agentStartedMsg struct {
	ch <-chan agent.Message
}

// waitForAgent reads the next message from the agent channel.
func waitForAgent(ch <-chan agent.Message) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return agentDoneMsg{}
		}
		return AgentMsg(msg)
	}
}

// agentDoneMsg signals the agent channel has closed.
type agentDoneMsg struct{}

// checkpointFoundMsg carries checkpoint content discovered at startup.
type checkpointFoundMsg struct {
	content string
}

// indexRebuildDoneMsg carries the result of an async shire index rebuild.
type indexRebuildDoneMsg struct {
	db  *index.DB
	err error
}

// handleAgentMsg routes agent messages to the appropriate component.
func (m Model) handleAgentMsg(msg AgentMsg) (Model, tea.Cmd) {
	am := agent.Message(msg)

	switch am.Type {
	case agent.MessageTextDelta:
		m.spinning = false
		m.output.AppendText(am.Text)

	case agent.MessageThinkingDelta:
		if m.showThinking {
			m.output.AppendThinking(am.Text)
		}

	case agent.MessageToolCall:
		m.spinning = false
		switch am.ToolName {
		case "Edit", "Write", "Bash":
			m.turnModifiedFiles = true
		}
		summary := summarizeToolInput(am.ToolName, am.ToolInput)
		m.output.AppendToolCall(am.ToolName, summary)

	case agent.MessageToolOutputDelta:
		m.output.AppendToolOutputDelta(am.Text)

	case agent.MessageToolResult:
		m.output.AppendToolResult(am.ToolOutput, am.ToolIsError)

	case agent.MessagePermissionRequest:
		m.state = StatePermission
		m.permission = &am
		m.output.AppendToolCall("Permission Required", am.PermissionSummary)
		// Don't continue draining — wait for user response.

	case agent.MessageError:
		m.spinning = false
		errMsg := "unknown error"
		if am.Err != nil {
			errMsg = am.Err.Error()
		}
		m.output.AppendError(errMsg)

	case agent.MessageDone:
		m.spinning = false
		m.totalInputTokens += am.InputTokens
		m.totalOutputTokens += am.OutputTokens
		m.statusbar.Update(m.agentCfg.Model, m.totalInputTokens, m.totalOutputTokens, m.turn)

		// Update context window usage.
		if am.LastRequestInputTokens > 0 {
			m.statusbar.UpdateContext(am.LastRequestInputTokens, m.agentCfg.Model)
			pct := m.statusbar.ContextPercent()

			// Fire threshold suggestions (once per crossing).
			if pct >= 80 && m.lastContextThreshold < 80 {
				m.lastContextThreshold = 80
				m.output.AppendSystem("Context window at " + fmt.Sprintf("%d%%", pct) + " -- consider running /compact")
			} else if pct >= 60 && m.lastContextThreshold < 60 {
				m.lastContextThreshold = 60
				m.output.AppendSystem("Context window at " + fmt.Sprintf("%d%%", pct) + " -- /compact available if needed")
			}
			// Reset threshold when context drops below 60% (e.g. after /compact).
			if pct < 60 {
				m.lastContextThreshold = 0
			}
		}

		if m.compacting {
			m.compacting = false
			summary := m.extractLastText()
			m.writeCheckpoint(summary)
			m.output.Clear()
			m.output.AppendSystem("Context compacted. Checkpoint saved to tmp/checkpoint.md.")
			if summary != "" {
				m.output.AppendSystem(summary)
			}
			if m.session != nil {
				m.session.Reset()
			}
		}

		var rebuildCmd tea.Cmd
		if m.turnModifiedFiles && m.indexDB != nil && m.indexerCfg.IndexerAutoRebuild() {
			rebuildCmd = m.rebuildIndexCmd()
		}
		m.turnModifiedFiles = false

		m.state = StateInput
		if rebuildCmd != nil {
			return m, tea.Batch(m.input.Focus(), rebuildCmd)
		}
		return m, m.input.Focus()

	case agent.MessageMaxTurnsReached:
		m.output.AppendError("max turns reached")
		m.state = StateInput
		return m, m.input.Focus()
	}

	return m, nil
}

// handlePermissionKey processes key presses during a permission prompt.
func (m Model) handlePermissionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.permission == nil {
		return m, nil
	}

	var resume bool
	switch msg.String() {
	case "y", "Y":
		if m.permission.PermissionResponse != nil {
			m.permission.PermissionResponse <- agent.PermissionApprove
		}
		m.permission = nil
		m.state = StateRunning
		resume = true
	case "a", "A":
		if m.permission.PermissionResponse != nil {
			m.permission.PermissionResponse <- agent.PermissionAlwaysApprove
		}
		m.permission = nil
		m.state = StateRunning
		resume = true
	case "n", "N":
		m.denyPermission()
		resume = true
	}

	if resume && m.agentCh != nil {
		return m, waitForAgent(m.agentCh)
	}
	return m, nil
}

// denyPermission sends a deny response and returns to running state.
func (m *Model) denyPermission() {
	if m.permission != nil && m.permission.PermissionResponse != nil {
		m.permission.PermissionResponse <- agent.PermissionDeny
	}
	m.permission = nil
	m.state = StateRunning
}

// handleCheckpointKey processes key presses during the checkpoint prompt.
func (m Model) handleCheckpointKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.agentCfg.SystemPrompt += "\n\n## Previous Session Context\n\n" + m.checkpointContent
		m.checkpointContent = ""
		m.state = StateInput
		return m, m.input.Focus()
	case "n", "N":
		m.checkpointContent = ""
		m.state = StateInput
		return m, m.input.Focus()
	}
	return m, nil
}

// extractLastText returns the content of the last text block in the output,
// used to capture the agent's compact summary.
func (m *Model) extractLastText() string {
	for i := len(m.output.blocks) - 1; i >= 0; i-- {
		if m.output.blocks[i].kind == blockText {
			return strings.TrimSpace(m.output.blocks[i].content)
		}
	}
	return ""
}

// writeCheckpoint writes the compact summary to tmp/checkpoint.md in the CWD.
func (m *Model) writeCheckpoint(summary string) {
	cwd := m.agentCfg.CWD
	if cwd == "" {
		return
	}

	dir := filepath.Join(cwd, "tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}

	branch := currentGitBranch(cwd)
	ts := time.Now().Format("2006-01-02 15:04")

	var content strings.Builder
	fmt.Fprintf(&content, "<!-- Checkpoint: %s -->\n", ts)
	fmt.Fprintf(&content, "<!-- Branch: %s -->\n\n", branch)
	content.WriteString(summary)
	content.WriteString("\n")

	_ = os.WriteFile(filepath.Join(dir, "checkpoint.md"), []byte(content.String()), 0o644)
}

// rebuildIndexCmd returns a tea.Cmd that runs the indexer in the background
// and reopens the index DB. Triggered after agent turns that modified files.
func (m Model) rebuildIndexCmd() tea.Cmd {
	cwd := m.agentCfg.CWD
	cmdName := m.indexerCfg.IndexerCommand()
	return func() tea.Msg {
		binPath, err := exec.LookPath(cmdName)
		if err != nil {
			return indexRebuildDoneMsg{err: fmt.Errorf("%s not found: %w", cmdName, err)}
		}

		cmd := exec.Command(binPath, "build", "--root", cwd)
		cmd.Dir = cwd
		if out, err := cmd.CombinedOutput(); err != nil {
			return indexRebuildDoneMsg{err: fmt.Errorf("%s build: %s\n%s", cmdName, err, out)}
		}

		dbPath := filepath.Join(cwd, ".shire", "index.db")
		db, err := index.Open(dbPath)
		if err != nil {
			return indexRebuildDoneMsg{err: fmt.Errorf("reopen index: %w", err)}
		}
		return indexRebuildDoneMsg{db: db}
	}
}

// currentGitBranch returns the current git branch name, or "unknown".
func currentGitBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// layoutComponents recalculates component dimensions after a resize.
func (m *Model) layoutComponents() {
	statusHeight := 1
	// Input area: border adds 2 rows (top+bottom), plus the textarea rows.
	inputBorderHeight := 2
	inputTotalHeight := inputHeight + inputBorderHeight

	outputHeight := m.height - inputTotalHeight - statusHeight
	if outputHeight < 1 {
		outputHeight = 1
	}

	m.input.SetWidth(m.width)
	m.input.SetHeight(inputHeight)
	m.output.SetSize(m.width, outputHeight)
	m.statusbar.SetWidth(m.width)
}

// View renders the full TUI layout.
func (m Model) View() string {
	// Layout: output (fills space) | spinner (optional) | status bar (1 line) | input (bottom)
	output := m.output.View()
	status := m.statusbar.View()

	var input string
	switch m.state {
	case StatePermission:
		input = m.renderPermissionPrompt()
	case StateCheckpoint:
		input = m.renderCheckpointPrompt()
	default:
		input = m.input.View()
	}

	parts := []string{output}
	if m.spinning {
		spinnerLine := m.styles.SpinnerText.Render(m.spinner.View() + " Thinking...")
		parts = append(parts, spinnerLine)
	}
	parts = append(parts, status, input)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// renderPermissionPrompt renders the inline permission prompt.
func (m Model) renderPermissionPrompt() string {
	title := m.styles.PermissionTitle.Render("Allow this action?")
	help := m.styles.PermissionHelp.Render("[y]es  [n]o  [a]lways")

	content := title + "\n" + help
	return m.styles.PermissionBorder.
		Width(m.width - 4).
		Render(content)
}

// renderCheckpointPrompt renders the inline checkpoint load prompt.
func (m Model) renderCheckpointPrompt() string {
	title := m.styles.PermissionTitle.Render("Load checkpoint from previous session?")
	help := m.styles.PermissionHelp.Render("[y]es  [n]o")

	content := title + "\n" + help
	return m.styles.PermissionBorder.
		Width(m.width - 4).
		Render(content)
}

// summarizeToolInput produces a short human-readable summary of a tool call's input.
func summarizeToolInput(toolName string, input map[string]any) string {
	switch toolName {
	case "Bash":
		if cmd, ok := input["command"]; ok {
			s := fmt.Sprintf("%v", cmd)
			if len(s) > 80 {
				return s[:77] + "..."
			}
			return s
		}
	case "Read":
		if p, ok := input["file_path"]; ok {
			return fmt.Sprintf("%v", p)
		}
	case "Write":
		if p, ok := input["file_path"]; ok {
			return fmt.Sprintf("%v", p)
		}
	case "Edit":
		if p, ok := input["file_path"]; ok {
			return fmt.Sprintf("%v", p)
		}
	case "Glob":
		if p, ok := input["pattern"]; ok {
			return fmt.Sprintf("%v", p)
		}
	case "Grep":
		if p, ok := input["pattern"]; ok {
			return fmt.Sprintf("%v", p)
		}
	}

	// Fallback: show first key=value pair.
	for k, v := range input {
		s := fmt.Sprintf("%s=%v", k, v)
		if len(s) > 80 {
			return s[:77] + "..."
		}
		return s
	}
	return "(no input)"
}
