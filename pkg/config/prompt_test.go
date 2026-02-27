package config

import (
	"strings"
	"testing"
)

func TestDefaultBaseInstructions(t *testing.T) {
	s := DefaultBaseInstructions()
	if s == "" {
		t.Fatal("DefaultBaseInstructions returned empty string")
	}
}

func TestBuildSystemPrompt_AllSections(t *testing.T) {
	tools := []ToolDescription{
		{Name: "Read", Description: "Reads a file."},
		{Name: "Write", Description: "Writes a file."},
	}

	result := BuildSystemPrompt("Base instructions.", tools, "Project rules.", "User prefs.")

	// Check ordering: base, tools, user, project.
	baseIdx := strings.Index(result, "Base instructions.")
	toolsIdx := strings.Index(result, "## Available Tools")
	userIdx := strings.Index(result, "## User Instructions")
	projIdx := strings.Index(result, "## Project Instructions")

	if baseIdx < 0 || toolsIdx < 0 || userIdx < 0 || projIdx < 0 {
		t.Fatalf("missing sections in prompt:\n%s", result)
	}

	if !(baseIdx < toolsIdx && toolsIdx < userIdx && userIdx < projIdx) {
		t.Errorf("sections out of order: base=%d tools=%d user=%d project=%d",
			baseIdx, toolsIdx, userIdx, projIdx)
	}

	// Tool names appear.
	if !strings.Contains(result, "### Read") {
		t.Error("missing Read tool description")
	}
	if !strings.Contains(result, "### Write") {
		t.Error("missing Write tool description")
	}
}

func TestBuildSystemPrompt_NoOptionalSections(t *testing.T) {
	result := BuildSystemPrompt("Base.", nil, "", "")

	if !strings.HasPrefix(result, "Base.") {
		t.Errorf("expected prompt to start with base instructions, got %q", result[:20])
	}
	if strings.Contains(result, "## Available Tools") {
		t.Error("should not include tools section when no tools provided")
	}
	if strings.Contains(result, "## User Instructions") {
		t.Error("should not include user section when empty")
	}
	if strings.Contains(result, "## Project Instructions") {
		t.Error("should not include project section when empty")
	}
}

func TestBuildSystemPrompt_OnlyProjectCLAUDE(t *testing.T) {
	result := BuildSystemPrompt("Base.", nil, "Project only.", "")

	if !strings.Contains(result, "## Project Instructions") {
		t.Error("missing project section")
	}
	if strings.Contains(result, "## User Instructions") {
		t.Error("should not include user section when empty")
	}
}
