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
	if recorder.failures[0].UserID != "user-1" || recorder.failures[0].NodeID != "node-1" || recorder.failures[0].StorageBackend != "s3" || recorder.failures[0].StoragePath != "drive/users/user-1/objects/node-1" {
		t.Fatalf("recorded failure = %+v, want cleanup context", recorder.failures[0])
	}
}

func TestRecordObjectCleanupFailureRecordsPendingObjects(t *testing.T) {
	t.Parallel()

	recorder := &recordingCleanupFailureRecorder{}
	service := NewService(nil, nil).WithObjectCleanupFailureRecorder(recorder)
	deleted := PermanentDeleteResult{
		Root: Node{ID: "node-1", UserID: "user-1"},
	}
	err := service.recordObjectCleanupFailure(context.Background(), deleted, ObjectCleanupError{
		StorageBackend: "s3",
		StoragePath:    "drive/users/user-1/objects/node-2",
		Pending: []DeletedObject{
			{StorageBackend: "s3", StoragePath: "drive/users/user-1/objects/node-2"},
			{StorageBackend: "local", StoragePath: "drive/users/user-1/objects/node-3"},
		},
		Err: errors.New("delete failed"),
	})
	if err != nil {
		t.Fatalf("recordObjectCleanupFailure returned error: %v", err)
	}
	if recorder.calls != 2 {
		t.Fatalf("recorder calls = %d, want 2", recorder.calls)
	}
	want := []ObjectCleanupFailure{
		{UserID: "user-1", NodeID: "node-1", StorageBackend: "s3", StoragePath: "drive/users/user-1/objects/node-2", LastError: "delete failed"},
		{UserID: "user-1", NodeID: "node-1", StorageBackend: "local", StoragePath: "drive/users/user-1/objects/node-3", LastError: "delete failed"},
	}
	for i := range want {
		if recorder.failures[i] != want[i] {
			t.Fatalf("recorded failure[%d] = %+v, want %+v", i, recorder.failures[i], want[i])
		}
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

func TestRecordCopiedObjectCleanupFailureUsesCopiedObjectContext(t *testing.T) {
	t.Parallel()

	recorder := &recordingCleanupFailureRecorder{}
	service := NewService(nil, nil).WithObjectCleanupFailureRecorder(recorder)
	err := service.recordCopiedObjectCleanupFailure(context.Background(), "user-1", "s3", "drive/users/user-1/objects/copy-1", errors.New("delete copied object failed"))
	if err != nil {
		t.Fatalf("recordCopiedObjectCleanupFailure returned error: %v", err)
	}
	if recorder.calls != 1 {
		t.Fatalf("recorder calls = %d, want 1", recorder.calls)
	}
	if recorder.failures[0].UserID != "user-1" || recorder.failures[0].NodeID != "" || recorder.failures[0].StorageBackend != "s3" || recorder.failures[0].StoragePath != "drive/users/user-1/objects/copy-1" {
		t.Fatalf("recorded copied failure = %+v, want copied object context without node id", recorder.failures[0])
	}
	if recorder.failures[0].LastError != "delete copied object failed" {
		t.Fatalf("last error = %q", recorder.failures[0].LastError)
	}
}

func TestRetryObjectCleanupFailuresDeletesAndResolvesPendingObjects(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	queue := &recordingCleanupFailureStore{
		failures: []ObjectCleanupFailure{{
			ID:             "failure-1",
			UserID:         "user-1",
			NodeID:         "node-1",
			StorageBackend: "s3",
			StoragePath:    "drive/users/user-1/objects/node-1",
		}},
	}
	service := NewService(nil, map[string]storage.Store{"s3": store}).WithObjectCleanupFailureStore(queue)

	result, err := service.RetryObjectCleanupFailures(context.Background(), ListObjectCleanupFailuresRequest{Limit: 25})
	if err != nil {
		t.Fatalf("RetryObjectCleanupFailures returned error: %v", err)
	}
	if result.Scanned != 1 || result.Deleted != 1 || result.Resolved != 1 || result.Failed != 0 {
		t.Fatalf("result = %+v, want one resolved delete", result)
	}
	if queue.listReq.Status != ObjectCleanupFailureStatusPending || queue.listReq.Limit != 25 {
		t.Fatalf("list request = %+v, want forced pending with caller limit", queue.listReq)
	}
	if queue.resolvedID != "failure-1" {
		t.Fatalf("resolvedID = %q, want failure-1", queue.resolvedID)
	}
}

func TestRetryObjectCleanupFailuresRecordsRetryFailureAndContinues(t *testing.T) {
	t.Parallel()

	store := &recordingStore{deleteErr: errors.New("still missing")}
	queue := &recordingCleanupFailureStore{
		failures: []ObjectCleanupFailure{{
			ID:             "failure-1",
			UserID:         "user-1",
			NodeID:         "node-1",
			StorageBackend: "s3",
			StoragePath:    "drive/users/user-1/objects/node-1",
		}},
	}
	service := NewService(nil, map[string]storage.Store{"s3": store}).WithObjectCleanupFailureStore(queue)

	result, err := service.RetryObjectCleanupFailures(context.Background(), ListObjectCleanupFailuresRequest{})
	if err == nil || !strings.Contains(err.Error(), "1 failures remain") {
		t.Fatalf("RetryObjectCleanupFailures err = %v, want remaining failure", err)
	}
	if result.Scanned != 1 || result.Failed != 1 || result.Resolved != 0 {
		t.Fatalf("result = %+v, want one failed retry", result)
	}
	if queue.recorded.UserID != "user-1" || queue.recorded.NodeID != "node-1" || queue.recorded.StoragePath != "drive/users/user-1/objects/node-1" {
		t.Fatalf("recorded retry failure = %+v, want failed object context", queue.recorded)
	}
}

func TestRetryObjectCleanupFailuresRejectsWrongUserObjectPath(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	queue := &recordingCleanupFailureStore{
		failures: []ObjectCleanupFailure{{
			ID:             "failure-1",
			UserID:         "user-1",
			NodeID:         "node-1",
			StorageBackend: "s3",
			StoragePath:    "drive/users/user-2/objects/node-1",
		}},
	}
	service := NewService(nil, map[string]storage.Store{"s3": store}).WithObjectCleanupFailureStore(queue)

	result, err := service.RetryObjectCleanupFailures(context.Background(), ListObjectCleanupFailuresRequest{})
	if err == nil || !strings.Contains(err.Error(), "1 failures remain") {
		t.Fatalf("RetryObjectCleanupFailures err = %v, want remaining failure", err)
	}
	if result.Scanned != 1 || result.Deleted != 0 || result.Failed != 1 || result.Resolved != 0 {
		t.Fatalf("result = %+v, want one validation failure without delete", result)
	}
	if len(store.deleted) != 0 {
		t.Fatalf("deleted paths = %v, want no delete for wrong-user path", store.deleted)
	}
	if queue.resolvedID != "" {
		t.Fatalf("resolvedID = %q, want unresolved validation failure", queue.resolvedID)
	}
}

func TestValidateDeletedObjectsBelongToUser(t *testing.T) {
	t.Parallel()

	err := validateDeletedObjectsBelongToUser("user-1", []DeletedObject{{StoragePath: "drive/users/user-1/objects/node-1"}})
	if err != nil {
		t.Fatalf("validateDeletedObjectsBelongToUser returned error: %v", err)
	}
	err = validateDeletedObjectsBelongToUser("user-1", []DeletedObject{{StoragePath: "drive/users/user-2/objects/node-1"}})
	if err == nil {
		t.Fatal("validateDeletedObjectsBelongToUser accepted another user's object")
	}
}

func TestRetryObjectCleanupFailuresRequiresStore(t *testing.T) {
	t.Parallel()

	_, err := (*Service)(nil).RetryObjectCleanupFailures(context.Background(), ListObjectCleanupFailuresRequest{})
	if err == nil || !strings.Contains(err.Error(), "drive cleanup failure store is required") {
		t.Fatalf("RetryObjectCleanupFailures err = %v, want store rejection", err)
	}
}

type recordingCleanupFailureRecorder struct {
	calls    int
	failures []ObjectCleanupFailure
	err      error
}

func (r *recordingCleanupFailureRecorder) RecordObjectCleanupFailure(_ context.Context, failure ObjectCleanupFailure) (ObjectCleanupFailure, error) {
	r.calls++
	r.failures = append(r.failures, failure)
	if r.err != nil {
		return ObjectCleanupFailure{}, r.err
	}
	return failure, nil
}

type recordingCleanupFailureStore struct {
	failures   []ObjectCleanupFailure
	listReq    ListObjectCleanupFailuresRequest
	resolvedID string
	recorded   ObjectCleanupFailure
}

func (s *recordingCleanupFailureStore) ListObjectCleanupFailures(_ context.Context, req ListObjectCleanupFailuresRequest) ([]ObjectCleanupFailure, error) {
	s.listReq = req
	return s.failures, nil
}

func (s *recordingCleanupFailureStore) ResolveObjectCleanupFailure(_ context.Context, req ResolveObjectCleanupFailureRequest) (ObjectCleanupFailure, error) {
	s.resolvedID = req.ID
	return ObjectCleanupFailure{ID: req.ID, Status: ObjectCleanupFailureStatusResolved}, nil
}

func (s *recordingCleanupFailureStore) RecordObjectCleanupFailure(_ context.Context, failure ObjectCleanupFailure) (ObjectCleanupFailure, error) {
	s.recorded = failure
	return failure, nil
}
