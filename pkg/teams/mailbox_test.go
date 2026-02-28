package teams

import (
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

			msg := AgentMessage{Kind: kind, From: "alice", To: "bob", Content: "urgent"}
			tr.Send(msg)

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

	// Should be a no-op, not an error.
	err := tr.Send(AgentMessage{Kind: MessageKindDM, From: "alice", To: "nobody", Content: "hello"})
	if err != nil {
		t.Errorf("expected no error for non-existent target, got: %v", err)
	}
}

func TestChannelTransport_UnsubscribeNonExistent(t *testing.T) {
	tr := NewChannelTransport()
	// Should not panic.
	tr.Unsubscribe("nobody")
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
