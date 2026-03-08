package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// MenuItemKind describes the type of a menu item.
type MenuItemKind int

const (
	// MenuSection is a non-selectable section header.
	MenuSection MenuItemKind = iota
	// MenuChoice opens an inline sub-list of options.
	MenuChoice
	// MenuToggle flips a boolean value on enter.
	MenuToggle
)

// MenuItem represents a single entry in the modal menu.
type MenuItem struct {
	Kind    MenuItemKind
	Label   string
	Value   string   // displayed value (e.g. "glamdring", "on")
	Active  bool     // current state for toggles
	Choices []string // available options for MenuChoice
	// ID identifies this item for change callbacks. Defaults to Label if empty.
	ID string
}

// ModalChange describes a setting that was changed via the modal.
type ModalChange struct {
	ID    string // item ID
	Value string // new value
}

// ModalModel manages an interactive modal overlay.
type ModalModel struct {
	title   string
	items   []MenuItem
	cursor  int
	palette ThemePalette

	// expanded tracks which MenuChoice item is showing its sub-list.
	// -1 means none expanded.
	expanded int
	// subCursor is the cursor within the expanded choice list.
	subCursor int
}

// NewModal creates a modal with the given title and items.
func NewModal(title string, items []MenuItem, palette ThemePalette) *ModalModel {
	m := &ModalModel{
		title:    title,
		items:    items,
		palette:  palette,
		expanded: -1,
	}
	// Advance cursor to first selectable item.
	m.skipToSelectable(1)
	return m
}

// FocusItem moves the cursor to the first item with the given label.
func (m *ModalModel) FocusItem(label string) {
	for i, item := range m.items {
		if item.Label == label && item.Kind != MenuSection {
			m.cursor = i
			m.expanded = -1
			return
		}
	}
}

// HandleKey processes a key press. Returns whether the modal should close
// and any setting change that was made.
func (m *ModalModel) HandleKey(key string) (close bool, change *ModalChange) {
	switch key {
	case "esc":
		if m.expanded >= 0 {
			m.expanded = -1
			return false, nil
		}
		return true, nil

	case "up", "k":
		if m.expanded >= 0 {
			if m.subCursor > 0 {
				m.subCursor--
			}
			return false, nil
		}
		m.cursor--
		m.skipToSelectable(-1)
		return false, nil

	case "down", "j":
		if m.expanded >= 0 {
			item := m.items[m.expanded]
			if m.subCursor < len(item.Choices)-1 {
				m.subCursor++
			}
			return false, nil
		}
		m.cursor++
		m.skipToSelectable(1)
		return false, nil

	case "enter":
		if m.cursor < 0 || m.cursor >= len(m.items) {
			return false, nil
		}
		item := &m.items[m.cursor]
		id := item.ID
		if id == "" {
			id = item.Label
		}

		switch item.Kind {
		case MenuToggle:
			item.Active = !item.Active
			if item.Active {
				item.Value = "on"
			} else {
				item.Value = "off"
			}
			return false, &ModalChange{ID: id, Value: item.Value}

		case MenuChoice:
			if m.expanded == m.cursor {
				// Select the current sub-choice.
				if len(item.Choices) == 0 {
					m.expanded = -1
					return false, nil
				}
				choice := item.Choices[m.subCursor]
				item.Value = choice
				m.expanded = -1
				return false, &ModalChange{ID: id, Value: choice}
			}
			// Expand this choice list.
			m.expanded = m.cursor
			m.subCursor = 0
			// Pre-select current value.
			for i, c := range item.Choices {
				if c == item.Value {
					m.subCursor = i
					break
				}
			}
			return false, nil
		}
		return false, nil
	}
	return false, nil
}

// skipToSelectable moves cursor in direction (1 or -1) to next non-section item.
// It is a no-op if no selectable items exist.
func (m *ModalModel) skipToSelectable(dir int) {
	// Guard: if no selectable items exist, leave cursor as-is.
	hasSelectable := false
	for _, item := range m.items {
		if item.Kind != MenuSection {
			hasSelectable = true
			break
		}
	}
	if !hasSelectable {
		return
	}

	for m.cursor >= 0 && m.cursor < len(m.items) && m.items[m.cursor].Kind == MenuSection {
		m.cursor += dir
	}
	if m.cursor < 0 {
		m.cursor = 0
		m.skipToSelectable(1)
	}
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
		m.skipToSelectable(-1)
	}
}

