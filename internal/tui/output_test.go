package tui

import (
	"strings"
	"testing"
)

func newTestOutput(width, height int) OutputModel {
	return NewOutputModel(DefaultStyles(builtinThemes["glamdring"]), width, height)
}

func TestFinalizedBlocksCacheRenderedOutput(t *testing.T) {
	m := newTestOutput(80, 24)

	// Append text (streaming, not finalized).
	m.AppendText("Hello world")

	// Append a tool call, which finalizes the previous text block.
	m.AppendToolCall("Read", "file.go")

	if len(m.blocks) < 2 {
		t.Fatalf("expected at least 2 blocks, got %d", len(m.blocks))
	}
	if !m.blocks[0].finalized {
		t.Error("expected first block to be finalized after tool call")
	}
	if m.blocks[0].rendered == "" {
		t.Error("expected first block to have cached rendered output")
	}

	// Save the cached value.
	cached := m.blocks[0].rendered

	// Append more text (triggers rerender), verify cached value is stable.
	m.AppendText("More text")
	if m.blocks[0].rendered != cached {
		t.Error("expected cached rendered output to be stable across rerenders")
	}
}

func TestStreamingBlocksNotCached(t *testing.T) {
	m := newTestOutput(80, 24)

	// Append text -- this creates a streaming (non-finalized) block.
	m.AppendText("streaming content")

	if len(m.blocks) == 0 {
		t.Fatal("expected at least 1 block")
	}
	if m.blocks[0].finalized {
		t.Error("expected streaming block to not be finalized")
	}
	if m.blocks[0].rendered != "" {
		t.Error("expected streaming block to not have cached rendered output")
	}
}

func TestSetSizeWidthInvalidatesCache(t *testing.T) {
	m := newTestOutput(80, 24)

	// Add a user message (finalized, width-dependent due to divider).
	m.AppendUserMessage("hello")

	if len(m.blocks) == 0 {
		t.Fatal("expected at least 1 block")
	}
	if !m.blocks[0].finalized {
		t.Error("expected user message block to be finalized")
	}
	originalRendered := m.blocks[0].rendered
	if originalRendered == "" {
		t.Error("expected user message block to have cached rendered output")
	}

	// Change width -- should invalidate and re-render with new width.
	m.SetSize(120, 24)

	if m.blocks[0].rendered == "" {
		t.Error("expected block to be re-rendered after width change")
	}
	if m.blocks[0].rendered == originalRendered {
		t.Error("expected re-rendered output to differ from original after width change")
	}
	// After SetSize calls rerender(), rendererDirty should be consumed.
	if m.rendererDirty {
		t.Error("expected rendererDirty to be consumed after rerender")
	}
}

func TestSetSizeHeightOnlyPreservesCache(t *testing.T) {
	m := newTestOutput(80, 24)

	// Add a user message (finalized on creation).
	m.AppendUserMessage("hello")

	cached := m.blocks[0].rendered
	if cached == "" {
		t.Fatal("expected user message block to have cached rendered output")
	}

	// Change only height -- should preserve cache.
	m.SetSize(80, 30)

	if m.blocks[0].rendered != cached {
		t.Error("expected cache to be preserved on height-only change")
	}
	if m.rendererDirty {
		t.Error("expected rendererDirty to remain false on height-only change")
	}
}

func TestToggleCollapseInvalidatesCache(t *testing.T) {
	m := newTestOutput(80, 24)

	// Create a long tool result that gets auto-collapsed.
	longOutput := strings.Repeat("line\n", collapseThreshold+10)
	m.AppendToolResult(longOutput, false)

	if len(m.blocks) == 0 {
		t.Fatal("expected at least 1 block")
	}
	if !m.collapsed[0] {
		t.Error("expected long tool result to be auto-collapsed")
	}
	if !m.blocks[0].finalized {
		t.Error("expected tool result block to be finalized")
	}
	collapsedRendered := m.blocks[0].rendered
	if collapsedRendered == "" {
		t.Error("expected finalized tool result to have cached rendered output")
	}

	// Toggle collapse (expand) -- should re-render with different content.
	m.ToggleCollapse(0)

	if m.blocks[0].rendered == "" {
		t.Error("expected block to be re-rendered after toggle")
	}
	if m.blocks[0].rendered == collapsedRendered {
		t.Error("expected expanded output to differ from collapsed output")
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		name           string
		val, lo, hi    int
		want           int
	}{
		{"below min", -5, 0, 10, 0},
		{"above max", 15, 0, 10, 10},
		{"in range", 5, 0, 10, 5},
		{"at min", 0, 0, 10, 0},
		{"at max", 10, 0, 10, 10},
		{"equal bounds", 5, 5, 5, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clamp(tt.val, tt.lo, tt.hi)
			if got != tt.want {
				t.Errorf("clamp(%d, %d, %d) = %d, want %d", tt.val, tt.lo, tt.hi, got, tt.want)
			}
		})
	}
}

