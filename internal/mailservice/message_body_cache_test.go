package mailservice

import (
	"strings"
	"testing"
	"time"
)

func TestMessageBodyCachePrunesExpiredEntriesOnPut(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	cache := newMessageBodyCache(4, time.Minute)
	cache.put("expired", parsedMessageBody{text: "old"}, now)
	cache.put("fresh", parsedMessageBody{text: "fresh"}, now.Add(90*time.Second))

	cache.put("new", parsedMessageBody{text: "new"}, now.Add(2*time.Minute))

	snapshot := cache.snapshot()
	if snapshot.Entries != 2 {
		t.Fatalf("Entries = %d, want 2 after pruning expired entry", snapshot.Entries)
	}
	if snapshot.Expired != 1 {
		t.Fatalf("Expired = %d, want 1", snapshot.Expired)
	}
	if _, ok := cache.get("expired", now.Add(2*time.Minute)); ok {
		t.Fatal("expired entry was still readable after prune")
	}
	if body, ok := cache.get("fresh", now.Add(2*time.Minute)); !ok || body.text != "fresh" {
		t.Fatalf("fresh entry = %+v/%v, want hit", body, ok)
	}
}

func TestMessageBodyCacheEvictionAndOversizeSkip(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	cache := newMessageBodyCache(1, time.Minute)
	cache.put("first", parsedMessageBody{text: "first"}, now)
	cache.put("second", parsedMessageBody{text: "second"}, now)
	cache.put("oversize", parsedMessageBody{text: strings.Repeat("x", maxCachedMessageBodyBytes+1)}, now)

	snapshot := cache.snapshot()
	if snapshot.Entries != 1 || snapshot.Evictions != 1 {
		t.Fatalf("snapshot = %+v, want one retained entry and one eviction", snapshot)
	}
	if _, ok := cache.get("first", now); ok {
		t.Fatal("first entry survived capacity eviction")
	}
	if _, ok := cache.get("oversize", now); ok {
		t.Fatal("oversize entry was cached")
	}
}
