package tui

import (
	"context"
	"encoding/base64"
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
	"github.com/justin/glamdring/pkg/api"
	"github.com/justin/glamdring/pkg/commands"
	"github.com/justin/glamdring/pkg/config"
	"github.com/justin/glamdring/pkg/hooks"
	"github.com/justin/glamdring/pkg/index"
	"github.com/justin/glamdring/pkg/mcp"
	"github.com/justin/glamdring/pkg/tools"
	"github.com/justin/glamdring/pkg/update"
	"golang.org/x/term"
)

// State represents the current UI mode.
type State int

const (
	StateInput      State = iota // user can type
	StateRunning                 // agent is working
	StatePermission              // waiting for permission response
	StateCheckpoint              // checkpoint found, awaiting user decision
	StateUpdate                  // waiting for update confirmation
	StateModal                   // interactive modal overlay is open
	StateIndexPrompt             // waiting for index build confirmation
)

// AgentMsg wraps an agent.Message for delivery through the bubbletea message system.
type AgentMsg agent.Message

// Model is the root bubbletea model for glamdring's TUI.
type Model struct {
	input     InputModel
	output    OutputModel
	statusbar StatusBar
	styles    Styles
	palette   ThemePalette
	state     State

	// permission holds the current permission request when in StatePermission.
	permission *agent.Message

	// modal holds the active modal overlay when in StateModal.
	modal *ModalModel
	// preModalState is restored when the modal closes.
	preModalState State

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

	// pendingIndexCheck holds an indexStartupCheckMsg that arrived while the
	// model was not in StateInput (e.g. StateCheckpoint). It is replayed once
	// the blocking state resolves.
	pendingIndexCheck *indexStartupCheckMsg

	// turnModifiedFiles tracks whether the current agent turn used file-modifying tools.
	turnModifiedFiles bool

	// cancelTurn cancels the context for the current agent turn.
	cancelTurn context.CancelFunc

	// lastCtrlC records the time of the last Ctrl+C press for double-tap quit.
	lastCtrlC time.Time

	// spinner and spinning track the thinking/typing indicator.
	spinner      spinner.Model
	spinning     bool
	spinnerLabel string

	// renderTickPending is true when a render tick has been scheduled but
	// not yet fired. Prevents scheduling duplicate ticks.
	renderTickPending bool

	// showThinking controls whether thinking blocks are displayed.
	showThinking bool

	// lastContextThreshold tracks the last fired context threshold (0, 60, or 80).
	// Used to avoid firing the same threshold multiple times.
	lastContextThreshold int

	// lastToolWasTodo is true when the most recent tool call was TodoWrite.
	// Used to suppress the corresponding tool result block.
	lastToolWasTodo bool

	// mcpMgr manages MCP server lifecycles, used by /mcp command.
	mcpMgr *mcp.Manager

	// mcpConfiguredCount tracks the total configured MCP servers (including dead)
	// for the status bar denominator.
	mcpConfiguredCount int

	// baseTools holds non-MCP tools so we can rebuild the full tool list
	// when MCP servers change (restart, disconnect, enable/disable).
	baseTools []tools.Tool

	// version is the compiled-in version string, used for update checks.
	version string

	// disableUpdateCheck suppresses the async startup update check.
	disableUpdateCheck bool

	// pendingUpdate holds a release awaiting user confirmation in StateUpdate.
	pendingUpdate *update.Release
}

// New creates the root TUI model without agent wiring.
func New() Model {
	palette := builtinThemes["glamdring"]
	styles := DefaultStyles(palette)
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(palette.Primary)

	// Pre-fetch terminal size so the first render isn't narrow.
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))
	if w == 0 {
		w = 80
	}
	if h == 0 {
		h = 24
	}

	m := Model{
		input:     NewInputModel(styles, palette),
		output:    NewOutputModel(styles, w, h),
		statusbar: NewStatusBar(styles),
		styles:    styles,
		palette:   palette,
		state:     StateInput,
		spinner:   s,
		width:     w,
		height:    h,
	}
	m.layoutComponents()
	return m
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

