# Image Paste via Ctrl+V -- Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable pasting clipboard images (screenshots, copied images) into glamdring via Ctrl+V for Claude's vision API.

**Architecture:** Ctrl+V checks clipboard for image data via `golang.design/x/clipboard`. Images are staged in InputModel, carried through SubmitMsg, and sent as base64-encoded PNG content blocks in the Claude Messages API. Text paste falls through when no image is present.

**Tech Stack:** Go, Bubbletea, `golang.design/x/clipboard` (CGO, cross-platform)

---

### Task 1: Add ImageSource to API types

**Files:**
- Modify: `pkg/api/types.go:57-78`
- Modify: `pkg/api/types_test.go`

**Step 1: Write the failing test**

Add to `pkg/api/types_test.go`:

```go
func TestContentBlockImageSerialization(t *testing.T) {
	block := ContentBlock{
		Type: "image",
		Source: &ImageSource{
			Type:      "base64",
			MediaType: "image/png",
			Data:      "iVBORw0KGgo=",
		},
	}

	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded["type"] != "image" {
		t.Errorf("type = %v, want %q", decoded["type"], "image")
	}
	source, ok := decoded["source"].(map[string]any)
	if !ok {
		t.Fatalf("source is %T, want map", decoded["source"])
	}
	if source["type"] != "base64" {
		t.Errorf("source.type = %v, want %q", source["type"], "base64")
	}
	if source["media_type"] != "image/png" {
		t.Errorf("source.media_type = %v, want %q", source["media_type"], "image/png")
	}
	if source["data"] != "iVBORw0KGgo=" {
		t.Errorf("source.data = %v, want %q", source["data"], "iVBORw0KGgo=")
	}
}

func TestContentBlockImageOmitsSourceWhenNil(t *testing.T) {
	block := ContentBlock{Type: "text", Text: "hello"}

	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, exists := decoded["source"]; exists {
		t.Error("source should be omitted when nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/justin/git/glamdring && go test ./pkg/api/ -run TestContentBlockImage -v`
Expected: FAIL -- `ImageSource` type undefined

**Step 3: Write minimal implementation**

Add to `pkg/api/types.go` inside `ContentBlock` (after the tool_result fields):

```go
	// type: "image"
	Source *ImageSource `json:"source,omitempty"`
```

Add new type after `ContentBlock`:

```go
// ImageSource represents the source data for an image content block.
type ImageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // "image/png"
	Data      string `json:"data"`       // base64-encoded image data
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/justin/git/glamdring && go test ./pkg/api/ -run TestContentBlockImage -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/api/types.go pkg/api/types_test.go
git commit -m "feat: add ImageSource to ContentBlock for vision API support"
```

---

### Task 2: Add TurnWithBlocks to Session

**Files:**
- Modify: `pkg/agent/session.go:78-90`
- Modify: `pkg/agent/session_test.go`

**Step 1: Write the failing test**

Add to `pkg/agent/session_test.go`:

```go
func TestSessionTurnWithBlocksAppendsToHistory(t *testing.T) {
	srv := newMockServer(buildSSEResponse("I can see the image!", "end_turn"))
	defer srv.Close()

	s := newTestSession(srv.URL)
	blocks := []api.ContentBlock{
		{Type: "image", Source: &api.ImageSource{
			Type:      "base64",
			MediaType: "image/png",
			Data:      "iVBORw0KGgo=",
		}},
		{Type: "text", Text: "What is in this image?"},
	}
	msgs := drainMessages(s.TurnWithBlocks(context.Background(), blocks))

	var gotText, gotDone bool
	for _, m := range msgs {
		if m.Type == MessageTextDelta && m.Text == "I can see the image!" {
			gotText = true
		}
		if m.Type == MessageDone {
			gotDone = true
		}
	}
	if !gotText {
		t.Error("expected text delta")
	}
	if !gotDone {
		t.Error("expected done message")
	}

	// History should have user + assistant.
	if len(s.Messages()) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(s.Messages()))
	}
	if s.Messages()[0].Role != "user" {
		t.Errorf("first message role = %q, want 'user'", s.Messages()[0].Role)
	}
	// Content should be []ContentBlock, not a string.
	contentBlocks, ok := s.Messages()[0].Content.([]api.ContentBlock)
	if !ok {
		t.Fatalf("expected Content to be []ContentBlock, got %T", s.Messages()[0].Content)
	}
	if len(contentBlocks) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(contentBlocks))
	}
	if contentBlocks[0].Type != "image" {
		t.Errorf("first block type = %q, want 'image'", contentBlocks[0].Type)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/justin/git/glamdring && go test ./pkg/agent/ -run TestSessionTurnWithBlocks -v`
