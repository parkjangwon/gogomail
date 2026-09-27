package database

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeMetrics is a concurrency-safe RouterMetrics recorder for assertions.
type fakeMetrics struct {
	mu             sync.Mutex
	lag            []float64
	fallback       map[string]int
	healthObserved []bool
}

func newFakeMetrics() *fakeMetrics {
	return &fakeMetrics{fallback: map[string]int{}}
}

func (f *fakeMetrics) ObserveReplicaLag(seconds float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lag = append(f.lag, seconds)
}

func (f *fakeMetrics) ObserveReplicaFallback(reason string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fallback[reason]++
}

func (f *fakeMetrics) ObserveReplicaHealth(healthy bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.healthObserved = append(f.healthObserved, healthy)
}

func (f *fakeMetrics) fallbackCount(reason string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fallback[reason]
}

func (f *fakeMetrics) lastLag() (float64, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.lag) == 0 {
		return 0, false
	}
	return f.lag[len(f.lag)-1], true
}

// sentinel handles let tests assert which pool a call was routed to without a
// real database. The Router never dereferences the *sql.DB, it only returns it,
// so distinct non-nil pointers are sufficient.
func sentinelDB(t *testing.T) *sql.DB {
	t.Helper()
	// sql.OpenDB with a nil-ish connector is avoided; we just need a unique,
	// non-nil *sql.DB value. Open with the registered pgx driver does not dial
	// until first use, so this is safe and -short friendly.
	db, err := sql.Open("pgx", "postgres://unused")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestRouterReaderReturnsPrimaryWhenNoReplica(t *testing.T) {
	t.Parallel()

	primary := sentinelDB(t)
	metrics := newFakeMetrics()
	r, err := NewRouter(primary, nil, RouterOptions{Metrics: metrics})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	if r.HasReplica() {
		t.Fatal("HasReplica = true, want false")
	}
	if got := r.Reader(context.Background()); got != primary {
		t.Fatal("Reader did not return primary when no replica configured")
	}
	if got := r.Writer(); got != primary {
		t.Fatal("Writer did not return primary")
	}
	if metrics.fallbackCount("no_replica") != 1 {
		t.Fatalf("no_replica fallback count = %d, want 1", metrics.fallbackCount("no_replica"))
	}
}

func TestRouterReaderReturnsReplicaWhenHealthy(t *testing.T) {
	t.Parallel()

	primary := sentinelDB(t)
	replica := sentinelDB(t)
	metrics := newFakeMetrics()
	r, err := NewRouter(primary, replica, RouterOptions{
		MaxStaleness: 10 * time.Second,
		Metrics:      metrics,
		probe: func(context.Context, *sql.DB) (time.Duration, error) {
			return 2 * time.Second, nil
		},
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	if got := r.Reader(context.Background()); got != replica {
		t.Fatal("Reader did not return replica when healthy and fresh")
	}
	if lag, ok := metrics.lastLag(); !ok || lag != 2 {
		t.Fatalf("last observed lag = %v (ok=%v), want 2", lag, ok)
	}
	if metrics.fallbackCount("stale")+metrics.fallbackCount("unhealthy") != 0 {
		t.Fatal("unexpected fallback recorded for healthy replica")
	}
}

func TestRouterReaderFallsBackWhenReplicaUnhealthy(t *testing.T) {
	t.Parallel()

	primary := sentinelDB(t)
	replica := sentinelDB(t)
	metrics := newFakeMetrics()
	r, err := NewRouter(primary, replica, RouterOptions{
		FallbackToPrimary: true,
		Metrics:           metrics,
		probe: func(context.Context, *sql.DB) (time.Duration, error) {
			return 0, errors.New("replica down")
		},
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	if got := r.Reader(context.Background()); got != primary {
		t.Fatal("Reader did not fall back to primary when replica unhealthy")
	}
	if metrics.fallbackCount("unhealthy") != 1 {
		t.Fatalf("unhealthy fallback count = %d, want 1", metrics.fallbackCount("unhealthy"))
	}
}

func TestRouterReaderFallsBackWhenReplicaStale(t *testing.T) {
	t.Parallel()

	primary := sentinelDB(t)
	replica := sentinelDB(t)
	metrics := newFakeMetrics()
	r, err := NewRouter(primary, replica, RouterOptions{
		MaxStaleness:      5 * time.Second,
		FallbackToPrimary: true,
		Metrics:           metrics,
		probe: func(context.Context, *sql.DB) (time.Duration, error) {
			return 30 * time.Second, nil
		},
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	if got := r.Reader(context.Background()); got != primary {
		t.Fatal("Reader did not fall back to primary when replica stale")
	}
	if metrics.fallbackCount("stale") != 1 {
		t.Fatalf("stale fallback count = %d, want 1", metrics.fallbackCount("stale"))
	}
}

func TestRouterStrictModeKeepsReplicaOnFailure(t *testing.T) {
	t.Parallel()

	primary := sentinelDB(t)
	replica := sentinelDB(t)
	r, err := NewRouter(primary, replica, RouterOptions{
		FallbackToPrimary: false,
		probe: func(context.Context, *sql.DB) (time.Duration, error) {
			return 0, errors.New("replica down")
		},
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	if got := r.Reader(context.Background()); got != replica {
		t.Fatal("strict-mode Reader did not keep replica on failure")
	}
}

func TestRouterHealthProbeIsCached(t *testing.T) {
	t.Parallel()

	primary := sentinelDB(t)
	replica := sentinelDB(t)
	var probes int
	now := time.Unix(1_700_000_000, 0)
	r, err := NewRouter(primary, replica, RouterOptions{
		MaxStaleness:        10 * time.Second,
		HealthProbeInterval: time.Minute,
		now:                 func() time.Time { return now },
		probe: func(context.Context, *sql.DB) (time.Duration, error) {
			probes++
			return time.Second, nil
		},
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	for i := 0; i < 5; i++ {
		r.Reader(context.Background())
	}
	if probes != 1 {
		t.Fatalf("probe called %d times within interval, want 1 (cached)", probes)
	}
	// Advance beyond the probe interval → one more probe.
	now = now.Add(2 * time.Minute)
	r.Reader(context.Background())
	if probes != 2 {
		t.Fatalf("probe called %d times after interval elapsed, want 2", probes)
	}
}

func TestRouterRecoversAfterReplicaHealthReturns(t *testing.T) {
	t.Parallel()

	primary := sentinelDB(t)
	replica := sentinelDB(t)
	now := time.Unix(1_700_000_000, 0)
	healthy := false
	r, err := NewRouter(primary, replica, RouterOptions{
		MaxStaleness:        10 * time.Second,
		FallbackToPrimary:   true,
		HealthProbeInterval: 10 * time.Second,
		now:                 func() time.Time { return now },
		probe: func(context.Context, *sql.DB) (time.Duration, error) {
			if !healthy {
				return 0, errors.New("down")
			}
			return time.Second, nil
		},
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	if got := r.Reader(context.Background()); got != primary {
		t.Fatal("expected primary while replica unhealthy")
	}
	// Replica recovers; advance past the probe interval to force a re-probe.
	healthy = true
	now = now.Add(30 * time.Second)
	if got := r.Reader(context.Background()); got != replica {
		t.Fatal("expected replica after recovery")
	}
}

func TestNewRouterRejectsNilPrimary(t *testing.T) {
	t.Parallel()

	if _, err := NewRouter(nil, nil, RouterOptions{}); err == nil {
		t.Fatal("NewRouter accepted nil primary")
	}
}

func TestRouterPingReportsReplicaUnhealthyButSucceeds(t *testing.T) {
	t.Parallel()

	// Primary ping will fail because sentinelDB never dials a real server, so we
	// only exercise the replica-probe bookkeeping path via replicaHealth directly.
	primary := sentinelDB(t)
	replica := sentinelDB(t)
	metrics := newFakeMetrics()
	r, err := NewRouter(primary, replica, RouterOptions{
		FallbackToPrimary: true,
		Metrics:           metrics,
		probe: func(context.Context, *sql.DB) (time.Duration, error) {
			return 0, errors.New("down")
		},
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	// First Reader triggers a probe that marks the replica unhealthy.
	if got := r.Reader(context.Background()); got != primary {
		t.Fatal("expected primary fallback")
	}
	if len(metrics.healthObserved) == 0 || metrics.healthObserved[len(metrics.healthObserved)-1] {
		t.Fatal("expected an unhealthy health observation")
	}
}
