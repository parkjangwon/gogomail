package imapgw

import (
	"context"
	"testing"
	"time"
)

func TestMailboxEventBrokerDeliversMatchingMailboxEvents(t *testing.T) {
	t.Parallel()

	broker := NewMailboxEventBroker(4)
	events, cancel, err := broker.Subscribe(context.Background(), "user-1", "inbox")
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	defer cancel()

	broker.Publish(context.Background(), MailboxEvent{Type: MailboxEventExists, MailboxID: "archive", Messages: 1})
	broker.Publish(context.Background(), MailboxEvent{Type: MailboxEventExists, MailboxID: "inbox", Messages: 2})

	select {
	case got := <-events:
		if got.MailboxID != "inbox" || got.Messages != 2 {
			t.Fatalf("event = %#v, want inbox exists event", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for mailbox event")
	}
}

func TestMailboxEventBrokerCancelClosesSubscription(t *testing.T) {
	t.Parallel()

	broker := NewMailboxEventBroker(1)
	events, cancel, err := broker.Subscribe(context.Background(), "user-1", "inbox")
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	cancel()

	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("subscription channel is still open after cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscription close")
	}
}

func TestMailboxEventBrokerRejectsBlankSubscription(t *testing.T) {
	t.Parallel()

	broker := NewMailboxEventBroker(1)
	if _, _, err := broker.Subscribe(context.Background(), "", "inbox"); err == nil {
		t.Fatal("Subscribe accepted blank user")
	}
	if err := broker.Publish(context.Background(), MailboxEvent{}); err == nil {
		t.Fatal("Publish accepted blank event")
	}
}
