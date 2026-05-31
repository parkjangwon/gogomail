package drive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

const MaxDriveStagedObjectBytes int64 = 25 << 20

type StoreStagedObjectRequest struct {
	UserID         string
	UploadID       string
	StorageBackend string
	Body           io.Reader
}

type StagedObject struct {
	UserID         string `json:"user_id"`
	UploadID       string `json:"upload_id"`
	StorageBackend string `json:"storage_backend"`
	StoragePath    string `json:"storage_path"`
	Size           int64  `json:"size"`
	ChecksumSHA256 string `json:"checksum_sha256"`
}

func (s *Service) StoreStagedObject(ctx context.Context, req StoreStagedObjectRequest) (StagedObject, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil {
		return StagedObject{}, fmt.Errorf("drive service is required")
	}
	if req.Body == nil {
		return StagedObject{}, fmt.Errorf("drive upload body is required")
	}
	userID, err := validateDriveObjectPathID("user_id", req.UserID)
	if err != nil {
		return StagedObject{}, err
	}
	uploadID, err := validateDriveObjectPathID("upload_id", req.UploadID)
	if err != nil {
		return StagedObject{}, err
	}
	storageBackend, err := validateStorageBackend(req.StorageBackend)
	if err != nil {
		return StagedObject{}, err
	}
	store := s.stores[storageBackend]
	if store == nil {
		return StagedObject{}, fmt.Errorf("storage store %q is required", storageBackend)
	}
	scope, scoped, err := s.objectPathScope(ctx, userID)
	if err != nil {
		return StagedObject{}, err
	}
	storagePath, err := buildServiceStagedObjectPath(scope, scoped, uploadID)
	if err != nil {
		return StagedObject{}, err
	}

	counter := &countingReader{reader: req.Body}
	limited := &io.LimitedReader{R: counter, N: MaxDriveStagedObjectBytes + 1}
	hash := sha256.New()
	if err := store.Put(ctx, storagePath, io.TeeReader(limited, hash)); err != nil {
		return StagedObject{}, fmt.Errorf("store staged drive object: %w", err)
	}
	if counter.bytesRead > MaxDriveStagedObjectBytes {
		s.deleteDriveObjectBestEffort(ctx, store, userID, storageBackend, storagePath, "staged_object_oversize")
		return StagedObject{}, fmt.Errorf("drive staged object exceeds %d bytes", MaxDriveStagedObjectBytes)
	}
	return StagedObject{
		UserID:         userID,
		UploadID:       uploadID,
		StorageBackend: storageBackend,
		StoragePath:    storagePath,
		Size:           counter.bytesRead,
		ChecksumSHA256: hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

type countingReader struct {
	reader    io.Reader
	bytesRead int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.bytesRead += int64(n)
	return n, err
}
