package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestThemePreview renders each theme to stdout for VHS screenshot capture.
// Run a single theme: go test -run TestThemePreview/glamdring -v
// Run all themes:     go test -run TestThemePreview -v
func TestThemePreview(t *testing.T) {
	if os.Getenv("GLAMDRING_PREVIEW") == "" {
		t.Skip("set GLAMDRING_PREVIEW=1 to render theme previews")
	}

	// Force true color output even when piped.
	lipgloss.SetColorProfile(termenv.TrueColor)

	const width = 100

	for _, name := range ThemeNames() {
		t.Run(name, func(t *testing.T) {
			palette := builtinThemes[name]
			styles := DefaultStyles(palette)

			r, _ := glamour.NewTermRenderer(
				glamour.WithStylePath("dark"),
				glamour.WithWordWrap(width-4),
			)

			// Build the output directly (bypass viewport scrolling).
			var parts []string

			// User message header.
			header := styles.UserHeader.Render("-- You ")
			divider := styles.UserHeader.Render(strings.Repeat("--", (width-8)/2))
			parts = append(parts, header+divider)

			// User question.
			userQ, _ := r.Render("How do I switch themes?")
			parts = append(parts, strings.TrimRight(userQ, "\n"))

			// Assistant response.
			resp, _ := r.Render("Use `/theme <name>` to switch. Five built-in themes are available.\n\nEach theme defines **Primary**, **Secondary**, **Success**, and **Error** accent colors.")
			parts = append(parts, strings.TrimRight(resp, "\n"))

			// Tool call.
			icon := styles.ToolCallIcon.Render("\u25b6")
			toolHeader := styles.ToolCallHeader.Render("Read: internal/tui/styles.go")
			parts = append(parts, icon+" "+toolHeader)

			// Tool result.
			result := styles.ToolResult.Render("  type ThemePalette struct { ... }")
			parts = append(parts, result)

			content := strings.Join(parts, "\n")

			// Status bar.
			statusText := fmt.Sprintf(" claude-opus-4-6 | in: 0 | out: 0 | cost: $0.0000 | turn: 0")
			status := styles.StatusBar.Width(width).Render(statusText)

			// Input area.
			promptStyle := lipgloss.NewStyle().Foreground(palette.Primary).Bold(true)
			placeholderStyle := lipgloss.NewStyle().Foreground(palette.FgDim).Italic(true)
			inputBorder := styles.InputBorder.Width(width - 4)
			input := inputBorder.Render(promptStyle.Render("\u276f ") + placeholderStyle.Render("ask glamdring something..."))

			// Compose full view.
			view := content + "\n" + status + "\n" + input

			// Pad each line with background color.
			bgStyle := lipgloss.NewStyle().Background(palette.Bg).Width(width)
			lines := strings.Split(view, "\n")
			for i, line := range lines {
				lines[i] = bgStyle.Render(line)
			}

			fmt.Print(strings.Join(lines, "\n"))
		})
	}
}
