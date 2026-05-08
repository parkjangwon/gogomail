package batchlock

import (
	"testing"
	"time"
)

func TestPostgresJobLockAcquireAndRelease(t *testing.T) {
	lock := NewPostgresJobLock(nil, "test-job")

	acquired, err := lock.Acquire()
	if err != nil {
		t.Fatalf("Acquire returned error: %v", err)
	}
	if !acquired {
		t.Fatal("Acquire should return true for first caller")
	}

	err = lock.Release()
	if err != nil {
		t.Fatalf("Release returned error: %v", err)
	}
}

func TestPostgresJobLockOnlyOneAcquires(t *testing.T) {
	lock1 := NewPostgresJobLock(nil, "exclusive-job")
	lock2 := NewPostgresJobLock(nil, "exclusive-job")

	acquired1, err := lock1.Acquire()
	if err != nil {
		t.Fatalf("lock1.Acquire returned error: %v", err)
	}
	if !acquired1 {
		t.Fatal("lock1 should acquire first")
	}

	acquired2, err := lock2.Acquire()
	if err != nil {
		t.Fatalf("lock2.Acquire returned error: %v", err)
	}
	if acquired2 {
		t.Fatal("lock2 should NOT acquire when lock1 holds it")
	}

	lock1.Release()

	acquired2Again, err := lock2.Acquire()
	if err != nil {
		t.Fatalf("lock2.Acquire after release returned error: %v", err)
	}
	if !acquired2Again {
		t.Fatal("lock2 should acquire after lock1 releases")
	}

	lock2.Release()
}

func TestPostgresJobLockDifferentJobNames(t *testing.T) {
	lock1 := NewPostgresJobLock(nil, "job-a")
	lock2 := NewPostgresJobLock(nil, "job-b")

	acquired1, err := lock1.Acquire()
	if err != nil {
		t.Fatalf("lock1.Acquire returned error: %v", err)
	}
	if !acquired1 {
		t.Fatal("lock1 should acquire")
	}

	acquired2, err := lock2.Acquire()
	if err != nil {
		t.Fatalf("lock2.Acquire returned error: %v", err)
	}
	if !acquired2 {
		t.Fatal("lock2 should also acquire (different job name)")
	}

	lock1.Release()
	lock2.Release()
}

func TestJobRegistry(t *testing.T) {
	registry := NewJobRegistry()

	executed := false
	registry.Register("test-job", func() error {
		executed = true
		return nil
	}, time.Minute)

	job, ok := registry.Get("test-job")
	if !ok {
		t.Fatal("registry.Get should return registered job")
	}
	if job.Name != "test-job" {
		t.Fatalf("job.Name = %s, want test-job", job.Name)
	}
	if job.Interval != time.Minute {
		t.Fatalf("job.Interval = %v, want 1m", job.Interval)
	}

	err := job.Handler()
	if err != nil {
		t.Fatalf("job.Handler returned error: %v", err)
	}
	if !executed {
		t.Fatal("job.Handler should have executed")
	}
}

func TestJobRegistryList(t *testing.T) {
	registry := NewJobRegistry()
	registry.Register("job-a", func() error { return nil }, time.Minute)
	registry.Register("job-b", func() error { return nil }, time.Hour)

	jobs := registry.List()
	if len(jobs) != 2 {
		t.Fatalf("registry.List returned %d jobs, want 2", len(jobs))
	}
}
