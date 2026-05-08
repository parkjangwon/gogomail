package deltasync

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

// IsExpired reports whether the cursor is older than the given TTL.
func (c *Cursor) IsExpired(ttl time.Duration) bool {
	return time.Since(c.CreatedAt) > ttl
}

// Change represents a single change in a mailbox.
type Change struct {
	ID      string
	Version int64
	Type    string
}

// ChangesResult contains changes and a new cursor for a sync operation.
type ChangesResult struct {
	Changes   []Change
	NewCursor *Cursor
}

// CursorStore persists device sync cursors.
type CursorStore interface {
	Save(ctx context.Context, cursor *Cursor) error
	Get(ctx context.Context, deviceID, mailboxID string) (*Cursor, error)
	ListByMailbox(ctx context.Context, mailboxID string) ([]*Cursor, error)
	Delete(ctx context.Context, id string) error
}

// MemoryCursorStore is an in-memory cursor store for testing.
type MemoryCursorStore struct {
	mu      sync.RWMutex
	cursors map[string]*Cursor
}

// NewMemoryCursorStore creates an in-memory cursor store.
func NewMemoryCursorStore() *MemoryCursorStore {
	return &MemoryCursorStore{
		cursors: make(map[string]*Cursor),
	}
}

// Save stores a cursor.
func (m *MemoryCursorStore) Save(ctx context.Context, cursor *Cursor) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cursors[cursor.ID] = cursor
	return nil
}

// Get retrieves a cursor by device and mailbox.
func (m *MemoryCursorStore) Get(ctx context.Context, deviceID, mailboxID string) (*Cursor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.cursors {
		if c.DeviceID == deviceID && c.MailboxID == mailboxID {
			return c, nil
		}
	}
	return nil, fmt.Errorf("cursor not found")
}

// ListByMailbox returns all cursors for a mailbox.
func (m *MemoryCursorStore) ListByMailbox(ctx context.Context, mailboxID string) ([]*Cursor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Cursor
	for _, c := range m.cursors {
		if c.MailboxID == mailboxID {
			result = append(result, c)
		}
	}
	return result, nil
}

// Delete removes a cursor by ID.
func (m *MemoryCursorStore) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cursors, id)
	return nil
}

// Event represents a mailbox change event for IMAP IDLE fan-out.
type Event struct {
	MailboxID string
	Type      string
	Version   int64
}

// FanOut broadcasts mailbox events to watchers.
type FanOut struct {
	mu       sync.RWMutex
	watchers map[string][]chan Event
}

// NewFanOut creates a new fan-out broadcaster.
func NewFanOut() *FanOut {
	return &FanOut{
		watchers: make(map[string][]chan Event),
	}
}

// Watch registers a channel to receive events for a mailbox.
func (f *FanOut) Watch(mailboxID string) chan Event {
	ch := make(chan Event, 1)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.watchers[mailboxID] = append(f.watchers[mailboxID], ch)
	return ch
}

// Unwatch removes a channel from a mailbox's watchers.
func (f *FanOut) Unwatch(mailboxID string, ch chan Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w := f.watchers[mailboxID]
	for i, c := range w {
		if c == ch {
			f.watchers[mailboxID] = append(w[:i], w[i+1:]...)
			close(ch)
			break
		}
	}
}

// Notify sends an event to all watchers of a mailbox.
func (f *FanOut) Notify(mailboxID string, event Event) {
	f.mu.RLock()
	w := f.watchers[mailboxID]
	f.mu.RUnlock()
	for _, ch := range w {
		select {
		case ch <- event:
		default:
		}
	}
}

// DeltaSync computes changes since a device's last known cursor.
type DeltaSync struct {
	store CursorStore
}

// NewDeltaSync creates a delta sync engine.
func NewDeltaSync(store CursorStore) *DeltaSync {
	return &DeltaSync{store: store}
}

// ChangesSince returns changes newer than the device's current cursor and a new cursor.
func (d *DeltaSync) ChangesSince(ctx context.Context, deviceID, mailboxID string, allChanges []Change) (*ChangesResult, error) {
	cursor, err := d.store.Get(ctx, deviceID, mailboxID)
	if err != nil {
		cursor = &Cursor{
			ID:        fmt.Sprintf("%s-%s", deviceID, mailboxID),
			DeviceID:  deviceID,
			MailboxID: mailboxID,
			Version:   0,
			CreatedAt: time.Now(),
		}
	}

	var filtered []Change
	var maxVersion int64
	for _, c := range allChanges {
		if c.Version > cursor.Version {
			filtered = append(filtered, c)
		}
		if c.Version > maxVersion {
			maxVersion = c.Version
		}
	}

	newCursor := &Cursor{
		ID:        cursor.ID,
		DeviceID:  deviceID,
		MailboxID: mailboxID,
		Version:   maxVersion,
		CreatedAt: time.Now(),
	}
	if err := d.store.Save(ctx, newCursor); err != nil {
		return nil, err
	}

	return &ChangesResult{
		Changes:   filtered,
		NewCursor: newCursor,
	}, nil
}
