package drive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/storage"
)

func TestStoreStagedObjectStreamsToConfiguredStore(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	service := NewService(nil, map[string]storage.Store{"s3": store})
	body := "hello drive"
	staged, err := service.StoreStagedObject(context.Background(), StoreStagedObjectRequest{
		UserID:         " user-1 ",
		UploadID:       " upload-1 ",
		StorageBackend: " s3 ",
		Body:           strings.NewReader(body),
	})
	if err != nil {
		t.Fatalf("StoreStagedObject returned error: %v", err)
	}
	if staged.StoragePath != "drive/users/user-1/staging/upload-1" || staged.Size != int64(len(body)) {
		t.Fatalf("staged = %+v, want canonical path and size", staged)
	}
	sum := sha256.Sum256([]byte(body))
	if staged.ChecksumSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("checksum = %q, want sha256 body checksum", staged.ChecksumSHA256)
	}
	info, err := store.Stat(context.Background(), staged.StoragePath)
	if err != nil {
		t.Fatalf("Stat staged object: %v", err)
	}
	if info.Size != int64(len(body)) {
		t.Fatalf("stored size = %d, want %d", info.Size, len(body))
	}
}

func TestStoreStagedObjectRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	tests := []StoreStagedObjectRequest{
		{UploadID: "upload-1", StorageBackend: "s3", Body: strings.NewReader("x")},
		{UserID: "user-1", StorageBackend: "s3", Body: strings.NewReader("x")},
		{UserID: "user/1", UploadID: "upload-1", StorageBackend: "s3", Body: strings.NewReader("x")},
		{UserID: "user-1", UploadID: "upload/1", StorageBackend: "s3", Body: strings.NewReader("x")},
		{UserID: "user-1", UploadID: "upload-1", Body: strings.NewReader("x")},
		{UserID: "user-1", UploadID: "upload-1", StorageBackend: "s3"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.UploadID+"-"+tc.StorageBackend, func(t *testing.T) {
			t.Parallel()

			service := NewService(nil, map[string]storage.Store{"s3": store})
			if _, err := service.StoreStagedObject(context.Background(), tc); err == nil {
				t.Fatalf("StoreStagedObject(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestStoreStagedObjectRejectsOversizedBodyAndDeletesObject(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	service := NewService(nil, map[string]storage.Store{"s3": store})
	_, err := service.StoreStagedObject(context.Background(), StoreStagedObjectRequest{
		UserID:         "user-1",
		UploadID:       "upload-1",
		StorageBackend: "s3",
		Body:           strings.NewReader(strings.Repeat("x", int(MaxDriveStagedObjectBytes)+1)),
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("StoreStagedObject err = %v, want oversized rejection", err)
	}
	if _, statErr := store.Stat(context.Background(), "drive/users/user-1/staging/upload-1"); statErr == nil {
		t.Fatal("oversized staged object still exists after rejection")
	}
}
