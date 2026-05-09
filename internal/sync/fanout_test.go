package sync

import (
	"sync"
	"testing"
	"time"
)

func TestFanOutWatchAndNotify(t *testing.T) {
	fo := NewFanOut()
	ch := fo.Watch("inbox")

	event := Event{MailboxID: "inbox", Type: "messageAdded", Version: 10}
	fo.Notify("inbox", event)

	select {
	case got := <-ch:
		if got.Type != "messageAdded" {
			t.Fatalf("unexpected event type: %s", got.Type)
		}
		if got.Version != 10 {
			t.Fatalf("unexpected version: %d", got.Version)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestFanOutMultipleWatchers(t *testing.T) {
	fo := NewFanOut()
	ch1 := fo.Watch("inbox")
	ch2 := fo.Watch("inbox")

	fo.Notify("inbox", Event{MailboxID: "inbox", Type: "messageAdded", Version: 5})

	for _, ch := range []chan Event{ch1, ch2} {
		select {
		case got := <-ch:
			if got.Version != 5 {
				t.Fatalf("unexpected version: %d", got.Version)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for event")
		}
	}
}

func TestFanOutDifferentMailboxes(t *testing.T) {
	fo := NewFanOut()
	inboxCh := fo.Watch("inbox")
	sentCh := fo.Watch("sent")

	fo.Notify("inbox", Event{MailboxID: "inbox", Type: "messageAdded", Version: 1})

	select {
	case <-inboxCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for inbox event")
	}

	select {
	case <-sentCh:
		t.Fatal("unexpected event on sent mailbox")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestFanOutUnwatch(t *testing.T) {
	fo := NewFanOut()
	ch := fo.Watch("inbox")
	fo.Unwatch("inbox", ch)

	fo.Notify("inbox", Event{MailboxID: "inbox", Type: "messageAdded", Version: 1})

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("unexpected event after unwatch")
		}
	case <-time.After(100 * time.Millisecond):
	}
}

// TestFanOutRaceCondition reproduces the channel-close race: multiple goroutines
// call Notify while Unwatch closes the channel concurrently.
// Run with: go test -race ./internal/sync/... -run TestFanOutRaceCondition
func TestFanOutRaceCondition(t *testing.T) {
	const goroutines = 50

	for iter := 0; iter < 100; iter++ {
		fo := NewFanOut()
		ch := fo.Watch("inbox")

		var wg sync.WaitGroup
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				fo.Notify("inbox", Event{MailboxID: "inbox", Type: "add", Version: int64(i)})
			}(i)
		}

		// Unwatch concurrently with the notifiers.
		wg.Add(1)
		go func() {
			defer wg.Done()
			fo.Unwatch("inbox", ch)
		}()

		wg.Wait()
		// Drain any remaining events (channel is closed after Unwatch).
		for range ch {
		}
	}
}

// TestFanOutConcurrentWatchers stresses concurrent Watch/Unwatch/Notify.
func TestFanOutConcurrentWatchers(t *testing.T) {
	fo := NewFanOut()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := fo.Watch("inbox")
			fo.Notify("inbox", Event{MailboxID: "inbox", Type: "add", Version: 1})
			fo.Unwatch("inbox", ch)
			// drain
			for range ch {
			}
		}()
	}
	wg.Wait()
}
