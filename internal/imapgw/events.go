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
	if strings.TrimSpace(string(userID)) == "" {
		return nil, nil, fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(string(mailboxID)) == "" {
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
	if strings.TrimSpace(string(event.MailboxID)) == "" {
		return fmt.Errorf("mailbox_id is required")
	}
	if event.Type == "" {
		return fmt.Errorf("event type is required")
	}

	b.mu.Lock()
	targets := make([]chan MailboxEvent, 0, len(b.subscribers))
	for _, sub := range b.subscribers {
		if sub.mailboxID == event.MailboxID {
			targets = append(targets, sub.events)
		}
	}
	b.mu.Unlock()

	for _, events := range targets {
		select {
		case events <- event:
		default:
		}
	}
	return nil
}
