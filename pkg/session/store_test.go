package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/justin/glamdring/pkg/api"
)

func TestAppendAndLoadMessages(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	meta, err := store.NewSession()
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	msgs := []api.RequestMessage{
		{Role: "user", Content: "hello world"},
		{Role: "assistant", Content: "hi there"},
	}
	if err := store.AppendMessages(meta.ID, msgs); err != nil {
		t.Fatalf("AppendMessages: %v", err)
	}

	loaded, err := store.LoadMessages(meta.ID)
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(loaded))
	}
	if loaded[0].Role != "user" {
		t.Errorf("expected role=user, got %q", loaded[0].Role)
	}
	if loaded[1].Role != "assistant" {
		t.Errorf("expected role=assistant, got %q", loaded[1].Role)
	}
}

func TestAtomicIndexRewrite(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	sessions := []SessionMeta{
		{ID: "abc", Title: "Test Session"},
	}
	if err := store.writeIndex(sessions); err != nil {
		t.Fatalf("writeIndex: %v", err)
	}

	// The tmp file should not exist after a successful write
	tmpPath := filepath.Join(dir, ".index.tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("expected .index.tmp to be gone after writeIndex, got err: %v", err)
	}

	// The index file should exist
	if _, err := os.Stat(store.indexPath()); err != nil {
		t.Errorf("expected index.json to exist: %v", err)
	}
}

func TestLoadMessages_MissingFile(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	msgs, err := store.LoadMessages("nonexistent-id")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if msgs == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(msgs))
	}
}

func TestRebuildIndex(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Create two session files manually
	for _, id := range []string{"session-one", "session-two"} {
		msgs := []api.RequestMessage{
			{Role: "user", Content: "message for " + id},
		}
		if err := store.AppendMessages(id, msgs); err != nil {
			t.Fatalf("AppendMessages: %v", err)
		}
	}

	count, err := store.RebuildIndex()
	if err != nil {
		t.Fatalf("RebuildIndex: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 sessions, got %d", count)
	}

	sessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions in index, got %d", len(sessions))
	}
}

func TestDeleteSession(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	meta, err := store.NewSession()
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	msgs := []api.RequestMessage{{Role: "user", Content: "test"}}
	if err := store.AppendMessages(meta.ID, msgs); err != nil {
		t.Fatalf("AppendMessages: %v", err)
	}
	if err := store.CloseSession(meta.ID, "test", 1); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	if err := store.DeleteSession(meta.ID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	// File should be gone
	if _, err := os.Stat(store.sessionPath(meta.ID)); !os.IsNotExist(err) {
		t.Error("expected session file to be deleted")
	}

	// Not in index
	sessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	for _, s := range sessions {
		if s.ID == meta.ID {
			t.Error("deleted session still present in index")
		}
	}
}

func TestListSessions_Empty(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	sessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions on empty store: %v", err)
	}
	if sessions == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}
