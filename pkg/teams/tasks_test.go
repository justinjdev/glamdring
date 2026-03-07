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

func TestFileTaskStorage_IDUniqueness(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	ids := make(map[string]bool)
	for range 10 {
		task, err := s.Create(Task{Subject: "task", Status: TaskStatusPending})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if ids[task.ID] {
			t.Fatalf("duplicate ID: %s", task.ID)
		}
		ids[task.ID] = true
	}
}

func TestFileTaskStorage_IDResumesAfterReopen(t *testing.T) {
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

	task, err := s2.Create(Task{Subject: "t3", Status: TaskStatusPending})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if task.ID == "1" || task.ID == "2" {
		t.Errorf("expected ID > 2 after reopen, got %s", task.ID)
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

	inProgress := TaskStatusInProgress
	s.Update(t1.ID, TaskUpdate{Status: &inProgress})
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

func TestFileTaskStorage_RejectClaimOnBlockedTask(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	// Create a task blocked by another.
	t1, _ := s.Create(Task{Subject: "blocker", Status: TaskStatusPending})
	t2, _ := s.Create(Task{Subject: "blocked", Status: TaskStatusPending, BlockedBy: []string{t1.ID}})

	// Attempt to claim the blocked task should fail.
	owner := "alice"
	_, err := s.Update(t2.ID, TaskUpdate{Owner: &owner})
	if err == nil {
		t.Fatal("expected error when claiming blocked task")
	}

	// After unblocking (completing t1), claiming should succeed.
	inProgress := TaskStatusInProgress
	s.Update(t1.ID, TaskUpdate{Status: &inProgress})
	completed := TaskStatusCompleted
	s.Update(t1.ID, TaskUpdate{Status: &completed})

	_, err = s.Update(t2.ID, TaskUpdate{Owner: &owner})
	if err != nil {
		t.Fatalf("expected claim to succeed after unblock: %v", err)
	}
}

func TestFileTaskStorage_AllowClearOwnerOnBlockedTask(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	t1, _ := s.Create(Task{Subject: "blocker", Status: TaskStatusPending})
	t2, _ := s.Create(Task{Subject: "blocked", Status: TaskStatusPending, Owner: "alice", BlockedBy: []string{t1.ID}})

	// Clearing the owner (empty string) on a blocked task should be allowed.
	empty := ""
	_, err := s.Update(t2.ID, TaskUpdate{Owner: &empty})
	if err != nil {
		t.Fatalf("expected clearing owner on blocked task to succeed: %v", err)
	}
}

func TestFileTaskStorage_RejectInvalidStatusTransition(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFileTaskStorage(dir)

	task, _ := s.Create(Task{Subject: "task", Status: TaskStatusPending})

	// pending -> completed is not allowed (must go through in_progress).
	completed := TaskStatusCompleted
	_, err := s.Update(task.ID, TaskUpdate{Status: &completed})
	if err == nil {
		t.Fatal("expected error for invalid transition pending -> completed")
	}
	if !strings.Contains(err.Error(), "invalid status transition") {
		t.Errorf("expected invalid status transition error, got: %v", err)
	}

	// pending -> in_progress is allowed.
	inProgress := TaskStatusInProgress
	_, err = s.Update(task.ID, TaskUpdate{Status: &inProgress})
	if err != nil {
		t.Fatalf("expected pending -> in_progress to succeed: %v", err)
	}

	// in_progress -> completed is allowed.
	_, err = s.Update(task.ID, TaskUpdate{Status: &completed})
	if err != nil {
		t.Fatalf("expected in_progress -> completed to succeed: %v", err)
	}

	// completed -> pending is not allowed.
	pending := TaskStatusPending
	_, err = s.Update(task.ID, TaskUpdate{Status: &pending})
	if err == nil {
		t.Fatal("expected error for invalid transition completed -> pending")
	}

	// completed -> deleted is allowed.
	deleted := TaskStatusDeleted
	_, err = s.Update(task.ID, TaskUpdate{Status: &deleted})
	if err != nil {
		t.Fatalf("expected completed -> deleted to succeed: %v", err)
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
