package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover_MarkdownAgent(t *testing.T) {
	dir := t.TempDir()

	content := `---
name: code-reviewer
description: Reviews code for issues
tools: [Read, Glob, Grep]
---
You are a code reviewer. Analyze the code carefully.`

	if err := os.WriteFile(filepath.Join(dir, "code-reviewer.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	agents := discover(dir, "")
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}

	a := agents[0]
	if a.Name != "code-reviewer" {
		t.Errorf("name: got %q, want %q", a.Name, "code-reviewer")
	}
	if a.Description != "Reviews code for issues" {
		t.Errorf("description: got %q, want %q", a.Description, "Reviews code for issues")
	}
	if a.Prompt != "You are a code reviewer. Analyze the code carefully." {
		t.Errorf("prompt: got %q", a.Prompt)
	}
	if len(a.Tools) != 3 {
		t.Fatalf("tools: got %d, want 3", len(a.Tools))
	}
	wantTools := []string{"Read", "Glob", "Grep"}
	for i, tool := range a.Tools {
		if tool != wantTools[i] {
			t.Errorf("tools[%d]: got %q, want %q", i, tool, wantTools[i])
		}
	}
}

func TestDiscover_MarkdownNoFrontmatter(t *testing.T) {
	dir := t.TempDir()

	content := "You are a helpful assistant."
	if err := os.WriteFile(filepath.Join(dir, "helper.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	agents := discover(dir, "")
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}

	a := agents[0]
	if a.Name != "helper" {
		t.Errorf("name: got %q, want %q", a.Name, "helper")
	}
	if a.Prompt != "You are a helpful assistant." {
		t.Errorf("prompt: got %q", a.Prompt)
	}
}

func TestDiscover_YAMLAgent(t *testing.T) {
	dir := t.TempDir()

	content := `name: security-auditor
description: Audits code for security issues
prompt: You are a security auditor. Check for vulnerabilities.
tools:
  - Read
  - Grep
  - Bash`

	if err := os.WriteFile(filepath.Join(dir, "security-auditor.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	agents := discover(dir, "")
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}

	a := agents[0]
	if a.Name != "security-auditor" {
		t.Errorf("name: got %q, want %q", a.Name, "security-auditor")
	}
	if a.Description != "Audits code for security issues" {
		t.Errorf("description: got %q", a.Description)
	}
	if a.Prompt != "You are a security auditor. Check for vulnerabilities." {
		t.Errorf("prompt: got %q", a.Prompt)
	}
	if len(a.Tools) != 3 {
		t.Fatalf("tools: got %d, want 3", len(a.Tools))
	}
}

func TestDiscover_YMLExtension(t *testing.T) {
	dir := t.TempDir()

	content := `name: tester
description: Runs tests
prompt: You run tests.
tools: [Bash]`

	if err := os.WriteFile(filepath.Join(dir, "tester.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	agents := discover(dir, "")
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Name != "tester" {
		t.Errorf("name: got %q, want %q", agents[0].Name, "tester")
	}
}

func TestDiscover_NoAgentsDir(t *testing.T) {
	agents := discover("", "")
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

func TestDiscover_NonexistentDir(t *testing.T) {
	agents := discover("/nonexistent/path", "")
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

func TestDiscover_IgnoresOtherFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "agent.md"), []byte("Agent"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("Notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	agents := discover(dir, "")
	if len(agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(agents))
	}
}

func TestDiscover_ProjectOverridesUser(t *testing.T) {
	projDir := t.TempDir()
	userDir := t.TempDir()

	projContent := `---
name: reviewer
description: Project reviewer
---
Project prompt.`
	userContent := `---
name: reviewer
description: User reviewer
---
User prompt.`

	if err := os.WriteFile(filepath.Join(projDir, "reviewer.md"), []byte(projContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "reviewer.md"), []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "writer.md"), []byte("Write things."), 0o644); err != nil {
		t.Fatal(err)
	}

	agents := discover(projDir, userDir)
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}

	byName := make(map[string]AgentDefinition)
	for _, a := range agents {
		byName[a.Name] = a
	}

	reviewer, ok := byName["reviewer"]
	if !ok {
		t.Fatal("expected to find 'reviewer'")
	}
	if reviewer.Description != "Project reviewer" {
		t.Errorf("reviewer description: got %q, want %q (project should win)", reviewer.Description, "Project reviewer")
	}

	_, ok = byName["writer"]
	if !ok {
		t.Fatal("expected to find 'writer' from user level")
	}
}

func TestParseKeyValues_MultilinePrompt(t *testing.T) {
	content := `name: writer
description: Writes content
prompt: |
  You are a writer.
  Write excellent prose.
tools: [Read]`

	var a AgentDefinition
	parseKeyValues(content, &a)

	if a.Name != "writer" {
		t.Errorf("name: got %q, want %q", a.Name, "writer")
	}
	wantPrompt := "You are a writer.\n  Write excellent prose."
	if a.Prompt != wantPrompt {
		t.Errorf("prompt: got %q, want %q", a.Prompt, wantPrompt)
	}
	if len(a.Tools) != 1 || a.Tools[0] != "Read" {
		t.Errorf("tools: got %v, want [Read]", a.Tools)
	}
}

func TestParseKeyValues_InlineToolsList(t *testing.T) {
	var a AgentDefinition
	parseKeyValues("tools: [Read, Glob, Grep]", &a)

	if len(a.Tools) != 3 {
		t.Fatalf("tools: got %d, want 3", len(a.Tools))
	}
	want := []string{"Read", "Glob", "Grep"}
	for i, tool := range a.Tools {
		if tool != want[i] {
			t.Errorf("tools[%d]: got %q, want %q", i, tool, want[i])
		}
	}
}

func TestParseKeyValues_BlockToolsList(t *testing.T) {
	content := `tools:
  - Read
  - Glob
  - Grep`

	var a AgentDefinition
	parseKeyValues(content, &a)

	if len(a.Tools) != 3 {
		t.Fatalf("tools: got %d, want 3", len(a.Tools))
	}
	want := []string{"Read", "Glob", "Grep"}
	for i, tool := range a.Tools {
		if tool != want[i] {
			t.Errorf("tools[%d]: got %q, want %q", i, tool, want[i])
		}
	}
}

func TestMarkdownFrontmatter_NameOverridesFilename(t *testing.T) {
	dir := t.TempDir()

	content := `---
name: custom-name
description: A custom agent
---
Do things.`

	if err := os.WriteFile(filepath.Join(dir, "filename.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	agents := discover(dir, "")
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Name != "custom-name" {
		t.Errorf("name: got %q, want %q (frontmatter name should override filename)", agents[0].Name, "custom-name")
	}
}

// TestDiscover_Integration tests the public Discover function with a real
// temp directory tree containing .claude/agents/.
func TestDiscover_Integration(t *testing.T) {
	root := t.TempDir()
	agentDir := filepath.Join(root, ".claude", "agents")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `---
name: test-agent
description: Test
---
Test prompt.`
	if err := os.WriteFile(filepath.Join(agentDir, "test-agent.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Discover will also pick up user-level agents from ~/.claude/agents/,
	// so we just verify our project agent is present.
	agents := Discover(root)
	found := false
	for _, a := range agents {
		if a.Name == "test-agent" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find project agent 'test-agent' in Discover results")
	}
}
