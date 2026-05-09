package maildb

import (
	"testing"
	"time"
)

func TestCountStuckScheduledMailNilDB(t *testing.T) {
	r := &Repository{}
	_, err := r.CountStuckScheduledMail(nil, 10*time.Minute) //nolint:staticcheck
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestCountStuckScheduledMailZeroDuration(t *testing.T) {
	r := &Repository{}
	_, err := r.CountStuckScheduledMail(nil, 0) //nolint:staticcheck
	if err == nil {
		t.Fatal("expected error for nil db (zero duration still hits db)")
	}
}
