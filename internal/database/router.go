package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// RouterMetrics receives observability signals from the read/write Router.
//
// Implementations must be safe for concurrent use. The observability adapter
// implements this interface; tests use lightweight fakes.
type RouterMetrics interface {
	// ObserveReplicaLag reports the most recently measured replica lag. It is
	// called after every successful replica health probe.
	ObserveReplicaLag(seconds float64)
	// ObserveReplicaFallback increments when a read that was eligible for the
	// replica was served by the primary instead. reason is a low-cardinality
	// label such as "unhealthy", "stale", or "no_replica".
	ObserveReplicaFallback(reason string)
	// ObserveReplicaHealth reports the replica health state after a probe.
	ObserveReplicaHealth(healthy bool)
}

// noopMetrics is used when no metrics sink is supplied.
type noopMetrics struct{}

func (noopMetrics) ObserveReplicaLag(float64)     {}
func (noopMetrics) ObserveReplicaFallback(string) {}
func (noopMetrics) ObserveReplicaHealth(bool)     {}

// RouterOptions configures replica routing behaviour.
type RouterOptions struct {
	// MaxStaleness is the replica lag threshold above which reads fall back to
	// the primary. Zero disables the staleness check (only connectivity is
	// considered).
	MaxStaleness time.Duration
	// FallbackToPrimary routes reads to the primary when the replica is
	// unhealthy. When false, reads still use the replica handle and surface
	// its errors to the caller (useful for strict read-scaling deployments).
	FallbackToPrimary bool
	// HealthProbeInterval throttles how often the replica is probed. A probe is
	// performed lazily on the first Reader call after the interval elapses.
	// Defaults to 5s when zero.
	HealthProbeInterval time.Duration
	// Metrics receives lag/fallback/health signals. Optional.
	Metrics RouterMetrics
	// now is injectable for tests. Defaults to time.Now.
	now func() time.Time
	// probe overrides the replica health probe for tests. Defaults to
	// probeReplica which queries pg_last_xact_replay_timestamp.
	probe func(ctx context.Context, db *sql.DB) (time.Duration, error)
}

func (o RouterOptions) withDefaults() RouterOptions {
	if o.HealthProbeInterval <= 0 {
		o.HealthProbeInterval = 5 * time.Second
	}
	if o.Metrics == nil {
		o.Metrics = noopMetrics{}
	}
	if o.now == nil {
		o.now = time.Now
	}
	if o.probe == nil {
		o.probe = probeReplica
	}
	return o
}

// Router routes read-only queries to a read replica when one is healthy and
// within the configured staleness bound, and always routes writes to the
// primary. When no replica is configured, every call is served by the primary.
//
// Router is safe for concurrent use.
type Router struct {
	primary *sql.DB
	replica *sql.DB
	opts    RouterOptions

	mu           sync.Mutex
	lastProbe    time.Time
	replicaOK    bool
	lastLag      time.Duration
	probeStarted bool
}

// NewRouter builds a Router from an already-open primary and optional replica.
// When replica is nil the Router degrades to primary-only mode.
func NewRouter(primary, replica *sql.DB, opts RouterOptions) (*Router, error) {
	if primary == nil {
		return nil, errors.New("primary database handle is required")
	}
	r := &Router{
		primary: primary,
		replica: replica,
		opts:    opts.withDefaults(),
	}
	// A freshly opened replica is assumed healthy until the first probe.
	r.replicaOK = replica != nil
	return r, nil
}

