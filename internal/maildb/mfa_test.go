package maildb

import (
	"context"
	"testing"
	"time"
)

func TestPruneExpiredTOTPCodesRejectsNilDB(t *testing.T) {
	t.Parallel()

	r := &Repository{db: nil}
	_, err := r.PruneExpiredTOTPCodes(context.Background(), time.Now())
	if err == nil {
		t.Fatal("PruneExpiredTOTPCodes with nil db should return error")
	}
}

func TestPruneExpiredTOTPCodesRejectsZeroCutoff(t *testing.T) {
	t.Parallel()

	r := &Repository{db: nil}
	_, err := r.PruneExpiredTOTPCodes(context.Background(), time.Time{})
	if err == nil {
		t.Fatal("PruneExpiredTOTPCodes with zero cutoff should return error")
	}
}
