package teams

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type mailbox struct {
	regular  chan AgentMessage
	priority chan AgentMessage
}

// ChannelTransport is a channel-based implementation of MessageTransport.
type ChannelTransport struct {
	mu        sync.RWMutex
	mailboxes map[string]*mailbox
	seqNum    atomic.Int64
}

// NewChannelTransport creates a new ChannelTransport.
func NewChannelTransport() *ChannelTransport {
	return &ChannelTransport{
		mailboxes: make(map[string]*mailbox),
	}
}

// Subscribe creates a mailbox for the named agent with buffered channels.
// Returns the regular and priority channels. Returns an error if already subscribed.
func (t *ChannelTransport) Subscribe(agentName string, bufferSize int) (<-chan AgentMessage, <-chan AgentMessage, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.mailboxes[agentName]; exists {
		return nil, nil, fmt.Errorf("agent %q already subscribed", agentName)
	}

	mb := &mailbox{
		regular:  make(chan AgentMessage, bufferSize),
		priority: make(chan AgentMessage, bufferSize),
	}
	t.mailboxes[agentName] = mb
	return mb.regular, mb.priority, nil
}

// Unsubscribe closes the agent's channels and removes its mailbox.
func (t *ChannelTransport) Unsubscribe(agentName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	mb, exists := t.mailboxes[agentName]
	if !exists {
		return
	}
	close(mb.regular)
	close(mb.priority)
	delete(t.mailboxes, agentName)
}

// Send routes a message to the appropriate mailbox. Priority messages
// (shutdown and approval related) go to the priority channel. Broadcast
// messages are sent to all agents except the sender. Sends are non-blocking;
// if a channel is full the message is dropped and an error is returned for
// direct messages.
func (t *ChannelTransport) Send(msg AgentMessage) error {
	if err := msg.Validate(); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	// Assign monotonic ordering.
	msg.Timestamp = time.Now()
	msg.SeqNum = int(t.seqNum.Add(1))

	t.mu.RLock()
	defer t.mu.RUnlock()

	isPriority := msg.Kind == MessageKindShutdownRequest ||
		msg.Kind == MessageKindShutdownResponse ||
		msg.Kind == MessageKindApprovalRequest ||
		msg.Kind == MessageKindApprovalResponse

	// Broadcast: send to all except sender.
	if msg.To == "" {
		var errs []error
		for name, mb := range t.mailboxes {
			if name == msg.From {
				continue
			}
			if !t.sendNonBlocking(mb, msg, isPriority, name) {
				if isPriority {
					errs = append(errs, fmt.Errorf("dropped priority %s to %q: channel full", msg.Kind, name))
				} else {
					log.Printf("warning: dropped broadcast message (kind=%s) from %q to %q: channel full", msg.Kind, msg.From, name)
				}
			}
		}
		return errors.Join(errs...)
	}

	// Direct message.
	mb, exists := t.mailboxes[msg.To]
	if !exists {
		return fmt.Errorf("recipient %q is not subscribed", msg.To)
	}
	if !t.sendNonBlocking(mb, msg, isPriority, msg.To) {
		return fmt.Errorf("message to %q dropped: channel full", msg.To)
	}
	return nil
}

func (t *ChannelTransport) sendNonBlocking(mb *mailbox, msg AgentMessage, isPriority bool, recipient string) bool {
	if isPriority {
		select {
		case mb.priority <- msg:
			return true
		default:
			log.Printf("warning: dropped priority message (kind=%s) from %q to %q: channel full", msg.Kind, msg.From, recipient)
			return false
		}
	}
	select {
	case mb.regular <- msg:
		return true
	default:
		log.Printf("warning: dropped regular message (kind=%s) from %q to %q: channel full", msg.Kind, msg.From, recipient)
		return false
	}
}
