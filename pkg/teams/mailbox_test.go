package teams

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestChannelTransport_SubscribeAndUnsubscribe(t *testing.T) {
	tr := NewChannelTransport()

	reg, pri, err := tr.Subscribe("alice", 10)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if reg == nil || pri == nil {
		t.Fatal("expected non-nil channels")
	}

	tr.Unsubscribe("alice")

	// Channels should be closed after unsubscribe.
	select {
	case _, ok := <-reg:
		if ok {
			t.Error("expected regular channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("regular channel not closed")
	}
}

func TestChannelTransport_SubscribeDuplicate(t *testing.T) {
	tr := NewChannelTransport()
	tr.Subscribe("alice", 10)

	_, _, err := tr.Subscribe("alice", 10)
	if err == nil {
		t.Fatal("expected error for duplicate subscribe")
	}
}

func TestChannelTransport_DMRouting(t *testing.T) {
	tr := NewChannelTransport()
	regA, _, _ := tr.Subscribe("alice", 10)
	regB, _, _ := tr.Subscribe("bob", 10)

	msg := AgentMessage{Kind: MessageKindDM, From: "alice", To: "bob", Content: "hello"}
	if err := tr.Send(msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Bob should receive the message.
	select {
	case got := <-regB:
		if got.Content != "hello" {
			t.Errorf("expected 'hello', got %q", got.Content)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("bob did not receive message")
	}

	// Alice should NOT receive the message.
	select {
	case <-regA:
		t.Error("alice should not receive her own DM to bob")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestChannelTransport_BroadcastRouting(t *testing.T) {
	tr := NewChannelTransport()
	regA, _, _ := tr.Subscribe("alice", 10)
	regB, _, _ := tr.Subscribe("bob", 10)
	regC, _, _ := tr.Subscribe("charlie", 10)

	msg := AgentMessage{Kind: MessageKindBroadcast, From: "alice", Content: "attention"}
	if err := tr.Send(msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Bob and Charlie should receive it.
	for _, ch := range []<-chan AgentMessage{regB, regC} {
		select {
		case got := <-ch:
			if got.Content != "attention" {
				t.Errorf("expected 'attention', got %q", got.Content)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("did not receive broadcast")
		}
	}

	// Alice (sender) should NOT receive.
	select {
	case <-regA:
		t.Error("sender should not receive their own broadcast")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestChannelTransport_PriorityRouting(t *testing.T) {
	priorityKinds := []MessageKind{
		MessageKindShutdownRequest,
		MessageKindShutdownResponse,
		MessageKindApprovalRequest,
		MessageKindApprovalResponse,
	}

	for _, kind := range priorityKinds {
		t.Run(string(kind), func(t *testing.T) {
			tr := NewChannelTransport()
			reg, pri, _ := tr.Subscribe("bob", 10)

			approve := true
			msg := AgentMessage{Kind: kind, From: "alice", To: "bob", Content: "urgent", RequestID: "req-1", Approve: &approve}
			if err := tr.Send(msg); err != nil {
				t.Fatalf("Send failed: %v", err)
			}

			// Should be on priority channel.
			select {
			case got := <-pri:
				if got.Content != "urgent" {
					t.Errorf("expected 'urgent', got %q", got.Content)
				}
			case <-time.After(100 * time.Millisecond):
				t.Fatal("priority message not received")
			}

			// Should NOT be on regular channel.
			select {
			case <-reg:
				t.Error("priority message should not be on regular channel")
			case <-time.After(50 * time.Millisecond):
				// expected
			}
		})
	}
}

func TestChannelTransport_RegularRouting(t *testing.T) {
	tr := NewChannelTransport()
	reg, pri, _ := tr.Subscribe("bob", 10)

	msg := AgentMessage{Kind: MessageKindDM, From: "alice", To: "bob", Content: "hey"}
	tr.Send(msg)

	select {
	case got := <-reg:
		if got.Content != "hey" {
			t.Errorf("expected 'hey', got %q", got.Content)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("regular message not received")
	}

	select {
	case <-pri:
		t.Error("regular message should not be on priority channel")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestChannelTransport_SendToNonExistent(t *testing.T) {
	tr := NewChannelTransport()

	err := tr.Send(AgentMessage{Kind: MessageKindDM, From: "alice", To: "nobody", Content: "hello"})
	if err == nil {
		t.Error("expected error for non-existent target")
	}
}

func TestChannelTransport_SendDropsWhenFull(t *testing.T) {
	tr := NewChannelTransport()
	reg, _, _ := tr.Subscribe("bob", 1) // buffer size 1

	// First message should be delivered.
	msg1 := AgentMessage{Kind: MessageKindDM, From: "alice", To: "bob", Content: "first"}
	if err := tr.Send(msg1); err != nil {
		t.Fatalf("first send failed: %v", err)
	}

	// Second message should be dropped (buffer full) and return an error.
	msg2 := AgentMessage{Kind: MessageKindDM, From: "alice", To: "bob", Content: "second"}
	if err := tr.Send(msg2); err == nil {
		t.Fatal("expected error for dropped DM, got nil")
	}

	// Only the first message should be in the channel.
	got := <-reg
	if got.Content != "first" {
		t.Errorf("expected 'first', got %q", got.Content)
	}

	// Channel should be empty now.
	select {
	case <-reg:
		t.Error("expected no more messages, but got one")
	default:
		// expected
	}

	// After draining, a new message should be deliverable (recovery).
	msg3 := AgentMessage{Kind: MessageKindDM, From: "alice", To: "bob", Content: "third"}
	if err := tr.Send(msg3); err != nil {
		t.Fatalf("third send failed: %v", err)
	}
	got = <-reg
	if got.Content != "third" {
		t.Errorf("expected 'third', got %q", got.Content)
	}
}

func TestChannelTransport_UnsubscribeNonExistent(t *testing.T) {
	tr := NewChannelTransport()
	// Should not panic.
	tr.Unsubscribe("nobody")
}

func TestChannelTransport_Concurrent(t *testing.T) {
	tr := NewChannelTransport()
	const n = 10

	// Subscribe agents.
	for i := range n {
		name := fmt.Sprintf("agent-%d", i)
		_, _, err := tr.Subscribe(name, 32)
		if err != nil {
			t.Fatalf("Subscribe %s: %v", name, err)
		}
	}

	var wg sync.WaitGroup

	// Concurrent sends.
	wg.Add(n * n)
	for i := range n {
		from := fmt.Sprintf("agent-%d", i)
		for j := range n {
			if i == j {
				wg.Done()
				continue
			}
			go func(from, to string) {
				defer wg.Done()
				tr.Send(AgentMessage{Kind: MessageKindDM, From: from, To: to, Content: "hello"})
			}(from, fmt.Sprintf("agent-%d", j))
		}
	}
	wg.Wait()

	// Concurrent unsubscribes.
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			tr.Unsubscribe(fmt.Sprintf("agent-%d", i))
		}(i)
	}
	wg.Wait()
}

func TestChannelTransport_MessageTimestamps(t *testing.T) {
	tr := NewChannelTransport()
	reg, _, _ := tr.Subscribe("bob", 10)

	before := time.Now()
	tr.Send(AgentMessage{Kind: MessageKindDM, From: "alice", To: "bob", Content: "hello"})
	after := time.Now()

	msg := <-reg
	if msg.Timestamp.Before(before) || msg.Timestamp.After(after) {
		t.Errorf("timestamp %v outside expected range [%v, %v]", msg.Timestamp, before, after)
	}
	if msg.SeqNum < 1 {
		t.Errorf("expected SeqNum >= 1, got %d", msg.SeqNum)
	}
}

func TestChannelTransport_MessageOrdering(t *testing.T) {
	tr := NewChannelTransport()
	reg, _, _ := tr.Subscribe("bob", 10)

	for i := 0; i < 5; i++ {
		tr.Send(AgentMessage{Kind: MessageKindDM, From: "alice", To: "bob", Content: fmt.Sprintf("msg-%d", i)})
	}

	var prevSeq int
	for i := 0; i < 5; i++ {
		msg := <-reg
		if msg.SeqNum <= prevSeq {
			t.Errorf("message %d: SeqNum %d not monotonically increasing (prev: %d)", i, msg.SeqNum, prevSeq)
		}
		prevSeq = msg.SeqNum
	}
}

func TestChannelTransport_UnsubscribeClosesChannels(t *testing.T) {
	tr := NewChannelTransport()
	reg, pri, _ := tr.Subscribe("alice", 10)

	tr.Unsubscribe("alice")

	// Both channels should be closed.
	if _, ok := <-reg; ok {
		t.Error("regular channel should be closed")
	}
	if _, ok := <-pri; ok {
		t.Error("priority channel should be closed")
	}
}
