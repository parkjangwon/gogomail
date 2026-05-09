package sync

import (
	"context"
	"fmt"
	"sync"
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
		t.Fatalf("unexpected Save error: %v", err)
	}

	got, err := store.Get(context.Background(), "dev-1", "inbox")
	if err != nil {
		t.Fatalf("unexpected Get error: %v", err)
	}
	if got.Version != 42 {
		t.Fatalf("expected version 42, got %d", got.Version)
	}
}

func TestCursorStoreRejectEmptyUserID(t *testing.T) {
	store := NewMemoryCursorStore()
	cursor := &Cursor{
		ID:        "c1",
		DeviceID:  "dev-1",
		UserID:    "", // empty — must be rejected
		MailboxID: "inbox",
		Version:   1,
		CreatedAt: time.Now(),
	}
	if err := store.Save(context.Background(), cursor); err == nil {
		t.Fatal("expected error for empty userID, got nil")
	}
}

func TestCursorStoreListByUserRejectEmpty(t *testing.T) {
	store := NewMemoryCursorStore()
	_, err := store.ListByUser(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty userID in ListByUser, got nil")
	}
}

func TestCursorStoreListByUser(t *testing.T) {
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
			t.Fatalf("unexpected Save error: %v", err)
		}
	}

	cursors, err := store.ListByUser(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected ListByUser error: %v", err)
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
	_ = store.Save(context.Background(), cursor)
	_ = store.Delete(context.Background(), "c1")

	_, err := store.Get(context.Background(), "dev-1", "inbox")
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestCursorIsExpired(t *testing.T) {
	cursor := &Cursor{
		ID:        "c1",
		DeviceID:  "dev-1",
		UserID:    "user-1",
		CreatedAt: time.Now().Add(-48 * time.Hour),
	}
	if !cursor.IsExpired(24 * time.Hour) {
		t.Fatal("expected cursor to be expired")
	}
	if cursor.IsExpired(72 * time.Hour) {
		t.Fatal("expected cursor not to be expired")
	}
}

// TestCursorConcurrentReadWrite exercises concurrent Save/Get/ListByUser to
// confirm the mutex prevents data races.
// Run with: go test -race ./internal/sync/... -run TestCursorConcurrentReadWrite
func TestCursorConcurrentReadWrite(t *testing.T) {
	store := NewMemoryCursorStore()

	const workers = 50
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cursor := &Cursor{
				ID:        fmt.Sprintf("c%d", i),
				DeviceID:  fmt.Sprintf("dev-%d", i),
				UserID:    "user-1",
				MailboxID: "inbox",
				Version:   int64(i),
				CreatedAt: time.Now(),
			}
			_ = store.Save(context.Background(), cursor)
		}(i)
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = store.ListByUser(context.Background(), "user-1")
		}()
	}

	wg.Wait()
}
