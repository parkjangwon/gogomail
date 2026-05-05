package storage

import (
	"context"
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

	if err := store.Delete(ctx, path); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := store.Get(ctx, path); err == nil {
		t.Fatal("Get succeeded after Delete")
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
		if err := store.Delete(context.Background(), objectPath); err == nil {
			t.Fatalf("Delete accepted ambiguous object key %q", objectPath)
		}
	}
}
