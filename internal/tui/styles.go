package tui

import (
	"sort"

	"github.com/charmbracelet/lipgloss"
)

func init() {
	// Prevent lipgloss from querying the terminal for background color.
	// Without this, the OSC response can leak into the textarea as text.
	lipgloss.SetHasDarkBackground(true)
}

// Styles holds all Lip Gloss styles for the TUI.
type Styles struct {
	// Input area
	InputBorder    lipgloss.Style
	InputPrompt    lipgloss.Style
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

// Color palette — a warm, amber-tinted dark theme inspired by aged parchment
// and lantern light. Think candlelit workspace, not neon arcade.
var (
	// Base tones
	colorBg         = lipgloss.Color("#1a1612")
	colorFg         = lipgloss.Color("#d4be98")
	colorFgDim      = lipgloss.Color("#7c6f64")
	colorFgBright   = lipgloss.Color("#ebdbb2")

	// Accent palette
	colorAmber       = lipgloss.Color("#e78a4e") // primary accent — warm amber
	colorGold        = lipgloss.Color("#d8a657") // secondary — burnished gold
	colorSage        = lipgloss.Color("#a9b665") // success / tool results
	colorRust        = lipgloss.Color("#ea6962") // errors / deny
	colorLavender    = lipgloss.Color("#d3869b") // thinking / subtle highlight
	colorTeal        = lipgloss.Color("#89b482") // permission / approve
	colorSky         = lipgloss.Color("#7daea3") // informational / model name

	// Surface tones
	colorSurface0    = lipgloss.Color("#282420")
	colorSurface1    = lipgloss.Color("#32302f")
	colorSurface2    = lipgloss.Color("#3c3836")
)

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

// DefaultStyles creates the default dark theme styles.
func DefaultStyles() Styles {
	return Styles{
		// Input area — bordered with amber accent
		InputBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAmber).
			Padding(0, 1),

		InputPrompt: lipgloss.NewStyle().
			Foreground(colorAmber).
			Bold(true),

		InputPlaceholder: lipgloss.NewStyle().
			Foreground(colorFgDim).
			Italic(true),

		// Output text
		OutputText: lipgloss.NewStyle().
			Foreground(colorFg),

		// Tool calls — sage green header, dimmer body
		ToolCallHeader: lipgloss.NewStyle().
			Foreground(colorSage).
			Bold(true).
			PaddingLeft(1),

		ToolCallIcon: lipgloss.NewStyle().
			Foreground(colorGold).
			Bold(true),

		ToolResult: lipgloss.NewStyle().
			Foreground(colorFgDim).
			PaddingLeft(3),

		ToolResultErr: lipgloss.NewStyle().
			Foreground(colorRust).
			PaddingLeft(3),

		// Thinking — dimmed lavender, italic
		ThinkingText: lipgloss.NewStyle().
			Foreground(colorLavender).
			Italic(true).
			PaddingLeft(2),

		ThinkingBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colorLavender).
			PaddingLeft(1),

		// Thinking separator — subtle dotted line between thinking and response
		ThinkingSeparator: lipgloss.NewStyle().
			Foreground(colorLavender).
			Faint(true).
			PaddingLeft(2),

		// New content indicator — shown when user has scrolled up
		NewContentIndicator: lipgloss.NewStyle().
			Background(colorAmber).
			Foreground(colorBg).
			Bold(true).
			Padding(0, 1),

		// Status bar — full-width surface strip
		StatusBar: lipgloss.NewStyle().
			Background(colorSurface1).
			Foreground(colorFgDim).
			Padding(0, 1),

		StatusBarKey: lipgloss.NewStyle().
			Background(colorSurface1).
			Foreground(colorFgDim),

		StatusBarValue: lipgloss.NewStyle().
			Background(colorSurface1).
			Foreground(colorFgBright),

		StatusBarSep: lipgloss.NewStyle().
			Background(colorSurface1).
			Foreground(colorSurface2).
			SetString(" \u2502 "),

		// Permission prompt
		PermissionBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorGold).
			Padding(0, 1).
			MarginTop(1),

		PermissionTitle: lipgloss.NewStyle().
			Foreground(colorGold).
			Bold(true),

		PermissionHelp: lipgloss.NewStyle().
			Foreground(colorFgDim).
			Italic(true),

		// Error
		ErrorText: lipgloss.NewStyle().
			Foreground(colorRust).
			Bold(true),

		// System messages (built-in command output)
		SystemText: lipgloss.NewStyle().
			Foreground(colorSky).
			PaddingLeft(1),

		SystemBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colorSky).
			PaddingLeft(1).
			PaddingTop(1),

		// Conversation role headers
		UserHeader: lipgloss.NewStyle().
			Foreground(colorSky).
			Bold(true).
			PaddingTop(1),

		// Spinner indicator
		SpinnerText: lipgloss.NewStyle().
			Foreground(colorAmber).
			PaddingLeft(1),

		// Status bar warning (e.g., YOLO indicator)
		StatusBarWarning: lipgloss.NewStyle().
			Background(colorSurface1).
			Foreground(colorRust).
			Bold(true),

		// Status bar caution (context window 60-79%)
		StatusBarCaution: lipgloss.NewStyle().
			Background(colorSurface1).
			Foreground(colorGold),

		// Status bar danger (context window >= 80%)
		StatusBarDanger: lipgloss.NewStyle().
			Background(colorSurface1).
			Foreground(colorRust).
			Bold(true),
	}
}