Expected: FAIL -- `TurnWithBlocks` method undefined

**Step 3: Write minimal implementation**

Add to `pkg/agent/session.go` after the `Turn` method:

```go
// TurnWithBlocks sends structured content blocks (text + images) as a user message.
// Use this instead of Turn when the message includes non-text content.
func (s *Session) TurnWithBlocks(ctx context.Context, blocks []api.ContentBlock) <-chan Message {
	s.messages = append(s.messages, api.RequestMessage{
		Role:    "user",
		Content: blocks,
	})

	out := make(chan Message, 64)
	go func() {
		defer close(out)
		s.runTurn(ctx, out)
	}()
	return out
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/justin/git/glamdring && go test ./pkg/agent/ -run TestSessionTurnWithBlocks -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/agent/session.go pkg/agent/session_test.go
git commit -m "feat: add TurnWithBlocks for structured content (images + text)"
```

---

### Task 3: Replace clipboard dependency

**Files:**
- Modify: `go.mod`
- Modify: `internal/tui/builtins.go:13` (import)
- Modify: `internal/tui/builtins.go:529-554` (cmdCopy)
- Create: `internal/tui/clipboard.go`

**Step 1: Add the new dependency**

```bash
cd /Users/justin/git/glamdring && go get golang.design/x/clipboard
```

**Step 2: Create clipboard abstraction**

Create `internal/tui/clipboard.go`:

```go
package tui

import (
	"golang.design/x/clipboard"
)

// InitClipboard initializes the clipboard subsystem. Must be called once
// before any clipboard reads or writes. Returns an error if the clipboard
// is not available on this platform.
func InitClipboard() error {
	return clipboard.Init()
}

// ReadClipboardImage reads PNG image data from the system clipboard.
// Returns the raw PNG bytes and true if image data was found,
// or nil and false if the clipboard does not contain an image.
func ReadClipboardImage() ([]byte, bool) {
	data := clipboard.Read(clipboard.FmtImage)
	if len(data) == 0 {
		return nil, false
	}
	return data, true
}

// ReadClipboardText reads UTF-8 text from the system clipboard.
// Returns the text and true if text was found, or empty string and false.
func ReadClipboardText() (string, bool) {
	data := clipboard.Read(clipboard.FmtText)
	if len(data) == 0 {
		return "", false
	}
	return string(data), true
}

// WriteClipboardText writes UTF-8 text to the system clipboard.
func WriteClipboardText(text string) {
	clipboard.Write(clipboard.FmtText, []byte(text))
}
```

**Step 3: Update /copy command to use new clipboard**

In `internal/tui/builtins.go`:
- Remove import `"github.com/atotto/clipboard"`
- Replace `clipboard.WriteAll(text)` with `WriteClipboardText(text)` (line 546)
- The `WriteClipboardText` function has no error return since `clipboard.Write` doesn't error. Remove the error check.

The updated `cmdCopy` function:

