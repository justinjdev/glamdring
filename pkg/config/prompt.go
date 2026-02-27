package config

import (
	"strings"
)

// ToolDescription holds the name and description for a tool, used when
// assembling the system prompt.
type ToolDescription struct {
	Name        string
	Description string
}

// DefaultBaseInstructions returns the default base system prompt.
func DefaultBaseInstructions() string {
	return "You are an AI coding assistant. Use the tools available to help the user with software engineering tasks. Be concise and direct."
}

// BuildSystemPrompt assembles the full system prompt from parts.
//
// Sections are included in this order:
//  1. Base instructions (hardcoded agent instructions)
//  2. Tool descriptions (## Available Tools)
//  3. User-level CLAUDE.md (## User Instructions)
//  4. Project-level CLAUDE.md (## Project Instructions)
//
// Project-level comes last so it takes precedence in the prompt.
func BuildSystemPrompt(baseInstructions string, toolDescriptions []ToolDescription, claudeMDProject, claudeMDUser string) string {
	var b strings.Builder

	// 1. Base instructions
	b.WriteString(baseInstructions)

	// 2. Tool descriptions
	if len(toolDescriptions) > 0 {
		b.WriteString("\n\n## Available Tools\n\n")
		for _, td := range toolDescriptions {
			b.WriteString("### ")
			b.WriteString(td.Name)
			b.WriteString("\n")
			b.WriteString(td.Description)
			b.WriteString("\n\n")
		}
	}

	// 3. User-level CLAUDE.md
	if claudeMDUser != "" {
		b.WriteString("\n\n## User Instructions\n\n")
		b.WriteString(claudeMDUser)
	}

	// 4. Project-level CLAUDE.md (last = highest precedence)
	if claudeMDProject != "" {
		b.WriteString("\n\n## Project Instructions\n\n")
		b.WriteString(claudeMDProject)
	}

	return b.String()
}
