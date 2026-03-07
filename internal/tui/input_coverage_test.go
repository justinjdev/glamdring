package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInputModel_Value(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	if m.Value() != "" {
		t.Errorf("expected empty value, got %q", m.Value())
	}
}

func TestInputModel_IsSlashCmd(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	if m.IsSlashCmd() {
		t.Error("expected IsSlashCmd() false on empty input")
	}
}

func TestInputModel_CmdName(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	if m.CmdName() != "" {
		t.Errorf("expected empty CmdName, got %q", m.CmdName())
	}
}

func TestInputModel_Blur(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.Blur()
	// Should not panic. No visible assertion needed.
}

func TestInputModel_SearchActive(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	if m.SearchActive() {
		t.Error("expected SearchActive() false initially")
	}
}

func TestInputModel_Init(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	cmd := m.Init()
	// Init returns textarea.Blink, should be non-nil.
	if cmd == nil {
		t.Error("expected non-nil cmd from Init")
	}
}

func TestInputModel_View(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.SetWidth(80)
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestInputModel_View_WithPendingImages(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.SetWidth(80)
	m.pendingImages = []PendingImage{
		{Data: []byte{1}, Width: 100, Height: 200},
	}
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view with pending images")
	}
}

func TestInputModel_View_WithPendingImageNoDimensions(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.SetWidth(80)
	m.pendingImages = []PendingImage{
		{Data: []byte{1}, Width: 0, Height: 0},
	}
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view with pending images (no dimensions)")
	}
}

// --- Update key handling ---

func TestInputModel_Update_EnterEmpty(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = result
	// Empty input should not produce a SubmitMsg.
	if cmd != nil {
		t.Error("expected nil cmd for empty Enter")
	}
}

func TestInputModel_Update_EnterWithText(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.textarea.SetValue("hello")

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = result
	if cmd == nil {
		t.Fatal("expected non-nil cmd for Enter with text")
	}
	msg := cmd()
	if submit, ok := msg.(SubmitMsg); !ok {
		t.Errorf("expected SubmitMsg, got %T", msg)
	} else if submit.Text != "hello" {
		t.Errorf("expected 'hello', got %q", submit.Text)
	}
}

func TestInputModel_Update_EnterWithImages(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.pendingImages = []PendingImage{{Data: []byte{1}}}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for Enter with images")
	}
	msg := cmd()
	if submit, ok := msg.(SubmitMsg); !ok {
		t.Errorf("expected SubmitMsg, got %T", msg)
	} else if len(submit.Images) != 1 {
		t.Errorf("expected 1 image, got %d", len(submit.Images))
	}
	// Pending images should be cleared.
	if len(result.pendingImages) != 0 {
		t.Error("expected pending images to be cleared after submit")
	}
}

func TestInputModel_Update_UpKey_EmptyHistory(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	_ = result
	// With empty history, up should not produce a cmd.
	if cmd != nil {
		t.Error("expected nil cmd for Up with empty history")
	}
}

func TestInputModel_Update_UpKey_WithHistory(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.history.Add("previous command")

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	_ = cmd
	if result.textarea.Value() != "previous command" {
		t.Errorf("expected 'previous command', got %q", result.textarea.Value())
	}
}

func TestInputModel_Update_DownKey_NotNavigating(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	_ = result
	if cmd != nil {
		t.Error("expected nil cmd for Down when not navigating")
	}
}

func TestInputModel_Update_DownKey_RestoresDraft(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.history.Add("entry1")
	m.history.Add("entry2")

	// Navigate up first.
	m.textarea.SetValue("my draft")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})

	// Then navigate back down.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	if m.textarea.Value() != "my draft" {
		t.Errorf("expected draft restored, got %q", m.textarea.Value())
	}
}

func TestInputModel_Update_TabComplete(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.SetAvailableCommands([]string{"help", "quit"})
	m.textarea.SetValue("/he")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if result.textarea.Value() != "/help " {
		t.Errorf("expected '/help ', got %q", result.textarea.Value())
	}
}

func TestInputModel_Update_TabComplete_NoMatch(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.SetAvailableCommands([]string{"help"})
	m.textarea.SetValue("/xyz")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if result.textarea.Value() != "/xyz" {
		t.Errorf("expected '/xyz' unchanged, got %q", result.textarea.Value())
	}
}

func TestInputModel_Update_TabComplete_NotSlashCmd(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.SetAvailableCommands([]string{"help"})
	m.textarea.SetValue("regular text")

	// Tab on non-slash text should pass through to textarea.
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	_ = result
}

// --- Ctrl+R search ---

func TestInputModel_Update_CtrlR_EntersSearch(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})

	if !result.searching {
		t.Error("expected searching to be true after Ctrl+R")
	}
}

func TestInputModel_SearchKey_TypeCharacter(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.history.Add("go test ./...")
	m.history.Add("git status")
	m.searching = true

	result, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if result.searchQuery != "g" {
		t.Errorf("expected search query 'g', got %q", result.searchQuery)
	}
	if len(result.searchResults) != 2 {
		t.Errorf("expected 2 search results for 'g', got %d", len(result.searchResults))
	}
}

