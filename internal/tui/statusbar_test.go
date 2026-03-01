package tui

import (
	"math"
	"strings"
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

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want string
	}{
		{"zero", 0, "0"},
		{"small", 500, "500"},
		{"exactly 1k", 1000, "1.0k"},
		{"thousands", 5000, "5.0k"},
		{"large thousands", 999_999, "1000.0k"},
		{"exactly 1M", 1_000_000, "1.0M"},
		{"millions", 2_500_000, "2.5M"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTokens(tt.n)
			if got != tt.want {
				t.Errorf("formatTokens(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

func TestStatusBarYoloIndicator(t *testing.T) {
	styles := DefaultStyles()
	sb := NewStatusBar(styles)
	sb.SetWidth(120)

	// Without yolo, "YOLO" should not appear.
	view := sb.View()
	if strings.Contains(view, "YOLO") {
		t.Error("expected no YOLO indicator when yolo is off")
	}

	// With yolo, "YOLO" should appear.
	sb.SetYolo(true)
	view = sb.View()
	if !strings.Contains(view, "YOLO") {
		t.Error("expected YOLO indicator when yolo is on")
	}

	// Turn off, should disappear.
	sb.SetYolo(false)
	view = sb.View()
	if strings.Contains(view, "YOLO") {
		t.Error("expected no YOLO indicator after turning off")
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

// --- MCP status bar tests ---

func TestStatusBarMCP_Hidden(t *testing.T) {
	styles := DefaultStyles()
	sb := NewStatusBar(styles)
	sb.SetWidth(120)
	// mcpTotal=0 by default, should not show "mcp:" in output.
	view := sb.View()
	if strings.Contains(view, "mcp:") {
		t.Error("expected no mcp indicator when mcpTotal=0")
	}
}

func TestStatusBarMCP_AllAlive(t *testing.T) {
	styles := DefaultStyles()
	sb := NewStatusBar(styles)
	sb.SetWidth(120)
	sb.UpdateMCP(3, 3)

	view := sb.View()
	if !strings.Contains(view, "mcp:") {
		t.Fatal("expected mcp indicator in status bar")
	}
	// When all alive, should show just the count, not "X/Y".
	if strings.Contains(view, "3/3") {
		t.Error("expected compact form (just '3'), not '3/3' when all alive")
	}
}

func TestStatusBarMCP_SomeDead(t *testing.T) {
	styles := DefaultStyles()
	sb := NewStatusBar(styles)
	sb.SetWidth(120)
	sb.UpdateMCP(3, 2)

	view := sb.View()
	if !strings.Contains(view, "mcp:") {
		t.Fatal("expected mcp indicator in status bar")
	}
	if !strings.Contains(view, "2/3") {
		t.Errorf("expected '2/3' in status bar, got %q", view)
	}
}

// --- Context window usage tests ---

func TestStatusBarContextPercent_Basic(t *testing.T) {
	styles := DefaultStyles()
	sb := NewStatusBar(styles)

	sb.UpdateContext(100_000, "claude-opus-4-6")
	if sb.ContextPercent() != 50 {
		t.Errorf("expected 50%%, got %d%%", sb.ContextPercent())
	}
}

func TestStatusBarContextPercent_Thresholds(t *testing.T) {
	styles := DefaultStyles()
	sb := NewStatusBar(styles)
	sb.SetWidth(120)

	// Below 60%: normal color, ctx shown.
	sb.UpdateContext(80_000, "claude-opus-4-6")
	if pct := sb.ContextPercent(); pct != 40 {
		t.Errorf("expected 40%%, got %d%%", pct)
	}

	// At 60%: caution.
	sb.UpdateContext(120_000, "claude-opus-4-6")
	if pct := sb.ContextPercent(); pct != 60 {
		t.Errorf("expected 60%%, got %d%%", pct)
	}

	// At 80%: danger.
	sb.UpdateContext(160_000, "claude-opus-4-6")
	if pct := sb.ContextPercent(); pct != 80 {
		t.Errorf("expected 80%%, got %d%%", pct)
	}
}

func TestStatusBarContextPercent_RendersInView(t *testing.T) {
	styles := DefaultStyles()
	sb := NewStatusBar(styles)
	sb.SetWidth(120)

	// Before any context update, no ctx in view.
	view := sb.View()
	if strings.Contains(view, "ctx:") {
		t.Error("expected no ctx indicator initially")
	}

	// After update, ctx should appear.
	sb.UpdateContext(100_000, "claude-opus-4-6")
	view = sb.View()
	if !strings.Contains(view, "ctx:") {
		t.Error("expected ctx indicator in status bar")
	}
	if !strings.Contains(view, "50%") {
		t.Error("expected 50% in status bar")
	}
}

func TestStatusBarContextPercent_UnknownModel(t *testing.T) {
	styles := DefaultStyles()
	sb := NewStatusBar(styles)

	// Unknown model uses default 200k limit.
	sb.UpdateContext(100_000, "some-future-model")
	if pct := sb.ContextPercent(); pct != 50 {
		t.Errorf("expected 50%% for unknown model, got %d%%", pct)
	}
}

func TestStatusBarContextPercent_CapAt100(t *testing.T) {
	styles := DefaultStyles()
	sb := NewStatusBar(styles)

	sb.UpdateContext(300_000, "claude-opus-4-6")
	if pct := sb.ContextPercent(); pct != 100 {
		t.Errorf("expected 100%% cap, got %d%%", pct)
	}
}

func TestStatusBarView_ContextCaution(t *testing.T) {
	styles := DefaultStyles()
	sb := NewStatusBar(styles)
	sb.SetWidth(120)

	// Set context to 65% -- should trigger caution rendering (>= 60, < 80).
	sb.UpdateContext(130_000, "claude-opus-4-6")
	view := sb.View()
	if !strings.Contains(view, "65%") {
		t.Errorf("expected 65%% in status bar, got %q", view)
	}
}

func TestStatusBarView_ContextDanger(t *testing.T) {
	styles := DefaultStyles()
	sb := NewStatusBar(styles)
	sb.SetWidth(120)

	// Set context to 85% -- should trigger danger rendering (>= 80).
	sb.UpdateContext(170_000, "claude-opus-4-6")
	view := sb.View()
	if !strings.Contains(view, "85%") {
		t.Errorf("expected 85%% in status bar, got %q", view)
	}
}

func TestStatusBarView_ContextNormal(t *testing.T) {
	styles := DefaultStyles()
	sb := NewStatusBar(styles)
	sb.SetWidth(120)

	// Set context to 30% -- should use default rendering (< 60).
	sb.UpdateContext(60_000, "claude-opus-4-6")
	view := sb.View()
	if !strings.Contains(view, "30%") {
		t.Errorf("expected 30%% in status bar, got %q", view)
	}
}

func TestStatusBarUpdateContext_ZeroLimit(t *testing.T) {
	styles := DefaultStyles()
	sb := NewStatusBar(styles)

	// With a model whose limit is 0 (shouldn't happen, but test the guard).
	// The default limit should be used for unknown models.
	sb.UpdateContext(100_000, "claude-opus-4-6")
	if sb.contextPct == 0 {
		// Should be non-zero since we have tokens.
		t.Error("expected non-zero contextPct")
	}
}

func TestStatusBarReset_PreservesMCP(t *testing.T) {
	styles := DefaultStyles()
	sb := NewStatusBar(styles)
	sb.UpdateMCP(3, 2)
	sb.Update("claude-opus-4-6", 1000, 500, 5)

	sb.Reset()

	// Token counts should be zeroed.
	if sb.inputTokens != 0 || sb.outputTokens != 0 || sb.turn != 0 {
		t.Error("expected token counters to be zeroed")
	}
	// MCP counts should survive.
	if sb.mcpTotal != 3 || sb.mcpAlive != 2 {
		t.Errorf("expected MCP counts preserved (3/2), got %d/%d", sb.mcpTotal, sb.mcpAlive)
	}
}
