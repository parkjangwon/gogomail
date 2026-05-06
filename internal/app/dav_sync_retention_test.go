package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/caldavgw"
	"github.com/gogomail/gogomail/internal/carddavgw"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/davsyncretention"
)

type fakeCalDAVSyncRetentionRunner struct {
	lastRequest caldavgw.PruneCalendarSyncChangesRequest
	result      caldavgw.CalendarSyncChangePruneResult
	err         error
}

func (f *fakeCalDAVSyncRetentionRunner) PruneCalendarSyncChanges(_ context.Context, req caldavgw.PruneCalendarSyncChangesRequest) (caldavgw.CalendarSyncChangePruneResult, error) {
	f.lastRequest = req
	return f.result, f.err
}

type fakeCardDAVSyncRetentionRunner struct {
	lastRequest carddavgw.PruneAddressBookChangesRequest
	result      carddavgw.AddressBookChangePruneResult
	err         error
}

func (f *fakeCardDAVSyncRetentionRunner) PruneAddressBookChanges(_ context.Context, req carddavgw.PruneAddressBookChangesRequest) (carddavgw.AddressBookChangePruneResult, error) {
	f.lastRequest = req
	return f.result, f.err
}

type fakeDAVSyncRetentionAuditRecorder struct {
	records []davsyncretention.RunRecord
}

func (f *fakeDAVSyncRetentionAuditRecorder) RecordRun(_ context.Context, record davsyncretention.RunRecord) (davsyncretention.RunRecord, error) {
	record.ID = "dav-sync-retention-test"
	f.records = append(f.records, record)
	return record, nil
}

func TestRunDAVSyncRetentionOnceBuildsGuardedRequests(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	calRunner := &fakeCalDAVSyncRetentionRunner{result: caldavgw.CalendarSyncChangePruneResult{
		CandidateCount: 7,
		DeletedCount:   0,
	}}
	cardRunner := &fakeCardDAVSyncRetentionRunner{result: carddavgw.AddressBookChangePruneResult{
		CandidateCount: 11,
		DeletedCount:   0,
	}}
	auditRecorder := &fakeDAVSyncRetentionAuditRecorder{}
	cfg := config.Config{
		DAVSyncRetentionCutoffAge:    14 * 24 * time.Hour,
		DAVSyncRetentionBatchSize:    500,
		DAVSyncRetentionDryRun:       true,
		DAVSyncRetentionConfirmReady: false,
	}

	result, err := runDAVSyncRetentionOnce(context.Background(), davSyncRetentionRunners{
		CalDAV:  calRunner,
		CardDAV: cardRunner,
		Audit:   auditRecorder,
	}, func() time.Time {
		return now
	}, cfg, nil)
	if err != nil {
		t.Fatalf("runDAVSyncRetentionOnce returned error: %v", err)
	}
	wantCutoff := now.Add(-14 * 24 * time.Hour)
	if !calRunner.lastRequest.Cutoff.Equal(wantCutoff) || !cardRunner.lastRequest.Cutoff.Equal(wantCutoff) {
		t.Fatalf("cutoffs = %s/%s, want %s", calRunner.lastRequest.Cutoff, cardRunner.lastRequest.Cutoff, wantCutoff)
	}
	if calRunner.lastRequest.Limit != 500 || cardRunner.lastRequest.Limit != 500 || !calRunner.lastRequest.DryRun || !cardRunner.lastRequest.DryRun {
		t.Fatalf("requests = %+v / %+v", calRunner.lastRequest, cardRunner.lastRequest)
	}
	if result.CalCandidates != 7 || result.CardCandidates != 11 || result.CalDeleted != 0 || result.CardDeleted != 0 || !result.DryRun {
		t.Fatalf("result = %+v", result)
	}
	if result.RunID != "dav-sync-retention-test" || result.Status != davsyncretention.RunStatusCompleted {
		t.Fatalf("audit result = %+v", result)
	}
	if len(auditRecorder.records) != 1 {
		t.Fatalf("audit records = %+v, want one record", auditRecorder.records)
	}
	record := auditRecorder.records[0]
	if !record.Cutoff.Equal(wantCutoff) || record.Limit != 500 || !record.DryRun || record.ConfirmReady || record.Status != davsyncretention.RunStatusCompleted {
		t.Fatalf("audit record = %+v", record)
	}
	if record.CalDAVCandidates != 7 || record.CardDAVCandidates != 11 || record.CalDAVDeleted != 0 || record.CardDAVDeleted != 0 || record.ErrorMessage != "" {
		t.Fatalf("audit counts = %+v", record)
	}
}

func TestRunDAVSyncRetentionOnceRejectsUnsafeDestructiveRun(t *testing.T) {
	t.Parallel()

	_, err := runDAVSyncRetentionOnce(context.Background(), davSyncRetentionRunners{
		CalDAV:  &fakeCalDAVSyncRetentionRunner{},
		CardDAV: &fakeCardDAVSyncRetentionRunner{},
	}, time.Now, config.Config{
		DAVSyncRetentionCutoffAge:    24 * time.Hour,
		DAVSyncRetentionBatchSize:    100,
		DAVSyncRetentionDryRun:       false,
		DAVSyncRetentionConfirmReady: false,
	}, nil)
	if err == nil {
		t.Fatal("runDAVSyncRetentionOnce error = nil, want destructive confirmation rejection")
	}
}

func TestRunDAVSyncRetentionOnceAuditsPartialFailure(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	calRunner := &fakeCalDAVSyncRetentionRunner{result: caldavgw.CalendarSyncChangePruneResult{
		CandidateCount: 7,
		DeletedCount:   3,
	}}
	cardErr := errors.New("carddav prune failed\nwith detail")
	cardRunner := &fakeCardDAVSyncRetentionRunner{err: cardErr}
	auditRecorder := &fakeDAVSyncRetentionAuditRecorder{}
	result, err := runDAVSyncRetentionOnce(context.Background(), davSyncRetentionRunners{
		CalDAV:  calRunner,
		CardDAV: cardRunner,
		Audit:   auditRecorder,
	}, func() time.Time {
		return now
	}, config.Config{
		DAVSyncRetentionCutoffAge:    14 * 24 * time.Hour,
		DAVSyncRetentionBatchSize:    500,
		DAVSyncRetentionDryRun:       false,
		DAVSyncRetentionConfirmReady: true,
	}, nil)
	if !errors.Is(err, cardErr) {
		t.Fatalf("runDAVSyncRetentionOnce err = %v, want card error", err)
	}
	if result.Status != davsyncretention.RunStatusFailed || result.RunID != "dav-sync-retention-test" {
		t.Fatalf("result = %+v, want failed audited run", result)
	}
	if len(auditRecorder.records) != 1 {
		t.Fatalf("audit records = %+v, want one record", auditRecorder.records)
	}
	record := auditRecorder.records[0]
	if record.Status != davsyncretention.RunStatusFailed || record.ErrorMessage != cardErr.Error() {
		t.Fatalf("audit record failure = %+v", record)
	}
	if record.CalDAVCandidates != 7 || record.CalDAVDeleted != 3 || record.CardDAVCandidates != 0 || record.CardDAVDeleted != 0 || record.DryRun || !record.ConfirmReady {
		t.Fatalf("audit record counts = %+v", record)
	}
}