func TestRenderThinkingBlock_WithContent(t *testing.T) {
	m := newTestOutput(80, 24)
	m.AppendThinking("deep thoughts")
	m.finalizePreviousBlock()
	m.rerender()

	if len(m.blocks) == 0 {
		t.Fatal("expected at least 1 block")
	}
	b := m.blocks[0]
	if b.kind != blockThinking {
		t.Fatalf("expected blockThinking, got %d", b.kind)
	}
	if b.content != "deep thoughts" {
		t.Errorf("expected content 'deep thoughts', got %q", b.content)
	}
	// After finalization and rerender, rendered should be non-empty.
	if b.rendered == "" {
		t.Error("expected non-empty rendered output for thinking block")
	}
}

func TestRenderThinkingBlock_Empty(t *testing.T) {
	m := newTestOutput(80, 24)
	m.AppendThinking("")
	m.finalizePreviousBlock()
	m.rerender()

	if len(m.blocks) == 0 {
		t.Fatal("expected at least 1 block")
	}
	b := m.blocks[0]
	if b.kind != blockThinking {
		t.Fatalf("expected blockThinking, got %d", b.kind)
	}
}

func TestAppendToolOutputDelta_CreatesBlock(t *testing.T) {
	m := newTestOutput(80, 24)
	m.AppendToolOutputDelta("partial output")

	if len(m.blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(m.blocks))
	}
	b := m.blocks[0]
	if b.kind != blockToolResult {
		t.Errorf("expected blockToolResult, got %d", b.kind)
	}
	if b.finalized {
		t.Error("expected streaming block to not be finalized")
	}
	if b.content != "partial output" {
		t.Errorf("expected 'partial output', got %q", b.content)
	}
}

func TestAppendToolOutputDelta_AppendsToExisting(t *testing.T) {
	m := newTestOutput(80, 24)
	m.AppendToolOutputDelta("part1")
	m.AppendToolOutputDelta("part2")

	if len(m.blocks) != 1 {
		t.Fatalf("expected 1 block after appending, got %d", len(m.blocks))
	}
	if m.blocks[0].content != "part1part2" {
		t.Errorf("expected 'part1part2', got %q", m.blocks[0].content)
	}
}

func TestAppendToolResult_FinalizesStreamingBlock(t *testing.T) {
	m := newTestOutput(80, 24)
	// Start a streaming tool result.
	m.AppendToolOutputDelta("streaming...")
	// Finalize with the full result.
	m.AppendToolResult("final output", false)

	if len(m.blocks) != 1 {
		t.Fatalf("expected 1 block (finalized in-place), got %d", len(m.blocks))
	}
	b := m.blocks[0]
	if !b.finalized {
		t.Error("expected block to be finalized")
	}
	if b.content != "final output" {
		t.Errorf("expected 'final output', got %q", b.content)
	}
}

func TestAppendToolResult_ErrorFlag(t *testing.T) {
	m := newTestOutput(80, 24)
	m.AppendToolResult("some error", true)

	if len(m.blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(m.blocks))
	}
	if !m.blocks[0].isError {
		t.Error("expected isError to be true")
	}
}

func TestToggleCollapse_InvalidIndex(t *testing.T) {
	m := newTestOutput(80, 24)
	if m.ToggleCollapse(-1) {
		t.Error("expected false for negative index")
	}
	if m.ToggleCollapse(0) {
		t.Error("expected false for out-of-bounds index")
	}
}

func TestToggleCollapse_NonToolResultBlock(t *testing.T) {
	m := newTestOutput(80, 24)
	m.AppendText("just text")
	if m.ToggleCollapse(0) {
		t.Error("expected false for non-tool-result block")
	}
}

func TestToggleLastToolResult_NoToolResults(t *testing.T) {
	m := newTestOutput(80, 24)
	m.AppendText("just text")
	if m.ToggleLastToolResult() {
		t.Error("expected false when no tool result blocks exist")
	}
}

func TestAppendError_Content(t *testing.T) {
	m := newTestOutput(80, 24)
	m.AppendError("something went wrong")

	if len(m.blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(m.blocks))
	}
	b := m.blocks[0]
	if b.kind != blockError {
		t.Errorf("expected blockError, got %d", b.kind)
	}
	if b.content != "something went wrong" {
		t.Errorf("expected error content, got %q", b.content)
	}
}

func TestClear_ResetsEverything(t *testing.T) {
	m := newTestOutput(80, 24)
	m.AppendText("text")
	m.AppendUserMessage("msg")
	longOutput := strings.Repeat("line\n", 30)
	m.AppendToolResult(longOutput, false)

	m.Clear()

	if len(m.blocks) != 0 {
		t.Errorf("expected 0 blocks after clear, got %d", len(m.blocks))
	}
	if len(m.collapsed) != 0 {
		t.Errorf("expected empty collapsed map, got %d entries", len(m.collapsed))
	}
}

func TestRendererDirtyRetriesOnFailure(t *testing.T) {
	m := newTestOutput(80, 24)

	// Set rendererDirty with width=1 (very small but valid).
	m.width = 1
	m.rendererDirty = true
	m.rerender()

	// glamour should succeed even with width 1, so dirty should be consumed.
	if m.rendererDirty {
		t.Error("expected rendererDirty to be consumed after successful rerender with width 1")
	}
}
