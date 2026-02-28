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
