package sync

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Cursor tracks the sync state for a device and mailbox.
type Cursor struct {
	ID        string
	DeviceID  string
	UserID    string
	MailboxID string
	Version   int64
	CreatedAt time.Time
}

// IsExpired reports whether the cursor is older than ttl.
func (c *Cursor) IsExpired(ttl time.Duration) bool {
	return time.Since(c.CreatedAt) > ttl
}

// CursorStore persists device sync cursors.
type CursorStore interface {
	Save(ctx context.Context, cursor *Cursor) error
	Get(ctx context.Context, deviceID, mailboxID string) (*Cursor, error)
	ListByUser(ctx context.Context, userID string) ([]*Cursor, error)
	Delete(ctx context.Context, id string) error
}

// MemoryCursorStore is a mutex-protected in-memory cursor store.
// All list and map operations are protected by mu to prevent concurrent
// read/write races.
type MemoryCursorStore struct {
	mu      sync.RWMutex
	cursors map[string]*Cursor
}

// NewMemoryCursorStore creates a new MemoryCursorStore.
func NewMemoryCursorStore() *MemoryCursorStore {
	return &MemoryCursorStore{
		cursors: make(map[string]*Cursor),
	}
}

// Save stores or updates a cursor. Returns an error if userID is empty.
func (m *MemoryCursorStore) Save(_ context.Context, cursor *Cursor) error {
	if cursor.UserID == "" {
		return fmt.Errorf("cursor userID must not be empty")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cursors[cursor.ID] = cursor
	return nil
}

// Get retrieves a cursor by deviceID and mailboxID.
func (m *MemoryCursorStore) Get(_ context.Context, deviceID, mailboxID string) (*Cursor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.cursors {
		if c.DeviceID == deviceID && c.MailboxID == mailboxID {
			return c, nil
		}
	}
	return nil, fmt.Errorf("cursor not found for device %s mailbox %s", deviceID, mailboxID)
}

// ListByUser returns all cursors for the given userID.
// Returns an error if userID is empty.
func (m *MemoryCursorStore) ListByUser(_ context.Context, userID string) ([]*Cursor, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID must not be empty")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Cursor
	for _, c := range m.cursors {
		if c.UserID == userID {
			result = append(result, c)
		}
	}
	return result, nil
}

// Delete removes a cursor by ID.
func (m *MemoryCursorStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cursors, id)
	return nil
}