```go
func cmdCopy(m *Model, args string) tea.Cmd {
	var text string
	for i := len(m.output.blocks) - 1; i >= 0; i-- {
		b := m.output.blocks[i]
		if b.kind == blockText && strings.TrimSpace(b.content) != "" {
			text = strings.TrimSpace(b.content)
			break
		}
	}

	if text == "" {
		m.output.AppendError("No response to copy.")
		return nil
	}

	WriteClipboardText(text)

	lines := strings.Count(text, "\n") + 1
	m.output.AppendSystem(fmt.Sprintf("Copied %d lines to clipboard.", lines))
	return nil
}
```

**Step 4: Remove old dependency**

```bash
cd /Users/justin/git/glamdring && go mod tidy
```

Verify `github.com/atotto/clipboard` is removed from go.mod and `golang.design/x/clipboard` is present.

**Step 5: Run all tests to verify no regressions**

Run: `cd /Users/justin/git/glamdring && go test ./... 2>&1 | tail -20`
Expected: All existing tests pass

**Step 6: Commit**

```bash
git add internal/tui/clipboard.go internal/tui/builtins.go go.mod go.sum
git commit -m "refactor: replace atotto/clipboard with golang.design/x/clipboard

Consolidates clipboard handling into a single library that supports
both text and image reads, enabling the upcoming image paste feature."
```

---

### Task 4: Add clipboard init to startup

**Files:**
- Modify: `cmd/glamdring/main.go`

**Step 1: Add clipboard initialization**

In `cmd/glamdring/main.go`, add import `"github.com/justin/glamdring/internal/tui"` (already imported) and call `tui.InitClipboard()` early in main, after flag parsing but before TUI setup. Non-fatal -- log a warning if it fails:

```go
if err := tui.InitClipboard(); err != nil {
	log.Printf("warning: clipboard not available: %v", err)
}
```

Add this after line 77 (after `workDir` is resolved), before settings loading.

**Step 2: Verify the app builds and starts**

Run: `cd /Users/justin/git/glamdring && go build ./cmd/glamdring/`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add cmd/glamdring/main.go
git commit -m "feat: initialize clipboard subsystem at startup"
```

---

### Task 5: Add image staging to InputModel

**Files:**
- Modify: `internal/tui/input.go`

**Step 1: Add PendingImage type and staging fields**

Add to `internal/tui/input.go`:

```go
// PendingImage holds a clipboard image staged for submission.
type PendingImage struct {
	Data   []byte // raw PNG bytes
	Width  int    // image width in pixels
	Height int    // image height in pixels
}
```

Add field to `InputModel`:

```go
	// pendingImages holds images staged via Ctrl+V for the next submission.
	pendingImages []PendingImage
```

**Step 2: Add Ctrl+V handler**

In `InputModel.Update()`, add a new case before the existing key switch (after the searching check, before `switch msg.Type`):

```go
		// Handle Ctrl+V: check clipboard for image first, then text.
		if msg.Type == tea.KeyCtrlV {
			if imgData, ok := ReadClipboardImage(); ok {
				w, h := pngDimensions(imgData)
				m.pendingImages = append(m.pendingImages, PendingImage{
					Data:   imgData,
					Width:  w,
					Height: h,
				})
				return m, nil
			}
			// No image -- fall through to paste text.
			if text, ok := ReadClipboardText(); ok {
				m.textarea.InsertString(text)
				return m, nil
			}
			return m, nil
		}
```

**Step 3: Add PNG dimensions helper**

Add to `internal/tui/input.go`:

```go
// pngDimensions extracts width and height from a PNG file's IHDR chunk.
// Returns 0, 0 if the data is not a valid PNG.
func pngDimensions(data []byte) (int, int) {
	// PNG header: 8-byte signature, then IHDR chunk.
	// IHDR starts at byte 16: 4 bytes width, 4 bytes height (big-endian).
	if len(data) < 24 {
		return 0, 0
	}
	// Check PNG signature.
	if data[0] != 0x89 || data[1] != 'P' || data[2] != 'N' || data[3] != 'G' {
		return 0, 0
	}
	w := int(data[16])<<24 | int(data[17])<<16 | int(data[18])<<8 | int(data[19])
	h := int(data[20])<<24 | int(data[21])<<16 | int(data[22])<<8 | int(data[23])
	return w, h
}
```

**Step 4: Update SubmitMsg to carry images**

Change `SubmitMsg`:

```go
type SubmitMsg struct {
	Text   string
	Images []PendingImage
}
```

Update the Enter handler in `Update()` to transfer images:

```go
		case tea.KeyEnter:
			value := m.textarea.Value()
			if value == "" && len(m.pendingImages) == 0 {
				return m, nil
			}
			m.history.ResetCursor()
			images := m.pendingImages
			m.pendingImages = nil
			return m, func() tea.Msg {
				return SubmitMsg{Text: value, Images: images}
			}
