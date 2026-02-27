package tui

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SubmitMsg is emitted when the user presses Enter to submit input.
type SubmitMsg struct {
	Text string
}

// InputModel wraps a bubbles textarea for multiline user input.
// Enter submits; Shift+Enter (Alt+Enter as fallback) inserts a newline.
type InputModel struct {
	textarea textarea.Model
	styles   Styles
	width    int

	// slashCmd tracks tab-completion state for slash commands.
	slashCmd SlashCommandState
}

// NewInputModel creates a configured input component.
func NewInputModel(styles Styles) InputModel {
	ta := textarea.New()
	ta.Placeholder = "ask glamdring something..."
	ta.CharLimit = 0 // no limit
	ta.ShowLineNumbers = false
	ta.SetHeight(3)

	// Style the textarea to match our theme.
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().
		Foreground(colorFgDim).
		Italic(true)
	ta.FocusedStyle.Text = lipgloss.NewStyle().
		Foreground(colorFgBright)
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().
		Foreground(colorAmber).
		Bold(true)
	ta.Prompt = "\u276f "

	ta.BlurredStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().
		Foreground(colorFgDim).
		Italic(true)
	ta.BlurredStyle.Text = lipgloss.NewStyle().
		Foreground(colorFg)
	ta.BlurredStyle.Prompt = lipgloss.NewStyle().
		Foreground(colorFgDim)
	ta.Prompt = "\u276f "

	// Swap Enter and Alt+Enter: Enter submits, Alt+Enter inserts newline.
	// The bubbles textarea treats Enter as newline by default.
	// We handle Enter in Update and use this key map for alt+enter.
	ta.KeyMap.InsertNewline.SetKeys("alt+enter")

	ta.Focus()

	return InputModel{
		textarea: ta,
		styles:   styles,
		slashCmd: NewSlashCommandState(),
	}
}

// Init returns the initial command for the input component.
func (m InputModel) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles messages for the input component.
func (m InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			value := m.textarea.Value()
			if value == "" {
				return m, nil
			}
			return m, func() tea.Msg {
				return SubmitMsg{Text: value}
			}

		case tea.KeyTab:
			// Tab completion for slash commands.
			value := m.textarea.Value()
			if IsSlashCommand(value) {
				completed := m.slashCmd.TabComplete(value)
				if completed != value {
					m.textarea.Reset()
					m.textarea.SetValue(completed)
					// Move cursor to end.
					m.textarea.CursorEnd()
				}
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// View renders the input component.
func (m InputModel) View() string {
	border := m.styles.InputBorder.Width(m.width - 2) // account for border
	return border.Render(m.textarea.View())
}

// Value returns the current text in the input.
func (m InputModel) Value() string {
	return m.textarea.Value()
}

// Reset clears the input text.
func (m *InputModel) Reset() {
	m.textarea.Reset()
}

// Focus gives focus to the input.
func (m *InputModel) Focus() tea.Cmd {
	return m.textarea.Focus()
}

// Blur removes focus from the input.
func (m *InputModel) Blur() {
	m.textarea.Blur()
}

// SetWidth updates the input width.
func (m *InputModel) SetWidth(w int) {
	m.width = w
	m.textarea.SetWidth(w - 4) // account for border + padding
}

// SetHeight updates the visible rows of the textarea.
func (m *InputModel) SetHeight(h int) {
	m.textarea.SetHeight(h)
}

// IsSlashCmd returns true if the current input text is a slash command.
func (m InputModel) IsSlashCmd() bool {
	return IsSlashCommand(m.textarea.Value())
}

// CmdName returns the command name if the input is a slash command.
func (m InputModel) CmdName() string {
	return CommandName(m.textarea.Value())
}

// SetAvailableCommands sets the list of slash command names for tab completion.
func (m *InputModel) SetAvailableCommands(names []string) {
	m.slashCmd.SetCommands(names)
}
