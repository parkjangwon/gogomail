package davsyncretention

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRecordRunRejectsNilDatabase(t *testing.T) {
	t.Parallel()

	_, err := (&Repository{}).RecordRun(context.Background(), RunRecord{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("RecordRun err = %v, want nil database rejection", err)
	}
}

func TestNormalizeRunRecordGeneratesSafeAuditShape(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	record, err := normalizeRunRecord(RunRecord{
		Cutoff:            now.Add(-90 * 24 * time.Hour),
		Limit:             100,
		DryRun:            false,
		ConfirmReady:      true,
		Status:            RunStatusFailed,
		ErrorMessage:      " failed\nwith\tdetail ",
		CalDAVCandidates:  7,
		CalDAVDeleted:     3,
		CardDAVCandidates: 11,
		CardDAVDeleted:    5,
	}, func() time.Time {
		return now
	})
	if err != nil {
		t.Fatalf("normalizeRunRecord returned error: %v", err)
	}
	if !strings.HasPrefix(record.ID, "dav-sync-retention-") || record.CreatedAt.IsZero() {
		t.Fatalf("record identity = %+v", record)
	}
	if record.ErrorMessage != "failed with detail" {
		t.Fatalf("ErrorMessage = %q", record.ErrorMessage)
	}
	if record.Status != RunStatusFailed || record.CalDAVCandidates != 7 || record.CardDAVDeleted != 5 {
		t.Fatalf("record = %+v", record)
	}
}

func TestNormalizeRunRecordRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	tests := []RunRecord{
		{},
		{Cutoff: now, Limit: 0},
		{Cutoff: now, Limit: 1, Status: "blocked"},
		{Cutoff: now, Limit: 1, CalDAVCandidates: -1},
		{Cutoff: now, Limit: 1, CalDAVDeleted: -1},
		{Cutoff: now, Limit: 1, CardDAVCandidates: -1},
		{Cutoff: now, Limit: 1, CardDAVDeleted: -1},
	}
	for _, record := range tests {
		record := record
		t.Run(record.Status.String(), func(t *testing.T) {
			t.Parallel()

			if _, err := normalizeRunRecord(record, func() time.Time { return now }); err == nil {
				t.Fatalf("normalizeRunRecord(%+v) error = nil, want rejection", record)
			}
		})
	}
}

func (s RunStatus) String() string {
	if s == "" {
		return "empty"
	}
	return string(s)
}
