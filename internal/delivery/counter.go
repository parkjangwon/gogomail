package delivery

import (
	"sort"
	"sync"
	"time"
)

// RouteCounters tracks per-pool delivery outcomes since process start.
// It is safe for concurrent use from multiple goroutines.
type RouteCounters struct {
	mu      sync.RWMutex
	pools   map[string]*poolCounter
	startAt time.Time
}

type poolCounter struct {
	Delivered int64
	Failed    int64
	Retried   int64
	Exhausted int64
}

// RouteCounterSnapshot is a point-in-time read of one pool's counters.
type RouteCounterSnapshot struct {
	Pool      string    `json:"pool"`
	Delivered int64     `json:"delivered"`
	Failed    int64     `json:"failed"`
	Retried   int64     `json:"retried"`
	Exhausted int64     `json:"exhausted"`
	Since     time.Time `json:"since"`
}

func NewRouteCounters() *RouteCounters {
	return &RouteCounters{pools: make(map[string]*poolCounter), startAt: time.Now().UTC()}
}

func (c *RouteCounters) observe(pool string, event MetricEvent) {
	if pool == "" {
		pool = "default"
	}
	c.mu.Lock()
	pc, ok := c.pools[pool]
	if !ok {
		pc = &poolCounter{}
		c.pools[pool] = pc
	}
	switch event.Stage {
	case MetricTransportDelivered:
		pc.Delivered++
	case MetricTransportFailed:
		pc.Failed++
	case MetricRetryScheduled:
		pc.Retried++
	case MetricRetryExhausted:
		pc.Exhausted++
	}
	c.mu.Unlock()
}

// Snapshot returns a sorted snapshot of all pool counters.
func (c *RouteCounters) Snapshot() []RouteCounterSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]RouteCounterSnapshot, 0, len(c.pools))
	for pool, pc := range c.pools {
		out = append(out, RouteCounterSnapshot{
			Pool:      pool,
			Delivered: pc.Delivered,
			Failed:    pc.Failed,
			Retried:   pc.Retried,
			Exhausted: pc.Exhausted,
			Since:     c.startAt,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Pool < out[j].Pool
	})
	return out
}
