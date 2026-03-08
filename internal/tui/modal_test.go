package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func testPalette() ThemePalette {
	return ThemePalette{
		Name:     "test",
		Bg:       lipgloss.Color("#000000"),
		Fg:       lipgloss.Color("#ffffff"),
		FgDim:    lipgloss.Color("#888888"),
		FgBright: lipgloss.Color("#ffffff"),
		Primary:  lipgloss.Color("#5588cc"),
		Success:  lipgloss.Color("#55cc55"),
		Subtle:   lipgloss.Color("#666666"),
		Surface1: lipgloss.Color("#222222"),
	}
}

func testItems() []MenuItem {
	return []MenuItem{
		{Kind: MenuSection, Label: "Appearance"},
		{Kind: MenuChoice, ID: "theme", Label: "Theme", Value: "glamdring", Choices: []string{"glamdring", "mithril", "shire"}},
		{Kind: MenuSection, Label: "Behavior"},
		{Kind: MenuToggle, ID: "thinking", Label: "Thinking", Value: "off", Active: false},
		{Kind: MenuToggle, ID: "yolo", Label: "Yolo", Value: "off", Active: false},
	}
}

func TestNewModal_CursorSkipsSection(t *testing.T) {
	m := NewModal("Settings", testItems(), testPalette())

	// Cursor should start on the first selectable item, not the section header.
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1 (Theme), got %d", m.cursor)
	}
}

func TestModal_NavigateDown(t *testing.T) {
	m := NewModal("Settings", testItems(), testPalette())

	// Start at Theme (index 1).
	m.HandleKey("down")
	// Should skip section header at index 2, land on Thinking (index 3).
	if m.cursor != 3 {
		t.Errorf("expected cursor at 3 (Thinking), got %d", m.cursor)
	}

	m.HandleKey("down")
	// Should be at Yolo (index 4).
	if m.cursor != 4 {
		t.Errorf("expected cursor at 4 (Yolo), got %d", m.cursor)
	}
}

func TestModal_NavigateUp(t *testing.T) {
	m := NewModal("Settings", testItems(), testPalette())

	// Move to last item.
	m.HandleKey("down")
	m.HandleKey("down")
	if m.cursor != 4 {
		t.Fatalf("expected cursor at 4, got %d", m.cursor)
	}

	m.HandleKey("up")
	// Should be at Thinking (index 3).
	if m.cursor != 3 {
		t.Errorf("expected cursor at 3 (Thinking), got %d", m.cursor)
	}

	m.HandleKey("up")
	// Should skip section at index 2, land on Theme (index 1).
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1 (Theme), got %d", m.cursor)
	}
}

func TestModal_NavigateUpAtTop(t *testing.T) {
	m := NewModal("Settings", testItems(), testPalette())

	// Already at first selectable item. Going up should stay.
	m.HandleKey("up")
	if m.cursor != 1 {
		t.Errorf("expected cursor to stay at 1, got %d", m.cursor)
	}
}

func TestModal_NavigateDownAtBottom(t *testing.T) {
	m := NewModal("Settings", testItems(), testPalette())

	// Move to last item.
	m.HandleKey("down")
	m.HandleKey("down")

	// Going down should stay at last item.
	m.HandleKey("down")
	if m.cursor != 4 {
		t.Errorf("expected cursor to stay at 4, got %d", m.cursor)
	}
}

func TestModal_ToggleOnOff(t *testing.T) {
	m := NewModal("Settings", testItems(), testPalette())

	// Move to Thinking toggle.
	m.HandleKey("down")
	if m.cursor != 3 {
		t.Fatalf("expected cursor at 3, got %d", m.cursor)
	}

	// Toggle on.
	close, change := m.HandleKey("enter")
	if close {
		t.Error("toggle should not close modal")
	}
	if change == nil {
		t.Fatal("expected change from toggle")
	}
	if change.ID != "thinking" {
		t.Errorf("expected change ID 'thinking', got %q", change.ID)
	}
	if change.Value != "on" {
		t.Errorf("expected change value 'on', got %q", change.Value)
	}
	if !m.items[3].Active {
		t.Error("expected item Active to be true")
	}

	// Toggle off.
	_, change = m.HandleKey("enter")
	if change == nil {
		t.Fatal("expected change from second toggle")
	}
	if change.Value != "off" {
		t.Errorf("expected change value 'off', got %q", change.Value)
	}
	if m.items[3].Active {
		t.Error("expected item Active to be false")
	}
}

