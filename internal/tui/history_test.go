package tui

import "testing"

func TestHistory_AddAndUp(t *testing.T) {
	h := &History{}
	h.Add("first")
	h.Add("second")
	h.Add("third")

	text, ok := h.Up("")
	if !ok || text != "third" {
		t.Errorf("expected 'third', got %q (ok=%v)", text, ok)
	}

	text, ok = h.Up("")
	if !ok || text != "second" {
		t.Errorf("expected 'second', got %q (ok=%v)", text, ok)
	}

	text, ok = h.Up("")
	if !ok || text != "first" {
		t.Errorf("expected 'first', got %q (ok=%v)", text, ok)
	}

	// Already at oldest.
	text, ok = h.Up("")
	if ok {
		t.Error("expected ok=false at oldest entry")
	}
	if text != "first" {
		t.Errorf("expected 'first' when at oldest, got %q", text)
	}
}

func TestHistory_UpDownDraft(t *testing.T) {
	h := &History{}
	h.Add("one")
	h.Add("two")

	// Start navigating with a draft.
	text, ok := h.Up("my draft")
	if !ok || text != "two" {
		t.Errorf("expected 'two', got %q", text)
	}

	text, ok = h.Up("")
	if !ok || text != "one" {
		t.Errorf("expected 'one', got %q", text)
	}

	text, ok = h.Down()
	if !ok || text != "two" {
		t.Errorf("expected 'two', got %q", text)
	}

	// Down past the end restores draft.
	text, ok = h.Down()
	if !ok || text != "my draft" {
		t.Errorf("expected 'my draft', got %q", text)
	}

	// No longer navigating.
	_, ok = h.Down()
	if ok {
		t.Error("expected ok=false when not navigating")
	}
}

func TestHistory_DeduplicateConsecutive(t *testing.T) {
	h := &History{}
	h.Add("same")
	h.Add("same")
	h.Add("same")

	if h.Len() != 1 {
		t.Errorf("expected 1 entry after dedup, got %d", h.Len())
	}
}

func TestHistory_SkipEmpty(t *testing.T) {
	h := &History{}
	h.Add("")
	h.Add("  ")
	h.Add("\t\n")

	if h.Len() != 0 {
		t.Errorf("expected 0 entries for empty input, got %d", h.Len())
	}
}

func TestHistory_RingBuffer(t *testing.T) {
	h := &History{}
	for i := 0; i < 150; i++ {
		h.Add(string(rune('a' + i%26)))
	}

	if h.Len() > historyMaxEntries {
		t.Errorf("expected max %d entries, got %d", historyMaxEntries, h.Len())
	}
}

func TestHistory_Search(t *testing.T) {
	h := &History{}
	h.Add("go build ./...")
	h.Add("go test ./pkg/...")
	h.Add("git status")
	h.Add("go test -v ./internal/...")

	results := h.Search("go test")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Newest first.
	if results[0] != "go test -v ./internal/..." {
		t.Errorf("expected newest match first, got %q", results[0])
	}
}

func TestHistory_SearchCaseInsensitive(t *testing.T) {
	h := &History{}
	h.Add("Hello World")
	h.Add("hello world")

	results := h.Search("HELLO")
	if len(results) != 2 {
		t.Errorf("expected 2 results for case-insensitive search, got %d", len(results))
	}
}

func TestHistory_SearchEmpty(t *testing.T) {
	h := &History{}
	h.Add("test")

	results := h.Search("")
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty query, got %d", len(results))
	}
}

func TestHistory_EmptyUp(t *testing.T) {
	h := &History{}
	_, ok := h.Up("draft")
	if ok {
		t.Error("expected ok=false on empty history")
	}
}

func TestHistory_ResetCursor(t *testing.T) {
	h := &History{}
	h.Add("test")

	h.Up("draft")
	h.ResetCursor()

	// After reset, Down should fail.
	_, ok := h.Down()
	if ok {
		t.Error("expected ok=false after ResetCursor")
	}
}
