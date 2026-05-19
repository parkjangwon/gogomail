package backpressure

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// leaderElectScript atomically acquires or renews the leader lock.
// Returns 1 if the caller holds the lock after execution, 0 otherwise.
var leaderElectScript = redis.NewScript(`
local current = redis.call("GET", KEYS[1])
if current == ARGV[1] then
    redis.call("PEXPIRE", KEYS[1], tonumber(ARGV[2]))
    return 1
elseif current == false then
    if redis.call("SET", KEYS[1], ARGV[1], "NX", "PX", ARGV[2]) then
        return 1
    end
end
return 0
`)

// AutoBackpressureConfig configures the automatic backpressure manager.
type AutoBackpressureConfig struct {
	// CheckInterval is how often to poll Redis metrics.
	CheckInterval time.Duration
	// Redis memory thresholds (used_memory / maxmemory ratio).
	WarningThreshold  float64
	DangerThreshold   float64
	CriticalThreshold float64
	// Queue depth thresholds (total XLEN of monitored streams).
	QueueWarningDepth  int64
	QueueDangerDepth   int64
	QueueCriticalDepth int64
	// MonitorStreams is the list of Redis Streams to measure depth.
	MonitorStreams []string
	// InstanceID uniquely identifies this process for leader election.
	// Defaults to hostname. Set explicitly when multiple processes share a host.
	InstanceID string
	// LeaderLockTTL is how long the leader lock is held between renewals.
	// Defaults to 3× CheckInterval. Another instance takes over after this
	// elapses without renewal, allowing failover when a leader crashes.
	LeaderLockTTL time.Duration
}

func (c *AutoBackpressureConfig) setDefaults() {
	if c.CheckInterval <= 0 {
		c.CheckInterval = 5 * time.Second
	}
	if c.WarningThreshold <= 0 {
		c.WarningThreshold = 0.70
	}
	if c.DangerThreshold <= 0 {
		c.DangerThreshold = 0.85
	}
	if c.CriticalThreshold <= 0 {
		c.CriticalThreshold = 0.95
	}
	if c.QueueWarningDepth <= 0 {
		c.QueueWarningDepth = 10_000
	}
	if c.QueueDangerDepth <= 0 {
		c.QueueDangerDepth = 50_000
	}
	if c.QueueCriticalDepth <= 0 {
		c.QueueCriticalDepth = 100_000
	}
	if c.InstanceID == "" {
		if h, err := os.Hostname(); err == nil {
			c.InstanceID = fmt.Sprintf("auto-bp:%s", h)
		} else {
			c.InstanceID = fmt.Sprintf("auto-bp:%d", time.Now().UnixNano())
		}
	}
	if c.LeaderLockTTL <= 0 {
		c.LeaderLockTTL = 3 * c.CheckInterval
	}
}

// AutoBackpressureManager monitors Redis memory and stream queue depth,
// and automatically adjusts the backpressure level.
//
// When multiple instances run concurrently they use Redis leader election:
// only the current leader writes the backpressure state. Other instances
// observe the lock and take over automatically if the leader crashes.
type AutoBackpressureManager struct {
	client *redis.Client
	bp     *RedisBackpressure
	cfg    AutoBackpressureConfig

	leaderKey  string
	instanceID string
	lockTTL    time.Duration

	mu           sync.RWMutex
	currentLevel string
	stopCh       chan struct{}
	done         chan struct{}
}

// leaderKeyFor derives the leader-election Redis key from the backpressure state key.
func leaderKeyFor(stateKey string) string {
	return stateKey + ":auto-leader"
}

// NewAutoBackpressureManager creates a new AutoBackpressureManager.
func NewAutoBackpressureManager(client *redis.Client, bp *RedisBackpressure, cfg AutoBackpressureConfig) *AutoBackpressureManager {
	cfg.setDefaults()
	return &AutoBackpressureManager{
		client:       client,
		bp:           bp,
		cfg:          cfg,
		leaderKey:    leaderKeyFor(bp.key),
		instanceID:   cfg.InstanceID,
		lockTTL:      cfg.LeaderLockTTL,
		currentLevel: "normal",
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
	}
}

