package app

import (
	"context"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/caldavgw"
	"github.com/gogomail/gogomail/internal/carddavgw"
	"github.com/gogomail/gogomail/internal/config"
)

type fakeCalDAVSyncRetentionRunner struct {
	lastRequest caldavgw.PruneCalendarSyncChangesRequest
	result      caldavgw.CalendarSyncChangePruneResult
}

func (f *fakeCalDAVSyncRetentionRunner) PruneCalendarSyncChanges(_ context.Context, req caldavgw.PruneCalendarSyncChangesRequest) (caldavgw.CalendarSyncChangePruneResult, error) {
	f.lastRequest = req
	return f.result, nil
}

type fakeCardDAVSyncRetentionRunner struct {
	lastRequest carddavgw.PruneAddressBookChangesRequest
	result      carddavgw.AddressBookChangePruneResult
}

func (f *fakeCardDAVSyncRetentionRunner) PruneAddressBookChanges(_ context.Context, req carddavgw.PruneAddressBookChangesRequest) (carddavgw.AddressBookChangePruneResult, error) {
	f.lastRequest = req
	return f.result, nil
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
	cfg := config.Config{
		DAVSyncRetentionCutoffAge:    14 * 24 * time.Hour,
		DAVSyncRetentionBatchSize:    500,
		DAVSyncRetentionDryRun:       true,
		DAVSyncRetentionConfirmReady: false,
	}

	result, err := runDAVSyncRetentionOnce(context.Background(), davSyncRetentionRunners{
		CalDAV:  calRunner,
		CardDAV: cardRunner,
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
