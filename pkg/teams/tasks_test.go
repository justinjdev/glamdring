package teams

import (
	"strings"
	"sync"
	"testing"
)

func TestFileTaskStorage_CreateAndGet(t *testing.T) {
	dir := t.TempDir()
	s, err := NewFileTaskStorage(dir)
	if err != nil {
		t.Fatalf("NewFileTaskStorage: %v", err)
	}

	task, err := s.Create(Task{Subject: "test task", Status: TaskStatusPending})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task.ID == "" {
		t.Error("expected non-empty ID")
	}
	if task.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	got, err := s.Get(task.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Subject != "test task" {
		t.Errorf("expected subject 'test task', got %q", got.Subject)
	}
}

func TestFileTaskStorage_GetNonExistent(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	_, err := s.Get("999")
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

func TestFileTaskStorage_NextIDUniqueness(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	ids := make(map[string]bool)
	for range 10 {
		id := s.NextID()
		if ids[id] {
			t.Fatalf("duplicate ID: %s", id)
		}
		ids[id] = true
	}
}

func TestFileTaskStorage_NextIDResumesAfterReopen(t *testing.T) {
	dir := t.TempDir()
	s1, _ := NewFileTaskStorage(dir)

	// Create a few tasks to advance the counter.
	s1.Create(Task{Subject: "t1", Status: TaskStatusPending})
	s1.Create(Task{Subject: "t2", Status: TaskStatusPending})

	// Reopen the storage.
	s2, err := NewFileTaskStorage(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}

	id := s2.NextID()
	if id == "1" || id == "2" {
		t.Errorf("expected ID > 2 after reopen, got %s", id)
	}
}

func TestFileTaskStorage_Update(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	task, _ := s.Create(Task{Subject: "original", Status: TaskStatusPending})

	newSubject := "updated"
	newStatus := TaskStatusInProgress
	updated, err := s.Update(task.ID, TaskUpdate{
		Subject: &newSubject,
		Status:  &newStatus,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Subject != "updated" {
		t.Errorf("expected subject 'updated', got %q", updated.Subject)
	}
	if updated.Status != TaskStatusInProgress {
		t.Errorf("expected status in_progress, got %q", updated.Status)
	}
	if !updated.UpdatedAt.After(updated.CreatedAt) {
		t.Error("expected UpdatedAt to be after CreatedAt")
	}
}

func TestFileTaskStorage_UpdateCASSuccess(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	owner := "alice"
	task, _ := s.Create(Task{Subject: "task", Status: TaskStatusPending, Owner: "alice"})

	newOwner := "bob"
	_, err := s.Update(task.ID, TaskUpdate{
		Owner:         &newOwner,
		ExpectedOwner: &owner,
	})
	if err != nil {
		t.Fatalf("CAS update should succeed: %v", err)
	}

	got, _ := s.Get(task.ID)
	if got.Owner != "bob" {
		t.Errorf("expected owner bob, got %q", got.Owner)
	}
}

func TestFileTaskStorage_UpdateCASConflict(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	task, _ := s.Create(Task{Subject: "task", Status: TaskStatusPending, Owner: "alice"})

	newOwner := "charlie"
	expectedOwner := "bob"
	_, err := s.Update(task.ID, TaskUpdate{
		Owner:         &newOwner,
		ExpectedOwner: &expectedOwner,
	})
	if err == nil {
		t.Fatal("expected CAS conflict error")
	}
	if !strings.Contains(err.Error(), "ownership conflict") {
		t.Errorf("expected ownership conflict error, got: %v", err)
	}
}

func TestFileTaskStorage_UpdateCASSkippedWhenNilExpected(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	task, _ := s.Create(Task{Subject: "task", Status: TaskStatusPending, Owner: "alice"})

	newOwner := "bob"
	_, err := s.Update(task.ID, TaskUpdate{
		Owner: &newOwner,
		// ExpectedOwner is nil, so CAS check is skipped.
	})
	if err != nil {
		t.Fatalf("update without ExpectedOwner should succeed: %v", err)
	}
}

func TestFileTaskStorage_CompletionClearsBlockedBy(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	t1, _ := s.Create(Task{Subject: "task 1", Status: TaskStatusPending})
	t2, _ := s.Create(Task{Subject: "task 2", Status: TaskStatusPending, BlockedBy: []string{t1.ID}})
	t3, _ := s.Create(Task{Subject: "task 3", Status: TaskStatusPending, BlockedBy: []string{t1.ID, "other"}})

	completed := TaskStatusCompleted
	_, err := s.Update(t1.ID, TaskUpdate{Status: &completed})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	// t2 should no longer be blocked by t1.
	got2, _ := s.Get(t2.ID)
	if len(got2.BlockedBy) != 0 {
		t.Errorf("expected t2 BlockedBy to be empty, got %v", got2.BlockedBy)
	}

	// t3 should still be blocked by "other" but not t1.
	got3, _ := s.Get(t3.ID)
	if len(got3.BlockedBy) != 1 || got3.BlockedBy[0] != "other" {
		t.Errorf("expected t3 BlockedBy [other], got %v", got3.BlockedBy)
	}
}

func TestFileTaskStorage_List(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	s.Create(Task{Subject: "task A", Status: TaskStatusPending})
	s.Create(Task{Subject: "task B", Status: TaskStatusInProgress})

	list := s.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(list))
	}
}

func TestFileTaskStorage_ListExcludesDeleted(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	s.Create(Task{Subject: "active", Status: TaskStatusPending})
	deleted, _ := s.Create(Task{Subject: "deleted", Status: TaskStatusPending})

	deletedStatus := TaskStatusDeleted
	s.Update(deleted.ID, TaskUpdate{Status: &deletedStatus})

	list := s.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 task after delete, got %d", len(list))
	}
	if list[0].Subject != "active" {
		t.Errorf("expected subject 'active', got %q", list[0].Subject)
	}
}

func TestFileTaskStorage_Delete(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	task, _ := s.Create(Task{Subject: "doomed", Status: TaskStatusPending})
	if err := s.Delete(task.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := s.Get(task.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestFileTaskStorage_DeleteNonExistent(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	err := s.Delete("999")
	if err == nil {
		t.Fatal("expected error for non-existent delete")
	}
}

func TestFileTaskStorage_UpdateAddBlocks(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	task, _ := s.Create(Task{Subject: "task", Status: TaskStatusPending})

	_, err := s.Update(task.ID, TaskUpdate{
		AddBlocks:    []string{"3", "4"},
		AddBlockedBy: []string{"1"},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := s.Get(task.ID)
	if len(got.Blocks) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(got.Blocks))
	}
	if len(got.BlockedBy) != 1 {
		t.Errorf("expected 1 blocked_by, got %d", len(got.BlockedBy))
	}
}

func TestFileTaskStorage_UpdateNonExistent(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	_, err := s.Update("999", TaskUpdate{})
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

func TestFileTaskStorage_Concurrent(t *testing.T) {
	dir := t.TempDir()
	s, err := NewFileTaskStorage(dir)
	if err != nil {
		t.Fatalf("NewFileTaskStorage: %v", err)
	}

	const n = 20
	var wg sync.WaitGroup

	// Concurrent creates.
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			s.Create(Task{Subject: "concurrent task", Status: TaskStatusPending})
		}()
	}
	wg.Wait()

	list := s.List()
	if len(list) != n {
		t.Errorf("expected %d tasks, got %d", n, len(list))
	}

	// Concurrent reads and updates.
	wg.Add(n * 2)
	for _, summary := range list {
		go func(id string) {
			defer wg.Done()
			s.Get(id)
		}(summary.ID)
		go func(id string) {
			defer wg.Done()
			newSubject := "updated"
			s.Update(id, TaskUpdate{Subject: &newSubject})
		}(summary.ID)
	}
	wg.Wait()
}
