package tui

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// renderInterval is the minimum time between viewport re-renders during
// streaming. Limits rendering to ~60fps to avoid jerkiness from re-rendering
// on every single delta event.
const renderInterval = 16 * time.Millisecond

// renderTickMsg signals that a pending render should be flushed.
type renderTickMsg struct{}

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

	// dirty is true when content has changed but the viewport has not yet
	// been re-rendered. Used for render throttling during streaming.
	dirty bool

	// Pending buffers accumulate streaming text between render ticks.
	// DrainPending moves a proportional chunk to the actual blocks each
	// tick, creating a smooth typewriter effect instead of bursty chunks.
	pendingText    string
	pendingThink   string
	pendingToolOut string

	// toolSpinner holds the current spinner frame (e.g. "⣾") to display
	// inline on the last tool call block while a tool is running.
	// Empty string means no tool is actively running.
	toolSpinner string

	// headerInfo stores the raw "ver\nmodel\ncwd" string for re-rendering
	// the banner when the theme changes.
	headerInfo string

	// starFrame tracks the animation frame for the assistant header star.
	// Even frames show ✧ (outline), odd frames show ✦ (filled).
	starFrame int
	// starDone marks the star as finalized (filled ✦ in primary color).
	starDone bool
}

type blockKind int

const (
	blockText blockKind = iota
	blockToolCall
	blockToolResult
	blockThinking
	blockError
	blockUserMessage
	blockHeader
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

// SetToolSpinner sets the inline spinner for the last tool call block
// and re-renders immediately.
func (m *OutputModel) SetToolSpinner(view string) {
	m.toolSpinner = view
	m.doRender()
}

// ClearToolSpinner removes the inline tool spinner.
func (m *OutputModel) ClearToolSpinner() {
	if m.toolSpinner != "" {
		m.toolSpinner = ""
	}
}

// lastToolCallIndex returns the index of the last blockToolCall, or -1.
func (m *OutputModel) lastToolCallIndex() int {
	for i := len(m.blocks) - 1; i >= 0; i-- {
		if m.blocks[i].kind == blockToolCall {
			return i
		}
	}
	return -1
}

// banner holds the ANSI Shadow figlet art for "GLAMDRING".
// Each line is colored with a gradient from the theme palette.
var banner = [6]string{
	" \u2588\u2588\u2588\u2588\u2588\u2588\u2557 \u2588\u2588\u2557      \u2588\u2588\u2588\u2588\u2588\u2557 \u2588\u2588\u2588\u2557   \u2588\u2588\u2588\u2557\u2588\u2588\u2588\u2588\u2588\u2588\u2557 \u2588\u2588\u2588\u2588\u2588\u2588\u2557 \u2588\u2588\u2557\u2588\u2588\u2588\u2557   \u2588\u2588\u2557 \u2588\u2588\u2588\u2588\u2588\u2588\u2557 ",
	"\u2588\u2588\u2554\u2550\u2550\u2550\u2550\u255d \u2588\u2588\u2551     \u2588\u2588\u2554\u2550\u2550\u2588\u2588\u2557\u2588\u2588\u2588\u2588\u2557 \u2588\u2588\u2588\u2588\u2551\u2588\u2588\u2554\u2550\u2550\u2588\u2588\u2557\u2588\u2588\u2554\u2550\u2550\u2588\u2588\u2557\u2588\u2588\u2551\u2588\u2588\u2588\u2588\u2557  \u2588\u2588\u2551\u2588\u2588\u2554\u2550\u2550\u2550\u2550\u255d ",
	"\u2588\u2588\u2551  \u2588\u2588\u2588\u2557\u2588\u2588\u2551     \u2588\u2588\u2588\u2588\u2588\u2588\u2588\u2551\u2588\u2588\u2554\u2588\u2588\u2588\u2588\u2554\u2588\u2588\u2551\u2588\u2588\u2551  \u2588\u2588\u2551\u2588\u2588\u2588\u2588\u2588\u2588\u2554\u255d\u2588\u2588\u2551\u2588\u2588\u2554\u2588\u2588\u2557 \u2588\u2588\u2551\u2588\u2588\u2551  \u2588\u2588\u2588\u2557",
	"\u2588\u2588\u2551   \u2588\u2588\u2551\u2588\u2588\u2551     \u2588\u2588\u2554\u2550\u2550\u2588\u2588\u2551\u2588\u2588\u2551\u255a\u2588\u2588\u2554\u255d\u2588\u2588\u2551\u2588\u2588\u2551  \u2588\u2588\u2551\u2588\u2588\u2554\u2550\u2550\u2588\u2588\u2557\u2588\u2588\u2551\u2588\u2588\u2551\u255a\u2588\u2588\u2557\u2588\u2588\u2551\u2588\u2588\u2551   \u2588\u2588\u2551",
	"\u255a\u2588\u2588\u2588\u2588\u2588\u2588\u2554\u255d\u2588\u2588\u2588\u2588\u2588\u2588\u2588\u2557\u2588\u2588\u2551  \u2588\u2588\u2551\u2588\u2588\u2551 \u255a\u2550\u255d \u2588\u2588\u2551\u2588\u2588\u2588\u2588\u2588\u2588\u2554\u255d\u2588\u2588\u2551  \u2588\u2588\u2551\u2588\u2588\u2551\u2588\u2588\u2551 \u255a\u2588\u2588\u2588\u2588\u2551\u255a\u2588\u2588\u2588\u2588\u2588\u2588\u2554\u255d",
	" \u255a\u2550\u2550\u2550\u2550\u2550\u255d \u255a\u2550\u2550\u2550\u2550\u2550\u2550\u255d\u255a\u2550\u255d  \u255a\u2550\u255d\u255a\u2550\u255d     \u255a\u2550\u255d\u255a\u2550\u2550\u2550\u2550\u2550\u255d \u255a\u2550\u255d  \u255a\u2550\u255d\u255a\u2550\u255d\u255a\u2550\u255d  \u255a\u2550\u2550\u2550\u255d \u255a\u2550\u2550\u2550\u2550\u2550\u255d ",
}

// AppendHeader adds a startup header block with the app banner.
// The content string has lines: "glamdring <ver>", "<model>", "<cwd>".
// If bannerImage is non-nil, it is rendered as half-block pixel art.
func (m *OutputModel) AppendHeader(content string, styles Styles, palette ThemePalette) {
	m.finalizePreviousBlock()
	m.headerInfo = content

	m.blocks = append(m.blocks, outputBlock{
		kind:      blockHeader,
		content:   renderBanner(content, palette),
		finalized: true,
	})
	m.doRender()
}

// RefreshHeader re-renders the startup banner with the current palette.
func (m *OutputModel) RefreshHeader(palette ThemePalette) {
	if m.headerInfo == "" {
		return
	}
	for i := range m.blocks {
		if m.blocks[i].kind == blockHeader {
			m.blocks[i].content = renderBanner(m.headerInfo, palette)
			m.blocks[i].rendered = "" // invalidate cache
			m.doRender()
			return
		}
	}
}

// renderBanner creates the styled GLAMDRING banner text with theme gradient.
func renderBanner(content string, palette ThemePalette) string {
	lines := strings.Split(content, "\n")
	ver, model, cwd := "dev", "", ""
	if len(lines) >= 1 {
		ver = lines[0]
	}
	if len(lines) >= 2 {
		model = lines[1]
	}
	if len(lines) >= 3 {
		cwd = lines[2]
	}

	dim := lipgloss.NewStyle().Foreground(palette.FgDim)
	info := dim.Render("  " + ver + " \u2502 " + model + " \u2502 " + cwd)

	gradient := []lipgloss.Color{
		palette.FgBright, palette.Primary, palette.Primary,
		palette.Secondary, palette.Subtle, palette.FgDim,
	}
	var parts []string
	for i, line := range banner {
		style := lipgloss.NewStyle().Foreground(gradient[i])
		parts = append(parts, style.Render(line))
	}
	parts = append(parts, info)
	return strings.Join(parts, "\n")
}

// AppendUserMessage adds a styled user message header and text.
func (m *OutputModel) AppendUserMessage(text string) {
	m.finalizePreviousBlock()
	m.blocks = append(m.blocks, outputBlock{
		kind:      blockUserMessage,
		content:   text,
		finalized: true,
	})
	m.doRender()
}

// StartAssistantStar begins the star animation for a new agent response.
// The star is rendered as a prefix on the first content block, not as a
// separate block, so the text flows on the same line.
func (m *OutputModel) StartAssistantStar() {
	m.starFrame = 0
	m.starDone = false
}

// TickStar advances the star animation frame and re-renders.
func (m *OutputModel) TickStar() {
	if !m.starDone {
		m.starFrame++
		m.doRender()
	}
}

// FinalizeStar marks the star as done (solid filled) and clears the render
// cache on the first content block so it re-renders with the final star.
func (m *OutputModel) FinalizeStar() {
	if m.starDone {
		return
	}
	m.starDone = true
	// Invalidate the cached render of the first content block after the last
	// user message so doRender picks up the final star glyph.
	lastUserIdx := -1
	for i, b := range m.blocks {
		if b.kind == blockUserMessage {
			lastUserIdx = i
		}
	}
	if lastUserIdx >= 0 {
		for i := lastUserIdx + 1; i < len(m.blocks); i++ {
			if m.blocks[i].kind != blockSystem && m.blocks[i].kind != blockError {
				m.blocks[i].rendered = ""
				break
			}
		}
	}
	m.doRender()
}

// renderStar returns the styled star prefix for the current frame.
func (m *OutputModel) renderStar() string {
	if m.starDone {
		return m.styles.AssistantStarDone.Render("\u2726")
	}
	// Alternate every 4 frames (~320ms per blink at ~80ms/frame).
	if (m.starFrame/4)%2 == 0 {
		return m.styles.AssistantStar.Render("\u2727")
	}
	return m.styles.AssistantStarDone.Render("\u2726")
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

// AppendText buffers agent text output for gradual draining via DrainPending.
func (m *OutputModel) AppendText(s string) {
	m.pendingText += s
}

// appendTextImmediate moves text directly into the current text block.
func (m *OutputModel) appendTextImmediate(s string) {
	if len(m.blocks) > 0 && m.blocks[len(m.blocks)-1].kind == blockText {
		m.blocks[len(m.blocks)-1].content += s
	} else {
		m.finalizePreviousBlock()
		m.blocks = append(m.blocks, outputBlock{kind: blockText, content: s})
	}
}

// AppendToolCall adds a tool call header block.
func (m *OutputModel) AppendToolCall(name, summary string) {
	m.finalizePreviousBlock()
	content := fmt.Sprintf("%s: %s", name, summary)
	m.blocks = append(m.blocks, outputBlock{kind: blockToolCall, content: content, finalized: true})
	m.doRender()
}

// AppendToolOutputDelta buffers streaming tool output for gradual draining.
func (m *OutputModel) AppendToolOutputDelta(text string) {
	m.pendingToolOut += text
}

// appendToolOutImmediate moves tool output directly into the current block.
func (m *OutputModel) appendToolOutImmediate(text string) {
	if len(m.blocks) > 0 {
		last := &m.blocks[len(m.blocks)-1]
		if last.kind == blockToolResult && !last.finalized {
			last.content += text
			last.rendered = "" // invalidate cache
			return
		}
	}
	m.finalizePreviousBlock()
	m.blocks = append(m.blocks, outputBlock{
		kind:    blockToolResult,
		content: text,
	})
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
			m.doRender()
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
	m.doRender()
}

// AppendThinking buffers thinking text for gradual draining.
func (m *OutputModel) AppendThinking(s string) {
	m.pendingThink += s
}

// appendThinkImmediate moves thinking text directly into the current block.
func (m *OutputModel) appendThinkImmediate(s string) {
	if len(m.blocks) > 0 && m.blocks[len(m.blocks)-1].kind == blockThinking {
		m.blocks[len(m.blocks)-1].content += s
	} else {
		m.finalizePreviousBlock()
		m.blocks = append(m.blocks, outputBlock{kind: blockThinking, content: s})
	}
}

// AppendSystem adds a system message block (for built-in command output).
func (m *OutputModel) AppendSystem(s string) {
	m.finalizePreviousBlock()
	m.blocks = append(m.blocks, outputBlock{kind: blockSystem, content: s, finalized: true})
	m.doRender()
}

// Clear removes all content blocks and resets the viewport.
func (m *OutputModel) Clear() {
	m.blocks = nil
	m.collapsed = make(map[int]bool)
	m.userScrolled = false
	m.hasNewContent = false
	m.ClearPending()
	m.toolSpinner = ""
	m.dirty = false
	m.viewport.SetContent("")
	m.viewport.GotoTop()
}

// AppendError adds an error message block.
func (m *OutputModel) AppendError(s string) {
	m.finalizePreviousBlock()
	m.blocks = append(m.blocks, outputBlock{kind: blockError, content: s, finalized: true})
	m.doRender()
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
	m.doRender()
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

	m.doRender()
}

// IsDirty returns true if content has changed since the last render.
func (m *OutputModel) IsDirty() bool {
	return m.dirty
}

// FlushRender performs the actual rendering if the output is dirty.
func (m *OutputModel) FlushRender() {
	if !m.dirty {
		return
	}
	m.dirty = false
	m.doRender()
}

// HasPending returns true if any pending text buffers have content.
func (m *OutputModel) HasPending() bool {
	return len(m.pendingText) > 0 || len(m.pendingThink) > 0 || len(m.pendingToolOut) > 0
}

// DrainPending moves a proportional chunk from each pending buffer into the
// actual blocks and renders. Returns true if there is still pending content.
func (m *OutputModel) DrainPending() bool {
	drained := false

	if len(m.pendingText) > 0 {
		take := runeAlignedChunk(m.pendingText)
		m.appendTextImmediate(m.pendingText[:take])
		m.pendingText = m.pendingText[take:]
		drained = true
	}
	if len(m.pendingThink) > 0 {
		take := runeAlignedChunk(m.pendingThink)
		m.appendThinkImmediate(m.pendingThink[:take])
		m.pendingThink = m.pendingThink[take:]
		drained = true
	}
	if len(m.pendingToolOut) > 0 {
		take := runeAlignedChunk(m.pendingToolOut)
		m.appendToolOutImmediate(m.pendingToolOut[:take])
		m.pendingToolOut = m.pendingToolOut[take:]
		drained = true
	}

	if drained {
		m.doRender()
	} else {
		m.FlushRender()
	}
	return m.HasPending()
}

// FlushAllPending moves all remaining pending text into blocks and renders.
func (m *OutputModel) FlushAllPending() {
	if len(m.pendingText) > 0 {
		m.appendTextImmediate(m.pendingText)
		m.pendingText = ""
	}
	if len(m.pendingThink) > 0 {
		m.appendThinkImmediate(m.pendingThink)
		m.pendingThink = ""
	}
	if len(m.pendingToolOut) > 0 {
		m.appendToolOutImmediate(m.pendingToolOut)
		m.pendingToolOut = ""
	}
	m.dirty = false
	m.doRender()
}

// ClearPending discards all pending text buffers without rendering them.
// Used when an agent turn is interrupted to prevent stale buffered text
// from leaking into the next response.
func (m *OutputModel) ClearPending() {
	m.pendingText = ""
	m.pendingThink = ""
	m.pendingToolOut = ""
}

// drainChunkSize returns how many bytes to move from a pending buffer of
// size n. Adapts so small buffers drip slowly (smooth) while large buffers
// drain fast enough to keep up with the API.
func drainChunkSize(n int) int {
	// Aim to drain the buffer in ~6 ticks (~96ms), with a floor of 2 chars.
	rate := n / 6
	if rate < 2 {
		rate = 2
	}
	if rate > n {
		rate = n
	}
	return rate
}

// runeAlignedChunk returns the byte length of a chunk of s that is safe to
// slice — i.e., it does not split a multi-byte UTF-8 sequence. It starts from
// drainChunkSize(len(s)) bytes and retreats to the nearest rune boundary,
// ensuring at least one complete rune is always consumed.
func runeAlignedChunk(s string) int {
	n := len(s)
	take := drainChunkSize(n)
	if take >= n {
		return n
	}
	// Retreat until we land on a rune-start byte.
	for take > 0 && !utf8.RuneStart(s[take]) {
		take--
	}
	// If we retreated all the way to 0, advance past the first rune so we
	// always make forward progress.
	if take == 0 {
		_, size := utf8.DecodeRuneInString(s)
		take = size
	}
	return take
}

// scheduleRenderTick returns a tea.Cmd that fires a renderTickMsg after renderInterval.
func scheduleRenderTick() tea.Cmd {
	return tea.Tick(renderInterval, func(time.Time) tea.Msg {
		return renderTickMsg{}
	})
}

// doRender converts all accumulated blocks into styled text and updates the viewport.
// Finalized blocks with a cached render are reused without re-rendering.
func (m *OutputModel) doRender() {
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

	activeToolIdx := -1
	if m.toolSpinner != "" {
		activeToolIdx = m.lastToolCallIndex()
	}

	// Track whether the star prefix needs to be placed on the next
	// content block. Set to true after each user message block.
	needsStar := false

	for i, b := range m.blocks {
		// Use cached render for finalized blocks, but skip cache for
		// the active tool call block (spinner changes each frame),
		// and skip cache for the first content block after a user
		// message while the star is still animating.
		skipCache := i == activeToolIdx || (needsStar && !m.starDone)
		if b.finalized && b.rendered != "" && !skipCache {
			if b.kind == blockUserMessage {
				needsStar = true
			} else if needsStar && b.kind != blockSystem && b.kind != blockError {
				needsStar = false
			}
			parts = append(parts, b.rendered)
			continue
		}

		var rendered string
		switch b.kind {
		case blockUserMessage:
			// Render user text with a highlighted background — no header.
			text := strings.TrimRight(b.content, "\n")
			lines := strings.Split(text, "\n")
			var styledLines []string
			for _, line := range lines {
				padded := line
				// Pad to full width so the background spans the line.
				if w := m.width; lipgloss.Width(padded) < w {
					padded += strings.Repeat(" ", w-lipgloss.Width(padded))
				}
				styledLines = append(styledLines, m.styles.UserMessage.Render(padded))
			}
			rendered = strings.Join(styledLines, "\n")
			needsStar = true

		case blockHeader:
			// Pre-styled in AppendHeader — render as-is.
			rendered = b.content

		case blockText:
			rendered = renderMarkdown(m.renderer, b.content)

		case blockToolCall:
			var icon string
			if m.toolSpinner != "" && i == activeToolIdx {
				icon = m.styles.ToolCallIcon.Render(m.toolSpinner)
			} else {
				icon = m.styles.ToolCallIcon.Render("\u25b6")
			}
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
			rendered = m.styles.ErrorText.Render("x error: " + b.content)
		}

		// Prepend the star prefix to the first content block after a user message.
		if needsStar && b.kind != blockUserMessage && b.kind != blockSystem && b.kind != blockError {
			rendered = m.renderStar() + " " + strings.TrimLeft(rendered, "\n")
			needsStar = false
		}

		// Cache the rendered output for finalized blocks. Block indices are
		// stable (append-only) so the cache key is valid for the session.
		// Don't cache the active tool call block (spinner changes each frame),
		// and don't cache blocks with an animating star prefix.
		if b.finalized && !skipCache {
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
