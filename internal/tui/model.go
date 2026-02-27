package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justin/glamdring/pkg/agent"
)

// State represents the current UI mode.
type State int

const (
	StateInput      State = iota // user can type
	StateRunning                 // agent is working
	StatePermission              // waiting for permission response
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

	// cumulative token tracking
	totalInputTokens  int
	totalOutputTokens int
	turn              int
}

// New creates the root TUI model.
func New() Model {
	styles := DefaultStyles()
	return Model{
		input:     NewInputModel(styles),
		output:    NewOutputModel(styles, 80, 20),
		statusbar: NewStatusBar(styles),
		styles:    styles,
		state:     StateInput,
	}
}

// Init initializes the TUI.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.input.Init(),
		m.output.Init(),
	)
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

	case AgentMsg:
		return m.handleAgentMsg(msg)
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
		return m, tea.Quit
	case "esc":
		if m.state == StatePermission {
			// Deny permission on Escape.
			m.denyPermission()
			return m, nil
		}
	}

	switch m.state {
	case StateInput:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd

	case StatePermission:
		return m.handlePermissionKey(msg)

	case StateRunning:
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

	// Display the user message in the output.
	m.output.AppendUserMessage(msg.Text)
	m.input.Reset()
	m.input.Blur()

	m.turn++
	m.state = StateRunning

	// NOTE: actual agent communication is not wired up yet.
	// In production, this would return a tea.Cmd that starts the agent loop
	// and delivers AgentMsg values back.
	return m, nil
}

// handleAgentMsg routes agent messages to the appropriate component.
func (m Model) handleAgentMsg(msg AgentMsg) (tea.Model, tea.Cmd) {
	am := agent.Message(msg)

	switch am.Type {
	case agent.MessageTextDelta:
		m.output.AppendText(am.Text)

	case agent.MessageThinkingDelta:
		m.output.AppendThinking(am.Text)

	case agent.MessageToolCall:
		summary := summarizeToolInput(am.ToolName, am.ToolInput)
		m.output.AppendToolCall(am.ToolName, summary)

	case agent.MessageToolResult:
		m.output.AppendToolResult(am.ToolOutput, am.ToolIsError)

	case agent.MessagePermissionRequest:
		m.state = StatePermission
		m.permission = &am
		m.output.AppendToolCall("Permission Required", am.PermissionSummary)

	case agent.MessageError:
		errMsg := "unknown error"
		if am.Err != nil {
			errMsg = am.Err.Error()
		}
		m.output.AppendError(errMsg)

	case agent.MessageDone:
		m.totalInputTokens += am.InputTokens
		m.totalOutputTokens += am.OutputTokens
		m.statusbar.Update("claude-opus-4-6", m.totalInputTokens, m.totalOutputTokens, m.turn)
		m.state = StateInput
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

	switch msg.String() {
	case "y", "Y":
		if m.permission.PermissionResponse != nil {
			m.permission.PermissionResponse <- agent.PermissionApprove
		}
		m.permission = nil
		m.state = StateRunning
	case "a", "A":
		if m.permission.PermissionResponse != nil {
			m.permission.PermissionResponse <- agent.PermissionAlwaysApprove
		}
		m.permission = nil
		m.state = StateRunning
	case "n", "N":
		m.denyPermission()
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
	// Layout: output (fills space) | status bar (1 line) | input (bottom)
	output := m.output.View()
	status := m.statusbar.View()

	var input string
	switch m.state {
	case StatePermission:
		input = m.renderPermissionPrompt()
	default:
		input = m.input.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		output,
		status,
		input,
	)
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
