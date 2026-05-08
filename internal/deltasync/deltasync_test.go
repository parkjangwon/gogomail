package deltasync

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestCursorStoreSaveAndGet(t *testing.T) {
	store := NewMemoryCursorStore()
	cursor := &Cursor{
		ID:        "c1",
		DeviceID:  "dev-1",
		UserID:    "user-1",
		MailboxID: "inbox",
		Version:   42,
		CreatedAt: time.Now(),
	}

	if err := store.Save(context.Background(), cursor); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := store.Get(context.Background(), "dev-1", "inbox")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Version != 42 {
		t.Fatalf("expected version 42, got %d", got.Version)
	}
}

func TestCursorStoreListByMailbox(t *testing.T) {
	store := NewMemoryCursorStore()
	for i := 1; i <= 3; i++ {
		cursor := &Cursor{
			ID:        fmt.Sprintf("c%d", i),
			DeviceID:  fmt.Sprintf("dev-%d", i),
			UserID:    "user-1",
			MailboxID: "inbox",
			Version:   int64(i),
			CreatedAt: time.Now(),
		}
		if err := store.Save(context.Background(), cursor); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	cursors, err := store.ListByMailbox(context.Background(), "inbox")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cursors) != 3 {
		t.Fatalf("expected 3 cursors, got %d", len(cursors))
	}
}

func TestCursorStoreDelete(t *testing.T) {
	store := NewMemoryCursorStore()
	cursor := &Cursor{
		ID:        "c1",
		DeviceID:  "dev-1",
		UserID:    "user-1",
		MailboxID: "inbox",
		Version:   1,
		CreatedAt: time.Now(),
	}
	store.Save(context.Background(), cursor)
	store.Delete(context.Background(), "c1")

	_, err := store.Get(context.Background(), "dev-1", "inbox")
	if err == nil {
		t.Fatalf("expected error after delete")
	}
}

func TestCursorStoreUpdate(t *testing.T) {
	store := NewMemoryCursorStore()
	cursor := &Cursor{
		ID:        "c1",
		DeviceID:  "dev-1",
		UserID:    "user-1",
		MailboxID: "inbox",
		Version:   1,
		CreatedAt: time.Now(),
	}
	store.Save(context.Background(), cursor)

	cursor.Version = 2
	store.Save(context.Background(), cursor)

	got, err := store.Get(context.Background(), "dev-1", "inbox")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Version != 2 {
		t.Fatalf("expected version 2, got %d", got.Version)
	}
}

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
		t.Fatalf("timed out waiting for event")
	}
}

func TestFanOutMultipleWatchers(t *testing.T) {
	fo := NewFanOut()
	ch1 := fo.Watch("inbox")
	ch2 := fo.Watch("inbox")

	event := Event{MailboxID: "inbox", Type: "messageAdded", Version: 5}
	fo.Notify("inbox", event)

	for _, ch := range []chan Event{ch1, ch2} {
		select {
		case got := <-ch:
			if got.Version != 5 {
				t.Fatalf("unexpected version: %d", got.Version)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for event")
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
		t.Fatalf("timed out waiting for inbox event")
	}

	select {
	case <-sentCh:
		t.Fatalf("unexpected event on sent mailbox")
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
			t.Fatalf("unexpected event after unwatch")
		}
	case <-time.After(100 * time.Millisecond):
	}
}

func TestCursorIsExpired(t *testing.T) {
	cursor := &Cursor{
		ID:        "c1",
		DeviceID:  "dev-1",
		CreatedAt: time.Now().Add(-48 * time.Hour),
	}
	if !cursor.IsExpired(24 * time.Hour) {
		t.Fatalf("expected expired cursor")
	}
	if cursor.IsExpired(72 * time.Hour) {
		t.Fatalf("expected non-expired cursor")
	}
}

func TestDeltaSyncChangesSince(t *testing.T) {
	store := NewMemoryCursorStore()
	ds := NewDeltaSync(store)

	cursor := &Cursor{
		ID:        "c1",
		DeviceID:  "dev-1",
		UserID:    "user-1",
		MailboxID: "inbox",
		Version:   5,
		CreatedAt: time.Now(),
	}
	store.Save(context.Background(), cursor)

	changes := []Change{
		{ID: "msg-6", Version: 6, Type: "add"},
		{ID: "msg-7", Version: 7, Type: "add"},
	}

	result, err := ds.ChangesSince(context.Background(), "dev-1", "inbox", changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(result.Changes))
	}
	if result.NewCursor.Version != 7 {
		t.Fatalf("expected new cursor version 7, got %d", result.NewCursor.Version)
	}
}

func TestDeltaSyncNoChanges(t *testing.T) {
	store := NewMemoryCursorStore()
	ds := NewDeltaSync(store)

	cursor := &Cursor{
		ID:        "c1",
		DeviceID:  "dev-1",
		MailboxID: "inbox",
		Version:   10,
		CreatedAt: time.Now(),
	}
	store.Save(context.Background(), cursor)

	changes := []Change{
		{ID: "msg-5", Version: 5, Type: "add"},
	}

	result, err := ds.ChangesSince(context.Background(), "dev-1", "inbox", changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Changes) != 0 {
		t.Fatalf("expected 0 changes, got %d", len(result.Changes))
	}
}