```

**Step 5: Update Reset to clear images**

Update `Reset()`:

```go
func (m *InputModel) Reset() {
	m.textarea.Reset()
	m.pendingImages = nil
}
```

**Step 6: Add HasImages and ImageCount helpers**

```go
// HasImages returns true if there are staged images.
func (m InputModel) HasImages() bool {
	return len(m.pendingImages) > 0
}

// ImageCount returns the number of staged images.
func (m InputModel) ImageCount() int {
	return len(m.pendingImages)
}
```

**Step 7: Update View to show image indicators**

Update the `View()` method (non-searching branch):

```go
func (m InputModel) View() string {
	if m.searching {
		return m.renderSearch()
	}
	border := m.styles.InputBorder.Width(m.width - 2)
	var content string
	if len(m.pendingImages) > 0 {
		var indicators []string
		for i, img := range m.pendingImages {
			if img.Width > 0 && img.Height > 0 {
				indicators = append(indicators, fmt.Sprintf("[Image %d: %dx%d]", i+1, img.Width, img.Height))
			} else {
				indicators = append(indicators, fmt.Sprintf("[Image %d]", i+1))
			}
		}
		imageBar := lipgloss.NewStyle().Foreground(colorAmber).Render(strings.Join(indicators, " "))
		content = imageBar + "\n" + m.textarea.View()
	} else {
		content = m.textarea.View()
	}
	return border.Render(content)
}
```

Add the necessary imports at the top of the file: `"fmt"`, `"strings"`.

**Step 8: Verify build**

Run: `cd /Users/justin/git/glamdring && go build ./...`
Expected: Build succeeds

**Step 9: Commit**

```bash
git add internal/tui/input.go
git commit -m "feat: add image staging via Ctrl+V in input component

Images are staged in pendingImages, shown as indicators above the
textarea, and carried to handleSubmit via SubmitMsg.Images."
```

---

### Task 6: Update handleSubmit to build content blocks

**Files:**
- Modify: `internal/tui/model.go:424-487`

**Step 1: Update handleSubmit**

Modify the `handleSubmit` method in `model.go`. After the slash command expansion section (around line 466), change the session turn call to handle images:

Replace the block starting at `m.turn++` (approx line 467) through the end of the function:

```go
	m.turn++
	m.turnModifiedFiles = false
	m.state = StateRunning
	m.spinning = true

	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	turnCtx, cancel := context.WithCancel(ctx)
	m.cancelTurn = cancel

	if m.session == nil {
		m.session = agent.NewSession(m.agentCfg)
	}

	var ch <-chan agent.Message
	if len(msg.Images) > 0 {
		blocks := buildContentBlocks(prompt, msg.Images)
		ch = m.session.TurnWithBlocks(turnCtx, blocks)
	} else {
		ch = m.session.Turn(turnCtx, prompt)
	}
	return m, tea.Batch(
		func() tea.Msg { return agentStartedMsg{ch: ch} },
		m.spinner.Tick,
	)
