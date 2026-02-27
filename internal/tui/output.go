package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

const (
	// maxToolResultLines is the max lines to show for a tool result before truncating.
	maxToolResultLines = 30
)

// OutputModel wraps a bubbles viewport for displaying conversation output.
// It accumulates raw content blocks and re-renders markdown as needed.
type OutputModel struct {
	viewport viewport.Model
	styles   Styles
	renderer *glamour.TermRenderer

	// blocks holds the raw content segments in order.
	blocks []outputBlock
	// userScrolled tracks whether the user has scrolled up from the bottom.
	userScrolled bool

	width int
}

type blockKind int

const (
	blockText blockKind = iota
	blockToolCall
	blockToolResult
	blockThinking
	blockError
	blockUserMessage
)

type outputBlock struct {
	kind    blockKind
	content string
	isError bool // for tool results
}

// NewOutputModel creates an output viewport with glamour markdown rendering.
func NewOutputModel(styles Styles, width, height int) OutputModel {
	vp := viewport.New(width, height)
	vp.Style = lipgloss.NewStyle()

	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width-4),
	)

	return OutputModel{
		viewport: vp,
		styles:   styles,
		renderer: r,
		width:    width,
	}
}

// Init returns the initial command for the output.
func (m OutputModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the output viewport.
func (m OutputModel) Update(msg tea.Msg) (OutputModel, tea.Cmd) {
	var cmd tea.Cmd

	// Track whether user has scrolled away from the bottom before updating.
	atBottom := m.viewport.AtBottom()

	m.viewport, cmd = m.viewport.Update(msg)

	// After the viewport processes the message, determine if the user scrolled up.
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg:
		m.userScrolled = !m.viewport.AtBottom()
	default:
		// For non-user messages, preserve the previous state.
		if !atBottom {
			m.userScrolled = true
		}
	}

	return m, cmd
}

// View renders the output viewport.
func (m OutputModel) View() string {
	return m.viewport.View()
}

// AppendUserMessage adds a styled user message header and text.
func (m *OutputModel) AppendUserMessage(text string) {
	m.blocks = append(m.blocks, outputBlock{
		kind:    blockUserMessage,
		content: text,
	})
	m.rerender()
}

// AppendText adds agent text output (markdown).
func (m *OutputModel) AppendText(s string) {
	// If the last block is a text block, append to it (streaming).
	if len(m.blocks) > 0 && m.blocks[len(m.blocks)-1].kind == blockText {
		m.blocks[len(m.blocks)-1].content += s
	} else {
		m.blocks = append(m.blocks, outputBlock{kind: blockText, content: s})
	}
	m.rerender()
}

// AppendToolCall adds a tool call header block.
func (m *OutputModel) AppendToolCall(name, summary string) {
	content := fmt.Sprintf("%s: %s", name, summary)
	m.blocks = append(m.blocks, outputBlock{kind: blockToolCall, content: content})
	m.rerender()
}

// AppendToolResult adds a tool result block, truncating large output.
func (m *OutputModel) AppendToolResult(output string, isError bool) {
	m.blocks = append(m.blocks, outputBlock{
		kind:    blockToolResult,
		content: output,
		isError: isError,
	})
	m.rerender()
}

// AppendThinking adds a thinking block.
func (m *OutputModel) AppendThinking(s string) {
	// If the last block is thinking, append to it (streaming).
	if len(m.blocks) > 0 && m.blocks[len(m.blocks)-1].kind == blockThinking {
		m.blocks[len(m.blocks)-1].content += s
	} else {
		m.blocks = append(m.blocks, outputBlock{kind: blockThinking, content: s})
	}
	m.rerender()
}

// AppendError adds an error message block.
func (m *OutputModel) AppendError(s string) {
	m.blocks = append(m.blocks, outputBlock{kind: blockError, content: s})
	m.rerender()
}

// SetSize updates the viewport and re-renders content.
func (m *OutputModel) SetSize(width, height int) {
	m.width = width
	m.viewport.Width = width
	m.viewport.Height = height

	// Recreate renderer with new width.
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width-4),
	)
	if err == nil {
		m.renderer = r
	}

	m.rerender()
}

// rerender converts all accumulated blocks into styled text and updates the viewport.
func (m *OutputModel) rerender() {
	var parts []string

	for _, b := range m.blocks {
		switch b.kind {
		case blockUserMessage:
			header := m.styles.UserHeader.Render("\u2500\u2500 You ")
			dividerWidth := m.width - lipgloss.Width(header) - 1
			if dividerWidth < 0 {
				dividerWidth = 0
			}
			divider := m.styles.UserHeader.Render(strings.Repeat("\u2500", dividerWidth))
			rendered := renderMarkdown(m.renderer, b.content)
			parts = append(parts, header+divider+"\n"+rendered)

		case blockText:
			rendered := renderMarkdown(m.renderer, b.content)
			parts = append(parts, rendered)

		case blockToolCall:
			icon := m.styles.ToolCallIcon.Render("\u25b6")
			header := m.styles.ToolCallHeader.Render(b.content)
			parts = append(parts, icon+" "+header)

		case blockToolResult:
			output := b.content
			lines := strings.Split(output, "\n")
			if len(lines) > maxToolResultLines {
				output = strings.Join(lines[:maxToolResultLines], "\n")
				output += fmt.Sprintf("\n... (%d lines truncated)", len(lines)-maxToolResultLines)
			}
			if b.isError {
				parts = append(parts, m.styles.ToolResultErr.Render(output))
			} else {
				parts = append(parts, m.styles.ToolResult.Render(output))
			}

		case blockThinking:
			styled := m.styles.ThinkingText.Render(b.content)
			parts = append(parts, m.styles.ThinkingBorder.Render(styled))

		case blockError:
			parts = append(parts, m.styles.ErrorText.Render("error: "+b.content))
		}
	}

	content := strings.Join(parts, "\n")
	m.viewport.SetContent(content)

	// Auto-scroll to bottom unless user has scrolled up.
	if !m.userScrolled {
		m.viewport.GotoBottom()
	}
}

// renderMarkdown renders markdown text via glamour, falling back to plain text on error.
func renderMarkdown(r *glamour.TermRenderer, text string) string {
	if r == nil || text == "" {
		return text
	}
	rendered, err := r.Render(text)
	if err != nil {
		return text
	}
	return strings.TrimRight(rendered, "\n")
}
