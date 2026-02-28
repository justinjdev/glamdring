package tui

import (
	"strings"
	"testing"
)

func newTestOutput(width, height int) OutputModel {
	return NewOutputModel(DefaultStyles(), width, height)
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
