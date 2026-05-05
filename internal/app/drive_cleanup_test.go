package app

import (
	"context"
	"errors"
	"testing"

	"github.com/gogomail/gogomail/internal/drive"
)

type fakeDriveCleanupRunner struct {
	req    drive.ListObjectCleanupFailuresRequest
	result drive.RetryObjectCleanupFailuresResult
	err    error
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