```

**Step 2: Add buildContentBlocks helper**

Add to `model.go`:

```go
// buildContentBlocks constructs a []api.ContentBlock from text and images
// for sending to the Claude vision API.
func buildContentBlocks(text string, images []PendingImage) []api.ContentBlock {
	var blocks []api.ContentBlock

	// Images first, then text -- matches Claude Code's ordering.
	for _, img := range images {
		blocks = append(blocks, api.ContentBlock{
			Type: "image",
			Source: &api.ImageSource{
				Type:      "base64",
				MediaType: "image/png",
				Data:      base64.StdEncoding.EncodeToString(img.Data),
			},
		})
	}

	if text != "" {
		blocks = append(blocks, api.ContentBlock{
			Type: "text",
			Text: text,
		})
	}

	return blocks
}
```

Add `"encoding/base64"` to the import block.

**Step 3: Update AppendUserMessage call to show images**

In `handleSubmit`, update the `m.output.AppendUserMessage` call (around line 449) to include image info:

```go
	if len(msg.Images) > 0 {
		var parts []string
		for i, img := range msg.Images {
			if img.Width > 0 && img.Height > 0 {
				parts = append(parts, fmt.Sprintf("[Image %d: %dx%d]", i+1, img.Width, img.Height))
			} else {
				parts = append(parts, fmt.Sprintf("[Image %d]", i+1))
			}
		}
		imageLabel := strings.Join(parts, " ")
		if msg.Text != "" {
			m.output.AppendUserMessage(imageLabel + "\n" + msg.Text)
		} else {
			m.output.AppendUserMessage(imageLabel)
		}
	} else {
		m.output.AppendUserMessage(msg.Text)
	}
```

**Step 4: Verify build**

Run: `cd /Users/justin/git/glamdring && go build ./...`
Expected: Build succeeds

**Step 5: Run all tests**

Run: `cd /Users/justin/git/glamdring && go test ./... 2>&1 | tail -20`
Expected: All tests pass

**Step 6: Commit**

```bash
git add internal/tui/model.go
git commit -m "feat: wire image paste through handleSubmit to session

When SubmitMsg carries images, builds []ContentBlock with base64-encoded
PNG data and uses TurnWithBlocks instead of Turn."
```

---

### Task 7: Add unit tests for image paste components

**Files:**
- Modify: `internal/tui/model_test.go`
- Create: `internal/tui/input_test.go`

**Step 1: Test pngDimensions**

Create `internal/tui/input_test.go`:

```go
package tui

import (
	"testing"
)

func TestPngDimensions_ValidPNG(t *testing.T) {
	// Minimal 1x1 white PNG (hex).
	data := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, // IHDR chunk length
		0x49, 0x48, 0x44, 0x52, // "IHDR"
		0x00, 0x00, 0x00, 0x01, // width: 1
		0x00, 0x00, 0x00, 0x01, // height: 1
		0x08, 0x02,             // bit depth, color type
		0x00, 0x00, 0x00,       // compression, filter, interlace
	}
	w, h := pngDimensions(data)
	if w != 1 || h != 1 {
		t.Errorf("pngDimensions = (%d, %d), want (1, 1)", w, h)
	}
}

func TestPngDimensions_LargerImage(t *testing.T) {
	// 1920x1080 PNG header.
	data := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x07, 0x80, // 1920
		0x00, 0x00, 0x04, 0x38, // 1080
		0x08, 0x02,
		0x00, 0x00, 0x00,
	}
	w, h := pngDimensions(data)
	if w != 1920 || h != 1080 {
		t.Errorf("pngDimensions = (%d, %d), want (1920, 1080)", w, h)
	}
}

func TestPngDimensions_TooShort(t *testing.T) {
	w, h := pngDimensions([]byte{0x89, 0x50})
	if w != 0 || h != 0 {
		t.Errorf("pngDimensions = (%d, %d), want (0, 0)", w, h)
	}
}

func TestPngDimensions_NotPNG(t *testing.T) {
	data := make([]byte, 30)
	data[0] = 0xFF // JPEG marker
	w, h := pngDimensions(data)
	if w != 0 || h != 0 {
		t.Errorf("pngDimensions = (%d, %d), want (0, 0)", w, h)
	}
}

