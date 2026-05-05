package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestDeletePrefixDeletesOneBoundedPage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewLocalStore(t.TempDir())
	for path, body := range map[string]string{
		"drive/user-1/docs/a.txt":  "a",
		"drive/user-1/docs/b.txt":  "bb",
		"drive/user-1/media/c.txt": "ccc",
		"mailstore/user-1/msg.eml": "mail",
	} {
		if err := store.Put(ctx, path, strings.NewReader(body)); err != nil {
			t.Fatalf("Put(%q) returned error: %v", path, err)
		}
	}

	first, err := DeletePrefix(ctx, store, DeletePrefixOptions{Prefix: "drive/user-1", Limit: 2})
	if err != nil {
		t.Fatalf("DeletePrefix returned error: %v", err)
	}
	if first.Deleted != 2 || !first.HasMore || first.NextCursor == "" {
		t.Fatalf("first result = %+v, want two deletes and next cursor", first)
	}
	if _, err := store.Get(ctx, "drive/user-1/docs/a.txt"); err == nil {
		t.Fatal("first object still exists after DeletePrefix")
	}
	if _, err := store.Get(ctx, "drive/user-1/docs/b.txt"); err == nil {
		t.Fatal("second object still exists after DeletePrefix")
	}
	if _, err := store.Get(ctx, "drive/user-1/media/c.txt"); err != nil {
		t.Fatalf("third object missing before next page: %v", err)
	}
	if _, err := store.Get(ctx, "mailstore/user-1/msg.eml"); err != nil {
		t.Fatalf("outside-prefix object missing: %v", err)
	}

	next, err := DeletePrefix(ctx, store, DeletePrefixOptions{Prefix: "drive/user-1", Limit: 2, Cursor: first.NextCursor})
	if err != nil {
		t.Fatalf("DeletePrefix next page returned error: %v", err)
	}
	if next.Deleted != 1 || next.HasMore || next.NextCursor != "" {
		t.Fatalf("next result = %+v, want final delete", next)
	}
	if _, err := store.Get(ctx, "drive/user-1/media/c.txt"); err == nil {
		t.Fatal("third object still exists after next DeletePrefix")
	}
	if _, err := store.Get(ctx, "mailstore/user-1/msg.eml"); err != nil {
		t.Fatalf("outside-prefix object missing after next page: %v", err)
	}
}

func TestDeletePrefixRejectsUnsafeOrEmptyPrefix(t *testing.T) {
	t.Parallel()

	store := NewLocalStore(t.TempDir())
	for _, prefix := range []string{"", "../bad", "drive//user-1", "drive/user-1\nbad"} {
		prefix := prefix
		t.Run(fmt.Sprintf("%q", prefix), func(t *testing.T) {
			t.Parallel()

			if _, err := DeletePrefix(context.Background(), store, DeletePrefixOptions{Prefix: prefix}); err == nil {
				t.Fatalf("DeletePrefix accepted prefix %q", prefix)
			}
		})
	}
}

func TestDeletePrefixReportsDeleteFailureWithProgress(t *testing.T) {
	t.Parallel()

	store := &deleteFailingStore{
		page: ObjectListPage{Objects: []ObjectInfo{
			{Path: "drive/user-1/a.txt"},
			{Path: "drive/user-1/b.txt"},
		}},
		failPath: "drive/user-1/b.txt",
		err:      errors.New("permission denied"),
	}
	result, err := DeletePrefix(context.Background(), store, DeletePrefixOptions{Prefix: "drive/user-1"})
	if err == nil || !strings.Contains(err.Error(), "drive/user-1/b.txt") {
		t.Fatalf("DeletePrefix err = %v, want failing object path", err)
	}
	if result.Deleted != 1 {
		t.Fatalf("Deleted = %d, want progress count before failure", result.Deleted)
	}
}

func TestDeletePrefixHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := DeletePrefix(ctx, NewLocalStore(t.TempDir()), DeletePrefixOptions{Prefix: "drive/user-1"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("DeletePrefix err = %v, want context.Canceled", err)
	}
}

type deleteFailingStore struct {
	page     ObjectListPage
	failPath string
	err      error
}

func (s *deleteFailingStore) Put(context.Context, string, io.Reader) error {
	return nil
}

func (s *deleteFailingStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, nil
}

func (s *deleteFailingStore) Stat(context.Context, string) (ObjectInfo, error) {
	return ObjectInfo{}, nil
}

func (s *deleteFailingStore) Copy(context.Context, string, string) error {
	return nil
}

func (s *deleteFailingStore) Move(context.Context, string, string) error {
	return nil
}

func (s *deleteFailingStore) List(context.Context, ListOptions) (ObjectListPage, error) {
	return s.page, nil
}

func (s *deleteFailingStore) Delete(_ context.Context, path string) error {
	if path == s.failPath {
		return s.err
	}
	return nil
}
