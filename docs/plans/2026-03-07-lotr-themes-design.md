# LOTR Theme System Design

## Summary

Replace the single hardcoded color palette with a theme system featuring five LOTR-inspired themes, a high contrast toggle, and runtime theme switching via `/theme` command.

## Themes

| Theme | Inspiration | Primary Accent | Background | Feel |
|---|---|---|---|---|
| `glamdring` (default) | Elvish blade glow | Ice blue `#7daea3` | `#1a1a1f` | Cold, precise |
| `rivendell` | Twilight valley | Soft teal `#6ec4a7` | `#171b22` | Ethereal, elegant |
| `mithril` | Dwarven forge | Bright cyan `#56d4e0` | `#141820` | Industrial-elven |
| `lothlorien` | Forest starlight | Cool gold `#c9b458` | `#151820` | Celestial, warm-cool |
| `shire` | Legacy warm amber | Amber `#e78a4e` | `#1a1612` | Warm, candlelit |

## Architecture

### ThemePalette struct

```go
type ThemePalette struct {
    Name     string
    Bg       lipgloss.Color
    Fg       lipgloss.Color
    FgDim    lipgloss.Color
    FgBright lipgloss.Color
    Primary   lipgloss.Color
    Secondary lipgloss.Color
    Success   lipgloss.Color
    Error     lipgloss.Color
    Info      lipgloss.Color
    Subtle    lipgloss.Color
    Surface0 lipgloss.Color
    Surface1 lipgloss.Color
    Surface2 lipgloss.Color
}
```

### Theme registry

A package-level `map[string]ThemePalette` containing all five themes. Lookup by name, fall back to `glamdring` if unknown.

### High contrast transform

`HighContrastTransform(p ThemePalette) ThemePalette` -- boosts FgBright toward white, brightens Primary/Error/Success by ~20%, darkens Bg toward `#101014`, increases Surface separation.

### Style construction

`DefaultStyles(palette ThemePalette) Styles` -- the existing function parameterized to build all lipgloss styles from a palette instead of hardcoded colors.

## Configuration

```json
{
  "theme": "glamdring",
  "high_contrast": false
}
```

Fields in `config.Settings`: `Theme string`, `HighContrast bool`, `Themes map[string]ThemePalette`.

### User-defined themes

Users can define custom themes in `settings.json`. User themes take precedence over built-in themes when names conflict.

```json
{
  "theme": "my-custom",
  "themes": {
    "my-custom": {
      "bg": "#1a1a1f",
      "fg": "#b0b8c4",
      "fg_dim": "#5a6270",
      "fg_bright": "#e0e4ea",
      "primary": "#ff6600",
      "secondary": "#cc9900",
      "success": "#00cc66",
      "error": "#cc3333",
      "info": "#3399cc",
      "subtle": "#9966cc",
      "surface0": "#202028",
      "surface1": "#2a2a34",
      "surface2": "#363640"
    }
  }
}
```

## Runtime switching

`/theme` -- lists available themes with current marked.
`/theme <name>` -- switches theme immediately, rebuilds styles, rerenders all components.

## Color Table

| Slot | glamdring | rivendell | mithril | lothlorien | shire |
|---|---|---|---|---|---|
| Bg | `#1a1a1f` | `#171b22` | `#141820` | `#151820` | `#1a1612` |
| Fg | `#b0b8c4` | `#a8b5c2` | `#b4bcc8` | `#b0b8a8` | `#d4be98` |
| FgDim | `#5a6270` | `#4e5a68` | `#4c5666` | `#566050` | `#7c6f64` |
| FgBright | `#e0e4ea` | `#d8dce4` | `#e2e6ec` | `#dce0d4` | `#ebdbb2` |
| Primary | `#7daea3` | `#6ec4a7` | `#56d4e0` | `#c9b458` | `#e78a4e` |
| Secondary | `#a0c4d0` | `#8ba8c4` | `#7e9ab8` | `#a8965c` | `#d8a657` |
| Success | `#7dba6e` | `#6eb88a` | `#5cc4a0` | `#8eb86e` | `#a9b665` |
| Error | `#d46a6a` | `#c87070` | `#e06060` | `#d47060` | `#ea6962` |
| Info | `#7daea3` | `#7a9cb8` | `#6eaac4` | `#8ea8b8` | `#7daea3` |
| Subtle | `#9080a8` | `#8878a0` | `#7880a8` | `#8890a0` | `#d3869b` |
| Surface0 | `#202028` | `#1e2430` | `#1a2028` | `#1c2028` | `#282420` |
| Surface1 | `#2a2a34` | `#262e3a` | `#222a34` | `#242a32` | `#32302f` |
| Surface2 | `#363640` | `#303a46` | `#2c3640` | `#2e363e` | `#3c3836` |

## Files changed

- `internal/tui/styles.go` -- ThemePalette, registry, HC transform, parameterize DefaultStyles
- `internal/tui/model.go` -- read theme from settings, apply palette
- `internal/tui/builtins.go` -- /theme command
- `pkg/config/settings.go` -- Theme, HighContrast, Themes fields (with JSON deserialization for user-defined palettes)
- `README.md` -- document theme configuration
