package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// detectGlamourStyle returns the appropriate glamour style option.
// Uses dark style for TTYs to avoid OSC terminal queries that leak
// escape sequences under alt screen. Falls back to notty for non-TTY
// environments (tests, pipes).
func detectGlamourStyle() glamour.TermRendererOption {
	if term.IsTerminal(int(os.Stdout.Fd())) {
		return glamour.WithStylePath("dark")
	}
	return glamour.WithStylePath("notty")
}

const (
	// collapseThreshold is the line count above which tool results are collapsed.
	collapseThreshold = 8

	// collapsePreviewLines is the number of lines to show when a tool result is collapsed.
	collapsePreviewLines = 3
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
	// hasNewContent is true when new content arrived while user is scrolled up.
	hasNewContent bool

	// collapsed tracks whether each tool result block is collapsed.
	// Keyed by the block's index in the blocks slice.
	collapsed map[int]bool

	width         int
	rendererDirty bool

	// glamourStyle is the resolved glamour style, detected once at creation
	// to avoid repeated OSC terminal queries on resize.
	glamourStyle glamour.TermRendererOption
}

type blockKind int

const (
	blockText blockKind = iota
	blockToolCall
	blockToolResult
	blockThinking
	blockError
	blockUserMessage
	blockSystem
)

type outputBlock struct {
	kind      blockKind
	content   string
	isError   bool   // for tool results
	finalized bool   // true when block is complete (no more appends)
	rendered  string // cached rendered output
}

// NewOutputModel creates an output viewport with glamour markdown rendering.
func NewOutputModel(styles Styles, width, height int) OutputModel {
	vp := viewport.New(width, height)
	vp.Style = lipgloss.NewStyle()

	// Use dark style directly to avoid OSC terminal background queries,
	// which leak escape sequences into stdin under alt screen.
	glamourStyle := detectGlamourStyle()

	r, _ := glamour.NewTermRenderer(
		glamourStyle,
		glamour.WithWordWrap(width-4),
	)

	return OutputModel{
		viewport:     vp,
		styles:       styles,
		renderer:     r,
		width:        width,
		collapsed:    make(map[int]bool),
		glamourStyle: glamourStyle,
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

	switch msg := msg.(type) {
	case tea.KeyMsg:
		cmd = m.handleScrollKey(msg)
		m.userScrolled = !m.viewport.AtBottom()
		// If user scrolled back to bottom, clear the new-content indicator.
		if m.viewport.AtBottom() {
			m.hasNewContent = false
		}
		return m, cmd

	case tea.MouseMsg:
		m.viewport, cmd = m.viewport.Update(msg)
		m.userScrolled = !m.viewport.AtBottom()
		if m.viewport.AtBottom() {
			m.hasNewContent = false
		}
		return m, cmd

	default:
		m.viewport, cmd = m.viewport.Update(msg)
		// For non-user messages, preserve the previous state.
		if !atBottom {
			m.userScrolled = true
		}
	}

	return m, cmd
}

// handleScrollKey processes keyboard input for viewport scrolling.
// Returns a tea.Cmd (always nil for scroll operations).
func (m *OutputModel) handleScrollKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyUp:
		m.viewport.LineUp(1)
	case tea.KeyDown:
		m.viewport.LineDown(1)
	case tea.KeyPgUp:
		m.viewport.ViewUp()
	case tea.KeyPgDown:
		m.viewport.ViewDown()
	case tea.KeyHome:
		m.viewport.GotoTop()
	case tea.KeyEnd:
		m.viewport.GotoBottom()
	default:
		switch msg.String() {
		case "k":
			m.viewport.LineUp(1)
		case "j":
			m.viewport.LineDown(1)
		case "ctrl+u":
			m.viewport.HalfViewUp()
		case "ctrl+d":
			m.viewport.HalfViewDown()
		case "G":
			m.viewport.GotoBottom()
		case "g":
			m.viewport.GotoTop()
		default:
			// Let the viewport handle anything else (mouse, etc.)
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return cmd
		}
	}
	return nil
}

// View renders the output viewport, plus a "new content below" indicator
// when the user has scrolled up and new content has arrived.
func (m OutputModel) View() string {
	view := m.viewport.View()
	if m.userScrolled && m.hasNewContent {
		indicator := m.styles.NewContentIndicator.Render(" new content below ")
		// Overlay the indicator at the bottom-right of the viewport.
		viewLines := strings.Split(view, "\n")
		if len(viewLines) > 0 {
			lastIdx := len(viewLines) - 1
			lastLine := viewLines[lastIdx]
			indicatorWidth := lipgloss.Width(indicator)
			lastLineWidth := lipgloss.Width(lastLine)
			if lastLineWidth+indicatorWidth < m.width {
				padding := m.width - indicatorWidth - 1
				if padding < 0 {
					padding = 0
				}
				viewLines[lastIdx] = strings.Repeat(" ", padding) + indicator
			} else {
				viewLines = append(viewLines, strings.Repeat(" ", m.width-indicatorWidth-1)+indicator)
			}
			return strings.Join(viewLines, "\n")
		}
	}
	return view
}

