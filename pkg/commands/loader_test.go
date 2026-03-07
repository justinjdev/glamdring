package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover_ProjectLevel(t *testing.T) {
	root := t.TempDir()
	cmdDir := filepath.Join(root, "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "review.md"), []byte("Review $ARGUMENTS"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds := discover(cmdDir, "")
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	if cmds[0].Name != "review" {
		t.Errorf("name: got %q, want %q", cmds[0].Name, "review")
	}
	if cmds[0].Source != "project" {
		t.Errorf("source: got %q, want %q", cmds[0].Source, "project")
	}
}

func TestDiscover_NestedCommand(t *testing.T) {
	root := t.TempDir()
	cmdDir := filepath.Join(root, "commands", "opsx")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "new.md"), []byte("New operation"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds := discover(filepath.Dir(cmdDir), "")
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	if cmds[0].Name != "opsx new" {
		t.Errorf("name: got %q, want %q", cmds[0].Name, "opsx new")
	}
}

func TestDiscover_IgnoresNonMarkdown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "review.md"), []byte("Review"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("Notes"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds := discover(dir, "")
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	if cmds[0].Name != "review" {
		t.Errorf("name: got %q, want %q", cmds[0].Name, "review")
	}
}

func TestDiscover_NoCommandsDir(t *testing.T) {
	cmds := discover("", "")
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands, got %d", len(cmds))
	}
}

func TestDiscover_NonexistentDir(t *testing.T) {
	cmds := discover("/nonexistent/path", "")
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands, got %d", len(cmds))
	}
}

func TestDiscover_MultipleCommands(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"alpha.md", "beta.md", "gamma.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("Content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cmds := discover(dir, "")
	if len(cmds) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(cmds))
	}

	names := make(map[string]bool)
	for _, cmd := range cmds {
		names[cmd.Name] = true
	}
	for _, want := range []string{"alpha", "beta", "gamma"} {
		if !names[want] {
			t.Errorf("missing command %q", want)
		}
	}
}

func TestDiscover_ProjectOverridesUser(t *testing.T) {
	projDir := t.TempDir()
	userDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(projDir, "review.md"), []byte("Project review"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "review.md"), []byte("User review"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "deploy.md"), []byte("User deploy"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds := discover(projDir, userDir)
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}

	byName := make(map[string]Command)
	for _, cmd := range cmds {
		byName[cmd.Name] = cmd
	}

	review, ok := byName["review"]
	if !ok {
		t.Fatal("expected to find 'review'")
	}
	if review.Source != "project" {
		t.Errorf("review source: got %q, want %q (project should take precedence)", review.Source, "project")
	}

	deploy, ok := byName["deploy"]
	if !ok {
		t.Fatal("expected to find 'deploy'")
	}
	if deploy.Source != "user" {
		t.Errorf("deploy source: got %q, want %q", deploy.Source, "user")
	}
}

func TestDiscover_UserLevelOnly(t *testing.T) {
	userDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(userDir, "help.md"), []byte("Help"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds := discover("", userDir)
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	if cmds[0].Source != "user" {
		t.Errorf("source: got %q, want %q", cmds[0].Source, "user")
	}
}

// TestDiscover_Integration tests the public Discover function with a real
// temp directory tree containing .claude/commands/.
func TestDiscover_Integration(t *testing.T) {
	root := t.TempDir()
	cmdDir := filepath.Join(root, ".claude", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "test.md"), []byte("Test"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds := Discover(root)
	found := false
	for _, cmd := range cmds {
		if cmd.Name == "test" && cmd.Source == "project" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find project command 'test' in Discover results")
	}
}

func TestDiscover_GlamdringDir(t *testing.T) {
	root := t.TempDir()
	cmdDir := filepath.Join(root, ".glamdring", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "deploy.md"), []byte("Deploy"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds := Discover(root)
	found := false
	for _, cmd := range cmds {
		if cmd.Name == "deploy" && cmd.Source == "project" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find project command 'deploy' from .glamdring/commands/")
	}
}
