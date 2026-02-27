package tui

import (
	"math"
	"testing"
)

func TestCostForModel_Opus(t *testing.T) {
	cost := costForModel("claude-opus-4-6", 1_000_000, 1_000_000)
	expected := 15.0 + 75.0
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("expected %f, got %f", expected, cost)
	}
}

func TestCostForModel_Sonnet(t *testing.T) {
	cost := costForModel("claude-sonnet-4-6", 1_000_000, 1_000_000)
	expected := 3.0 + 15.0
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("expected %f, got %f", expected, cost)
	}
}

func TestCostForModel_Haiku(t *testing.T) {
	cost := costForModel("claude-haiku-4-5", 1_000_000, 1_000_000)
	expected := 0.80 + 4.0
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("expected %f, got %f", expected, cost)
	}
}

func TestCostForModel_Unknown(t *testing.T) {
	// Unknown models fall back to Opus pricing.
	cost := costForModel("some-unknown-model", 1_000_000, 1_000_000)
	expected := 15.0 + 75.0
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("expected %f (Opus fallback), got %f", expected, cost)
	}
}

func TestCostForModel_SmallTokens(t *testing.T) {
	// 5000 input, 1000 output at Opus pricing.
	cost := costForModel("claude-opus-4-6", 5000, 1000)
	expected := 5000.0/1_000_000*15.0 + 1000.0/1_000_000*75.0
	if math.Abs(cost-expected) > 0.0001 {
		t.Errorf("expected %f, got %f", expected, cost)
	}
}

func TestStatusBarUpdate_UsesCostForModel(t *testing.T) {
	styles := DefaultStyles()
	sb := NewStatusBar(styles)

	sb.Update("claude-sonnet-4-6", 1_000_000, 1_000_000, 5)
	expected := 3.0 + 15.0
	if math.Abs(sb.cost-expected) > 0.001 {
		t.Errorf("expected cost %f for sonnet, got %f", expected, sb.cost)
	}
}