// AppendUserMessage adds a styled user message header and text.
func (m *OutputModel) AppendUserMessage(text string) {
	m.finalizePreviousBlock()
	m.blocks = append(m.blocks, outputBlock{
		kind:      blockUserMessage,
		content:   text,
		finalized: true,
	})
	m.rerender()
}

// finalizePreviousBlock marks the last block as finalized if it is not
// already. Called when a new block starts, indicating the previous streaming
// block is complete.
func (m *OutputModel) finalizePreviousBlock() {
	if len(m.blocks) == 0 {
		return
	}
	m.blocks[len(m.blocks)-1].finalized = true
}

// AppendText adds agent text output (markdown).
func (m *OutputModel) AppendText(s string) {
	// If the last block is a text block, append to it (streaming).
	if len(m.blocks) > 0 && m.blocks[len(m.blocks)-1].kind == blockText {
		m.blocks[len(m.blocks)-1].content += s
	} else {
		m.finalizePreviousBlock()
		m.blocks = append(m.blocks, outputBlock{kind: blockText, content: s})
	}
	m.rerender()
}

// AppendToolCall adds a tool call header block.
func (m *OutputModel) AppendToolCall(name, summary string) {
	m.finalizePreviousBlock()
	content := fmt.Sprintf("%s: %s", name, summary)
	m.blocks = append(m.blocks, outputBlock{kind: blockToolCall, content: content, finalized: true})
	m.rerender()
}

// AppendToolOutputDelta appends streaming output from a tool execution.
// Creates a non-finalized blockToolResult if one doesn't exist, or appends
// to the existing one.
func (m *OutputModel) AppendToolOutputDelta(text string) {
	if len(m.blocks) > 0 {
		last := &m.blocks[len(m.blocks)-1]
		if last.kind == blockToolResult && !last.finalized {
			last.content += text
			last.rendered = "" // invalidate cache
			m.rerender()
			return
		}
	}
	m.finalizePreviousBlock()
	m.blocks = append(m.blocks, outputBlock{
		kind:    blockToolResult,
		content: text,
	})
	m.rerender()
}

// AppendToolResult adds a tool result block. If the last block is an
// in-progress streaming tool result, it finalizes that block with the
// final output instead of creating a new one.
func (m *OutputModel) AppendToolResult(output string, isError bool) {
	// Check if we have a streaming block to finalize.
	if len(m.blocks) > 0 {
		last := &m.blocks[len(m.blocks)-1]
		if last.kind == blockToolResult && !last.finalized {
			last.content = output
			last.isError = isError
			last.finalized = true
			last.rendered = "" // invalidate cache
			idx := len(m.blocks) - 1
			lines := strings.Split(output, "\n")
			if len(lines) >= collapseThreshold {
				m.collapsed[idx] = true
			}
			m.rerender()
			return
		}
	}

	m.finalizePreviousBlock()
	idx := len(m.blocks)
	m.blocks = append(m.blocks, outputBlock{
		kind:      blockToolResult,
		content:   output,
		isError:   isError,
		finalized: true,
	})
	// Auto-collapse large tool results.
	lines := strings.Split(output, "\n")
	if len(lines) >= collapseThreshold {
		m.collapsed[idx] = true
	}
	m.rerender()
}

// AppendThinking adds a thinking block.
func (m *OutputModel) AppendThinking(s string) {
	// If the last block is thinking, append to it (streaming).
	if len(m.blocks) > 0 && m.blocks[len(m.blocks)-1].kind == blockThinking {
		m.blocks[len(m.blocks)-1].content += s
	} else {
		m.finalizePreviousBlock()
		m.blocks = append(m.blocks, outputBlock{kind: blockThinking, content: s})
	}
	m.rerender()
}

// AppendSystem adds a system message block (for built-in command output).
func (m *OutputModel) AppendSystem(s string) {
	m.finalizePreviousBlock()
	m.blocks = append(m.blocks, outputBlock{kind: blockSystem, content: s, finalized: true})
	m.rerender()
}

// Clear removes all content blocks and resets the viewport.
func (m *OutputModel) Clear() {
	m.blocks = nil
	m.collapsed = make(map[int]bool)
	m.userScrolled = false
	m.hasNewContent = false
	m.viewport.SetContent("")
	m.viewport.GotoTop()
}

// AppendError adds an error message block.
func (m *OutputModel) AppendError(s string) {
	m.finalizePreviousBlock()
	m.blocks = append(m.blocks, outputBlock{kind: blockError, content: s, finalized: true})
	m.rerender()
}

// ToggleCollapse toggles the collapsed state of the tool result block at
// the given block index. Returns true if the index was valid and toggled.
func (m *OutputModel) ToggleCollapse(blockIdx int) bool {
	if blockIdx < 0 || blockIdx >= len(m.blocks) {
		return false
	}
	if m.blocks[blockIdx].kind != blockToolResult {
		return false
	}
	m.collapsed[blockIdx] = !m.collapsed[blockIdx]
	m.blocks[blockIdx].rendered = "" // invalidate cache
	m.rerender()
	return true
}

