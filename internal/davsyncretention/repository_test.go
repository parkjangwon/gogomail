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
	if _, err := (&Repository{}).ListRuns(context.Background(), RunListRequest{}); err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("ListRuns err = %v, want nil database rejection", err)
	}
	if _, err := (&Repository{}).GetRun(context.Background(), "dav-sync-retention-test"); err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("GetRun err = %v, want nil database rejection", err)
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

func TestNormalizeRunListRequest(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 5, 5, 12, 0, 0, 0, time.FixedZone("KST", 9*60*60))
	to := from.Add(time.Hour)
	req, err := normalizeRunListRequest(RunListRequest{
		Limit:       MaxRunListLimit + 25,
		Status:      RunStatusFailed,
		CreatedFrom: from,
		CreatedTo:   to,
	})
	if err != nil {
		t.Fatalf("normalizeRunListRequest returned error: %v", err)
	}
	if req.Limit != MaxRunListLimit || req.Status != RunStatusFailed {
		t.Fatalf("request = %+v", req)
	}
	if req.CreatedFrom.Location() != time.UTC || req.CreatedTo.Location() != time.UTC {
		t.Fatalf("times were not normalized to UTC: %+v", req)
	}

	for _, unsafe := range []RunListRequest{
		{Limit: -1},
		{Limit: 1, Status: "blocked"},
		{Limit: 1, CreatedFrom: to, CreatedTo: from},
		{Limit: 1, CreatedFrom: from, CreatedTo: from},
	} {
		unsafe := unsafe
		t.Run(unsafe.Status.String(), func(t *testing.T) {
			t.Parallel()

			if _, err := normalizeRunListRequest(unsafe); err == nil {
				t.Fatalf("normalizeRunListRequest(%+v) error = nil, want rejection", unsafe)
			}
		})
	}
}

func TestValidateRunID(t *testing.T) {
	t.Parallel()

	id, err := validateRunID(" dav-sync-retention-1 ")
	if err != nil {
		t.Fatalf("validateRunID returned error: %v", err)
	}
	if id != "dav-sync-retention-1" {
		t.Fatalf("id = %q", id)
	}
	for _, unsafe := range []string{"", "dav-sync-retention\n1", strings.Repeat("x", maxRunIDBytes+1)} {
		unsafe := unsafe
		t.Run(unsafe, func(t *testing.T) {
			t.Parallel()

			if _, err := validateRunID(unsafe); err == nil {
				t.Fatalf("validateRunID(%q) error = nil, want rejection", unsafe)
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
