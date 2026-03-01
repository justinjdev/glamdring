# Image Paste via Ctrl+V

## Overview

Enable pasting images from the system clipboard into glamdring via Ctrl+V, matching Claude Code's behavior. When the clipboard contains image data (screenshots, copied images), Ctrl+V stages the image for submission. When the clipboard contains text, Ctrl+V pastes text as usual.

## Requirements

- Trigger: Ctrl+V detects clipboard content type and acts accordingly
- Sources: macOS screenshots (Cmd+Shift+3/4) and images copied from apps
- Multiple images per message supported (each Ctrl+V adds another)
- Visual indicator shows staged images before submission
- Images sent as base64-encoded PNG via Claude's vision API

## Architecture

### Data Flow

```
Ctrl+V -> clipboard.Read(FmtImage) -> PNG bytes -> base64 encode ->
  stage in InputModel.pendingImages -> on Enter, build []ContentBlock ->
  SubmitMsg carries images -> Session.TurnWithBlocks(ctx, blocks) ->
  API sends {type:"image", source:{type:"base64", media_type:"image/png", data:"..."}}
```

### 1. API Types (`pkg/api/types.go`)

Add `ImageSource` struct to `ContentBlock`:

```go
// type: "image"
Source *ImageSource `json:"source,omitempty"`

type ImageSource struct {
    Type      string `json:"type"`       // "base64"
    MediaType string `json:"media_type"` // "image/png"
    Data      string `json:"data"`       // base64-encoded PNG
}
```

`RequestMessage.Content` is already `any`, so sending `[]ContentBlock` for user messages with images works without changes to the request structure.

### 2. Input Layer (`internal/tui/input.go`)

Add image staging to `InputModel`:

```go
type PendingImage struct {
    Data   []byte // raw PNG bytes
    Width  int    // from PNG header (for display)
    Height int
}

// Added to InputModel:
pendingImages []PendingImage
```

Ctrl+V handling in `Update()`:
- Intercept `tea.KeyCtrlV` before passing to textarea
- Attempt `clipboard.Read(clipboard.FmtImage)` -- if non-empty, stage the image
- If no image data, fall through to normal text paste (read text clipboard, insert into textarea)

View renders image indicators above the textarea inside the border:
```
[Image 1: 1280x720] [Image 2: 640x480]
> describe these screenshots
```

`SubmitMsg` gains an `Images []PendingImage` field. On Enter, images transfer from `InputModel` to the message. `InputModel.Reset()` clears pending images.

### 3. Clipboard Module (`internal/tui/clipboard.go`)

New file with a clean interface:

```go
func ReadClipboardImage() ([]byte, bool)
func ReadClipboardText() (string, bool)
```

Uses `golang.design/x/clipboard` internally. `clipboard.Init()` called once at startup.

Replaces `atotto/clipboard` in `/copy` command as well, consolidating to one clipboard library.

### 4. Session Layer (`pkg/agent/session.go`)

New method alongside `Turn`:

```go
func (s *Session) TurnWithBlocks(ctx context.Context, blocks []api.ContentBlock) <-chan Message
```

Called by `handleSubmit` when images are present. Sends `Content: blocks` (a `[]ContentBlock`) instead of a plain string. The existing `Turn(ctx, string)` remains for text-only messages.

### 5. Output Display (`internal/tui/model.go`)

When user submits images, `AppendUserMessage` shows:
```
You: [Image 1: 1280x720] [Image 2: 640x480]
describe these screenshots
```

No inline image rendering in the terminal for v1.

## Dependencies

- Add: `golang.design/x/clipboard` (cross-platform clipboard with image support, requires CGO)
- Remove: `github.com/atotto/clipboard` (text-only, replaced)

## Files Modified

- `pkg/api/types.go` -- add ImageSource struct and field
- `pkg/agent/session.go` -- add TurnWithBlocks method
- `internal/tui/input.go` -- add pendingImages, Ctrl+V handler, image indicators
- `internal/tui/clipboard.go` -- new file, clipboard abstraction
- `internal/tui/model.go` -- update handleSubmit to build content blocks
- `internal/tui/builtins.go` -- update /copy to use new clipboard library
- `cmd/glamdring/main.go` -- clipboard.Init() at startup
- `go.mod` / `go.sum` -- dependency changes