// OpenRouter opens the primary (and, when replicaURL is non-empty, the replica)
// connection pools and returns a Router wrapping them. The pool tuning options
// apply to both pools.
func OpenRouter(ctx context.Context, primaryURL, replicaURL string, poolOpts Options, routerOpts RouterOptions) (*Router, error) {
	primary, err := Open(ctx, primaryURL, poolOpts)
	if err != nil {
		return nil, fmt.Errorf("open primary database: %w", err)
	}
	var replica *sql.DB
	if strings.TrimSpace(replicaURL) != "" {
		replica, err = Open(ctx, replicaURL, poolOpts)
		if err != nil {
			// A replica that cannot be opened must not take down the service:
			// close it and continue primary-only. The caller can still serve
			// traffic; reads fall back to primary.
			if closeErr := primary.Close(); closeErr != nil {
				return nil, fmt.Errorf("open replica database: %w (and closing primary failed: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("open replica database: %w", err)
		}
	}
	return NewRouter(primary, replica, routerOpts)
}

// Primary returns the primary connection pool. All writes must use this handle.
func (r *Router) Primary() *sql.DB { return r.primary }

// Writer is an alias for Primary, documenting intent at call sites.
func (r *Router) Writer() *sql.DB { return r.primary }

// HasReplica reports whether a read replica is configured.
func (r *Router) HasReplica() bool { return r.replica != nil }

// Reader returns the connection pool that read-only queries should use. It
// returns the replica when one is configured, healthy, and within the staleness
// bound; otherwise it returns the primary (and records a fallback). Callers must
// only route queries with no side effects through Reader.
func (r *Router) Reader(ctx context.Context) *sql.DB {
	if r.replica == nil {
		r.opts.Metrics.ObserveReplicaFallback("no_replica")
		return r.primary
	}
	healthy, lag := r.replicaHealth(ctx)
	if !healthy {
		if r.opts.FallbackToPrimary {
			r.opts.Metrics.ObserveReplicaFallback("unhealthy")
			return r.primary
		}
		// Strict mode: still use the replica so its errors surface.
		return r.replica
	}
	if r.opts.MaxStaleness > 0 && lag > r.opts.MaxStaleness {
		if r.opts.FallbackToPrimary {
			r.opts.Metrics.ObserveReplicaFallback("stale")
			return r.primary
		}
		return r.replica
	}
	return r.replica
}

// replicaHealth returns the cached replica health, refreshing it via a probe
// when the probe interval has elapsed. The lag returned is the last measured
// replication lag.
func (r *Router) replicaHealth(ctx context.Context) (bool, time.Duration) {
	r.mu.Lock()
	needsProbe := !r.probeStarted || r.opts.now().Sub(r.lastProbe) >= r.opts.HealthProbeInterval
	if !needsProbe {
		ok, lag := r.replicaOK, r.lastLag
		r.mu.Unlock()
		return ok, lag
	}
	r.probeStarted = true
	r.lastProbe = r.opts.now()
	r.mu.Unlock()

	lag, err := r.opts.probe(ctx, r.replica)

	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		r.replicaOK = false
		r.opts.Metrics.ObserveReplicaHealth(false)
		return false, r.lastLag
	}
	r.replicaOK = true
	r.lastLag = lag
	r.opts.Metrics.ObserveReplicaHealth(true)
	r.opts.Metrics.ObserveReplicaLag(lag.Seconds())
	return true, lag
}

// Ping verifies primary connectivity and, when configured, probes the replica.
// A replica probe failure is not fatal: it is recorded but does not cause Ping
// to return an error, because the service can still operate primary-only.
func (r *Router) Ping(ctx context.Context) error {
	if err := r.primary.PingContext(ctx); err != nil {
		return fmt.Errorf("ping primary: %w", err)
	}
	if r.replica != nil {
		if _, err := r.opts.probe(ctx, r.replica); err != nil {
			r.mu.Lock()
			r.replicaOK = false
			r.probeStarted = true
			r.lastProbe = r.opts.now()
			r.mu.Unlock()
			r.opts.Metrics.ObserveReplicaHealth(false)
		}
	}
	return nil
}

// Close closes both pools, returning the first error encountered.
func (r *Router) Close() error {
	var errs []error
	if r.replica != nil {
		if err := r.replica.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close replica: %w", err))
		}
	}
	if err := r.primary.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close primary: %w", err))
	}
	return errors.Join(errs...)
}

// probeReplica measures replication lag using the standard PostgreSQL streaming
// replication function. On a replica, pg_last_xact_replay_timestamp() returns
// the time of the last transaction replayed; the difference from now() is the
// apply lag. A NULL result (no replayed transaction yet, or a promoted primary)
// is treated as zero lag with a successful probe.
func probeReplica(ctx context.Context, db *sql.DB) (time.Duration, error) {
	if db == nil {
		return 0, errors.New("replica handle is nil")
	}
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var lagSeconds sql.NullFloat64
	// COALESCE keeps the query result non-NULL even before the first replay.
	const q = `SELECT COALESCE(EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())), 0)`
	if err := db.QueryRowContext(probeCtx, q).Scan(&lagSeconds); err != nil {
		return 0, fmt.Errorf("probe replica lag: %w", err)
	}
	if !lagSeconds.Valid || lagSeconds.Float64 < 0 {
		return 0, nil
	}
	return time.Duration(lagSeconds.Float64 * float64(time.Second)), nil
}
