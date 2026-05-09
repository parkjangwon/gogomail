package maildb

import (
	"testing"
	"time"
)

func TestSetMFAGraceDeadlineNilDB(t *testing.T) {
	r := &Repository{}
	err := r.SetMFAGraceDeadline(nil, "user-1", time.Now().Add(7*24*time.Hour)) //nolint:staticcheck
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestFindExpiredMFAGraceUsersNilDB(t *testing.T) {
	r := &Repository{}
	_, err := r.FindExpiredMFAGraceUsers(nil, 100) //nolint:staticcheck
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestClearMFAGraceDeadlineNilDB(t *testing.T) {
	r := &Repository{}
	err := r.ClearMFAGraceDeadline(nil, "user-1") //nolint:staticcheck
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestFindExpiredMFAGraceUsersZeroLimit(t *testing.T) {
	r := &Repository{}
	_, err := r.FindExpiredMFAGraceUsers(nil, 0) //nolint:staticcheck
	if err == nil {
		t.Fatal("expected error for nil db (zero limit still hits db)")
	}
}
