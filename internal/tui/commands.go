package tui

import "strings"

// SlashCommandState tracks tab-completion state for slash commands.
type SlashCommandState struct {
	// available holds the full list of known command names (without the leading /).
	available []string

	// matches holds the subset of available commands matching the current prefix.
	matches []string

	// matchIndex tracks which match we're currently showing for tab cycling.
	matchIndex int

	// prefix is the partial command text (after /) when tab completion started.
	prefix string
}

// NewSlashCommandState creates a new slash command state with no commands.
func NewSlashCommandState() SlashCommandState {
	return SlashCommandState{}
}

// SetCommands sets the list of available slash command names (without leading /).
func (s *SlashCommandState) SetCommands(names []string) {
	s.available = make([]string, len(names))
	copy(s.available, names)
	s.resetMatches()
}

// IsSlashCommand returns true if the given input text starts with /.
func IsSlashCommand(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "/")
}

// CommandName extracts the command name from slash command input.
// For "/review auth.go" it returns "review".
// Returns empty string if not a slash command.
func CommandName(text string) string {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "/") {
		return ""
	}
	// Strip the leading /
	rest := trimmed[1:]
	// Take the first word as the command name.
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// CommandArgs extracts everything after the command name.
// For "/review auth.go" it returns "auth.go".
func CommandArgs(text string) string {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "/") {
		return ""
	}
	rest := trimmed[1:]
	parts := strings.Fields(rest)
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[1:], " ")
}

// TabComplete attempts to complete or cycle through matching command names.
// It takes the current input text and returns the completed text, or the
// original text if no match is found.
func (s *SlashCommandState) TabComplete(text string) string {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "/") {
		return text
	}

	// Extract the partial command (everything after / up to first space).
	rest := trimmed[1:]
	parts := strings.Fields(rest)

	// If there's already a space after the command name, don't complete.
	if len(parts) > 1 || (len(rest) > 0 && rest[len(rest)-1] == ' ') {
		return text
	}

	partial := ""
	if len(parts) > 0 {
		partial = parts[0]
	}

	// If the prefix changed, recompute matches.
	if partial != s.prefix {
		s.prefix = partial
		s.recomputeMatches()
	} else {
		// Same prefix, cycle to next match.
		if len(s.matches) > 0 {
			s.matchIndex = (s.matchIndex + 1) % len(s.matches)
		}
	}

	if len(s.matches) == 0 {
		return text
	}

	return "/" + s.matches[s.matchIndex] + " "
}

// resetMatches clears the completion state.
func (s *SlashCommandState) resetMatches() {
	s.matches = nil
	s.matchIndex = 0
	s.prefix = ""
}

// recomputeMatches finds all commands matching the current prefix.
func (s *SlashCommandState) recomputeMatches() {
	s.matches = nil
	s.matchIndex = 0

	for _, name := range s.available {
		if strings.HasPrefix(name, s.prefix) {
			s.matches = append(s.matches, name)
		}
	}
}
