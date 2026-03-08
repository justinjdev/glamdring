# Interactive Config Modal

## Summary

Add a floating modal overlay system to glamdring, starting with an interactive
`/config` settings panel. The modal renders as a centered box over the viewport
content. Arrow keys navigate, enter selects/toggles, esc closes.

## Architecture

Three layers:

1. **Modal renderer** (`modal.go`) - Draws a bordered box centered over the
   viewport by replacing lines in the fully-rendered view string.
2. **List/menu component** (embedded in modal) - Cursor-navigable items with
   highlight. Supports selectable items, toggles, and section headers.
3. **Config menu** - Concrete settings content composed from list items.

## Modal Renderer

- `ModalModel` struct: `items []MenuItem`, `cursor int`, `title string`,
  `width/height int`, `onSelect func`, `onClose func`
- `View()` returns a box string with border, title, items, and help text
- Overlay logic in `Model.View()`: render normal view, split lines, replace
  center rows with modal box, rejoin

## Menu Item Types

```go
type MenuItemKind int
const (
    MenuSection  MenuItemKind = iota  // non-selectable header
    MenuChoice                         // pick from list (theme, model)
    MenuToggle                         // on/off switch
)

type MenuItem struct {
    Kind     MenuItemKind
    Label    string
    Value    string       // current value display
    Active   bool         // for toggles
    Children []MenuItem   // for expandable choices
}
```

## Config Menu Layout

```
 Settings
 ─────────────────────────────
   Theme         > glamdring
   Model           claude-opus-4-6
   Thinking        on
   Yolo            off
   High contrast   off

   esc close  enter select/toggle
```

Theme and Model expand inline to show choices when selected. Toggles flip on
enter. Theme live-previews on cursor move.

## State Integration

- New `StateModal` state constant
- New `modal *ModalModel` field on Model
- `handleKeyMsg` routes all keys to modal when `StateModal`
- Esc closes modal, restores `StateInput`
- Changes apply immediately and persist via `SaveUserSetting`

## Entry Points

- `/config` - full settings modal
- `/theme` (no args) - config modal focused on theme
- `/model` (no args) - config modal focused on model

## Implementation Plan

1. Create `modal.go` with `ModalModel` struct, `Update()`, `View()`
2. Add overlay rendering to `Model.View()`
3. Add `StateModal` and key routing
4. Build config menu items from current settings
5. Wire `/config`, update `/theme` and `/model`
6. Add styles for modal (border, highlight, section headers)
7. Persist changes via `SaveUserSetting`
