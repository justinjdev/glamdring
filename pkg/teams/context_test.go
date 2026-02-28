package teams

import "testing"

func TestContextCache_StoreAndLoad(t *testing.T) {
	c := NewInMemoryContextCache()

	c.Store("key1", "value1")

	val, ok := c.Load("key1")
	if !ok {
		t.Fatal("expected key1 to be found")
	}
	if val != "value1" {
		t.Errorf("expected 'value1', got %q", val)
	}
}

func TestContextCache_LoadNonExistent(t *testing.T) {
	c := NewInMemoryContextCache()

	_, ok := c.Load("missing")
	if ok {
		t.Error("expected false for non-existent key")
	}
}

func TestContextCache_Delete(t *testing.T) {
	c := NewInMemoryContextCache()

	c.Store("key1", "value1")
	c.Delete("key1")

	_, ok := c.Load("key1")
	if ok {
		t.Error("expected key1 to be deleted")
	}
}

func TestContextCache_Overwrite(t *testing.T) {
	c := NewInMemoryContextCache()

	c.Store("key1", "old")
	c.Store("key1", "new")

	val, ok := c.Load("key1")
	if !ok {
		t.Fatal("expected key1 to be found")
	}
	if val != "new" {
		t.Errorf("expected 'new', got %q", val)
	}
}

func TestContextCache_DeleteNonExistent(t *testing.T) {
	c := NewInMemoryContextCache()
	// Should not panic.
	c.Delete("missing")
}
