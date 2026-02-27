package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Cost per million tokens for Claude Opus (input / output).
const (
	opusInputCostPerMillion  = 5.0
	opusOutputCostPerMillion = 25.0
)

// StatusBar displays model info, token usage, cost, and turn number.
type StatusBar struct {
	styles       Styles
	model        string
	inputTokens  int
	outputTokens int
	turn         int
	cost         float64
	width        int
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
	s.cost = float64(inputTokens)/1_000_000*opusInputCostPerMillion +
		float64(outputTokens)/1_000_000*opusOutputCostPerMillion
}

// SetWidth updates the status bar width.
func (s *StatusBar) SetWidth(w int) {
	s.width = w
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

// Height returns the rendered height of the status bar (always 1).
func (s StatusBar) Height() int {
	return lipgloss.Height(s.View())
}
