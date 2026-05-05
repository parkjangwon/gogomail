package drive

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/storage"
)

func TestCleanupDeletedObjectsDeletesEachUniqueBackendObject(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	result, err := CleanupDeletedObjects(context.Background(), map[string]storage.Store{
		"s3": store,
	}, []DeletedObject{
		{StorageBackend: " s3 ", StoragePath: "drive/user-1/a.txt"},
		{StorageBackend: "s3", StoragePath: "drive/user-1/a.txt"},
		{StorageBackend: "s3", StoragePath: "drive/user-1/b.txt"},
	})
	if err != nil {
		t.Fatalf("CleanupDeletedObjects returned error: %v", err)
	}
	if result.Deleted != 2 {
		t.Fatalf("Deleted = %d, want 2", result.Deleted)
	}
	if got := strings.Join(store.deleted, ","); got != "drive/user-1/a.txt,drive/user-1/b.txt" {
		t.Fatalf("deleted paths = %q, want both unique paths", got)
	}
}

func TestCleanupDeletedObjectsReportsProgressOnDeleteFailure(t *testing.T) {
	t.Parallel()

	store := &recordingStore{deleteErr: errors.New("boom")}
	result, err := CleanupDeletedObjects(context.Background(), map[string]storage.Store{
		"s3": store,
	}, []DeletedObject{
		{StorageBackend: "s3", StoragePath: "drive/user-1/a.txt"},
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("CleanupDeletedObjects err = %v, want delete failure", err)
	}
	var cleanupErr ObjectCleanupError
	if !errors.As(err, &cleanupErr) {
		t.Fatalf("CleanupDeletedObjects err = %T, want ObjectCleanupError", err)
	}
	if cleanupErr.StorageBackend != "s3" || cleanupErr.StoragePath != "drive/user-1/a.txt" || cleanupErr.Deleted != 0 {
		t.Fatalf("cleanup error = %+v, want failed object context", cleanupErr)
	}
	if result.Deleted != 0 {
		t.Fatalf("Deleted = %d, want 0 after first-object failure", result.Deleted)
	}
}

func TestCleanupDeletedObjectsRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		stores  map[string]storage.Store
		objects []DeletedObject
	}{
		{
			name:    "missing store map",
			objects: []DeletedObject{{StorageBackend: "s3", StoragePath: "drive/user-1/a.txt"}},
		},
		{
			name:   "missing backend store",
			stores: map[string]storage.Store{"local": &recordingStore{}},
			objects: []DeletedObject{
				{StorageBackend: "s3", StoragePath: "drive/user-1/a.txt"},
			},
		},
		{
			name:   "unsafe backend",
			stores: map[string]storage.Store{"s3": &recordingStore{}},
			objects: []DeletedObject{
				{StorageBackend: "s3\nbad", StoragePath: "drive/user-1/a.txt"},
			},
		},
		{
			name:   "unsafe path",
			stores: map[string]storage.Store{"s3": &recordingStore{}},
			objects: []DeletedObject{
				{StorageBackend: "s3", StoragePath: "../bad"},
			},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := CleanupDeletedObjects(context.Background(), tc.stores, tc.objects); err == nil {
				t.Fatalf("CleanupDeletedObjects accepted %s", tc.name)
			}
		})
	}
}

func TestCleanupDeletedObjectsHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := CleanupDeletedObjects(ctx, map[string]storage.Store{"s3": &recordingStore{}}, []DeletedObject{
		{StorageBackend: "s3", StoragePath: "drive/user-1/a.txt"},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CleanupDeletedObjects err = %v, want context.Canceled", err)
	}
}

type recordingStore struct {
	deleted   []string
	deleteErr error
}

func (s *recordingStore) Put(context.Context, string, io.Reader) error {
	return nil
}

func (s *recordingStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, nil
}

func (s *recordingStore) Stat(context.Context, string) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{}, nil
}

func (s *recordingStore) Copy(context.Context, string, string) error {
	return nil
}

func (s *recordingStore) Move(context.Context, string, string) error {
	return nil
}

func (s *recordingStore) List(context.Context, storage.ListOptions) (storage.ObjectListPage, error) {
	return storage.ObjectListPage{}, nil
}

func (s *recordingStore) Delete(_ context.Context, path string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleted = append(s.deleted, path)
	return nil
}
