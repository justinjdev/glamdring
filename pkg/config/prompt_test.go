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

	result := BuildSystemPrompt("Base instructions.", tools, "Project rules.", "User prefs.", EnvironmentInfo{})

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
	result := BuildSystemPrompt("Base.", nil, "", "", EnvironmentInfo{})

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
	result := BuildSystemPrompt("Base.", nil, "Project only.", "", EnvironmentInfo{})

	if !strings.Contains(result, "## Project Instructions") {
		t.Error("missing project section")
	}
	if strings.Contains(result, "## User Instructions") {
		t.Error("should not include user section when empty")
	}
}

func TestBuildSystemPrompt_EnvironmentInfo(t *testing.T) {
	env := EnvironmentInfo{
		Platform: "darwin",
		Shell:    "/bin/zsh",
		CWD:      "/home/user/project",
		Date:     "2026-02-27",
		Model:    "claude-opus-4-6",
	}
	tools := []ToolDescription{
		{Name: "Read", Description: "Reads a file."},
	}

	result := BuildSystemPrompt("Base.", tools, "", "", env)

	// Environment section should exist.
	envIdx := strings.Index(result, "## Environment")
	if envIdx < 0 {
		t.Fatalf("missing Environment section in prompt:\n%s", result)
	}

	// Should appear between base and tools.
	baseIdx := strings.Index(result, "Base.")
	toolsIdx := strings.Index(result, "## Available Tools")
	if !(baseIdx < envIdx && envIdx < toolsIdx) {
		t.Errorf("Environment section not between base and tools: base=%d env=%d tools=%d",
			baseIdx, envIdx, toolsIdx)
	}

	// All fields should appear.
	for _, want := range []string{"darwin", "/bin/zsh", "/home/user/project", "2026-02-27", "claude-opus-4-6"} {
		if !strings.Contains(result, want) {
			t.Errorf("expected %q in environment section", want)
		}
	}
}

func TestBuildSystemPrompt_EmptyEnvironment(t *testing.T) {
	result := BuildSystemPrompt("Base.", nil, "", "", EnvironmentInfo{})
	if strings.Contains(result, "## Environment") {
		t.Error("should not include environment section when all fields empty")
	}
}
