package tui

import "strings"

const historyMaxEntries = 100

// History stores input history in a ring buffer with navigation and search.
type History struct {
	entries []string
	cursor  int    // current position when navigating; -1 means "at draft"
	draft   string // saved draft when user starts navigating
}

// Add appends an entry to the history. Empty strings and consecutive
// duplicates are skipped.
func (h *History) Add(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	// Deduplicate consecutive entries.
	if len(h.entries) > 0 && h.entries[len(h.entries)-1] == text {
		return
	}
	h.entries = append(h.entries, text)
	// Enforce ring buffer size.
	if len(h.entries) > historyMaxEntries {
		h.entries = h.entries[len(h.entries)-historyMaxEntries:]
	}
	h.ResetCursor()
}

// Up moves backward through history. On the first call, currentText is saved
// as the draft. Returns the history entry and true, or empty and false if
// already at the oldest entry.
func (h *History) Up(currentText string) (string, bool) {
	if len(h.entries) == 0 {
		return "", false
	}

	if h.cursor == -1 {
		// First navigation: save draft and start at most recent entry.
		h.draft = currentText
		h.cursor = len(h.entries) - 1
		return h.entries[h.cursor], true
	}

	if h.cursor > 0 {
		h.cursor--
		return h.entries[h.cursor], true
	}

	// Already at oldest entry.
	return h.entries[h.cursor], false
}

// Down moves forward through history. When reaching the end, restores the
// saved draft. Returns the entry/draft and true, or empty and false if
// not currently navigating.
func (h *History) Down() (string, bool) {
	if h.cursor == -1 {
		return "", false
	}

	if h.cursor < len(h.entries)-1 {
		h.cursor++
		return h.entries[h.cursor], true
	}

	// At the end: restore draft.
	draft := h.draft
	h.ResetCursor()
	return draft, true
}

// Search returns entries matching the query (case-insensitive substring),
// newest first.
func (h *History) Search(query string) []string {
	if query == "" {
		return nil
	}
	lower := strings.ToLower(query)
	var results []string
	for i := len(h.entries) - 1; i >= 0; i-- {
		if strings.Contains(strings.ToLower(h.entries[i]), lower) {
			results = append(results, h.entries[i])
		}
	}
	return results
}

// ResetCursor resets navigation state, clearing the saved draft.
func (h *History) ResetCursor() {
	h.cursor = -1
	h.draft = ""
}

// Len returns the number of entries in the history.
func (h *History) Len() int {
	return len(h.entries)
}
