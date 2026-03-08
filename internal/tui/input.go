package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PendingImage holds a clipboard image staged for submission.
type PendingImage struct {
	Data   []byte // raw PNG bytes
	Width  int    // image width in pixels
	Height int    // image height in pixels
}

// SubmitMsg is emitted when the user presses Enter to submit input.
type SubmitMsg struct {
	Text   string
	Images []PendingImage
}

// IsSearching returns true if the input is in Ctrl+R reverse search mode.
func (m InputModel) IsSearching() bool {
	return m.searching
}

// TrySubmit attempts a synchronous submission. Returns a SubmitMsg if there
// is content to submit (text or images), or nil if the input is empty.
// Resets the input state on success. Called by the parent model to handle
// Enter synchronously so the user message appears in the same render frame.
func (m *InputModel) TrySubmit() *SubmitMsg {
	value := m.textarea.Value()
	if value == "" && len(m.pendingImages) == 0 {
		return nil
	}
	m.history.ResetCursor()
	m.textarea.Reset()
	images := m.pendingImages
	m.pendingImages = nil
	return &SubmitMsg{Text: value, Images: images}
}

// InputModel wraps a bubbles textarea for multiline user input.
// Enter submits; Shift+Enter (Alt+Enter as fallback) inserts a newline.
type InputModel struct {
	textarea textarea.Model
	styles   Styles
	palette  ThemePalette
	width    int

	// slashCmd tracks tab-completion state for slash commands.
	slashCmd SlashCommandState

	// history tracks input history for Up/Down navigation.
	history History

	// pendingImages holds images staged via Ctrl+V for the next submission.
	pendingImages []PendingImage

	// searching is true when Ctrl+R reverse search is active.
	searching bool
	// searchQuery is the current search input during Ctrl+R.
	searchQuery string
	// searchResults holds the filtered history entries.
	searchResults []string
	// searchIdx is the index into searchResults.
	searchIdx int
}

// maxInputHeight is the maximum number of visible rows for the input textarea.
const maxInputHeight = 8

// NewInputModel creates a configured input component.
func NewInputModel(styles Styles, palette ThemePalette) InputModel {
	ta := textarea.New()
	ta.Placeholder = "ask glamdring something..."
	ta.CharLimit = 0 // no limit
	ta.ShowLineNumbers = false
	ta.SetHeight(1)

	// Style the textarea to match our theme.
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().
		Foreground(palette.FgDim).
		Italic(true)
	ta.FocusedStyle.Text = lipgloss.NewStyle().
		Foreground(palette.FgBright)
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().
		Foreground(palette.Primary).
		Bold(true)
	ta.Prompt = "\u276f "

	ta.BlurredStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().
		Foreground(palette.FgDim).
		Italic(true)
	ta.BlurredStyle.Text = lipgloss.NewStyle().
		Foreground(palette.Fg)
	ta.BlurredStyle.Prompt = lipgloss.NewStyle().
		Foreground(palette.FgDim)
	ta.Prompt = "\u276f "

	// Swap Enter and Alt+Enter: Enter submits, Alt+Enter inserts newline.
	// The bubbles textarea treats Enter as newline by default.
	// We handle Enter in Update and use this key map for alt+enter.
	ta.KeyMap.InsertNewline.SetKeys("alt+enter")

	ta.Focus()

	return InputModel{
		textarea: ta,
		styles:   styles,
		palette:  palette,
		slashCmd: NewSlashCommandState(),
	}
}

// SetTheme updates the input styling without destroying state (history,
// tab completion, pending images, current text).
func (m *InputModel) SetTheme(styles Styles, palette ThemePalette) {
	m.styles = styles
	m.palette = palette
	m.textarea.FocusedStyle.Placeholder = lipgloss.NewStyle().
		Foreground(palette.FgDim).
		Italic(true)
	m.textarea.FocusedStyle.Text = lipgloss.NewStyle().
		Foreground(palette.FgBright)
	m.textarea.FocusedStyle.Prompt = lipgloss.NewStyle().
		Foreground(palette.Primary).
		Bold(true)
	m.textarea.BlurredStyle.Placeholder = lipgloss.NewStyle().
		Foreground(palette.FgDim).
		Italic(true)
	m.textarea.BlurredStyle.Text = lipgloss.NewStyle().
		Foreground(palette.Fg)
	m.textarea.BlurredStyle.Prompt = lipgloss.NewStyle().
		Foreground(palette.FgDim)
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

		// Handle Ctrl+V: check clipboard for image first, then text.
		if msg.Type == tea.KeyCtrlV {
			if imgData, ok := ReadClipboardImage(); ok {
				w, h := pngDimensions(imgData)
				if w > 0 && h > 0 {
					m.pendingImages = append(m.pendingImages, PendingImage{
						Data:   imgData,
						Width:  w,
						Height: h,
					})
					return m, nil
				}
				// Not a valid PNG; fall through to text paste.
			}
			// No image -- fall through to paste text.
			if text, ok := ReadClipboardText(); ok {
				m.textarea.InsertString(text)
				return m, nil
			}
			return m, nil
		}

		switch msg.Type {
		case tea.KeyEnter:
			if sub := m.TrySubmit(); sub != nil {
				return m, func() tea.Msg { return *sub }
			}
			return m, nil

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
	border := m.styles.InputBorder.Width(m.width - 2)
	var content string
	if len(m.pendingImages) > 0 {
		var indicators []string
		for i, img := range m.pendingImages {
			if img.Width > 0 && img.Height > 0 {
				indicators = append(indicators, fmt.Sprintf("[Image %d: %dx%d]", i+1, img.Width, img.Height))
			} else {
				indicators = append(indicators, fmt.Sprintf("[Image %d]", i+1))
			}
		}
		imageBar := lipgloss.NewStyle().Foreground(m.palette.Primary).Render(strings.Join(indicators, " "))
		content = imageBar + "\n" + m.textarea.View()
	} else {
		content = m.textarea.View()
	}
	return border.Render(content)
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

// Reset clears the input text and any staged images.
func (m *InputModel) Reset() {
	m.textarea.Reset()
	m.pendingImages = nil
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

// DesiredHeight returns the number of rows the textarea wants based on content,
// clamped between 1 and maxInputHeight.
func (m InputModel) DesiredHeight() int {
	lines := m.textarea.LineCount()
	if lines < 1 {
		lines = 1
	}
	if lines > maxInputHeight {
		lines = maxInputHeight
	}
	return lines
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

// pngDimensions extracts width and height from a PNG file's IHDR chunk.
// Returns 0, 0 if the data is not a valid PNG.
func pngDimensions(data []byte) (int, int) {
	// PNG header: 8-byte signature, then IHDR chunk.
	// IHDR starts at byte 16: 4 bytes width, 4 bytes height (big-endian).
	if len(data) < 24 {
		return 0, 0
	}
	// Check PNG signature.
	if data[0] != 0x89 || data[1] != 'P' || data[2] != 'N' || data[3] != 'G' {
		return 0, 0
	}
	// Verify IHDR chunk type.
	if data[12] != 'I' || data[13] != 'H' || data[14] != 'D' || data[15] != 'R' {
		return 0, 0
	}
	w := int(data[16])<<24 | int(data[17])<<16 | int(data[18])<<8 | int(data[19])
	h := int(data[20])<<24 | int(data[21])<<16 | int(data[22])<<8 | int(data[23])
	return w, h
}