// SetTheme applies a theme palette to the model, rebuilding all styles.
func (m *Model) SetTheme(p ThemePalette, highContrast bool) {
	if highContrast {
		p = HighContrastTransform(p)
	}
	m.palette = p
	m.styles = DefaultStyles(p)
	m.input.SetTheme(m.styles, p)
	m.output.styles = m.styles
	m.statusbar.styles = m.styles
	m.spinner.Style = lipgloss.NewStyle().Foreground(p.Primary)
}

// PopulateDemoContent fills the output viewport with representative sample
// content for theme screenshots. Call after SetTheme if a non-default theme
// is desired.
func (m *Model) PopulateDemoContent() {
	m.output.AppendUserMessage("How do I switch themes in glamdring?")
	m.output.AppendText("Use `/theme <name>` to switch at runtime. Six built-in themes ship with glamdring.\n\nEach theme defines **Primary**, **Secondary**, **Success**, and **Error** accent colors that are applied across the entire interface.\n\n```go\npalette, ok := tui.LookupTheme(\"mithril\")\n```\n")
	m.output.AppendToolCall("Read", "internal/tui/styles.go")
	m.output.AppendToolResult("type ThemePalette struct {\n    Name     string\n    Bg       lipgloss.Color\n    Primary  lipgloss.Color\n    // ... 11 more color slots\n}", false)
	m.output.AppendText("The `ThemePalette` struct defines all color slots. You can also create custom themes in your settings file.")
}

