package teams

import (
	"sort"
	"testing"
)

func TestManagerRegistry_CreateAndGet(t *testing.T) {
	reg := NewManagerRegistry()
	dir := t.TempDir()

	cfg := TeamConfig{Name: "alpha"}
	mgr, err := reg.Create(cfg, dir)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
	if mgr.Config.Name != "alpha" {
		t.Errorf("expected config name 'alpha', got %q", mgr.Config.Name)
	}

	got := reg.Get("alpha")
	if got != mgr {
		t.Error("Get returned a different manager instance")
	}
}

func TestManagerRegistry_GetNonExistent(t *testing.T) {
	reg := NewManagerRegistry()
	if reg.Get("nope") != nil {
		t.Error("expected nil for non-existent team")
	}
}

func TestManagerRegistry_DuplicateCreateError(t *testing.T) {
	reg := NewManagerRegistry()
	dir := t.TempDir()

	cfg := TeamConfig{Name: "alpha"}
	_, err := reg.Create(cfg, dir)
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	_, err = reg.Create(cfg, dir)
	if err == nil {
		t.Fatal("expected error for duplicate create")
	}
}

func TestManagerRegistry_Delete(t *testing.T) {
	reg := NewManagerRegistry()
	dir := t.TempDir()

	cfg := TeamConfig{Name: "alpha"}
	reg.Create(cfg, dir)

	if err := reg.Delete("alpha"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if reg.Get("alpha") != nil {
		t.Error("team should be removed after delete")
	}
}

func TestManagerRegistry_DeleteNonExistent(t *testing.T) {
	reg := NewManagerRegistry()
	if err := reg.Delete("nope"); err == nil {
		t.Fatal("expected error for deleting non-existent team")
	}
}

func TestManagerRegistry_DeleteWithActiveMembers(t *testing.T) {
	reg := NewManagerRegistry()
	dir := t.TempDir()

	cfg := TeamConfig{Name: "alpha"}
	mgr, _ := reg.Create(cfg, dir)

	mgr.Members.Add(Member{Name: "alice", Status: MemberStatusActive})

	if err := reg.Delete("alpha"); err == nil {
		t.Fatal("expected error when deleting team with active members")
	}

	// Team should still exist.
	if reg.Get("alpha") == nil {
		t.Error("team should still exist after failed delete")
	}
}

func TestManagerRegistry_List(t *testing.T) {
	reg := NewManagerRegistry()

	names := reg.List()
	if len(names) != 0 {
		t.Errorf("expected empty list, got %v", names)
	}

	reg.Create(TeamConfig{Name: "beta"}, t.TempDir())
	reg.Create(TeamConfig{Name: "alpha"}, t.TempDir())

	names = reg.List()
	sort.Strings(names)
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("expected [alpha beta], got %v", names)
	}
}
