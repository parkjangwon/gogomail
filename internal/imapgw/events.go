package imapgw

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type MailboxEventBroker struct {
	mu          sync.Mutex
	nextID      uint64
	dropped     uint64
	bufferDepth int
	subscribers map[uint64]mailboxEventSubscriber
}

type mailboxEventSubscriber struct {
	userID    UserID
	mailboxID MailboxID
	events    chan MailboxEvent
}

func NewMailboxEventBroker(bufferDepth int) *MailboxEventBroker {
	if bufferDepth < 0 {
		bufferDepth = 0
	}
	return &MailboxEventBroker{
		bufferDepth: bufferDepth,
		subscribers: make(map[uint64]mailboxEventSubscriber),
	}
}

func (b *MailboxEventBroker) Subscribe(ctx context.Context, userID UserID, mailboxID MailboxID) (<-chan MailboxEvent, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	userID = UserID(strings.TrimSpace(string(userID)))
	if userID == "" {
		return nil, nil, fmt.Errorf("user_id is required")
	}
	mailboxID = MailboxID(strings.TrimSpace(string(mailboxID)))
	if mailboxID == "" {
		return nil, nil, fmt.Errorf("mailbox_id is required")
	}

	b.mu.Lock()
	b.nextID++
	id := b.nextID
	events := make(chan MailboxEvent, b.bufferDepth)
	b.subscribers[id] = mailboxEventSubscriber{
		userID:    userID,
		mailboxID: mailboxID,
		events:    events,
	}
	b.mu.Unlock()

	done := make(chan struct{})
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			b.mu.Lock()
			sub, ok := b.subscribers[id]
			if ok {
				delete(b.subscribers, id)
			}
			b.mu.Unlock()
			if ok {
				close(sub.events)
			}
			close(done)
		})
	}

	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-done:
		}
	}()

	return events, cancel, nil
}

func (b *MailboxEventBroker) Publish(ctx context.Context, event MailboxEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	event.MailboxID = MailboxID(strings.TrimSpace(string(event.MailboxID)))
	if event.MailboxID == "" {
		return fmt.Errorf("mailbox_id is required")
	}
	event.UserID = UserID(strings.TrimSpace(string(event.UserID)))
	if event.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	var err error
	event.Type, err = normalizeMailboxEventType(event.Type)
	if err != nil {
		return err
	}

	b.mu.Lock()
	for _, sub := range b.subscribers {
		if sub.userID == event.UserID && sub.mailboxID == event.MailboxID {
			select {
			case sub.events <- event:
			default:
				b.dropped++
			}
		}
	}
	b.mu.Unlock()
	return nil
}

func (b *MailboxEventBroker) DroppedEvents() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.dropped
}

func normalizeMailboxEventType(eventType MailboxEventType) (MailboxEventType, error) {
	eventType = MailboxEventType(strings.TrimSpace(string(eventType)))
	switch eventType {
	case MailboxEventExists, MailboxEventExpunge, MailboxEventFlags:
		return eventType, nil
	case "":
		return "", fmt.Errorf("event type is required")
	default:
		return "", fmt.Errorf("unsupported event type %q", eventType)
	}
}
