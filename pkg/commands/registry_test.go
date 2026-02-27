package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewRegistry_Get(t *testing.T) {
	reg := NewRegistry([]Command{
		{Name: "review", Path: "/tmp/review.md", Source: "project"},
		{Name: "deploy", Path: "/tmp/deploy.md", Source: "user"},
	})

	cmd, ok := reg.Get("review")
	if !ok {
		t.Fatal("expected to find 'review'")
	}
	if cmd.Source != "project" {
		t.Errorf("source: got %q, want %q", cmd.Source, "project")
	}

	_, ok = reg.Get("nonexistent")
	if ok {
		t.Error("expected 'nonexistent' to not be found")
	}
}

func TestNewRegistry_FirstEntryWins(t *testing.T) {
	reg := NewRegistry([]Command{
		{Name: "review", Path: "/project/review.md", Source: "project"},
		{Name: "review", Path: "/user/review.md", Source: "user"},
	})

	cmd, ok := reg.Get("review")
	if !ok {
		t.Fatal("expected to find 'review'")
	}
	if cmd.Source != "project" {
		t.Errorf("source: got %q, want %q (project should win)", cmd.Source, "project")
	}
}

func TestRegistry_Names(t *testing.T) {
	reg := NewRegistry([]Command{
		{Name: "deploy", Path: "/tmp/deploy.md", Source: "user"},
		{Name: "review", Path: "/tmp/review.md", Source: "project"},
		{Name: "alpha", Path: "/tmp/alpha.md", Source: "project"},
	})

	names := reg.Names()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}

	// Should be sorted.
	want := []string{"alpha", "deploy", "review"}
	for i, name := range names {
		if name != want[i] {
			t.Errorf("names[%d]: got %q, want %q", i, name, want[i])
		}
	}
}

func TestRegistry_Expand(t *testing.T) {
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "review.md")
	if err := os.WriteFile(mdFile, []byte("Review the file: $ARGUMENTS\nBe thorough."), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry([]Command{
		{Name: "review", Path: mdFile, Source: "project"},
	})

	got, err := reg.Expand("review", "auth.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "Review the file: auth.go\nBe thorough."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRegistry_Expand_NoArguments(t *testing.T) {
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "review.md")
	if err := os.WriteFile(mdFile, []byte("Review $ARGUMENTS carefully."), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry([]Command{
		{Name: "review", Path: mdFile, Source: "project"},
	})

	got, err := reg.Expand("review", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "Review  carefully."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRegistry_Expand_UnknownCommand(t *testing.T) {
	reg := NewRegistry(nil)
	_, err := reg.Expand("nonexistent", "")
	if err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestRegistry_Expand_MultipleReplacements(t *testing.T) {
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(mdFile, []byte("First: $ARGUMENTS\nSecond: $ARGUMENTS"), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry([]Command{
		{Name: "test", Path: mdFile, Source: "project"},
	})

	got, err := reg.Expand("test", "value")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "First: value\nSecond: value"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRegistry_Empty(t *testing.T) {
	reg := NewRegistry(nil)
	if names := reg.Names(); len(names) != 0 {
		t.Errorf("expected 0 names, got %d", len(names))
	}
}
