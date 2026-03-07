package agents

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/justin/glamdring/pkg/config"
)

// AgentDefinition describes a custom agent loaded from a definition file.
type AgentDefinition struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Prompt      string   `json:"prompt"`
	Tools       []string `json:"tools"`
}

// Discover scans agent directories for agent definition files.
// Checks .glamdring/agents/ then .claude/agents/ at both project and user levels.
// Supports .md (with frontmatter) and .yaml/.yml files.
// Project agents take precedence over user agents with the same name.
func Discover(cwd string) []AgentDefinition {
	var projectDir, userDir string

	if projectRoot := config.FindProjectRoot(cwd); projectRoot != "" {
		projectDir = config.ResolveDir(projectRoot, "agents")
	}

	if userCfg := config.UserConfigDir(); userCfg != "" {
		candidate := filepath.Join(userCfg, "agents")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			userDir = candidate
		}
	}

	return discover(projectDir, userDir)
}

// discover scans the given project and user agent directories.
// Either directory may be empty to skip scanning that level.
func discover(projectDir, userDir string) []AgentDefinition {
	seen := make(map[string]bool)
	var agents []AgentDefinition

	// Project-level agents first (they take precedence).
	if projectDir != "" {
		for _, a := range scanAgentDir(projectDir) {
			seen[a.Name] = true
			agents = append(agents, a)
		}
	}

	// User-level agents.
	if userDir != "" {
		for _, a := range scanAgentDir(userDir) {
			if !seen[a.Name] {
				agents = append(agents, a)
			}
		}
	}

	return agents
}

// scanAgentDir walks a directory for agent definition files.
func scanAgentDir(dir string) []AgentDefinition {
	var agents []AgentDefinition

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		path := filepath.Join(dir, name)

		switch {
		case strings.HasSuffix(name, ".md"):
			if a, ok := parseMarkdownAgent(path, strings.TrimSuffix(name, ".md")); ok {
				agents = append(agents, a)
			}
		case strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml"):
			ext := filepath.Ext(name)
			if a, ok := parseYAMLAgent(path, strings.TrimSuffix(name, ext)); ok {
				agents = append(agents, a)
			}
		}
	}

	return agents
}

// parseMarkdownAgent parses an agent definition from a markdown file with
// YAML frontmatter. The frontmatter (between --- delimiters) contains
// key-value pairs; the body after frontmatter is the prompt.
func parseMarkdownAgent(path string, fallbackName string) (AgentDefinition, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return AgentDefinition{}, false
	}

	content := string(data)
	a := AgentDefinition{Name: fallbackName}

	// Split on frontmatter delimiters.
	if strings.HasPrefix(content, "---") {
		rest := content[3:]
		endIdx := strings.Index(rest, "---")
		if endIdx >= 0 {
			frontmatter := strings.TrimSpace(rest[:endIdx])
			body := strings.TrimSpace(rest[endIdx+3:])
			parseKeyValues(frontmatter, &a)
			a.Prompt = body
		} else {
			// No closing delimiter; treat entire content as prompt.
			a.Prompt = strings.TrimSpace(content)
		}
	} else {
		// No frontmatter; entire file is the prompt.
		a.Prompt = strings.TrimSpace(content)
	}

	return a, true
}

// parseYAMLAgent parses an agent definition from a simple YAML file.
// Handles key: value pairs and a tools list in "- item" format.
func parseYAMLAgent(path string, fallbackName string) (AgentDefinition, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return AgentDefinition{}, false
	}

	a := AgentDefinition{Name: fallbackName}
	parseKeyValues(string(data), &a)
	return a, true
}

// parseKeyValues parses simple key: value lines and populates an
// AgentDefinition. Supports a multi-line "tools:" block with "- item"
// entries, and a multi-line "prompt:" block using "prompt: |" syntax.
func parseKeyValues(text string, a *AgentDefinition) {
	lines := strings.Split(text, "\n")
	inTools := false
	inPrompt := false
	var promptLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// If we're collecting prompt lines (from "prompt: |")
		if inPrompt {
			// A new top-level key ends the prompt block.
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && strings.Contains(line, ":") {
				a.Prompt = strings.TrimSpace(strings.Join(promptLines, "\n"))
				inPrompt = false
				// Fall through to process this line normally.
			} else {
				promptLines = append(promptLines, line)
				continue
			}
		}

		// If we're collecting tool list items.
		if inTools {
			if strings.HasPrefix(trimmed, "- ") {
				tool := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
				a.Tools = append(a.Tools, tool)
				continue
			}
			inTools = false
			// Fall through to process as a normal line.
		}

		if trimmed == "" {
			continue
		}

		colonIdx := strings.Index(trimmed, ":")
		if colonIdx < 0 {
			continue
		}

		key := strings.TrimSpace(trimmed[:colonIdx])
		value := strings.TrimSpace(trimmed[colonIdx+1:])

		switch key {
		case "name":
			if value != "" {
				a.Name = value
			}
		case "description":
			a.Description = value
		case "prompt":
			if value == "|" {
				// Multi-line prompt block.
				inPrompt = true
				promptLines = nil
			} else {
				a.Prompt = value
			}
		case "tools":
			// Could be inline [A, B, C] or start of a list block.
			if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
				// Inline list: [Read, Glob, Grep]
				inner := value[1 : len(value)-1]
				for _, item := range strings.Split(inner, ",") {
					item = strings.TrimSpace(item)
					if item != "" {
						a.Tools = append(a.Tools, item)
					}
				}
			} else if value == "" {
				// Block list follows.
				inTools = true
			}
		}
	}

	// Close any open prompt block.
	if inPrompt && len(promptLines) > 0 {
		a.Prompt = strings.TrimSpace(strings.Join(promptLines, "\n"))
	}
}