// PopulateDemoIndexPrompt fills the output with representative content and
// places the model in StateIndexPrompt for screenshot capture.
func (m *Model) PopulateDemoIndexPrompt() {
	m.output.AppendUserMessage("Refactor the auth package to use interface-based mocking")
	m.output.AppendToolCall("Read", "internal/auth/provider.go")
	m.output.AppendToolResult("type Provider struct {\n    db *sql.DB\n}\n\nfunc (p *Provider) Authenticate(token string) (*User, error) {\n    // ...\n}", false)
	m.output.AppendText("I'll extract an `Authenticator` interface and update the package to depend on it.")
	m.output.AppendSystem("No code index found. Build it now?")
	m.state = StateIndexPrompt
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

// SetVersion sets the compiled-in version string for update checks.
func (m *Model) SetVersion(v string) {
	m.version = v
}

// SetDisableUpdateCheck suppresses the async startup update check.
func (m *Model) SetDisableUpdateCheck(v bool) {
	m.disableUpdateCheck = v
}

// InitMCPStatus initializes the MCP status bar counts. Call this
// synchronously before tea.NewProgram to ensure the counts are captured.
func (m *Model) InitMCPStatus() {
	if m.mcpMgr != nil {
		m.statusbar.UpdateMCP(m.mcpConfiguredCount, m.mcpMgr.ServerCount())
	}
}

// refreshIndexTools rebuilds the agent tool list after the shire index DB is
// replaced. Unlike refreshMCPTools, it does not reset the session: the tool
// interfaces are identical — only the underlying data has changed.
func (m *Model) refreshIndexTools() {
	var newTools []tools.Tool
	newTools = append(newTools, m.baseTools...)
	if m.indexDB != nil {
		newTools = append(newTools, index.Tools(m.indexDB)...)
	}
	if m.mcpMgr != nil {
		newTools = append(newTools, m.mcpMgr.Tools()...)
	}
	m.agentCfg.Tools = newTools
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
	cmds := []tea.Cmd{
		m.input.Init(),
		m.output.Init(),
		m.renderStartupHeader(),
		m.startupCmd(),
		m.spinner.Tick,
		m.checkIndexStartupCmd(),
	}
	if !m.disableUpdateCheck {
		cmds = append(cmds, m.checkUpdateCmd())
	}
	return tea.Batch(cmds...)
}

// startupHeaderMsg carries the rendered header for display.
type startupHeaderMsg struct{ content string }

// renderStartupHeader builds the startup banner showing app name, version, model, and cwd.
func (m Model) renderStartupHeader() tea.Cmd {
	return func() tea.Msg {
		ver := m.version
		if ver == "" {
			ver = "dev"
		}
		model := m.agentCfg.Model
		if model == "" {
			model = "default"
		}
		cwd := m.agentCfg.CWD
		if cwd == "" {
			cwd, _ = os.Getwd()
		}
		// Shorten home directory.
		if home, err := os.UserHomeDir(); err == nil {
			cwd = strings.Replace(cwd, home, "~", 1)
		}

		return startupHeaderMsg{content: fmt.Sprintf(
			"glamdring %s\n%s\n%s", ver, model, cwd,
		)}
	}
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

// checkUpdateCmd returns a tea.Cmd that checks for a newer version in the background.
func (m Model) checkUpdateCmd() tea.Cmd {
	version := m.version
	return func() tea.Msg {
		rel, err := update.CheckLatest(version)
		if err != nil {
			log.Printf("update check failed: %v", err)
			return nil
		}
		if rel == nil {
			return nil
		}
		return updateAvailableMsg{version: rel.Version}
	}
}

// checkIndexStartupCmd returns a tea.Cmd that fires at startup when no index
// DB is open. Checks whether to prompt, auto-build, or show an install hint.
// Returns nil if the index is present, disabled, or auto_build=false.
func (m Model) checkIndexStartupCmd() tea.Cmd {
	if m.indexDB != nil {
		return nil
	}
	enabled := m.indexerCfg.IndexerEnabled()
	if enabled != nil && !*enabled {
		return nil
	}
	autoBuild := m.indexerCfg.IndexerAutoBuild()
	if autoBuild != nil && !*autoBuild {
		return nil
	}
	cmdName := m.indexerCfg.IndexerCommand()
	return func() tea.Msg {
		if _, err := exec.LookPath(cmdName); err != nil {
			return indexStartupCheckMsg{notInstalled: true}
		}
		if autoBuild != nil && *autoBuild {
			return indexStartupCheckMsg{autoBuild: true}
		}
		return indexStartupCheckMsg{}
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

	case tea.MouseMsg:
		// Always route mouse events to the output viewport so scroll wheel
		// works regardless of which component has focus.
		var cmd tea.Cmd
		m.output, cmd = m.output.Update(msg)
		return m, cmd

	case startupHeaderMsg:
		m.output.AppendHeader(msg.content, m.styles, m.palette)
		return m, nil

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
			m.output.AppendError(fmt.Sprintf("Index build failed: %v", msg.err))
			return m, nil
		}
		if msg.db != nil {
			if m.indexDB != nil {
				m.indexDB.Close()
			}
			m.indexDB = msg.db
			m.refreshIndexTools()
		}
		return m, nil

	case indexStartupCheckMsg:
		// If another startup prompt is active (e.g. checkpoint recovery), defer
		// this message until that state resolves so it is not lost.
		if m.state != StateInput {
			m.pendingIndexCheck = &msg
			return m, nil
		}
		if msg.notInstalled {
			m.output.AppendSystem(
				"Code index not found. Install shire to enable code search:\n  https://github.com/justinjdev/shire",
			)
			return m, nil
		}
		if msg.autoBuild {
			m.output.AppendSystem("Building code index...")
			return m, tea.Batch(m.input.Focus(), m.rebuildIndexCmd())
		}
		m.state = StateIndexPrompt
		m.output.AppendSystem("No code index found. Build it now?")
		return m, nil

	case updateAvailableMsg:
		m.output.AppendSystem(fmt.Sprintf("Update available: %s -- run /update to install", msg.version))
		return m, nil

	case updateCheckDoneMsg:
		m.spinning = false
		if msg.err != nil {
			m.output.AppendError(fmt.Sprintf("Update check failed: %s", msg.err))
			m.state = StateInput
			return m, m.input.Focus()
		}
		if msg.rel == nil {
			m.output.AppendSystem(fmt.Sprintf("glamdring %s is up to date.", m.version))
			m.state = StateInput
			return m, m.input.Focus()
		}
		m.pendingUpdate = msg.rel
		m.state = StateUpdate
		m.output.AppendSystem(fmt.Sprintf("Update glamdring %s -> %s?", m.version, msg.rel.Version))
		return m, nil

	case updateDoneMsg:
		if msg.err != nil {
			m.output.AppendError(fmt.Sprintf("Update failed: %s", msg.err))
		} else {
			m.output.AppendSystem(fmt.Sprintf("Updated to %s. Restart glamdring to use the new version.", msg.version))
		}
		m.state = StateInput
		return m, m.input.Focus()

	case renderTickMsg:
		m.renderTickPending = false
		hasMore := m.output.DrainPending()
		if hasMore {
			m.renderTickPending = true
			return m, scheduleRenderTick()
		}
		return m, nil

	case spinner.TickMsg:
		if m.spinning {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			if m.spinnerLabel == "" {
				// Tool running -- update inline spinner on the tool call block.
				m.output.SetToolSpinner(m.spinner.View())
			}
			// Animate the assistant header star while working.
			m.output.TickStar()
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
		if m.state == StateModal {
			cmd := m.closeModal()
			return m, cmd
		}
		if m.state == StateRunning || m.state == StatePermission {
			// Cancel the current agent turn.
			if m.cancelTurn != nil {
				m.cancelTurn()
				m.cancelTurn = nil
			}
			m.spinning = false
			m.spinnerLabel = ""
			m.permission = nil
			m.agentCh = nil
			m.state = StateInput
			m.output.ClearPending()
			m.output.ClearToolSpinner()
			m.output.FinalizeTaskList()
			m.lastToolWasTodo = false
			m.output.FinalizeStar()
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
		// Route scroll keys to the output viewport so users can scroll
		// the conversation while composing input.
		if isScrollKey(msg) {
			var cmd tea.Cmd
			m.output, cmd = m.output.Update(msg)
			return m, cmd
		}
		// Intercept Enter to handle submission synchronously so the
		// user message appears in the same frame as the input clears.
		// Skip if the input is in search mode (Ctrl+R) — let input handle it.
		if msg.Type == tea.KeyEnter && !m.input.IsSearching() {
			submit := m.input.TrySubmit()
			if submit != nil {
				return m.handleSubmit(*submit)
			}
			// Enter consumed but no submission (empty input) — fall through.
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.layoutComponents()
		return m, cmd

	case StatePermission:
		return m.handlePermissionKey(msg)

	case StateCheckpoint:
		return m.handleCheckpointKey(msg)

	case StateUpdate:
		return m.handleUpdateKey(msg)

	case StateIndexPrompt:
		return m.handleIndexPromptKey(msg)

	case StateModal:
		return m.handleModalKey(msg)

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
	if msg.Text == "" && len(msg.Images) == 0 {
		return m, nil
	}

	// Check for built-in commands first (before displaying the message).
	if IsSlashCommand(msg.Text) {
		cmdName := CommandName(msg.Text)
		args := CommandArgs(msg.Text)

		if handler, ok := DispatchBuiltin(cmdName); ok {
			m.input.Reset()
			cmd := handler(&m, args)
			if m.state != StateRunning && m.state != StateUpdate && m.state != StateModal && m.state != StateIndexPrompt {
				// Normal built-in — stay in input mode.
				m.state = StateInput
				return m, tea.Batch(cmd, m.input.Focus())
			}
			// Handler started the agent (e.g., /compact).
			m.input.Blur()
			return m, cmd
		}
	}

	if msg.Text != "" {
		m.input.history.Add(msg.Text)
	}
	// Reset input and relayout BEFORE appending user message so doRender()
	// uses the correct viewport height (input shrinks after clearing).
	m.input.Reset()
	m.input.Blur()
	m.layoutComponents()

	if len(msg.Images) > 0 {
		var parts []string
		for i, img := range msg.Images {
			if img.Width > 0 && img.Height > 0 {
				parts = append(parts, fmt.Sprintf("[Image %d: %dx%d]", i+1, img.Width, img.Height))
			} else {
				parts = append(parts, fmt.Sprintf("[Image %d]", i+1))
			}
		}
		imageLabel := strings.Join(parts, " ")
		if msg.Text != "" {
			m.output.AppendUserMessage(imageLabel + "\n" + msg.Text)
		} else {
			m.output.AppendUserMessage(imageLabel)
		}
	} else {
		m.output.AppendUserMessage(msg.Text)
	}

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
	m.spinnerLabel = "Thinking..."
	m.output.StartAssistantStar()

	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	turnCtx, cancel := context.WithCancel(ctx)
	m.cancelTurn = cancel

	if m.session == nil {
		m.session = agent.NewSession(m.agentCfg)
	}
	var ch <-chan agent.Message
	if len(msg.Images) > 0 {
		blocks := buildContentBlocks(prompt, msg.Images)
		ch = m.session.TurnWithBlocks(turnCtx, blocks)
	} else {
		ch = m.session.Turn(turnCtx, prompt)
	}
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

// indexStartupCheckMsg is returned by checkIndexStartupCmd to signal what
// action to take when no index is found at startup.
type indexStartupCheckMsg struct {
	notInstalled bool // shire binary not found in PATH
	autoBuild    bool // auto_build=true, skip the prompt and build directly
}

// updateAvailableMsg signals that a newer version is available.
type updateAvailableMsg struct {
	version string
}

// updateCheckDoneMsg signals that a manual /update check completed.
type updateCheckDoneMsg struct {
	rel *update.Release
	err error
}

// updateDoneMsg signals that a download attempt completed.
type updateDoneMsg struct {
	version string
	err     error
}

// handleAgentMsg routes agent messages to the appropriate component.
func (m Model) handleAgentMsg(msg AgentMsg) (Model, tea.Cmd) {
	am := agent.Message(msg)

	switch am.Type {
	case agent.MessageTextDelta:
		m.spinning = false
		m.output.ClearToolSpinner()
		m.output.AppendText(am.Text)
		if !m.renderTickPending {
			m.renderTickPending = true
			return m, scheduleRenderTick()
		}
		return m, nil

	case agent.MessageThinkingDelta:
		if m.showThinking {
			m.output.AppendThinking(am.Text)
			if !m.renderTickPending {
				m.renderTickPending = true
				return m, scheduleRenderTick()
			}
		}
		return m, nil

	case agent.MessageToolCall:
		m.output.FlushAllPending()
		m.output.ClearToolSpinner()
		m.lastToolWasTodo = am.ToolName == "TodoWrite"
		if m.lastToolWasTodo {
			todos := parseTodos(am.ToolInput)
			if len(todos) > 0 {
				m.output.UpdateTaskList(todos)
				return m, nil
			}
			// Malformed/empty payload: fall through to normal tool-call rendering.
			m.lastToolWasTodo = false
		}
		switch am.ToolName {
		case "Edit", "Write", "Bash":
			m.turnModifiedFiles = true
		}
		summary := summarizeToolInput(am.ToolName, am.ToolInput)
		m.output.AppendToolCall(am.ToolName, summary)
		m.spinning = true
		m.spinnerLabel = ""
		m.output.SetToolSpinner(m.spinner.View())
		return m, m.spinner.Tick

	case agent.MessageToolOutputDelta:
		m.spinning = false
		m.output.ClearToolSpinner()
		m.output.AppendToolOutputDelta(am.Text)
		if !m.renderTickPending {
			m.renderTickPending = true
			return m, scheduleRenderTick()
		}
		return m, nil

	case agent.MessageToolResult:
		m.output.ClearToolSpinner()
		m.output.FlushAllPending()
		if m.lastToolWasTodo {
			m.lastToolWasTodo = false
			if am.ToolIsError {
				m.output.AppendToolResult(am.ToolOutput, true)
				m.spinning = true
				m.spinnerLabel = "Thinking..."
				return m, m.spinner.Tick
			}
			m.spinning = true
			m.spinnerLabel = "Thinking..."
			return m, m.spinner.Tick
		}
		m.output.AppendToolResult(am.ToolOutput, am.ToolIsError)
		m.spinning = true
		m.spinnerLabel = "Thinking..."
		return m, m.spinner.Tick

	case agent.MessagePermissionRequest:
		m.state = StatePermission
		m.permission = &am
		m.output.AppendToolCall("Permission Required", am.PermissionSummary)
		// Don't continue draining — wait for user response.

	case agent.MessageError:
		m.spinning = false
		m.output.ClearToolSpinner()
		m.output.FlushAllPending()
		m.output.FinalizeTaskList()
		m.lastToolWasTodo = false
		m.output.FinalizeStar()
		errMsg := "unknown error"
		if am.Err != nil {
			errMsg = am.Err.Error()
		}
		m.output.AppendError(errMsg)

	case agent.MessageDone:
		m.spinning = false
		m.output.ClearToolSpinner()
		m.output.FlushAllPending()
		m.output.FinalizeTaskList()
		m.output.FinalizeStar()
		m.totalInputTokens += am.InputTokens
		m.totalOutputTokens += am.OutputTokens
		m.statusbar.Update(m.agentCfg.Model, m.totalInputTokens, m.totalOutputTokens, m.turn)

		// Update context window usage.
		if am.LastRequestInputTokens > 0 {
			m.statusbar.UpdateContext(am.LastRequestInputTokens, m.agentCfg.Model)
			pct := m.statusbar.ContextPercent()

			// Fire threshold suggestions and hooks (once per crossing).
			if pct >= 80 && m.lastContextThreshold < 80 {
				m.lastContextThreshold = 80
				m.output.AppendSystem("Context window at " + fmt.Sprintf("%d%%", pct) + " -- consider running /compact")
				m.fireContextThresholdHook(pct)
			} else if pct >= 60 && m.lastContextThreshold < 60 {
				m.lastContextThreshold = 60
				m.output.AppendSystem("Context window at " + fmt.Sprintf("%d%%", pct) + " -- /compact available if needed")
				m.fireContextThresholdHook(pct)
			}
			// Reset threshold when context drops below 60% (e.g. after /compact).
			if pct < 60 {
				m.lastContextThreshold = 0
			}
		}

		if m.compacting {
			m.compacting = false
			summary := m.extractLastText()
			if err := m.writeCheckpoint(summary); err != nil {
				log.Printf("checkpoint failed: %v", err)
			}
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
		m.output.FinalizeTaskList()
		m.lastToolWasTodo = false
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
		m.removeCheckpointFile()
		m.checkpointContent = ""
		m.state = StateInput
		return m, tea.Batch(m.input.Focus(), m.flushPendingIndexCheck())
	case "n", "N":
		m.removeCheckpointFile()
		m.checkpointContent = ""
		m.output.Clear()
		m.state = StateInput
		return m, tea.Batch(m.input.Focus(), m.flushPendingIndexCheck())
	}
	return m, nil
}

// flushPendingIndexCheck replays a deferred indexStartupCheckMsg if one was
// stored while another startup prompt was active. Returns nil if none pending.
func (m *Model) flushPendingIndexCheck() tea.Cmd {
	if m.pendingIndexCheck == nil {
		return nil
	}
	msg := *m.pendingIndexCheck
	m.pendingIndexCheck = nil
	return func() tea.Msg { return msg }
}

// handleUpdateKey processes key presses during the update confirmation prompt.
func (m Model) handleUpdateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		rel := m.pendingUpdate
		m.pendingUpdate = nil
		m.state = StateInput
		m.output.AppendSystem("Downloading update...")
		return m, func() tea.Msg {
			exe, err := os.Executable()
			if err != nil {
				return updateDoneMsg{err: err}
			}
			exe, err = filepath.EvalSymlinks(exe)
			if err != nil {
				return updateDoneMsg{err: err}
			}
			if err := update.Download(rel, exe); err != nil {
				return updateDoneMsg{err: err}
			}
			return updateDoneMsg{version: rel.Version}
		}
	case "n", "N":
		m.pendingUpdate = nil
		m.state = StateInput
		m.output.AppendSystem("Update cancelled.")
		return m, m.input.Focus()
	}
	return m, nil
}

// handleIndexPromptKey processes key presses during the index build prompt.
func (m Model) handleIndexPromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.state = StateInput
		m.output.AppendSystem("Building code index...")
		return m, tea.Batch(m.input.Focus(), m.rebuildIndexCmd())
	case "n", "N":
		m.state = StateInput
		m.output.AppendSystem("Index build skipped.")
		return m, m.input.Focus()
	}
	return m, nil
}

// openModal switches to StateModal with the given modal.
func (m *Model) openModal(modal *ModalModel) {
	m.preModalState = m.state
	m.modal = modal
	m.state = StateModal
	m.input.Blur()
}

// closeModal dismisses the modal and restores the previous state.
func (m *Model) closeModal() tea.Cmd {
	m.modal = nil
	m.state = m.preModalState
	if m.state == StateInput {
		return m.input.Focus()
	}
	return nil
}

// handleModalKey routes key presses to the active modal.
func (m Model) handleModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.modal == nil {
		cmd := m.closeModal()
		return m, cmd
	}

	shouldClose, change := m.modal.HandleKey(msg.String())
	if change != nil {
		m.applyModalChange(change)
	}
	if shouldClose {
		cmd := m.closeModal()
		return m, cmd
	}
	return m, nil
}

// applyModalChange applies a setting change from the modal to the model.
func (m *Model) applyModalChange(c *ModalChange) {
	switch c.ID {
	case "theme":
		m.applyTheme(c.Value)
	case "model":
		m.agentCfg.Model = c.Value
		m.session = nil
		m.statusbar.Update(c.Value, m.totalInputTokens, m.totalOutputTokens, m.turn)
		_ = config.SaveUserSetting("model", c.Value)
	case "thinking":
		m.showThinking = c.Value == "on"
	case "yolo":
		wantYolo := c.Value == "on"
		if m.session != nil {
			if m.session.IsYolo() != wantYolo {
				m.session.ToggleYolo()
			}
			m.agentCfg.Yolo = wantYolo
			m.statusbar.SetYolo(wantYolo)
		} else {
			m.agentCfg.Yolo = wantYolo
			m.statusbar.SetYolo(wantYolo)
		}
	case "high_contrast":
		m.settings.HighContrast = c.Value == "on"
		m.SetTheme(m.palette, m.settings.HighContrast)
		m.layoutComponents()
		m.output.RefreshHeader(m.palette)
		_ = config.SaveUserSetting("high_contrast", m.settings.HighContrast)
	}
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
func (m *Model) writeCheckpoint(summary string) error {
	cwd := m.agentCfg.CWD
	if cwd == "" {
		return nil
	}

	dir := filepath.Join(cwd, "tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create checkpoint dir: %w", err)
	}

	branch := currentGitBranch(cwd)
	ts := time.Now().Format("2006-01-02 15:04")

	var content strings.Builder
	fmt.Fprintf(&content, "<!-- Checkpoint: %s -->\n", ts)
	fmt.Fprintf(&content, "<!-- Branch: %s -->\n\n", branch)
	content.WriteString(summary)
	content.WriteString("\n")

	if err := os.WriteFile(filepath.Join(dir, "checkpoint.md"), []byte(content.String()), 0o644); err != nil {
		return fmt.Errorf("write checkpoint file: %w", err)
	}
	return nil
}

// removeCheckpointFile deletes tmp/checkpoint.md so it doesn't prompt again.
func (m *Model) removeCheckpointFile() {
	if m.agentCfg.CWD == "" {
		return
	}
	path := filepath.Join(m.agentCfg.CWD, "tmp", "checkpoint.md")
	os.Remove(path)
}

// fireContextThresholdHook runs the ContextThreshold hook if configured.
func (m *Model) fireContextThresholdHook(pct int) {
	if m.agentCfg.HookRunner == nil {
		return
	}
	go func() {
		input := []byte(fmt.Sprintf(`{"percentage":%d}`, pct))
		if err := m.agentCfg.HookRunner.Run(context.Background(), hooks.ContextThreshold, "", input); err != nil {
			log.Printf("warning: ContextThreshold hook: %v", err)
		}
	}()
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

// isScrollKey returns true if the key message is a viewport scroll command
// that should be routed to the output viewport even when the input is focused.
func isScrollKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd:
		return true
	}
	switch msg.String() {
	case "ctrl+u", "ctrl+d":
		return true
	}
	return false
}

// layoutComponents recalculates component dimensions after a resize.
func (m *Model) layoutComponents() {
	statusHeight := 1
	paddingLines := 1 // blank line above status bar
	// Input area: border adds 2 rows (top+bottom), plus the textarea rows.
	inputBorderHeight := 2
	desiredInput := m.input.DesiredHeight()
	inputTotalHeight := desiredInput + inputBorderHeight

	spinnerHeight := 1 // always reserved for the spinner line (empty when not spinning)
	outputHeight := m.height - inputTotalHeight - statusHeight - paddingLines - spinnerHeight
	if outputHeight < 1 {
		outputHeight = 1
	}

	m.input.SetWidth(m.width)
	m.input.SetHeight(desiredInput)
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
	case StateUpdate:
		input = m.renderUpdatePrompt()
	case StateIndexPrompt:
		input = m.renderIndexPrompt()
	case StateModal:
		input = m.input.View()
	default:
		input = m.input.View()
	}

	var spinnerLine string
	if m.spinning && m.spinnerLabel != "" {
		spinnerLine = m.styles.SpinnerText.Render(m.spinner.View() + " " + m.spinnerLabel)
	}
	parts := []string{output, spinnerLine, "", status, input}

	base := lipgloss.JoinVertical(lipgloss.Left, parts...)

	// Overlay modal if active.
	if m.state == StateModal && m.modal != nil {
		modalBox := m.modal.OverlayView(m.width, m.height)
		base = RenderOverlay(base, modalBox, m.width, m.height)
	}

	return base
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

// renderUpdatePrompt renders the inline update confirmation prompt.
func (m Model) renderUpdatePrompt() string {
	title := m.styles.PermissionTitle.Render("Install update?")
	help := m.styles.PermissionHelp.Render("[y]es  [n]o")
	content := title + "\n" + help
	return m.styles.PermissionBorder.
		Width(m.width - 4).
		Render(content)
}

// renderIndexPrompt renders the inline index build confirmation prompt.
func (m Model) renderIndexPrompt() string {
	title := m.styles.PermissionTitle.Render("Build code index?")
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

// buildContentBlocks constructs a []api.ContentBlock from text and images
// for sending to the Claude vision API.
func buildContentBlocks(text string, images []PendingImage) []api.ContentBlock {
	var blocks []api.ContentBlock

	// Images first, then text -- matches Claude Code's ordering.
	for _, img := range images {
		blocks = append(blocks, api.ContentBlock{
			Type: "image",
			Source: &api.ImageSource{
				Type:      "base64",
				MediaType: "image/png",
				Data:      base64.StdEncoding.EncodeToString(img.Data),
			},
		})
	}

	if text != "" {
		blocks = append(blocks, api.ContentBlock{
			Type: "text",
			Text: text,
		})
	}

	return blocks
}
