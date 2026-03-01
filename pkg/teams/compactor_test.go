package teams

import (
	"fmt"
	"testing"
)

func TestNoOpCompactor_ReturnsInput(t *testing.T) {
	c := NoOpCompactor{}
	input := "some conversation history"
	out, err := c.Compact(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != input {
		t.Errorf("expected %q, got %q", input, out)
	}
}

func TestCallbackCompactor_Delegates(t *testing.T) {
	called := false
	c := CallbackCompactor{
		Fn: func(history string) (string, error) {
			called = true
			return "summary of: " + history, nil
		},
	}

	out, err := c.Compact("long history")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected callback to be called")
	}
	if out != "summary of: long history" {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestCallbackCompactor_NilFnReturnsInput(t *testing.T) {
	c := CallbackCompactor{Fn: nil}
	input := "some history"
	out, err := c.Compact(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != input {
		t.Errorf("expected %q, got %q", input, out)
	}
}

func TestCallbackCompactor_ErrorPropagation(t *testing.T) {
	c := CallbackCompactor{
		Fn: func(history string) (string, error) {
			return "", fmt.Errorf("compaction failed")
		},
	}

	_, err := c.Compact("history")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "compaction failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestArchivingCompactor_ArchivesBeforeCompacting(t *testing.T) {
	cache := NewInMemoryContextCache()
	inner := CallbackCompactor{
		Fn: func(history string) (string, error) {
			return "compacted", nil
		},
	}

	c := ArchivingCompactor{Inner: inner, Cache: cache}
	input := "long conversation history"
	out, err := c.Compact(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "compacted" {
		t.Errorf("expected 'compacted', got %q", out)
	}

	// Verify the raw input was archived.
	key := fmt.Sprintf("archive-%d", len(input))
	archived, ok := cache.Load(key)
	if !ok {
		t.Fatal("expected archived entry in cache")
	}
	if archived != input {
		t.Errorf("expected archived value %q, got %q", input, archived)
	}
}

func TestArchivingCompactor_NilInnerReturnsInput(t *testing.T) {
	cache := NewInMemoryContextCache()
	c := ArchivingCompactor{Inner: nil, Cache: cache}

	input := "history"
	out, err := c.Compact(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != input {
		t.Errorf("expected %q, got %q", input, out)
	}
}

func TestArchivingCompactor_NilCacheDoesNotPanic(t *testing.T) {
	inner := NoOpCompactor{}
	c := ArchivingCompactor{Inner: inner, Cache: nil}

	out, err := c.Compact("history")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "history" {
		t.Errorf("expected 'history', got %q", out)
	}
}

func TestArchivingCompactor_PropagatesInnerError(t *testing.T) {
	cache := NewInMemoryContextCache()
	inner := CallbackCompactor{
		Fn: func(history string) (string, error) {
			return "", fmt.Errorf("inner failed")
		},
	}

	c := ArchivingCompactor{Inner: inner, Cache: cache}
	_, err := c.Compact("history")
	if err == nil {
		t.Fatal("expected error")
	}

	// Even on error, the raw input should have been archived.
	key := fmt.Sprintf("archive-%d", len("history"))
	_, ok := cache.Load(key)
	if !ok {
		t.Fatal("expected archived entry even on inner error")
	}
}