// View renders the modal content (without the overlay border -- that's added by the caller).
func (m *ModalModel) View(maxWidth int) string {
	p := m.palette

	titleStyle := lipgloss.NewStyle().
		Foreground(p.Primary).
		Bold(true)

	sectionStyle := lipgloss.NewStyle().
		Foreground(p.FgDim).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(p.Fg)

	selectedStyle := lipgloss.NewStyle().
		Foreground(p.FgBright).
		Background(p.Surface1).
		Bold(true)

	valueStyle := lipgloss.NewStyle().
		Foreground(p.FgDim)

	activeValueStyle := lipgloss.NewStyle().
		Foreground(p.Success)

	choiceStyle := lipgloss.NewStyle().
		Foreground(p.FgDim).
		PaddingLeft(4)

	choiceSelectedStyle := lipgloss.NewStyle().
		Foreground(p.FgBright).
		Background(p.Surface1).
		Bold(true).
		PaddingLeft(4)

	helpStyle := lipgloss.NewStyle().
		Foreground(p.Subtle)

	// Content width inside border+padding.
	contentWidth := maxWidth - 4
	if contentWidth < 0 {
		contentWidth = 0
	}

	var lines []string

	// Title.
	lines = append(lines, titleStyle.Render(m.title))
	lines = append(lines, sectionStyle.Render(strings.Repeat("\u2500", contentWidth)))

	for i, item := range m.items {
		isCursor := i == m.cursor

		switch item.Kind {
		case MenuSection:
			if i > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, sectionStyle.Render(item.Label))

		case MenuChoice, MenuToggle:
			label := item.Label
			val := item.Value

			padding := contentWidth - lipgloss.Width(label) - lipgloss.Width(val) - 2
			if padding < 2 {
				padding = 2
			}

			if isCursor {
				// Highlight full width.
				line := "  " + label + strings.Repeat(" ", padding) + val
				if lipgloss.Width(line) < contentWidth {
					line += strings.Repeat(" ", contentWidth-lipgloss.Width(line))
				}
				lines = append(lines, selectedStyle.Render(line))
			} else {
				// Style value separately for toggles.
				valRendered := valueStyle.Render(val)
				if item.Kind == MenuToggle && item.Active {
					valRendered = activeValueStyle.Render(val)
				}
				line := normalStyle.Render("  "+label+strings.Repeat(" ", padding)) + valRendered
				lines = append(lines, line)
			}

			// Show expanded choices.
			if item.Kind == MenuChoice && m.expanded == i {
				for j, choice := range item.Choices {
					marker := "  "
					if choice == item.Value && j != m.subCursor {
						marker = "* "
					}
					cLine := marker + choice
					if lipgloss.Width(cLine) < contentWidth-4 {
						cLine += strings.Repeat(" ", contentWidth-4-lipgloss.Width(cLine))
					}
					if j == m.subCursor {
						pad := contentWidth - 4 - lipgloss.Width("> "+choice)
						if pad < 0 {
							pad = 0
						}
						lines = append(lines, choiceSelectedStyle.Render("> "+choice+strings.Repeat(" ", pad)))
					} else {
						lines = append(lines, choiceStyle.Render(cLine))
					}
				}
			}
		}
	}

	// Help text.
	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("  esc close  enter select/toggle  arrows navigate"))

	return strings.Join(lines, "\n")
}

// OverlayView renders the full modal as a bordered box.
func (m *ModalModel) OverlayView(maxWidth, maxHeight int) string {
	p := m.palette

	// Modal takes at most 60% of terminal width, min 40 cols.
	modalWidth := maxWidth * 3 / 5
	if modalWidth > 60 {
		modalWidth = 60
	}
	if modalWidth < 40 {
		modalWidth = 40
	}
	if modalWidth > maxWidth-4 {
		modalWidth = maxWidth - 4
	}

	content := m.View(modalWidth)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Primary).
		Background(p.Bg).
		Foreground(p.Fg).
		Padding(1, 1).
		Width(modalWidth).
		Render(content)

	return box
}

// RenderOverlay composites a modal box centered over a base view.
func RenderOverlay(base, modal string, termWidth, termHeight int) string {
	baseLines := strings.Split(base, "\n")
	modalLines := strings.Split(modal, "\n")

	// Pad base to fill terminal.
	for len(baseLines) < termHeight {
		baseLines = append(baseLines, "")
	}

	modalH := len(modalLines)
	modalW := 0
	for _, l := range modalLines {
		if w := lipgloss.Width(l); w > modalW {
			modalW = w
		}
	}

	// Center vertically and horizontally.
	startY := (termHeight - modalH) / 2
	if startY < 0 {
		startY = 0
	}
	startX := (termWidth - modalW) / 2
	if startX < 0 {
		startX = 0
	}

	// Replace lines.
	for i, mLine := range modalLines {
		y := startY + i
		if y >= len(baseLines) {
			break
		}
		// Build the new line: left pad + modal line + rest
		leftPad := strings.Repeat(" ", startX)
		baseLines[y] = leftPad + mLine
	}

	return strings.Join(baseLines, "\n")
}
