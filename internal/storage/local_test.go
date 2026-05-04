package storage

import (
	"context"
	"io"
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
