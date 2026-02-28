package tui

import (
	"fmt"
)

// modelPricing maps model names to [input, output] cost per million tokens.
var modelPricing = map[string][2]float64{
	"claude-opus-4-6":   {15.0, 75.0},
	"claude-sonnet-4-6": {3.0, 15.0},
	"claude-haiku-4-5":  {0.80, 4.0},
}

// defaultPricing is used for unknown models (falls back to Opus pricing).
var defaultPricing = [2]float64{15.0, 75.0}

// costForModel calculates the cost for the given model and token counts.
func costForModel(model string, inputTokens, outputTokens int) float64 {
	pricing, ok := modelPricing[model]
	if !ok {
		pricing = defaultPricing
	}
	return float64(inputTokens)/1_000_000*pricing[0] +
		float64(outputTokens)/1_000_000*pricing[1]
}

// modelContextLimits maps model names to their context window size in tokens.
var modelContextLimits = map[string]int{
	"claude-opus-4-6":   200_000,
	"claude-sonnet-4-6": 200_000,
	"claude-haiku-4-5":  200_000,
}

// defaultContextLimit is used for unknown models.
const defaultContextLimit = 200_000

// StatusBar displays model info, token usage, cost, and turn number.
type StatusBar struct {
	styles       Styles
	model        string
	inputTokens  int
	outputTokens int
	turn         int
	cost         float64
	width        int
	yolo         bool
	mcpTotal     int // total configured MCP servers
	mcpAlive     int // currently running MCP servers
	contextPct   int // context window usage percentage (0-100)
}

// NewStatusBar creates a status bar with default values.
func NewStatusBar(styles Styles) StatusBar {
	return StatusBar{
		styles: styles,
		model:  "claude-opus-4-6",
	}
}

// Update recalculates the status bar with new values.
func (s *StatusBar) Update(model string, inputTokens, outputTokens, turn int) {
	s.model = model
	s.inputTokens = inputTokens
	s.outputTokens = outputTokens
	s.turn = turn
	s.cost = costForModel(model, inputTokens, outputTokens)
}

// SetWidth updates the status bar width.
func (s *StatusBar) SetWidth(w int) {
	s.width = w
}

// SetYolo updates the YOLO indicator state.
func (s *StatusBar) SetYolo(on bool) {
	s.yolo = on
}

// View renders the status bar as a single styled line.
func (s StatusBar) View() string {
	sep := s.styles.StatusBarSep.String()

	modelStr := s.styles.StatusBarValue.Render(s.model)

	tokIn := s.styles.StatusBarKey.Render("in:") + " " +
		s.styles.StatusBarValue.Render(formatTokens(s.inputTokens))

	tokOut := s.styles.StatusBarKey.Render("out:") + " " +
		s.styles.StatusBarValue.Render(formatTokens(s.outputTokens))

	costStr := s.styles.StatusBarKey.Render("cost:") + " " +
		s.styles.StatusBarValue.Render(fmt.Sprintf("$%.4f", s.cost))

	turnStr := s.styles.StatusBarKey.Render("turn:") + " " +
		s.styles.StatusBarValue.Render(fmt.Sprintf("%d", s.turn))

	left := modelStr + sep + tokIn + sep + tokOut + sep + costStr + sep + turnStr

	// Context window usage with color thresholds.
	if s.contextPct > 0 {
		ctxLabel := s.styles.StatusBarKey.Render("ctx:")
		ctxVal := fmt.Sprintf("%d%%", s.contextPct)
		var ctxStyled string
		switch {
		case s.contextPct >= 80:
			ctxStyled = s.styles.StatusBarDanger.Render(ctxVal)
		case s.contextPct >= 60:
			ctxStyled = s.styles.StatusBarCaution.Render(ctxVal)
		default:
			ctxStyled = s.styles.StatusBarValue.Render(ctxVal)
		}
		left += sep + ctxLabel + " " + ctxStyled
	}

	if s.yolo {
		left += sep + s.styles.StatusBarWarning.Render("YOLO")
	}

	if s.mcpTotal > 0 {
		var mcpVal string
		if s.mcpAlive == s.mcpTotal {
			mcpVal = fmt.Sprintf("%d", s.mcpAlive)
		} else {
			mcpVal = fmt.Sprintf("%d/%d", s.mcpAlive, s.mcpTotal)
		}
		mcpStr := s.styles.StatusBarKey.Render("mcp:") + " " +
			s.styles.StatusBarValue.Render(mcpVal)
		left += sep + mcpStr
	}

	return s.styles.StatusBar.
		Width(s.width).
		Render(left)
}

// formatTokens renders a token count in a compact human-friendly form.
func formatTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// UpdateContext calculates and sets the context window usage percentage.
func (s *StatusBar) UpdateContext(lastInputTokens int, model string) {
	limit, ok := modelContextLimits[model]
	if !ok {
		limit = defaultContextLimit
	}
	if limit <= 0 {
		s.contextPct = 0
		return
	}
	s.contextPct = lastInputTokens * 100 / limit
	if s.contextPct > 100 {
		s.contextPct = 100
	}
}

// ContextPercent returns the current context window usage percentage.
func (s *StatusBar) ContextPercent() int {
	return s.contextPct
}

// UpdateMCP sets the MCP server counts for the status bar.
func (s *StatusBar) UpdateMCP(total, alive int) {
	s.mcpTotal = total
	s.mcpAlive = alive
}

// Reset zeroes all counters (used by /clear).
// MCP counts intentionally preserved — servers survive /clear.
func (s *StatusBar) Reset() {
	s.inputTokens = 0
	s.outputTokens = 0
	s.turn = 0
	s.cost = 0
}
