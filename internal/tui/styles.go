package tui

import "github.com/charmbracelet/lipgloss"

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
	}
}