// Start begins the background monitoring goroutine.
func (m *AutoBackpressureManager) Start(ctx context.Context) {
	go m.run(ctx)
}

// Stop signals the monitoring goroutine to stop and waits for it to finish.
func (m *AutoBackpressureManager) Stop() {
	close(m.stopCh)
	<-m.done
}

// CurrentLevel returns the most recently computed backpressure level.
func (m *AutoBackpressureManager) CurrentLevel() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentLevel
}

func (m *AutoBackpressureManager) run(ctx context.Context) {
	defer close(m.done)
	ticker := time.NewTicker(m.cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.check(ctx)
		}
	}
}

func (m *AutoBackpressureManager) isLeader(ctx context.Context) bool {
	ttlMs := m.lockTTL.Milliseconds()
	result, err := leaderElectScript.Run(ctx, m.client,
		[]string{m.leaderKey},
		m.instanceID,
		ttlMs,
	).Int64()
	return err == nil && result == 1
}

func (m *AutoBackpressureManager) check(ctx context.Context) {
	if !m.isLeader(ctx) {
		return
	}
	level := m.computeLevel(ctx)
	m.mu.Lock()
	changed := level != m.currentLevel
	if changed {
		m.currentLevel = level
	}
	m.mu.Unlock()

	if changed {
		// Only write to Redis when level changes.
		m.bp.SetState(ctx, StateUpdate{Level: level, Reason: "auto-backpressure"})
	}
}

func (m *AutoBackpressureManager) computeLevel(ctx context.Context) string {
	memLevel := m.memoryLevel(ctx)
	queueLevel := m.queueLevel(ctx)
	return higherLevel(memLevel, queueLevel)
}

func (m *AutoBackpressureManager) memoryLevel(ctx context.Context) string {
	info, err := m.client.Info(ctx, "memory").Result()
	if err != nil {
		// fail-open: keep current level
		m.mu.RLock()
		l := m.currentLevel
		m.mu.RUnlock()
		return l
	}

	var usedMemory, maxMemory int64
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "used_memory:") {
			_, after, ok := strings.Cut(line, ":")
			if ok {
				parseInt64(&usedMemory, strings.TrimSpace(after))
			}
		} else if strings.HasPrefix(line, "maxmemory:") {
			_, after, ok := strings.Cut(line, ":")
			if ok {
				parseInt64(&maxMemory, strings.TrimSpace(after))
			}
		}
	}

	if maxMemory <= 0 {
		// maxmemory not set — skip memory check
		return "normal"
	}

	ratio := float64(usedMemory) / float64(maxMemory)
	return levelFromRatio(ratio, m.cfg.WarningThreshold, m.cfg.DangerThreshold, m.cfg.CriticalThreshold)
}

func (m *AutoBackpressureManager) queueLevel(ctx context.Context) string {
	if len(m.cfg.MonitorStreams) == 0 {
		return "normal"
	}
	var total int64
	for _, stream := range m.cfg.MonitorStreams {
		n, err := m.client.XLen(ctx, stream).Result()
		if err != nil {
			// fail-open
			continue
		}
		total += n
	}
	return levelFromDepth(total, m.cfg.QueueWarningDepth, m.cfg.QueueDangerDepth, m.cfg.QueueCriticalDepth)
}

func levelFromRatio(ratio, warn, danger, critical float64) string {
	switch {
	case ratio >= critical:
		return "critical"
	case ratio >= danger:
		return "danger"
	case ratio >= warn:
		return "warning"
	default:
		return "normal"
	}
}

func levelFromDepth(depth, warn, danger, critical int64) string {
	switch {
	case depth >= critical:
		return "critical"
	case depth >= danger:
		return "danger"
	case depth >= warn:
		return "warning"
	default:
		return "normal"
	}
}

// levelOrder maps level names to numeric priority (higher = more severe).
var levelOrder = map[string]int{
	"normal":   0,
	"warning":  1,
	"danger":   2,
	"critical": 3,
}

func higherLevel(a, b string) string {
	if levelOrder[a] >= levelOrder[b] {
		return a
	}
	return b
}

func parseInt64(dst *int64, s string) {
	var v int64
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		v = v*10 + int64(c-'0')
	}
	*dst = v
}