func TestModal_ChoiceExpandAndSelect(t *testing.T) {
	m := NewModal("Settings", testItems(), testPalette())

	// Cursor is on Theme (index 1). Enter to expand.
	close, change := m.HandleKey("enter")
	if close {
		t.Error("expanding should not close modal")
	}
	if change != nil {
		t.Error("expanding should not produce a change")
	}
	if m.expanded != 1 {
		t.Errorf("expected expanded=1, got %d", m.expanded)
	}
	// subCursor should be on current value "glamdring" (index 0).
	if m.subCursor != 0 {
		t.Errorf("expected subCursor=0, got %d", m.subCursor)
	}

	// Navigate down in sub-list to "mithril".
	m.HandleKey("down")
	if m.subCursor != 1 {
		t.Errorf("expected subCursor=1, got %d", m.subCursor)
	}

	// Select "mithril".
	close, change = m.HandleKey("enter")
	if close {
		t.Error("selecting should not close modal")
	}
	if change == nil {
		t.Fatal("expected change from selection")
	}
	if change.ID != "theme" {
		t.Errorf("expected change ID 'theme', got %q", change.ID)
	}
	if change.Value != "mithril" {
		t.Errorf("expected change value 'mithril', got %q", change.Value)
	}
	if m.expanded != -1 {
		t.Errorf("expected expanded=-1 after selection, got %d", m.expanded)
	}
	if m.items[1].Value != "mithril" {
		t.Errorf("expected item value updated to 'mithril', got %q", m.items[1].Value)
	}
}

func TestModal_ChoiceSubCursorBounds(t *testing.T) {
	m := NewModal("Settings", testItems(), testPalette())

	// Expand theme choices.
	m.HandleKey("enter")

	// Navigate down past last choice.
	m.HandleKey("down") // mithril
	m.HandleKey("down") // shire
	m.HandleKey("down") // should stay at shire
	if m.subCursor != 2 {
		t.Errorf("expected subCursor=2 (clamped), got %d", m.subCursor)
	}

	// Navigate up past first choice.
	m.HandleKey("up") // mithril
	m.HandleKey("up") // glamdring
	m.HandleKey("up") // should stay at glamdring
	if m.subCursor != 0 {
		t.Errorf("expected subCursor=0 (clamped), got %d", m.subCursor)
	}
}

func TestModal_ChoicePreSelectsCurrent(t *testing.T) {
	items := testItems()
	items[1].Value = "shire" // Set current to shire (index 2 in choices).

	m := NewModal("Settings", items, testPalette())
	m.HandleKey("enter") // expand

	if m.subCursor != 2 {
		t.Errorf("expected subCursor=2 (pre-selected 'shire'), got %d", m.subCursor)
	}
}

func TestModal_EscClosesExpandedFirst(t *testing.T) {
	m := NewModal("Settings", testItems(), testPalette())

	// Expand theme choices.
	m.HandleKey("enter")
	if m.expanded != 1 {
		t.Fatalf("expected expanded=1, got %d", m.expanded)
	}

	// Esc should collapse, not close modal.
	close, _ := m.HandleKey("esc")
	if close {
		t.Error("esc should collapse expanded list, not close modal")
	}
	if m.expanded != -1 {
		t.Errorf("expected expanded=-1 after esc, got %d", m.expanded)
	}

	// Second esc should close modal.
	close, _ = m.HandleKey("esc")
	if !close {
		t.Error("second esc should close modal")
	}
}

func TestModal_EscClosesWhenNotExpanded(t *testing.T) {
	m := NewModal("Settings", testItems(), testPalette())

	close, _ := m.HandleKey("esc")
	if !close {
		t.Error("esc should close modal when nothing expanded")
	}
}

func TestModal_VimKeys(t *testing.T) {
	m := NewModal("Settings", testItems(), testPalette())

	m.HandleKey("j") // down
	if m.cursor != 3 {
		t.Errorf("j should move down, cursor=%d", m.cursor)
	}

	m.HandleKey("k") // up
	if m.cursor != 1 {
		t.Errorf("k should move up, cursor=%d", m.cursor)
	}
}

func TestModal_FocusItem(t *testing.T) {
	m := NewModal("Settings", testItems(), testPalette())

	m.FocusItem("Thinking")
	if m.cursor != 3 {
		t.Errorf("expected cursor at 3 (Thinking), got %d", m.cursor)
	}

	m.FocusItem("Yolo")
	if m.cursor != 4 {
		t.Errorf("expected cursor at 4 (Yolo), got %d", m.cursor)
	}
}

func TestModal_FocusItem_IgnoresSection(t *testing.T) {
	m := NewModal("Settings", testItems(), testPalette())

	// Sections should not be focusable.
	m.FocusItem("Appearance")
	// Cursor should not have moved from initial position.
	if m.cursor != 1 {
		t.Errorf("FocusItem on section should be no-op, cursor=%d", m.cursor)
	}
}

func TestModal_View_NotEmpty(t *testing.T) {
	m := NewModal("Settings", testItems(), testPalette())

	view := m.View(60)
	if len(view) == 0 {
		t.Error("expected non-empty view")
	}
}

func TestModal_OverlayView_NotEmpty(t *testing.T) {
	m := NewModal("Settings", testItems(), testPalette())

	view := m.OverlayView(80, 24)
	if len(view) == 0 {
		t.Error("expected non-empty overlay view")
	}
}

func TestRenderOverlay(t *testing.T) {
	base := "line1\nline2\nline3\nline4\nline5"
	modal := "MODAL"

	result := RenderOverlay(base, modal, 10, 5)
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
	// Modal should be present in the result.
	if !containsStr(result, "MODAL") {
		t.Error("expected MODAL in overlay result")
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