// ToggleLastToolResult finds the last tool result block and toggles its
// collapsed state. Used by the 'e' key binding during StateRunning.
func (m *OutputModel) ToggleLastToolResult() bool {
	for i := len(m.blocks) - 1; i >= 0; i-- {
		if m.blocks[i].kind == blockToolResult {
			return m.ToggleCollapse(i)
		}
	}
	return false
}

// SetSize updates the viewport and re-renders content.
// Only width changes trigger renderer recreation and cache invalidation;
// height-only changes just update the viewport dimensions.
func (m *OutputModel) SetSize(width, height int) {
	m.viewport.Width = width
	m.viewport.Height = height

	if width != m.width {
		m.width = width
		m.rendererDirty = true
		// Invalidate render cache since width changed.
		for i := range m.blocks {
			m.blocks[i].rendered = ""
		}
	}

	m.rerender()
}

// rerender converts all accumulated blocks into styled text and updates the viewport.
// Finalized blocks with a cached render are reused without re-rendering.
func (m *OutputModel) rerender() {
	if m.rendererDirty {
		wrapWidth := m.width - 4
		if wrapWidth < 1 {
			wrapWidth = 1
		}
		r, err := glamour.NewTermRenderer(
			m.glamourStyle,
			glamour.WithWordWrap(wrapWidth),
		)
		if err == nil {
			m.renderer = r
			m.rendererDirty = false
		}
	}

	var parts []string

	for i, b := range m.blocks {
		// Use cached render for finalized blocks.
		if b.finalized && b.rendered != "" {
			parts = append(parts, b.rendered)
			continue
		}

		var rendered string
		switch b.kind {
		case blockUserMessage:
			header := m.styles.UserHeader.Render("\u2500\u2500 You ")
			dividerWidth := m.width - lipgloss.Width(header) - 1
			if dividerWidth < 0 {
				dividerWidth = 0
			}
			divider := m.styles.UserHeader.Render(strings.Repeat("\u2500", dividerWidth))
			md := renderMarkdown(m.renderer, b.content)
			rendered = header + divider + "\n" + md

		case blockText:
			rendered = renderMarkdown(m.renderer, b.content)

		case blockToolCall:
			icon := m.styles.ToolCallIcon.Render("\u25b6")
			header := m.styles.ToolCallHeader.Render(b.content)
			rendered = icon + " " + header

		case blockToolResult:
			output := m.renderToolResult(i, b)
			if b.isError {
				rendered = m.styles.ToolResultErr.Render(output)
			} else {
				rendered = m.styles.ToolResult.Render(output)
			}

		case blockThinking:
			rendered = m.renderThinkingBlock(i, b)

		case blockSystem:
			styled := m.styles.SystemText.Render(b.content)
			rendered = m.styles.SystemBorder.Render(styled)

		case blockError:
			rendered = m.styles.ErrorText.Render("error: " + b.content)
		}

		// Cache the rendered output for finalized blocks. Block indices are
		// stable (append-only) so the cache key is valid for the session.
		if b.finalized {
			m.blocks[i].rendered = rendered
		}
		parts = append(parts, rendered)
	}

	content := strings.Join(parts, "\n")
	m.viewport.SetContent(content)

	// Auto-scroll to bottom unless user has scrolled up.
	if !m.userScrolled {
		m.viewport.GotoBottom()
	} else {
		// Mark that there is new content the user hasn't seen.
		m.hasNewContent = true
	}
}

// renderToolResult renders a tool result block, handling collapse.
func (m *OutputModel) renderToolResult(idx int, b outputBlock) string {
	output := b.content
	lines := strings.Split(output, "\n")
	totalLines := len(lines)

	// Check if this block is collapsed.
	if m.collapsed[idx] && totalLines >= collapseThreshold {
		preview := strings.Join(lines[:collapsePreviewLines], "\n")
		remaining := totalLines - collapsePreviewLines
		return preview + fmt.Sprintf("\n... (%d more lines, press 'e' to expand)", remaining)
	}

	return output
}

// renderThinkingBlock renders a thinking block with lavender border, italic text,
// and a visual separator between thinking and subsequent content.
func (m *OutputModel) renderThinkingBlock(idx int, b outputBlock) string {
	styled := m.styles.ThinkingText.Render(b.content)
	block := m.styles.ThinkingBorder.Render(styled)

	// Add a visual separator after the thinking block to distinguish it
	// from the response text that follows.
	separator := m.styles.ThinkingSeparator.Render(
		strings.Repeat("\u2508", clamp(m.width/2, 1, 40)),
	)
	return block + "\n" + separator
}

// clamp constrains val between lo and hi.
func clamp(val, lo, hi int) int {
	if val < lo {
		return lo
	}
	if val > hi {
		return hi
	}
	return val
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
