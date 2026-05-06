package storage

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestLocalStorePortabilityContract(t *testing.T) {
	t.Parallel()

	assertStorePortabilityContract(t, NewLocalStore(t.TempDir()), "contract/local")
}

func assertStorePortabilityContract(t *testing.T, store Store, basePrefix string) {
	t.Helper()

	ctx := context.Background()
	basePrefix = strings.Trim(basePrefix, "/")
	if basePrefix == "" {
		t.Fatal("basePrefix is required")
	}
	defer func() {
		_, _ = DeletePrefix(ctx, store, DeletePrefixOptions{Prefix: basePrefix, Limit: 100})
	}()

	parentPrefix := basePrefix + "/tenant+1/user@example.com/Inbox and Projects"
	objectPath := parentPrefix + "/message=001.eml"
	relatedPath := parentPrefix + "/attachment+1.bin"
	copyPath := basePrefix + "/tenant+1/user@example.com/Copy/message+001-copy.eml"
	movePath := basePrefix + "/tenant+1/user@example.com/Archive/message moved.eml"
	body := "Subject: portability\r\n\r\nhello backend"
	relatedBody := "attachment bytes"

	if err := store.Put(ctx, objectPath, strings.NewReader(body)); err != nil {
		t.Fatalf("Put primary object returned error: %v", err)
	}
	if err := store.Put(ctx, relatedPath, strings.NewReader(relatedBody)); err != nil {
		t.Fatalf("Put related object returned error: %v", err)
	}
	if got := readStoreObject(t, store, objectPath); got != body {
		t.Fatalf("Get body = %q, want %q", got, body)
	}

	offset := int64(strings.Index(body, "portability"))
	ranged, err := store.GetRange(ctx, objectPath, RangeRequest{Offset: offset, Length: int64(len("portability"))})
	if err != nil {
		t.Fatalf("GetRange returned error: %v", err)
	}
	rangeBody, err := io.ReadAll(ranged)
	if err != nil {
		t.Fatalf("read range body: %v", err)
	}
	if err := ranged.Close(); err != nil {
		t.Fatalf("close range body: %v", err)
	}
	if string(rangeBody) != "portability" {
		t.Fatalf("range body = %q, want portability", rangeBody)
	}

	info, err := store.Stat(ctx, objectPath)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if info.Path != objectPath || info.Size != int64(len(body)) {
		t.Fatalf("object info = %+v", info)
	}

	page, err := store.List(ctx, ListOptions{Prefix: parentPrefix, Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if !listContainsPath(page, objectPath) || !listContainsPath(page, relatedPath) {
		t.Fatalf("List page = %+v, want primary and related objects", page)
	}

	if err := store.Copy(ctx, objectPath, copyPath); err != nil {
		t.Fatalf("Copy returned error: %v", err)
	}
	if got := readStoreObject(t, store, copyPath); got != body {
		t.Fatalf("copied body = %q, want %q", got, body)
	}
	if err := store.Move(ctx, copyPath, movePath); err != nil {
		t.Fatalf("Move returned error: %v", err)
	}
	if _, err := store.Get(ctx, copyPath); err == nil {
		t.Fatal("Get copied object succeeded after Move")
	}
	if got := readStoreObject(t, store, movePath); got != body {
		t.Fatalf("moved body = %q, want %q", got, body)
	}

	if err := store.Delete(ctx, objectPath); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if err := store.Delete(ctx, objectPath); err != nil {
		t.Fatalf("idempotent Delete returned error: %v", err)
	}

	result, err := DeletePrefix(ctx, store, DeletePrefixOptions{Prefix: basePrefix, Limit: 100})
	if err != nil {
		t.Fatalf("DeletePrefix returned error: %v", err)
	}
	if result.Deleted != 2 || result.HasMore || result.NextCursor != "" {
		t.Fatalf("DeletePrefix result = %+v, want two remaining objects deleted", result)
	}
	empty, err := store.List(ctx, ListOptions{Prefix: basePrefix, Limit: 10})
	if err != nil {
		t.Fatalf("List after DeletePrefix returned error: %v", err)
	}
	if len(empty.Objects) != 0 || empty.HasMore {
		t.Fatalf("List after DeletePrefix = %+v, want empty page", empty)
	}
}

func readStoreObject(t *testing.T, store Store, path string) string {
	t.Helper()

	body, err := store.Get(context.Background(), path)
	if err != nil {
		t.Fatalf("Get(%q) returned error: %v", path, err)
	}
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	if err := body.Close(); err != nil {
		t.Fatalf("close %q: %v", path, err)
	}
	return string(got)
}

func listContainsPath(page ObjectListPage, path string) bool {
	for _, object := range page.Objects {
		if object.Path == path {
			return true
		}
	}
	return false
}
