package imapgw

import (
	"context"
	"sync"
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

	broker.Publish(context.Background(), MailboxEvent{Type: MailboxEventExists, UserID: "user-1", MailboxID: "archive", Messages: 1})
	broker.Publish(context.Background(), MailboxEvent{Type: MailboxEventExists, UserID: "user-2", MailboxID: "inbox", Messages: 9})
	broker.Publish(context.Background(), MailboxEvent{Type: MailboxEventExists, UserID: "user-1", MailboxID: "inbox", Messages: 2})

	select {
	case got := <-events:
		if got.UserID != "user-1" || got.MailboxID != "inbox" || got.Messages != 2 {
			t.Fatalf("event = %#v, want inbox exists event", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for mailbox event")
	}
}

func TestMailboxEventBrokerNormalizesMailboxEventIdentity(t *testing.T) {
	t.Parallel()

	broker := NewMailboxEventBroker(1)
	events, cancel, err := broker.Subscribe(context.Background(), " user-1 ", " inbox ")
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	defer cancel()

	if err := broker.Publish(context.Background(), MailboxEvent{Type: MailboxEventExists, UserID: " user-1 ", MailboxID: " inbox ", Messages: 3}); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	select {
	case got := <-events:
		if got.UserID != "user-1" || got.MailboxID != "inbox" || got.Messages != 3 {
			t.Fatalf("event = %#v, want normalized inbox exists event", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for normalized mailbox event")
	}
}

func TestMailboxEventBrokerNormalizesMailboxEventType(t *testing.T) {
	t.Parallel()

	broker := NewMailboxEventBroker(1)
	events, cancel, err := broker.Subscribe(context.Background(), "user-1", "inbox")
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	defer cancel()

	if err := broker.Publish(context.Background(), MailboxEvent{Type: " exists ", UserID: "user-1", MailboxID: "inbox", Messages: 3}); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	select {
	case got := <-events:
		if got.Type != MailboxEventExists || got.Messages != 3 {
			t.Fatalf("event = %#v, want normalized exists event", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for normalized event type")
	}

	if err := broker.Publish(context.Background(), MailboxEvent{Type: "unknown", UserID: "user-1", MailboxID: "inbox", Messages: 4}); err == nil {
		t.Fatal("Publish accepted unsupported event type")
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

func TestMailboxEventBrokerPublishDoesNotBlockOnSlowSubscriber(t *testing.T) {
	t.Parallel()

	broker := NewMailboxEventBroker(0)
	events, cancel, err := broker.Subscribe(context.Background(), "user-1", "inbox")
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- broker.Publish(context.Background(), MailboxEvent{Type: MailboxEventExists, UserID: "user-1", MailboxID: "inbox", Messages: 1})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Publish returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on an unread unbuffered subscriber")
	}

	select {
	case got := <-events:
		t.Fatalf("slow subscriber unexpectedly received non-blocking event: %#v", got)
	default:
	}
	if got := broker.DroppedEvents(); got != 1 {
		t.Fatalf("DroppedEvents = %d, want 1", got)
	}
	if got := broker.DroppedEventsFor(" user-1 ", " inbox "); got != 1 {
		t.Fatalf("DroppedEventsFor = %d, want 1", got)
	}
	if got := broker.DroppedEventsFor("user-1", "archive"); got != 0 {
		t.Fatalf("DroppedEventsFor archive = %d, want 0", got)
	}
}

func TestMailboxEventBrokerContextCancellationRemovesSubscription(t *testing.T) {
	t.Parallel()

	broker := NewMailboxEventBroker(1)
	ctx, cancelContext := context.WithCancel(context.Background())
	events, cancelSubscription, err := broker.Subscribe(ctx, "user-1", "inbox")
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	defer cancelSubscription()

	cancelContext()
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("subscription channel is still open after context cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscription close")
	}

	if err := broker.Publish(context.Background(), MailboxEvent{Type: MailboxEventExists, UserID: "user-1", MailboxID: "inbox", Messages: 1}); err != nil {
		t.Fatalf("Publish after cancellation returned error: %v", err)
	}
}

func TestMailboxEventBrokerConcurrentPublishCancelDoesNotPanic(t *testing.T) {
	t.Parallel()

	broker := NewMailboxEventBroker(1)
	for i := 0; i < 1000; i++ {
		_, cancel, err := broker.Subscribe(context.Background(), "user-1", "inbox")
		if err != nil {
			t.Fatalf("Subscribe returned error: %v", err)
		}

		var wg sync.WaitGroup
		wg.Add(2)
		start := make(chan struct{})
		go func() {
			defer wg.Done()
			<-start
			_ = broker.Publish(context.Background(), MailboxEvent{Type: MailboxEventExists, UserID: "user-1", MailboxID: "inbox", Messages: 1})
		}()
		go func() {
			defer wg.Done()
			<-start
			cancel()
		}()
		close(start)
		wg.Wait()
	}
}

func TestMailboxEventBrokerRejectsCanceledPublishContext(t *testing.T) {
	t.Parallel()

	broker := NewMailboxEventBroker(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := broker.Publish(ctx, MailboxEvent{Type: MailboxEventExists, UserID: "user-1", MailboxID: "inbox", Messages: 1}); err == nil {
		t.Fatal("Publish accepted canceled context")
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
