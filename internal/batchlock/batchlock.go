package batchlock

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"log/slog"
	"sync"
	"time"
)

var (
	memLocks   = make(map[string]bool)
	memLocksMu sync.Mutex
)

type JobLock interface {
	Acquire() (bool, error)
	Release() error
}

type PostgresJobLock struct {
	db      *sql.DB
	jobName string
	mu      sync.Mutex
	held    bool
}

func NewPostgresJobLock(db *sql.DB, jobName string) *PostgresJobLock {
	return &PostgresJobLock{
		db:      db,
		jobName: jobName,
	}
}

func (l *PostgresJobLock) Acquire() (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.held {
		return true, nil
	}

	if l.db == nil {
		memLocksMu.Lock()
		defer memLocksMu.Unlock()

		if memLocks[l.jobName] {
			return false, nil
		}
		memLocks[l.jobName] = true
		l.held = true
		return true, nil
	}

	lockID := l.lockID()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var acquired bool
	err := l.db.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired)
	if err != nil {
		return false, fmt.Errorf("try advisory lock: %w", err)
	}

	if acquired {
		l.held = true
	}

	return acquired, nil
}

func (l *PostgresJobLock) Release() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.held {
		return nil
	}

	if l.db == nil {
		memLocksMu.Lock()
		defer memLocksMu.Unlock()

		delete(memLocks, l.jobName)
		l.held = false
		return nil
	}

	lockID := l.lockID()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := l.db.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", lockID)
	if err != nil {
		return fmt.Errorf("advisory unlock: %w", err)
	}

	l.held = false
	return nil
}

func (l *PostgresJobLock) lockID() int64 {
	h := fnv.New64a()
	h.Write([]byte(l.jobName))
	return int64(h.Sum64())
}

type Job struct {
	Name     string
	Handler  func() error
	Interval time.Duration
}

type JobRegistry struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

func NewJobRegistry() *JobRegistry {
	return &JobRegistry{
		jobs: make(map[string]*Job),
	}
}

func (r *JobRegistry) Register(name string, handler func() error, interval time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.jobs[name] = &Job{
		Name:     name,
		Handler:  handler,
		Interval: interval,
	}
}

func (r *JobRegistry) Get(name string) (*Job, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	job, ok := r.jobs[name]
	return job, ok
}

func (r *JobRegistry) List() []*Job {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*Job, 0, len(r.jobs))
	for _, job := range r.jobs {
		list = append(list, job)
	}
	return list
}

type Worker struct {
	registry *JobRegistry
	db       *sql.DB
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

func NewWorker(registry *JobRegistry, db *sql.DB) *Worker {
	return &Worker{
		registry: registry,
		db:       db,
		stopCh:   make(chan struct{}),
	}
}

func (w *Worker) Start() {
	for _, job := range w.registry.List() {
		w.wg.Add(1)
		go w.runJob(job)
	}
}

func (w *Worker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

func (w *Worker) runJob(job *Job) {
	defer w.wg.Done()

	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()

	lock := NewPostgresJobLock(w.db, job.Name)

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			acquired, err := lock.Acquire()
			if err != nil || !acquired {
				continue
			}

			w.runJobSafely(job, lock)
		}
	}
}

func (w *Worker) runJobSafely(job *Job, lock JobLock) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("job handler panicked", "job", job.Name, "panic", r)
		}
		if err := lock.Release(); err != nil {
			slog.Error("failed to release lock", "job", job.Name, "err", err)
		}
	}()
	if err := job.Handler(); err != nil {
		slog.Error("job handler error", "job", job.Name, "err", err)
	}
}
