package maildb

import (
	"context"
	"testing"
	"time"
)

func TestCountStuckScheduledMailNilDB(t *testing.T) {
	r := &Repository{}
	_, err := r.CountStuckScheduledMail(context.Background(), 10*time.Minute)
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestCountStuckScheduledMailZeroDuration(t *testing.T) {
	r := &Repository{}
	_, err := r.CountStuckScheduledMail(context.Background(), 0)
	if err == nil {
		t.Fatal("expected error for nil db (zero duration still hits db)")
	}
}
