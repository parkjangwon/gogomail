package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/drive"
)

type fakeDriveCleanupRunner struct {
	req       drive.ListObjectCleanupFailuresRequest
	expireReq drive.ExpireUploadSessionsRequest
	expired   []drive.UploadSession
	result    drive.RetryObjectCleanupFailuresResult
	err       error
	expireErr error
}

func (f *fakeDriveCleanupRunner) ExpireUploadSessions(_ context.Context, req drive.ExpireUploadSessionsRequest) ([]drive.UploadSession, error) {
	f.expireReq = req
	if f.expireErr != nil {
		return nil, f.expireErr
	}
	return f.expired, nil
}

func (f *fakeDriveCleanupRunner) RetryObjectCleanupFailures(_ context.Context, req drive.ListObjectCleanupFailuresRequest) (drive.RetryObjectCleanupFailuresResult, error) {
	f.req = req
	if f.err != nil {
		return f.result, f.err
	}
	return f.result, nil
}

func TestRetryDriveObjectCleanupOnceUsesPendingBatch(t *testing.T) {
	t.Parallel()

	runner := &fakeDriveCleanupRunner{
		result: drive.RetryObjectCleanupFailuresResult{Scanned: 2, Deleted: 2, Resolved: 2},
	}
	result, err := retryDriveObjectCleanupOnce(context.Background(), runner, 25, nil)
	if err != nil {
		t.Fatalf("retryDriveObjectCleanupOnce returned error: %v", err)
	}
	if runner.req.Status != drive.ObjectCleanupFailureStatusPending || runner.req.Limit != 25 {
		t.Fatalf("request = %+v, want pending/25", runner.req)
	}
	if result.Resolved != 2 {
		t.Fatalf("result = %+v, want two resolved retries", result)
	}
}

func TestCleanupDriveOnceExpiresSessionsAndRetriesObjects(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	runner := &fakeDriveCleanupRunner{
		expired: []drive.UploadSession{{ID: "session-1"}, {ID: "session-2"}},
		result:  drive.RetryObjectCleanupFailuresResult{Scanned: 1, Resolved: 1},
	}
	result, err := cleanupDriveOnce(context.Background(), runner, func() time.Time { return now }, 25, nil)
	if err != nil {
		t.Fatalf("cleanupDriveOnce returned error: %v", err)
	}
	if !runner.expireReq.Before.Equal(now) || runner.expireReq.Limit != 25 {
		t.Fatalf("expire request = %+v, want now/25", runner.expireReq)
	}
	if runner.req.Status != drive.ObjectCleanupFailureStatusPending || runner.req.Limit != 25 {
		t.Fatalf("object cleanup request = %+v, want pending/25", runner.req)
	}
	if result.ExpiredSessions != 2 || result.ObjectCleanup.Resolved != 1 {
		t.Fatalf("result = %+v, want expired sessions and object cleanup progress", result)
	}
}

func TestRetryDriveObjectCleanupOncePropagatesProgressWithError(t *testing.T) {
	t.Parallel()

	runner := &fakeDriveCleanupRunner{
		result: drive.RetryObjectCleanupFailuresResult{Scanned: 1, Failed: 1},
		err:    errors.New("retry failed"),
	}
	result, err := retryDriveObjectCleanupOnce(context.Background(), runner, 10, nil)
	if err == nil || !errors.Is(err, runner.err) {
		t.Fatalf("retryDriveObjectCleanupOnce err = %v, want retry failed", err)
	}
	if result.Failed != 1 {
		t.Fatalf("result = %+v, want failed progress", result)
	}
}

func TestRetryDriveObjectCleanupOnceRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	if _, err := retryDriveObjectCleanupOnce(context.Background(), nil, 10, nil); err == nil {
		t.Fatal("retryDriveObjectCleanupOnce accepted nil runner")
	}
	if _, err := retryDriveObjectCleanupOnce(context.Background(), &fakeDriveCleanupRunner{}, 0, nil); err == nil {
		t.Fatal("retryDriveObjectCleanupOnce accepted zero batch size")
	}
}
