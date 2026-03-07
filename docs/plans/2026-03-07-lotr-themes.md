# LOTR Theme System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the hardcoded color palette with a theme system featuring five LOTR-inspired themes, high contrast toggle, user-defined themes, and runtime `/theme` switching.

**Architecture:** A `ThemePalette` struct defines named color slots. Built-in themes are a package-level registry. `DefaultStyles()` is parameterized to accept a palette. Settings adds `theme`, `high_contrast`, and `themes` fields. A `/theme` command switches at runtime.

**Tech Stack:** Go, lipgloss, bubbletea, existing config/settings infrastructure.

---

### Task 1: Add Theme and HighContrast fields to Settings

**Files:**
- Modify: `pkg/config/settings.go`
- Test: `pkg/config/settings_test.go`

**Step 1: Write the failing test**

Add to `pkg/config/settings_test.go`:

```go
func TestSettings_ThemeFields(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{
		"theme": "rivendell",
		"high_contrast": true,
		"themes": {
			"custom": {
				"bg": "#111111",
				"fg": "#eeeeee",
				"fg_dim": "#888888",
				"fg_bright": "#ffffff",
				"primary": "#ff0000",
				"secondary": "#00ff00",
				"success": "#00cc00",
				"error": "#cc0000",
				"info": "#0000cc",
				"subtle": "#880088",
				"surface0": "#222222",
				"surface1": "#333333",
				"surface2": "#444444"
			}
		}
	}`), 0o644)

	s := config.LoadSettings(dir)
	if s.Theme != "rivendell" {
		t.Errorf("Theme = %q, want rivendell", s.Theme)
	}
	if !s.HighContrast {
		t.Error("HighContrast = false, want true")
	}
	if len(s.Themes) != 1 {
		t.Fatalf("Themes count = %d, want 1", len(s.Themes))
	}
	custom := s.Themes["custom"]
	if custom.Bg != "#111111" {
		t.Errorf("custom.Bg = %q, want #111111", custom.Bg)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/config/ -run TestSettings_ThemeFields -v`
Expected: FAIL (Theme, HighContrast, Themes fields don't exist)

**Step 3: Implement the settings fields**

In `pkg/config/settings.go`, add to `Settings` struct:

```go
type Settings struct {
	Model        string                       `json:"model,omitempty"`
	MaxTurns     *int                         `json:"max_turns,omitempty"`
	MCPServers   map[string]MCPServerConfig   `json:"mcp_servers,omitempty"`
	Indexer      IndexerConfig                `json:"indexer,omitempty"`
	Experimental ExperimentalConfig           `json:"experimental,omitempty"`
	Workflows    map[string]WorkflowConfig    `json:"workflows,omitempty"`
	Theme        string                       `json:"theme,omitempty"`
	HighContrast bool                         `json:"high_contrast,omitempty"`
	Themes       map[string]UserThemeConfig   `json:"themes,omitempty"`
}
```

Add `UserThemeConfig`:

```go
// UserThemeConfig holds user-defined theme colors from settings.json.
type UserThemeConfig struct {
	Bg       string `json:"bg"`
	Fg       string `json:"fg"`
	FgDim    string `json:"fg_dim"`
	FgBright string `json:"fg_bright"`
	Primary  string `json:"primary"`
	Secondary string `json:"secondary"`
	Success  string `json:"success"`
	Error    string `json:"error"`
	Info     string `json:"info"`
	Subtle   string `json:"subtle"`
	Surface0 string `json:"surface0"`
	Surface1 string `json:"surface1"`
	Surface2 string `json:"surface2"`
}
```

Add merge logic in `mergeSettings`:

```go
if override.Theme != "" {
	base.Theme = override.Theme
}
if override.HighContrast {
	base.HighContrast = true
}
if override.Themes != nil {
	if base.Themes == nil {
		base.Themes = make(map[string]UserThemeConfig)
	}
	for k, v := range override.Themes {
		base.Themes[k] = v
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/config/ -run TestSettings_ThemeFields -v`
Expected: PASS

**Step 5: Commit**

```
git add pkg/config/settings.go pkg/config/settings_test.go
git commit -m "feat: add theme, high_contrast, and custom themes to settings"
```

---

### Task 2: Define ThemePalette struct and built-in theme registry

**Files:**
- Modify: `internal/tui/styles.go`
- Test: `internal/tui/styles_test.go` (create if needed)

**Step 1: Write the failing test**

```go
func TestThemeRegistry_AllBuiltins(t *testing.T) {
	expected := []string{"glamdring", "rivendell", "mithril", "lothlorien", "shire"}
	for _, name := range expected {
		p, ok := LookupTheme(name)
		if !ok {
			t.Errorf("theme %q not found in registry", name)
			continue
		}
		if p.Name != name {
			t.Errorf("theme %q has Name=%q", name, p.Name)
		}
		if p.Bg == "" || p.Fg == "" || p.Primary == "" {
			t.Errorf("theme %q has empty required fields", name)
		}
	}
}

func TestThemeRegistry_UnknownFallsBack(t *testing.T) {
	p, ok := LookupTheme("nonexistent")
	if ok {
		t.Error("expected ok=false for unknown theme")
	}
	if p.Name != "glamdring" {
		t.Errorf("fallback Name=%q, want glamdring", p.Name)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run TestThemeRegistry -v`
Expected: FAIL (LookupTheme undefined)

**Step 3: Implement ThemePalette and registry**

In `internal/tui/styles.go`, replace the color variables and add:

```go
// ThemePalette defines the named color slots for a theme.
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

var builtinThemes = map[string]ThemePalette{
	"glamdring": {
		Name: "glamdring", Bg: "#1a1a1f", Fg: "#b0b8c4", FgDim: "#5a6270", FgBright: "#e0e4ea",
		Primary: "#7daea3", Secondary: "#a0c4d0", Success: "#7dba6e", Error: "#d46a6a",
		Info: "#7daea3", Subtle: "#9080a8", Surface0: "#202028", Surface1: "#2a2a34", Surface2: "#363640",
	},
	"rivendell": {
		Name: "rivendell", Bg: "#171b22", Fg: "#a8b5c2", FgDim: "#4e5a68", FgBright: "#d8dce4",
		Primary: "#6ec4a7", Secondary: "#8ba8c4", Success: "#6eb88a", Error: "#c87070",
		Info: "#7a9cb8", Subtle: "#8878a0", Surface0: "#1e2430", Surface1: "#262e3a", Surface2: "#303a46",
	},
	"mithril": {
		Name: "mithril", Bg: "#141820", Fg: "#b4bcc8", FgDim: "#4c5666", FgBright: "#e2e6ec",
		Primary: "#56d4e0", Secondary: "#7e9ab8", Success: "#5cc4a0", Error: "#e06060",
		Info: "#6eaac4", Subtle: "#7880a8", Surface0: "#1a2028", Surface1: "#222a34", Surface2: "#2c3640",
	},
	"lothlorien": {
		Name: "lothlorien", Bg: "#151820", Fg: "#b0b8a8", FgDim: "#566050", FgBright: "#dce0d4",
		Primary: "#c9b458", Secondary: "#a8965c", Success: "#8eb86e", Error: "#d47060",
		Info: "#8ea8b8", Subtle: "#8890a0", Surface0: "#1c2028", Surface1: "#242a32", Surface2: "#2e363e",
	},
	"shire": {
		Name: "shire", Bg: "#1a1612", Fg: "#d4be98", FgDim: "#7c6f64", FgBright: "#ebdbb2",
		Primary: "#e78a4e", Secondary: "#d8a657", Success: "#a9b665", Error: "#ea6962",
		Info: "#7daea3", Subtle: "#d3869b", Surface0: "#282420", Surface1: "#32302f", Surface2: "#3c3836",
	},
}

// LookupTheme returns the named theme. If not found, returns the glamdring
// default and ok=false.
func LookupTheme(name string) (ThemePalette, bool) {
	if p, ok := builtinThemes[name]; ok {
		return p, true
	}
	return builtinThemes["glamdring"], false
}

// ThemeNames returns a sorted list of built-in theme names.
func ThemeNames() []string {
	names := make([]string, 0, len(builtinThemes))
	for name := range builtinThemes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/ -run TestThemeRegistry -v`
Expected: PASS

**Step 5: Commit**

```
git add internal/tui/styles.go internal/tui/styles_test.go
git commit -m "feat: add ThemePalette struct and five built-in LOTR themes"
```

---

### Task 3: Parameterize DefaultStyles to accept ThemePalette

**Files:**
- Modify: `internal/tui/styles.go`
- Modify: `internal/tui/model.go` (update callers)

**Step 1: Write the failing test**

```go
func TestDefaultStyles_UsesThemePalette(t *testing.T) {
	p := ThemePalette{
		Name: "test", Bg: "#000000", Fg: "#ffffff", FgDim: "#888888", FgBright: "#ffffff",
		Primary: "#ff0000", Secondary: "#00ff00", Success: "#00cc00", Error: "#cc0000",
		Info: "#0000ff", Subtle: "#880088", Surface0: "#111111", Surface1: "#222222", Surface2: "#333333",
	}
	s := DefaultStyles(p)
	// Verify the styles were built (non-zero struct).
	rendered := s.StatusBar.Render("test")
	if rendered == "" {
		t.Error("expected non-empty rendered status bar")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run TestDefaultStyles_UsesThemePalette -v`
Expected: FAIL (DefaultStyles takes no args / wrong signature)

**Step 3: Change DefaultStyles signature and implementation**

Change `func DefaultStyles() Styles` to `func DefaultStyles(p ThemePalette) Styles`.

Replace all hardcoded color variables (`colorBg`, `colorAmber`, etc.) with palette fields (`p.Bg`, `p.Primary`, etc.). Remove the old `var` block of color constants (except keep any that are truly unused -- the linter flagged `colorTeal` and `colorSurface0` as unused already).

Mapping from old colors to palette fields:
- `colorBg` -> `p.Bg`
- `colorFg` -> `p.Fg`
- `colorFgDim` -> `p.FgDim`
- `colorFgBright` -> `p.FgBright`
- `colorAmber` -> `p.Primary`
- `colorGold` -> `p.Secondary`
- `colorSage` -> `p.Success`
- `colorRust` -> `p.Error`
- `colorSky` -> `p.Info`
- `colorLavender` -> `p.Subtle`
- `colorSurface0` -> `p.Surface0`
- `colorSurface1` -> `p.Surface1`
- `colorSurface2` -> `p.Surface2`

Update callers in `model.go`:
- `New()`: change `DefaultStyles()` to `DefaultStyles(builtinThemes["glamdring"])`
- Also update the `colorAmber` reference in the spinner style to use a palette field. Store the palette on the Model struct.

Add to `Model` struct:

```go
palette ThemePalette
```

**Step 4: Run all tests**

Run: `go test ./internal/tui/ -v`
Expected: PASS

**Step 5: Commit**

```
git add internal/tui/styles.go internal/tui/model.go
git commit -m "refactor: parameterize DefaultStyles to accept ThemePalette"
```

---

### Task 4: Wire theme selection from settings into Model

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `cmd/glamdring/main.go`

**Step 1: Add SetTheme method and wire settings**

Add to `model.go`:

```go
// SetTheme applies a theme palette to the model, rebuilding all styles.
func (m *Model) SetTheme(p ThemePalette, highContrast bool) {
	if highContrast {
		p = HighContrastTransform(p)
	}
	m.palette = p
	m.styles = DefaultStyles(p)
	m.input.styles = m.styles
	m.output.styles = m.styles
	m.statusbar.styles = m.styles
	m.spinner.Style = lipgloss.NewStyle().Foreground(p.Primary)
}
```

In `cmd/glamdring/main.go`, after `m.SetSettings(settings)`:

```go
// Apply theme from settings.
palette, _ := tui.LookupTheme(settings.Theme)
// Check for user-defined theme override.
if settings.Themes != nil {
	if userTheme, ok := settings.Themes[settings.Theme]; ok {
		palette = tui.PaletteFromUserConfig(userTheme)
	}
}
m.SetTheme(palette, settings.HighContrast)
```

Add `PaletteFromUserConfig` in `styles.go`:

```go
// PaletteFromUserConfig converts a UserThemeConfig to a ThemePalette.
func PaletteFromUserConfig(name string, c config.UserThemeConfig) ThemePalette {
	return ThemePalette{
		Name: name, Bg: lipgloss.Color(c.Bg), Fg: lipgloss.Color(c.Fg),
		FgDim: lipgloss.Color(c.FgDim), FgBright: lipgloss.Color(c.FgBright),
		Primary: lipgloss.Color(c.Primary), Secondary: lipgloss.Color(c.Secondary),
		Success: lipgloss.Color(c.Success), Error: lipgloss.Color(c.Error),
		Info: lipgloss.Color(c.Info), Subtle: lipgloss.Color(c.Subtle),
		Surface0: lipgloss.Color(c.Surface0), Surface1: lipgloss.Color(c.Surface1),
		Surface2: lipgloss.Color(c.Surface2),
	}
}
```

**Step 2: Build and verify**

Run: `go build ./cmd/glamdring`
Expected: builds cleanly

**Step 3: Run all tests**

Run: `go test ./...`
Expected: PASS

**Step 4: Commit**

```
git add internal/tui/model.go internal/tui/styles.go cmd/glamdring/main.go
git commit -m "feat: wire theme selection from settings into TUI model"
```

---

### Task 5: Implement HighContrastTransform

**Files:**
- Modify: `internal/tui/styles.go`
- Test: `internal/tui/styles_test.go`

**Step 1: Write the failing test**

```go
func TestHighContrastTransform(t *testing.T) {
	base, _ := LookupTheme("glamdring")
	hc := HighContrastTransform(base)

	if hc.Name != "glamdring" {
		t.Errorf("Name changed to %q", hc.Name)
	}
	// HC should brighten FgBright toward white.
	if hc.FgBright == base.FgBright {
		t.Error("FgBright unchanged after HC transform")
	}
	// HC should darken Bg.
	if hc.Bg == base.Bg {
		t.Error("Bg unchanged after HC transform")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run TestHighContrastTransform -v`
Expected: FAIL (HighContrastTransform undefined)

**Step 3: Implement**

```go
// HighContrastTransform boosts contrast on a palette for accessibility.
// Brightens text colors, darkens background, and increases accent saturation.
func HighContrastTransform(p ThemePalette) ThemePalette {
	p.Bg = "#0c0c10"
	p.FgBright = "#f4f4f8"
	p.Fg = brighten(p.Fg, 25)
	p.Primary = brighten(p.Primary, 20)
	p.Success = brighten(p.Success, 20)
	p.Error = brighten(p.Error, 20)
	p.Info = brighten(p.Info, 20)
	p.Secondary = brighten(p.Secondary, 15)
	p.Surface0 = "#161620"
	p.Surface1 = "#222230"
	p.Surface2 = "#303042"
	return p
}

// brighten takes a hex color and increases its brightness by the given percentage.
func brighten(c lipgloss.Color, pct int) lipgloss.Color {
	hex := string(c)
	if len(hex) != 7 || hex[0] != '#' {
		return c
	}
	r, _ := strconv.ParseInt(hex[1:3], 16, 32)
	g, _ := strconv.ParseInt(hex[3:5], 16, 32)
	b, _ := strconv.ParseInt(hex[5:7], 16, 32)

	boost := func(v int64) int64 {
		v = v + v*int64(pct)/100
		if v > 255 {
			v = 255
		}
		return v
	}
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", boost(r), boost(g), boost(b)))
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/ -run TestHighContrastTransform -v`
Expected: PASS

**Step 5: Commit**

```
git add internal/tui/styles.go internal/tui/styles_test.go
git commit -m "feat: add HighContrastTransform for accessibility"
```

---

### Task 6: Add /theme command

**Files:**
- Modify: `internal/tui/builtins.go`
- Test: `internal/tui/builtins_test.go`

**Step 1: Write the failing test**

```go
func TestCmdTheme_ListsThemes(t *testing.T) {
	th := NewTestHarness(t, nil)
	th.ProcessBuiltin("theme", "")
	if !th.OutputContains("glamdring") {
		t.Error("expected theme list to include glamdring")
	}
	if !th.OutputContains("rivendell") {
		t.Error("expected theme list to include rivendell")
	}
}

func TestCmdTheme_SwitchesTheme(t *testing.T) {
	th := NewTestHarness(t, nil)
	th.ProcessBuiltin("theme", "mithril")
	if !th.OutputContains("mithril") {
		t.Error("expected confirmation of theme switch")
	}
	if th.Model.palette.Name != "mithril" {
		t.Errorf("palette.Name = %q, want mithril", th.Model.palette.Name)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run TestCmdTheme -v`
Expected: FAIL (cmdTheme undefined)

**Step 3: Implement the /theme command**

In `builtins.go`:

```go
func cmdTheme(m *Model, args string) tea.Cmd {
	args = strings.TrimSpace(args)
	if args == "" {
		// List available themes.
		var lines []string
		for _, name := range ThemeNames() {
			marker := "  "
			if name == m.palette.Name {
				marker = "> "
			}
			lines = append(lines, marker+name)
		}
		m.output.AppendSystem("Available themes:\n" + strings.Join(lines, "\n"))
		return nil
	}

	p, ok := LookupTheme(args)
	if !ok {
		// Check user-defined themes.
		if m.settings.Themes != nil {
			if userTheme, exists := m.settings.Themes[args]; exists {
				p = PaletteFromUserConfig(args, userTheme)
				ok = true
			}
		}
	}
	if !ok {
		m.output.AppendError(fmt.Sprintf("unknown theme: %s", args))
		return nil
	}

	m.SetTheme(p, m.settings.HighContrast)
	m.output.AppendSystem(fmt.Sprintf("Switched to theme: %s", args))
	m.layoutComponents()
	return nil
}
```

Register in `builtinCommands`:

```go
"theme": cmdTheme,
```

Add to `builtinDescriptions`:

```go
"theme": "List or switch themes",
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/ -run TestCmdTheme -v`
Expected: PASS

**Step 5: Commit**

```
git add internal/tui/builtins.go internal/tui/builtins_test.go
git commit -m "feat: add /theme command for runtime theme switching"
```

---

### Task 7: Update README

**Files:**
- Modify: `README.md`

**Step 1: Add theme documentation**

Add a `### Themes` section under Configuration:

```markdown
### Themes

Glamdring ships with five LOTR-inspired themes. Set in `settings.json`:

\`\`\`json
{
  "theme": "glamdring",
  "high_contrast": false
}
\`\`\`

| Theme | Inspiration | Feel |
|---|---|---|
| `glamdring` (default) | Elvish blade glow | Cool blue, precise |
| `rivendell` | Twilight valley | Teal, ethereal |
| `mithril` | Dwarven forge | Cyan, industrial |
| `lothlorien` | Forest starlight | Cool gold, celestial |
| `shire` | Warm amber | Candlelit, cozy |

Switch at runtime with `/theme`:

| Command | Description |
|---|---|
| `/theme` | List available themes |
| `/theme <name>` | Switch to a theme |

Enable high contrast mode for accessibility:

\`\`\`json
{
  "high_contrast": true
}
\`\`\`

Define custom themes:

\`\`\`json
{
  "theme": "my-custom",
  "themes": {
    "my-custom": {
      "bg": "#1a1a1f", "fg": "#b0b8c4", "fg_dim": "#5a6270", "fg_bright": "#e0e4ea",
      "primary": "#ff6600", "secondary": "#cc9900", "success": "#00cc66",
      "error": "#cc3333", "info": "#3399cc", "subtle": "#9966cc",
      "surface0": "#202028", "surface1": "#2a2a34", "surface2": "#363640"
    }
  }
}
\`\`\`
```

**Step 2: Commit**

```
git add README.md
git commit -m "docs: add theme system documentation"
```

---

### Task 8: Final integration test and cleanup

**Step 1: Run full test suite**

Run: `go test ./... -race`
Expected: PASS

**Step 2: Build and manual smoke test**

Run: `go build -o /tmp/glamdring-test ./cmd/glamdring`

Test manually:
- Launch, verify default glamdring theme (blue accents)
- `/theme` lists all themes
- `/theme shire` switches to warm amber
- `/theme mithril` switches to cyan
- Verify theme persists across agent turns

**Step 3: Commit any fixups, push branch, create PR**
