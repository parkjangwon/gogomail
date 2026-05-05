package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStorePutGetDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewLocalStore(t.TempDir())
	path := "mailstore/company/domain/user/2026/05/message.eml"

	if err := store.Put(ctx, path, strings.NewReader("Subject: hello\r\n\r\nbody")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}

	body, err := store.Get(ctx, path)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	defer body.Close()

	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(got) != "Subject: hello\r\n\r\nbody" {
		t.Fatalf("stored body = %q", string(got))
	}
	info, err := store.Stat(ctx, path)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if info.Path != path || info.Size != int64(len("Subject: hello\r\n\r\nbody")) || info.LastModified.IsZero() {
		t.Fatalf("object info = %+v", info)
	}
	copyPath := "mailstore/company/domain/user/2026/05/message-copy.eml"
	if err := store.Copy(ctx, path, copyPath); err != nil {
		t.Fatalf("Copy returned error: %v", err)
	}
	copied, err := store.Get(ctx, copyPath)
	if err != nil {
		t.Fatalf("Get copied object returned error: %v", err)
	}
	copiedBody, err := io.ReadAll(copied)
	if err != nil {
		t.Fatalf("read copied object: %v", err)
	}
	if err := copied.Close(); err != nil {
		t.Fatalf("close copied object: %v", err)
	}
	if string(copiedBody) != "Subject: hello\r\n\r\nbody" {
		t.Fatalf("copied body = %q", copiedBody)
	}

	if err := store.Delete(ctx, path); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := store.Get(ctx, path); err == nil {
		t.Fatal("Get succeeded after Delete")
	}
}

func TestLocalStoreListObjectsByPrefix(t *testing.T) {
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

	page, err := store.List(ctx, ListOptions{Prefix: "drive/user-1", Limit: 2})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if !page.HasMore || page.NextCursor == "" || len(page.Objects) != 2 {
		t.Fatalf("first page = %+v, want two objects and next cursor", page)
	}
	if page.Objects[0].Path != "drive/user-1/docs/a.txt" || page.Objects[0].Size != 1 {
		t.Fatalf("first object = %+v", page.Objects[0])
	}
	if page.Objects[1].Path != "drive/user-1/docs/b.txt" || page.Objects[1].Size != 2 {
		t.Fatalf("second object = %+v", page.Objects[1])
	}

	next, err := store.List(ctx, ListOptions{Prefix: "drive/user-1", Limit: 2, Cursor: page.NextCursor})
	if err != nil {
		t.Fatalf("List next page returned error: %v", err)
	}
	if next.HasMore || next.NextCursor != "" || len(next.Objects) != 1 || next.Objects[0].Path != "drive/user-1/media/c.txt" {
		t.Fatalf("next page = %+v, want final media object", next)
	}
}

func TestLocalStoreRejectsNilPutBody(t *testing.T) {
	t.Parallel()

	store := NewLocalStore(t.TempDir())
	err := store.Put(context.Background(), "mailstore/company/domain/message.eml", nil)
	if err == nil || !strings.Contains(err.Error(), "storage body is required") {
		t.Fatalf("Put err = %v, want nil body rejection", err)
	}
}

func TestLocalStoreCheckProbesWritableStorage(t *testing.T) {
	t.Parallel()

	store := NewLocalStore(t.TempDir())
	if err := store.Check(context.Background()); err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
}

func TestReadStorageCheckBodyBoundsReadinessBody(t *testing.T) {
	t.Parallel()

	got, err := readStorageCheckBody(strings.NewReader("gogomail storage readiness\nextra"), len("gogomail storage readiness\n"))
	if err != nil {
		t.Fatalf("readStorageCheckBody returned error: %v", err)
	}
	if string(got) != "gogomail storage readiness\ne" {
		t.Fatalf("bounded check body = %q", got)
	}
}

