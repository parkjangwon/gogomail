package mailservice

import (
	"container/list"
	"sync"
	"time"
)

const (
	defaultMessageBodyCacheEntries = 256
	defaultMessageBodyCacheTTL     = 5 * time.Minute
	maxCachedMessageBodyBytes      = 256 * 1024
)

type parsedMessageBody struct {
	text string
	html string
}

type messageBodyCache struct {
	mu       sync.Mutex
	capacity int
	ttl      time.Duration
	entries  map[string]*list.Element
	lru      *list.List
}

type messageBodyCacheEntry struct {
	key       string
	body      parsedMessageBody
	expiresAt time.Time
}

func newMessageBodyCache(capacity int, ttl time.Duration) *messageBodyCache {
	if capacity <= 0 || ttl <= 0 {
		return nil
	}
	return &messageBodyCache{
		capacity: capacity,
		ttl:      ttl,
		entries:  make(map[string]*list.Element, capacity),
		lru:      list.New(),
	}
}

func (c *messageBodyCache) get(key string, now time.Time) (parsedMessageBody, bool) {
	if c == nil || key == "" {
		return parsedMessageBody{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	elem := c.entries[key]
	if elem == nil {
		return parsedMessageBody{}, false
	}
	entry := elem.Value.(messageBodyCacheEntry)
	if !entry.expiresAt.After(now) {
		c.lru.Remove(elem)
		delete(c.entries, key)
		return parsedMessageBody{}, false
	}
	c.lru.MoveToFront(elem)
	return entry.body, true
}

func (c *messageBodyCache) put(key string, body parsedMessageBody, now time.Time) {
	if c == nil || key == "" || len(body.text)+len(body.html) > maxCachedMessageBodyBytes {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := messageBodyCacheEntry{key: key, body: body, expiresAt: now.Add(c.ttl)}
	if elem := c.entries[key]; elem != nil {
		elem.Value = entry
		c.lru.MoveToFront(elem)
		return
	}
	elem := c.lru.PushFront(entry)
	c.entries[key] = elem
	for len(c.entries) > c.capacity {
		last := c.lru.Back()
		if last == nil {
			return
		}
		evicted := last.Value.(messageBodyCacheEntry)
		c.lru.Remove(last)
		delete(c.entries, evicted.key)
	}
}