func TestInputModel_SearchKey_Backspace(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.history.Add("go test ./...")
	m.searching = true
	m.searchQuery = "go"

	result, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if result.searchQuery != "g" {
		t.Errorf("expected search query 'g' after backspace, got %q", result.searchQuery)
	}
}

func TestInputModel_SearchKey_BackspaceEmpty(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.searching = true
	m.searchQuery = ""

	result, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if result.searchQuery != "" {
		t.Errorf("expected empty query after backspace on empty, got %q", result.searchQuery)
	}
}

func TestInputModel_SearchKey_EnterAccepts(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.history.Add("go test ./...")
	m.searching = true
	m.searchQuery = "go"
	m.searchResults = []string{"go test ./..."}
	m.searchIdx = 0

	result, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEnter})
	if result.searching {
		t.Error("expected searching to be false after Enter")
	}
	if result.textarea.Value() != "go test ./..." {
		t.Errorf("expected textarea to have search result, got %q", result.textarea.Value())
	}
}

func TestInputModel_SearchKey_EnterNoResults(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.searching = true
	m.searchQuery = "xyz"
	m.searchResults = nil

	result, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEnter})
	if result.searching {
		t.Error("expected searching to be false after Enter")
	}
}

func TestInputModel_SearchKey_EscCancels(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.searching = true
	m.searchQuery = "test"

	result, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEsc})
	if result.searching {
		t.Error("expected searching to be false after Esc")
	}
	if result.searchQuery != "" {
		t.Error("expected search query cleared after Esc")
	}
}

func TestInputModel_SearchKey_CtrlCCancels(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.searching = true

	result, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if result.searching {
		t.Error("expected searching to be false after Ctrl+C")
	}
}

func TestInputModel_SearchKey_CtrlRCycles(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.history.Add("go test 1")
	m.history.Add("go test 2")
	m.searching = true
	m.searchQuery = "go"
	m.searchResults = []string{"go test 2", "go test 1"}
	m.searchIdx = 0

	result, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyCtrlR})
	if result.searchIdx != 1 {
		t.Errorf("expected searchIdx 1 after cycling, got %d", result.searchIdx)
	}

	// Cycle wraps around.
	result, _ = result.handleSearchKey(tea.KeyMsg{Type: tea.KeyCtrlR})
	if result.searchIdx != 0 {
		t.Errorf("expected searchIdx 0 after wrapping, got %d", result.searchIdx)
	}
}

func TestInputModel_SearchKey_UnhandledKey(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.searching = true

	result, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyF1})
	// Should not crash and searching should still be active.
	if !result.searching {
		t.Error("expected searching to remain true for unhandled key")
	}
}

func TestInputModel_Update_SearchModeRedirects(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.searching = true

	// Any key during search should go to handleSearchKey.
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if result.searchQuery != "a" {
		t.Errorf("expected search query 'a', got %q", result.searchQuery)
	}
}

// --- renderSearch ---

func TestInputModel_RenderSearch_NoMatch(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.SetWidth(80)
	m.searching = true
	m.searchQuery = "xyz"
	m.searchResults = nil

	view := m.renderSearch()
	if view == "" {
		t.Error("expected non-empty search render")
	}
}

func TestInputModel_RenderSearch_WithMatch(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.SetWidth(80)
	m.searching = true
	m.searchQuery = "test"
	m.searchResults = []string{"go test ./..."}
	m.searchIdx = 0

	view := m.renderSearch()
	if view == "" {
		t.Error("expected non-empty search render")
	}
}

func TestInputModel_RenderSearch_LongMatch(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.SetWidth(80)
	m.searching = true
	m.searchQuery = "x"
	longEntry := ""
	for i := 0; i < 100; i++ {
		longEntry += "x"
	}
	m.searchResults = []string{longEntry}
	m.searchIdx = 0

	view := m.renderSearch()
	if view == "" {
		t.Error("expected non-empty search render")
	}
}

func TestInputModel_RenderSearch_EmptyQuery(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.SetWidth(80)
	m.searching = true
	m.searchQuery = ""

	view := m.renderSearch()
	if view == "" {
		t.Error("expected non-empty search render")
	}
}

func TestInputModel_View_SearchActive(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	m.SetWidth(80)
	m.searching = true

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view when searching")
	}
}

// --- pngDimensions edge case ---

func TestPngDimensions_BadIHDR(t *testing.T) {
	data := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, // chunk length
		0x00, 0x00, 0x00, 0x00, // NOT "IHDR"
		0x00, 0x00, 0x00, 0x01, // width
		0x00, 0x00, 0x00, 0x01, // height
		0x08, 0x02,
		0x00, 0x00, 0x00,
	}
	w, h := pngDimensions(data)
	if w != 0 || h != 0 {
		t.Errorf("expected (0, 0) for bad IHDR, got (%d, %d)", w, h)
	}
}

// --- Ctrl+V paste ---

func TestInputModel_Update_CtrlV_NoClipboard(t *testing.T) {
	m := NewInputModel(DefaultStyles(builtinThemes["glamdring"]), builtinThemes["glamdring"])
	// Ctrl+V without clipboard init should not panic.
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlV})
	_ = result
}
