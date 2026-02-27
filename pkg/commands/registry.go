package commands

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// Registry provides lookup and expansion of slash commands.
type Registry struct {
	commands map[string]Command
}

// NewRegistry creates a Registry from a list of discovered commands.
func NewRegistry(commands []Command) *Registry {
	m := make(map[string]Command, len(commands))
	for _, cmd := range commands {
		// First entry wins (project-level should come before user-level
		// in the input from Discover).
		if _, exists := m[cmd.Name]; !exists {
			m[cmd.Name] = cmd
		}
	}
	return &Registry{commands: m}
}

// Get returns the command with the given name, if it exists.
func (r *Registry) Get(name string) (Command, bool) {
	cmd, ok := r.commands[name]
	return cmd, ok
}

// Names returns a sorted list of all command names (for tab completion).
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.commands))
	for name := range r.commands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Expand reads the command's .md file and replaces $ARGUMENTS with the
// provided arguments string. Returns the expanded prompt content.
func (r *Registry) Expand(name string, arguments string) (string, error) {
	cmd, ok := r.commands[name]
	if !ok {
		return "", fmt.Errorf("unknown command: %s", name)
	}

	data, err := os.ReadFile(cmd.Path)
	if err != nil {
		return "", fmt.Errorf("reading command file %s: %w", cmd.Path, err)
	}

	content := string(data)
	content = strings.ReplaceAll(content, "$ARGUMENTS", arguments)
	return content, nil
}
