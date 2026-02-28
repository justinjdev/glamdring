package tui

import "testing"

func TestIsSlashCommand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"slash command", "/cmd", true},
		{"slash command with space", "/help args", true},
		{"slash command with leading spaces", "  /help", true},
		{"not a slash command", "not slash", false},
		{"empty string", "", false},
		{"bare slash", "/", true},
		{"slash with spaces only", "/  ", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSlashCommand(tt.input)
			if got != tt.want {
				t.Errorf("IsSlashCommand(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCommandName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple command", "/help", "help"},
		{"command with args", "/help args", "help"},
		{"command with multiple args", "/review auth.go main.go", "review"},
		{"no slash", "help", ""},
		{"bare slash", "/", ""},
		{"slash with space only", "/  ", ""},
		{"leading whitespace", "  /model", "model"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CommandName(tt.input)
			if got != tt.want {
				t.Errorf("CommandName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCommandArgs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"with args", "/review auth.go", "auth.go"},
		{"multiple args", "/review auth.go main.go", "auth.go main.go"},
		{"no args", "/help", ""},
		{"no slash", "help args", ""},
		{"bare slash", "/", ""},
		{"leading whitespace", "  /model claude-sonnet-4-6", "claude-sonnet-4-6"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CommandArgs(tt.input)
			if got != tt.want {
				t.Errorf("CommandArgs(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTabComplete_PartialMatch(t *testing.T) {
	s := NewSlashCommandState()
	s.SetCommands([]string{"help", "history", "quit"})

	result := s.TabComplete("/he")
	if result != "/help " {
		t.Errorf("expected '/help ', got %q", result)
	}
}

func TestTabComplete_FullMatch(t *testing.T) {
	s := NewSlashCommandState()
	s.SetCommands([]string{"help", "quit"})

	result := s.TabComplete("/help")
	if result != "/help " {
		t.Errorf("expected '/help ', got %q", result)
	}
}

func TestTabComplete_NoMatch(t *testing.T) {
	s := NewSlashCommandState()
	s.SetCommands([]string{"help", "quit"})

	result := s.TabComplete("/xyz")
	if result != "/xyz" {
		t.Errorf("expected '/xyz' (unchanged), got %q", result)
	}
}

func TestTabComplete_MultipleMatches_Cycle(t *testing.T) {
	s := NewSlashCommandState()
	s.SetCommands([]string{"help", "history"})

	first := s.TabComplete("/h")
	if first != "/help " && first != "/history " {
		t.Fatalf("expected a match, got %q", first)
	}

	// Tab again with same prefix should cycle.
	second := s.TabComplete("/h")
	if second == first {
		t.Errorf("expected cycling to produce different result, got same %q", second)
	}
}

func TestTabComplete_NotSlashCommand(t *testing.T) {
	s := NewSlashCommandState()
	s.SetCommands([]string{"help"})

	result := s.TabComplete("not a command")
	if result != "not a command" {
		t.Errorf("expected unchanged text, got %q", result)
	}
}

func TestTabComplete_AlreadyHasSpace(t *testing.T) {
	s := NewSlashCommandState()
	s.SetCommands([]string{"help"})

	// Already has args after command -- should not complete.
	result := s.TabComplete("/help args")
	if result != "/help args" {
		t.Errorf("expected unchanged text, got %q", result)
	}
}

func TestTabComplete_EmptyAfterSlash(t *testing.T) {
	s := NewSlashCommandState()
	s.SetCommands([]string{"help", "quit"})

	// "/" with no partial -- prefix starts as "" and stays "",
	// but matches are empty so no completion occurs.
	result := s.TabComplete("/")
	if result != "/" {
		t.Errorf("expected '/' unchanged (no matches computed on same prefix), got %q", result)
	}
}

func TestTabComplete_PrefixChange(t *testing.T) {
	s := NewSlashCommandState()
	s.SetCommands([]string{"help", "history", "quit"})

	// First call with "h" triggers recompute.
	r1 := s.TabComplete("/h")
	if r1 == "/h" {
		t.Error("expected a completion for '/h'")
	}

	// Change prefix to "q" -- should find "quit".
	r2 := s.TabComplete("/q")
	if r2 != "/quit " {
		t.Errorf("expected '/quit ', got %q", r2)
	}
}
