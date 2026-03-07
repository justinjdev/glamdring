package tui

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/justin/glamdring/pkg/config"
)

func init() {
	// Prevent lipgloss from querying the terminal for background color.
	// Without this, the OSC response can leak into the textarea as text.
	lipgloss.SetHasDarkBackground(true)
}

// Styles holds all Lip Gloss styles for the TUI.
type Styles struct {
	// Input area
	InputBorder      lipgloss.Style
	InputPrompt      lipgloss.Style
	InputPlaceholder lipgloss.Style

	// Output text
	OutputText lipgloss.Style

	// Tool calls
	ToolCallHeader lipgloss.Style
	ToolCallIcon   lipgloss.Style
	ToolResult     lipgloss.Style
	ToolResultErr  lipgloss.Style

	// Thinking
	ThinkingText      lipgloss.Style
	ThinkingBorder    lipgloss.Style
	ThinkingSeparator lipgloss.Style

	// Scroll indicator
	NewContentIndicator lipgloss.Style

	// Status bar
	StatusBar      lipgloss.Style
	StatusBarKey   lipgloss.Style
	StatusBarValue lipgloss.Style
	StatusBarSep   lipgloss.Style

	// Permission prompt
	PermissionBorder lipgloss.Style
	PermissionTitle  lipgloss.Style
	PermissionHelp   lipgloss.Style

	// Error
	ErrorText lipgloss.Style

	// System message (built-in command output)
	SystemText   lipgloss.Style
	SystemBorder lipgloss.Style

	// User message header
	UserHeader lipgloss.Style

	// Spinner
	SpinnerText lipgloss.Style

	// Status bar warning (e.g., YOLO indicator)
	StatusBarWarning lipgloss.Style

	// Status bar caution (context window 60-79%)
	StatusBarCaution lipgloss.Style

	// Status bar danger (context window >= 80%)
	StatusBarDanger lipgloss.Style
}

// ThemePalette defines the named color slots for a theme.
type ThemePalette struct {
	Name     string
	Bg       lipgloss.Color
	Fg       lipgloss.Color
	FgDim    lipgloss.Color
	FgBright lipgloss.Color
	Primary  lipgloss.Color
	Secondary lipgloss.Color
	Success  lipgloss.Color
	Error    lipgloss.Color
	Info     lipgloss.Color
	Subtle   lipgloss.Color
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

// DefaultStyles creates theme styles from the given palette.
func DefaultStyles(p ThemePalette) Styles {
	return Styles{
		// Input area — bordered with primary accent
		InputBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.Primary).
			Padding(0, 1),

		InputPrompt: lipgloss.NewStyle().
			Foreground(p.Primary).
			Bold(true),

		InputPlaceholder: lipgloss.NewStyle().
			Foreground(p.FgDim).
			Italic(true),

		// Output text
		OutputText: lipgloss.NewStyle().
			Foreground(p.Fg),

		// Tool calls — success header, dimmer body
		ToolCallHeader: lipgloss.NewStyle().
			Foreground(p.Success).
			Bold(true).
			PaddingLeft(1),

		ToolCallIcon: lipgloss.NewStyle().
			Foreground(p.Secondary).
			Bold(true),

		ToolResult: lipgloss.NewStyle().
			Foreground(p.FgDim).
			PaddingLeft(3),

		ToolResultErr: lipgloss.NewStyle().
			Foreground(p.Error).
			PaddingLeft(3),

		// Thinking — dimmed subtle, italic
		ThinkingText: lipgloss.NewStyle().
			Foreground(p.Subtle).
			Italic(true).
			PaddingLeft(2),

		ThinkingBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(p.Subtle).
			PaddingLeft(1),

		// Thinking separator — subtle dotted line between thinking and response
		ThinkingSeparator: lipgloss.NewStyle().
			Foreground(p.Subtle).
			Faint(true).
			PaddingLeft(2),

		// New content indicator — shown when user has scrolled up
		NewContentIndicator: lipgloss.NewStyle().
			Background(p.Primary).
			Foreground(p.Bg).
			Bold(true).
			Padding(0, 1),

		// Status bar — full-width surface strip
		StatusBar: lipgloss.NewStyle().
			Background(p.Surface1).
			Foreground(p.FgDim).
			Padding(0, 1),

		StatusBarKey: lipgloss.NewStyle().
			Background(p.Surface1).
			Foreground(p.FgDim),

		StatusBarValue: lipgloss.NewStyle().
			Background(p.Surface1).
			Foreground(p.FgBright),

		StatusBarSep: lipgloss.NewStyle().
			Background(p.Surface1).
			Foreground(p.Surface2).
			SetString(" \u2502 "),

		// Permission prompt
		PermissionBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.Secondary).
			Padding(0, 1).
			MarginTop(1),

		PermissionTitle: lipgloss.NewStyle().
			Foreground(p.Secondary).
			Bold(true),

		PermissionHelp: lipgloss.NewStyle().
			Foreground(p.FgDim).
			Italic(true),

		// Error
		ErrorText: lipgloss.NewStyle().
			Foreground(p.Error).
			Bold(true),

		// System messages (built-in command output)
		SystemText: lipgloss.NewStyle().
			Foreground(p.Info).
			PaddingLeft(1),

		SystemBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(p.Info).
			PaddingLeft(1).
			PaddingTop(1),

		// Conversation role headers
		UserHeader: lipgloss.NewStyle().
			Foreground(p.Info).
			Bold(true).
			PaddingTop(1),

		// Spinner indicator
		SpinnerText: lipgloss.NewStyle().
			Foreground(p.Primary).
			PaddingLeft(1),

		// Status bar warning (e.g., YOLO indicator)
		StatusBarWarning: lipgloss.NewStyle().
			Background(p.Surface1).
			Foreground(p.Error).
			Bold(true),

		// Status bar caution (context window 60-79%)
		StatusBarCaution: lipgloss.NewStyle().
			Background(p.Surface1).
			Foreground(p.Secondary),

		// Status bar danger (context window >= 80%)
		StatusBarDanger: lipgloss.NewStyle().
			Background(p.Surface1).
			Foreground(p.Error).
			Bold(true),
	}
}