func TestInputResetClearsPendingImages(t *testing.T) {
	m := NewInputModel(DefaultStyles())
	m.pendingImages = []PendingImage{{Data: []byte{1, 2, 3}}}
	m.Reset()
	if len(m.pendingImages) != 0 {
		t.Errorf("expected 0 pending images after reset, got %d", len(m.pendingImages))
	}
}

func TestInputHasImages(t *testing.T) {
	m := NewInputModel(DefaultStyles())
	if m.HasImages() {
		t.Error("expected no images initially")
	}
	m.pendingImages = []PendingImage{{Data: []byte{1}}}
	if !m.HasImages() {
		t.Error("expected images after staging")
	}
}

func TestSubmitMsgCarriesImages(t *testing.T) {
	images := []PendingImage{
		{Data: []byte{1}, Width: 100, Height: 200},
		{Data: []byte{2}, Width: 300, Height: 400},
	}
	msg := SubmitMsg{Text: "test", Images: images}
	if len(msg.Images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(msg.Images))
	}
	if msg.Images[0].Width != 100 {
		t.Errorf("first image width = %d, want 100", msg.Images[0].Width)
	}
}
```

**Step 2: Test buildContentBlocks**

Add to `internal/tui/model_test.go`:

```go
func TestBuildContentBlocks_ImagesAndText(t *testing.T) {
	images := []PendingImage{
		{Data: []byte{0x89, 0x50, 0x4E, 0x47}, Width: 100, Height: 200},
	}
	blocks := buildContentBlocks("describe this", images)

	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Type != "image" {
		t.Errorf("first block type = %q, want 'image'", blocks[0].Type)
	}
	if blocks[0].Source == nil {
		t.Fatal("expected non-nil source on image block")
	}
	if blocks[0].Source.MediaType != "image/png" {
		t.Errorf("media_type = %q, want 'image/png'", blocks[0].Source.MediaType)
	}
	if blocks[1].Type != "text" {
		t.Errorf("second block type = %q, want 'text'", blocks[1].Type)
	}
	if blocks[1].Text != "describe this" {
		t.Errorf("text = %q, want 'describe this'", blocks[1].Text)
	}
}

func TestBuildContentBlocks_ImagesOnly(t *testing.T) {
	images := []PendingImage{
		{Data: []byte{1}},
		{Data: []byte{2}},
	}
	blocks := buildContentBlocks("", images)

	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks (no text), got %d", len(blocks))
	}
	for _, b := range blocks {
		if b.Type != "image" {
			t.Errorf("expected all image blocks, got %q", b.Type)
		}
	}
}

func TestBuildContentBlocks_TextOnly(t *testing.T) {
	blocks := buildContentBlocks("hello", nil)

	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Type != "text" || blocks[0].Text != "hello" {
		t.Errorf("expected text block with 'hello'")
	}
}
```

**Step 3: Run all tests**

Run: `cd /Users/justin/git/glamdring && go test ./... 2>&1 | tail -20`
Expected: All tests pass

**Step 4: Commit**

```bash
git add internal/tui/input_test.go internal/tui/model_test.go
git commit -m "test: add unit tests for image paste components"
```

---

### Task 8: Update README

**Files:**
- Modify: `README.md`

**Step 1: Add image paste to keybindings/features section**

Find the keybindings table or features list in README.md and add:

```markdown
| Ctrl+V | Paste image from clipboard (or text if no image) |
```

Also add a brief note about image support in the features section mentioning that glamdring supports Claude's vision API via clipboard image paste.

**Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add image paste feature to README"
```

---

### Task 9: Integration smoke test

**Step 1: Build and manual verification**

```bash
cd /Users/justin/git/glamdring && go build ./cmd/glamdring/
```

**Step 2: Run full test suite**

```bash
cd /Users/justin/git/glamdring && go test ./... 2>&1 | tail -20
```

Expected: All tests pass, build succeeds, no compiler warnings.
