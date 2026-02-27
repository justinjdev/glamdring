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

	return InputModel{
		textarea: ta,
		styles:   styles,
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
