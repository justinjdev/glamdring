package teams

import "testing"

func TestMemberRegistry_AddAndGet(t *testing.T) {
	r := NewInMemoryMemberRegistry()

	m := Member{Name: "alice", AgentType: "coder", Status: MemberStatusIdle}
	if err := r.Add(m); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := r.Get("alice")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "alice" {
		t.Errorf("expected name alice, got %q", got.Name)
	}
	if got.AgentType != "coder" {
		t.Errorf("expected agent_type coder, got %q", got.AgentType)
	}
	if got.Status != MemberStatusIdle {
		t.Errorf("expected status idle, got %q", got.Status)
	}
}

func TestMemberRegistry_GetReturnsCopy(t *testing.T) {
	r := NewInMemoryMemberRegistry()
	_ = r.Add(Member{Name: "alice", Status: MemberStatusIdle})

	got, _ := r.Get("alice")
	got.Status = MemberStatusShutdown

	// Original should be unchanged.
	original, _ := r.Get("alice")
	if original.Status != MemberStatusIdle {
		t.Error("Get should return a copy; modifying it should not affect stored member")
	}
}

func TestMemberRegistry_AddDuplicate(t *testing.T) {
	r := NewInMemoryMemberRegistry()
	_ = r.Add(Member{Name: "alice", Status: MemberStatusIdle})

	err := r.Add(Member{Name: "alice", Status: MemberStatusActive})
	if err == nil {
		t.Fatal("expected error for duplicate add")
	}
}

func TestMemberRegistry_Remove(t *testing.T) {
	r := NewInMemoryMemberRegistry()
	_ = r.Add(Member{Name: "alice", Status: MemberStatusIdle})

	if err := r.Remove("alice"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	_, err := r.Get("alice")
	if err == nil {
		t.Error("expected error after removal")
	}
}

func TestMemberRegistry_RemoveNonExistent(t *testing.T) {
	r := NewInMemoryMemberRegistry()
	err := r.Remove("nobody")
	if err == nil {
		t.Fatal("expected error for removing non-existent member")
	}
}

func TestMemberRegistry_GetNonExistent(t *testing.T) {
	r := NewInMemoryMemberRegistry()
	_, err := r.Get("nobody")
	if err == nil {
		t.Fatal("expected error for non-existent member")
	}
}

func TestMemberRegistry_SetStatus(t *testing.T) {
	r := NewInMemoryMemberRegistry()
	_ = r.Add(Member{Name: "alice", Status: MemberStatusIdle})

	if err := r.SetStatus("alice", MemberStatusActive); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	got, _ := r.Get("alice")
	if got.Status != MemberStatusActive {
		t.Errorf("expected active, got %q", got.Status)
	}
}

func TestMemberRegistry_SetStatusNonExistent(t *testing.T) {
	r := NewInMemoryMemberRegistry()
	err := r.SetStatus("nobody", MemberStatusActive)
	if err == nil {
		t.Fatal("expected error for non-existent member")
	}
}

func TestMemberRegistry_ListAlphabetical(t *testing.T) {
	r := NewInMemoryMemberRegistry()
	_ = r.Add(Member{Name: "charlie", Status: MemberStatusIdle})
	_ = r.Add(Member{Name: "alice", Status: MemberStatusIdle})
	_ = r.Add(Member{Name: "bob", Status: MemberStatusIdle})

	list := r.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 members, got %d", len(list))
	}
	expected := []string{"alice", "bob", "charlie"}
	for i, m := range list {
		if m.Name != expected[i] {
			t.Errorf("position %d: expected %q, got %q", i, expected[i], m.Name)
		}
	}
}

func TestMemberRegistry_Count(t *testing.T) {
	r := NewInMemoryMemberRegistry()
	if r.Count() != 0 {
		t.Errorf("expected 0, got %d", r.Count())
	}

	_ = r.Add(Member{Name: "alice", Status: MemberStatusIdle})
	_ = r.Add(Member{Name: "bob", Status: MemberStatusIdle})
	if r.Count() != 2 {
		t.Errorf("expected 2, got %d", r.Count())
	}
}

func TestMemberRegistry_ActiveCount(t *testing.T) {
	r := NewInMemoryMemberRegistry()
	_ = r.Add(Member{Name: "alice", Status: MemberStatusActive})
	_ = r.Add(Member{Name: "bob", Status: MemberStatusIdle})
	_ = r.Add(Member{Name: "charlie", Status: MemberStatusShutdown})

	if got := r.ActiveCount(); got != 2 {
		t.Errorf("expected 2 active, got %d", got)
	}
}