func TestLocalStorePutUsesUniqueTemporaryObject(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewLocalStore(root)
	objectPath := "mailstore/company/domain/message.eml"
	fullPath := filepath.Join(root, filepath.FromSlash(objectPath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("create object dir: %v", err)
	}
	staleTmpPath := fullPath + ".tmp"
	if err := os.WriteFile(staleTmpPath, []byte("stale temp"), 0o644); err != nil {
		t.Fatalf("write stale fixed temp object: %v", err)
	}

	if err := store.Put(context.Background(), objectPath, strings.NewReader("fresh body")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	stale, err := os.ReadFile(staleTmpPath)
	if err != nil {
		t.Fatalf("read stale fixed temp object: %v", err)
	}
	if string(stale) != "stale temp" {
		t.Fatalf("stale fixed temp object = %q", stale)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(fullPath), "."+filepath.Base(fullPath)+".*.tmp"))
	if err != nil {
		t.Fatalf("glob temp objects: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary objects left behind: %v", matches)
	}

	body, err := store.Get(context.Background(), objectPath)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if err := body.Close(); err != nil {
		t.Fatalf("close body: %v", err)
	}
	if string(got) != "fresh body" {
		t.Fatalf("stored body = %q", got)
	}
}

func TestLocalStorePutHonorsContextCancellationDuringCopy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewLocalStore(root)
	ctx, cancel := context.WithCancel(context.Background())
	body := cancelingReader{
		cancel: cancel,
		chunks: []string{"partial", " body"},
	}
	objectPath := "mailstore/company/domain/message.eml"

	err := store.Put(ctx, objectPath, &body)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Put err = %v, want context.Canceled", err)
	}
	if _, err := store.Get(context.Background(), objectPath); err == nil {
		t.Fatal("Get succeeded after canceled Put")
	}
	matches, err := filepath.Glob(filepath.Join(root, "mailstore/company/domain", ".message.eml.*.tmp"))
	if err != nil {
		t.Fatalf("glob temp objects: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary objects left behind after canceled Put: %v", matches)
	}
}

type cancelingReader struct {
	cancel context.CancelFunc
	chunks []string
	index  int
}

func (r *cancelingReader) Read(p []byte) (int, error) {
	if r.index >= len(r.chunks) {
		return 0, io.EOF
	}
	if r.index == 0 {
		r.cancel()
	}
	chunk := r.chunks[r.index]
	r.index++
	return copy(p, chunk), nil
}

func TestLocalStoreDeleteIsIdempotentForMissingObjects(t *testing.T) {
	t.Parallel()

	store := NewLocalStore(t.TempDir())
	if err := store.Delete(context.Background(), "mailstore/company/domain/missing.eml"); err != nil {
		t.Fatalf("Delete returned error for missing object: %v", err)
	}
}

func TestLocalStoreCheckReportsUnwritableStorage(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	blockingFile := filepath.Join(root, "mailstore-file")
	if err := os.WriteFile(blockingFile, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write blocking file: %v", err)
	}
	store := NewLocalStore(blockingFile)
	if err := store.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "write readiness probe") {
		t.Fatalf("Check err = %v", err)
	}
}

func TestLocalStoreRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	store := NewLocalStore(t.TempDir())

	if err := store.Put(context.Background(), "../escape.eml", strings.NewReader("bad")); err == nil {
		t.Fatal("Put accepted path traversal")
	}
}

func TestLocalStoreRejectsAmbiguousObjectKeys(t *testing.T) {
	t.Parallel()

	store := NewLocalStore(t.TempDir())
	for _, objectPath := range []string{
		"mailstore//message.eml",
		"mailstore/./message.eml",
		`mailstore\message.eml`,
		"mailstore/message\n.eml",
		"/mailstore/message.eml",
	} {
		if err := store.Put(context.Background(), objectPath, strings.NewReader("bad")); err == nil {
			t.Fatalf("Put accepted ambiguous object key %q", objectPath)
		}
		if _, err := store.Get(context.Background(), objectPath); err == nil {
			t.Fatalf("Get accepted ambiguous object key %q", objectPath)
		}
		if _, err := store.Stat(context.Background(), objectPath); err == nil {
			t.Fatalf("Stat accepted ambiguous object key %q", objectPath)
		}
		if err := store.Copy(context.Background(), objectPath, "mailstore/copy.eml"); err == nil {
			t.Fatalf("Copy accepted ambiguous source object key %q", objectPath)
		}
		if err := store.Copy(context.Background(), "mailstore/source.eml", objectPath); err == nil {
			t.Fatalf("Copy accepted ambiguous destination object key %q", objectPath)
		}
		if _, err := store.List(context.Background(), ListOptions{Prefix: objectPath}); err == nil {
			t.Fatalf("List accepted ambiguous object prefix %q", objectPath)
		}
		if err := store.Delete(context.Background(), objectPath); err == nil {
			t.Fatalf("Delete accepted ambiguous object key %q", objectPath)
		}
	}
}
