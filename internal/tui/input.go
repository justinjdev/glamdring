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

	// history tracks input history for Up/Down navigation.
	history History

	// searching is true when Ctrl+R reverse search is active.
	searching bool
	// searchQuery is the current search input during Ctrl+R.
	searchQuery string
	// searchResults holds the filtered history entries.
	searchResults []string
	// searchIdx is the index into searchResults.
	searchIdx int
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
		// Handle Ctrl+R search mode.
		if m.searching {
			return m.handleSearchKey(msg)
		}

		switch msg.Type {
		case tea.KeyEnter:
			value := m.textarea.Value()
			if value == "" {
				return m, nil
			}
			m.history.ResetCursor()
			return m, func() tea.Msg {
				return SubmitMsg{Text: value}
			}

		case tea.KeyUp:
			// Only activate history when cursor is on the first line.
			if m.textarea.Line() == 0 {
				if text, ok := m.history.Up(m.textarea.Value()); ok {
					m.textarea.Reset()
					m.textarea.SetValue(text)
					m.textarea.CursorEnd()
					return m, nil
				}
				return m, nil
			}

		case tea.KeyDown:
			// Only activate history when cursor is on the last line.
			if m.textarea.Line() == m.textarea.LineCount()-1 {
				if text, ok := m.history.Down(); ok {
					m.textarea.Reset()
					m.textarea.SetValue(text)
					m.textarea.CursorEnd()
					return m, nil
				}
				return m, nil
			}

		case tea.KeyTab:
			// Tab completion for slash commands.
			value := m.textarea.Value()
			if IsSlashCommand(value) {
				completed := m.slashCmd.TabComplete(value)
				if completed != value {
					m.textarea.Reset()
					m.textarea.SetValue(completed)
					m.textarea.CursorEnd()
				}
				return m, nil
			}

		case tea.KeyCtrlR:
			m.searching = true
			m.searchQuery = ""
			m.searchResults = nil
			m.searchIdx = 0
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// handleSearchKey processes key events during Ctrl+R reverse search.
func (m InputModel) handleSearchKey(msg tea.KeyMsg) (InputModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		// Accept the current match.
		if len(m.searchResults) > 0 && m.searchIdx < len(m.searchResults) {
			m.textarea.Reset()
			m.textarea.SetValue(m.searchResults[m.searchIdx])
			m.textarea.CursorEnd()
		}
		m.searching = false
		m.searchQuery = ""
		m.searchResults = nil
		return m, nil

	case tea.KeyEsc, tea.KeyCtrlC:
		// Cancel search.
		m.searching = false
		m.searchQuery = ""
		m.searchResults = nil
		return m, nil

	case tea.KeyCtrlR:
		// Cycle to next result.
		if len(m.searchResults) > 0 {
			m.searchIdx = (m.searchIdx + 1) % len(m.searchResults)
		}
		return m, nil

	case tea.KeyBackspace:
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.searchResults = m.history.Search(m.searchQuery)
			m.searchIdx = 0
		}
		return m, nil

	case tea.KeyRunes:
		m.searchQuery += string(msg.Runes)
		m.searchResults = m.history.Search(m.searchQuery)
		m.searchIdx = 0
		return m, nil
	}

	return m, nil
}

// View renders the input component.
func (m InputModel) View() string {
	if m.searching {
		return m.renderSearch()
	}
	border := m.styles.InputBorder.Width(m.width - 2) // account for border
	return border.Render(m.textarea.View())
}

// renderSearch renders the Ctrl+R reverse search prompt.
func (m InputModel) renderSearch() string {
	prompt := "reverse-i-search: " + m.searchQuery + "_"
	var match string
	if len(m.searchResults) > 0 && m.searchIdx < len(m.searchResults) {
		match = m.searchResults[m.searchIdx]
		// Truncate long matches for display.
		if len(match) > 60 {
			match = match[:57] + "..."
		}
	}
	var content string
	if match != "" {
		content = prompt + "\n" + match
	} else if m.searchQuery != "" {
		content = prompt + "\n(no match)"
	} else {
		content = prompt
	}
	border := m.styles.InputBorder.Width(m.width - 2)
	return border.Render(content)
}

// SearchActive returns true if reverse search is currently active.
func (m InputModel) SearchActive() bool {
	return m.searching
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
