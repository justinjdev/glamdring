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

	if s.yolo {
		left += sep + s.styles.StatusBarWarning.Render("YOLO")
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

// Reset zeroes all counters (used by /clear).
func (s *StatusBar) Reset() {
	s.inputTokens = 0
	s.outputTokens = 0
	s.turn = 0
	s.cost = 0
}
