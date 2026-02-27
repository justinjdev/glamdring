package config

import (
	"fmt"
	"strings"
)

// ToolDescription holds the name and description for a tool, used when
// assembling the system prompt.
type ToolDescription struct {
	Name        string
	Description string
}

// EnvironmentInfo holds runtime environment details for the system prompt.
type EnvironmentInfo struct {
	Platform string // e.g. "darwin"
	Shell    string // e.g. "/bin/zsh"
	CWD      string // working directory
	Date     string // e.g. "2026-02-27"
	Model    string // e.g. "claude-opus-4-6"
}

// DefaultBaseInstructions returns the default base system prompt.
func DefaultBaseInstructions() string {
	return "You are an AI coding assistant. Use the tools available to help the user with software engineering tasks. Be concise and direct."
}

// BuildSystemPrompt assembles the full system prompt from parts.
//
// Sections are included in this order:
//  1. Base instructions (hardcoded agent instructions)
//  2. Environment info (## Environment)
//  3. Tool descriptions (## Available Tools)
//  4. User-level CLAUDE.md (## User Instructions)
//  5. Project-level CLAUDE.md (## Project Instructions)
//
// Project-level comes last so it takes precedence in the prompt.
func BuildSystemPrompt(baseInstructions string, toolDescriptions []ToolDescription, claudeMDProject, claudeMDUser string, env EnvironmentInfo) string {
	var b strings.Builder

	// 1. Base instructions
	b.WriteString(baseInstructions)

	// 2. Environment info
	if env.Platform != "" || env.Shell != "" || env.CWD != "" || env.Date != "" || env.Model != "" {
		b.WriteString("\n\n## Environment\n\n")
		if env.Platform != "" {
			fmt.Fprintf(&b, "- Platform: %s\n", env.Platform)
		}
		if env.Shell != "" {
			fmt.Fprintf(&b, "- Shell: %s\n", env.Shell)
		}
		if env.CWD != "" {
			fmt.Fprintf(&b, "- Working directory: %s\n", env.CWD)
		}
		if env.Date != "" {
			fmt.Fprintf(&b, "- Date: %s\n", env.Date)
		}
		if env.Model != "" {
			fmt.Fprintf(&b, "- Model: %s\n", env.Model)
		}
	}

	// 3. Tool descriptions
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

	// 4. User-level CLAUDE.md
	if claudeMDUser != "" {
		b.WriteString("\n\n## User Instructions\n\n")
		b.WriteString(claudeMDUser)
	}

	// 5. Project-level CLAUDE.md (last = highest precedence)
	if claudeMDProject != "" {
		b.WriteString("\n\n## Project Instructions\n\n")
		b.WriteString(claudeMDProject)
	}

	return b.String()
}
