package drive

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/storage"
)

func TestNewServiceCopiesStoreMap(t *testing.T) {
	t.Parallel()

	original := map[string]storage.Store{"s3": &recordingStore{}}
	service := NewService(nil, original)
	original["s3"] = nil

	if service.stores["s3"] == nil {
		t.Fatal("NewService store map was mutated through caller map")
	}
}

func TestPermanentDeleteNodeRequiresRepository(t *testing.T) {
	t.Parallel()

	_, err := (*Service)(nil).PermanentDeleteNode(context.Background(), PermanentDeleteNodeRequest{})
	if err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("PermanentDeleteNode err = %v, want repository rejection", err)
	}

	service := NewService(nil, map[string]storage.Store{"s3": &recordingStore{}})
	_, err = service.PermanentDeleteNode(context.Background(), PermanentDeleteNodeRequest{})
	if err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("PermanentDeleteNode err = %v, want repository rejection", err)
	}
}

func TestRecordObjectCleanupFailureUsesCleanupErrorContext(t *testing.T) {
	t.Parallel()

	recorder := &recordingCleanupFailureRecorder{}
	service := NewService(nil, nil).WithObjectCleanupFailureRecorder(recorder)
	deleted := PermanentDeleteResult{
		Root: Node{ID: "node-1", UserID: "user-1"},
	}
	err := service.recordObjectCleanupFailure(context.Background(), deleted, ObjectCleanupError{
		StorageBackend: "s3",
		StoragePath:    "drive/users/user-1/objects/node-1",
		Err:            errors.New("delete failed"),
	})
	if err != nil {
		t.Fatalf("recordObjectCleanupFailure returned error: %v", err)
	}
	if recorder.calls != 1 {
		t.Fatalf("recorder calls = %d, want 1", recorder.calls)
	}
	if recorder.failure.UserID != "user-1" || recorder.failure.NodeID != "node-1" || recorder.failure.StorageBackend != "s3" || recorder.failure.StoragePath != "drive/users/user-1/objects/node-1" {
		t.Fatalf("recorded failure = %+v, want cleanup context", recorder.failure)
	}
}

func TestRecordObjectCleanupFailureIgnoresUnstructuredErrors(t *testing.T) {
	t.Parallel()

	recorder := &recordingCleanupFailureRecorder{}
	service := NewService(nil, nil).WithObjectCleanupFailureRecorder(recorder)
	if err := service.recordObjectCleanupFailure(context.Background(), PermanentDeleteResult{}, errors.New("plain")); err != nil {
		t.Fatalf("recordObjectCleanupFailure returned error: %v", err)
	}
	if recorder.calls != 0 {
		t.Fatalf("recorder calls = %d, want 0", recorder.calls)
	}
}

type recordingCleanupFailureRecorder struct {
	calls   int
	failure ObjectCleanupFailure
	err     error
}

func (r *recordingCleanupFailureRecorder) RecordObjectCleanupFailure(_ context.Context, failure ObjectCleanupFailure) (ObjectCleanupFailure, error) {
	r.calls++
	r.failure = failure
	if r.err != nil {
		return ObjectCleanupFailure{}, r.err
	}
	return failure, nil
}
