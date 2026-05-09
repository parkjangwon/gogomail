package sync

import (
	"sync"
)

// Event represents a mailbox change event for fan-out broadcasting.
type Event struct {
	MailboxID string
	UserID    string
	Type      string
	Version   int64
}

// watcher holds a channel protected by a mutex for safe concurrent send/close.
// The mu serialises send and close so a send never races with close.
type watcher struct {
	mu     sync.Mutex
	ch     chan Event
	closed bool
}

// send delivers an event to the channel if not yet closed.
// Holding mu prevents a concurrent close from racing with the channel send.
func (w *watcher) send(event Event) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	select {
	case w.ch <- event:
	default:
	}
}

// close closes the channel exactly once.
// Holding mu ensures no send can race with the close.
func (w *watcher) close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.closed {
		w.closed = true
		close(w.ch)
	}
}

// FanOut broadcasts mailbox events to registered watchers.
// It is safe for concurrent use. Channel close is protected by a per-watcher
// mutex so sends and closes never race.
type FanOut struct {
	mu       sync.RWMutex
	watchers map[string][]*watcher
}

// NewFanOut creates a new FanOut broadcaster.
func NewFanOut() *FanOut {
	return &FanOut{
		watchers: make(map[string][]*watcher),
	}
}

// Watch registers a channel to receive events for the given mailboxID.
// The returned channel is buffered (capacity 1).
func (f *FanOut) Watch(mailboxID string) chan Event {
	w := &watcher{
		ch: make(chan Event, 1),
	}
	f.mu.Lock()
	f.watchers[mailboxID] = append(f.watchers[mailboxID], w)
	f.mu.Unlock()
	return w.ch
}

// Unwatch removes the channel from a mailbox's watcher list and closes it.
// It is safe to call concurrently with Notify.
func (f *FanOut) Unwatch(mailboxID string, ch chan Event) {
	f.mu.Lock()
	ws := f.watchers[mailboxID]
	var found *watcher
	for i, w := range ws {
		if w.ch == ch {
			f.watchers[mailboxID] = append(ws[:i], ws[i+1:]...)
			found = w
			break
		}
	}
	f.mu.Unlock()

	if found != nil {
		found.close()
	}
}

// Notify sends an event to all current watchers of mailboxID.
func (f *FanOut) Notify(mailboxID string, event Event) {
	f.mu.RLock()
	ws := make([]*watcher, len(f.watchers[mailboxID]))
	copy(ws, f.watchers[mailboxID])
	f.mu.RUnlock()

	for _, w := range ws {
		w.send(event)
	}
}
