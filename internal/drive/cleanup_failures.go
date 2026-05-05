package drive

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogomail/gogomail/internal/storage"
)

const maxObjectCleanupErrorBytes = 2048

type ObjectCleanupFailure struct {
	ID             string
	UserID         string
	NodeID         string
	StorageBackend string
	StoragePath    string
	Status         string
	Attempts       int
	LastError      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (r *Repository) RecordObjectCleanupFailure(ctx context.Context, failure ObjectCleanupFailure) (ObjectCleanupFailure, error) {
	if r == nil || r.db == nil {
		return ObjectCleanupFailure{}, fmt.Errorf("database handle is required")
	}
	failure, err := ValidateObjectCleanupFailure(failure)
	if err != nil {
		return ObjectCleanupFailure{}, err
	}
	const query = `
INSERT INTO drive_object_cleanup_failures (
  user_id,
  node_id,
  storage_backend,
  storage_path,
  last_error
)
VALUES ($1::uuid, NULLIF($2, '')::uuid, $3, $4, $5)
ON CONFLICT (storage_backend, storage_path)
WHERE status = 'pending'
DO UPDATE SET
  user_id = EXCLUDED.user_id,
  node_id = EXCLUDED.node_id,
  attempts = drive_object_cleanup_failures.attempts + 1,
  last_error = EXCLUDED.last_error,
  updated_at = now()
RETURNING
  id::text,
  user_id::text,
  COALESCE(node_id::text, ''),
  storage_backend,
  storage_path,
  status,
  attempts,
  last_error,
  created_at,
  updated_at`
	var recorded ObjectCleanupFailure
	err = r.db.QueryRowContext(
		ctx,
		query,
		failure.UserID,
		failure.NodeID,
		failure.StorageBackend,
		failure.StoragePath,
		failure.LastError,
	).Scan(
		&recorded.ID,
		&recorded.UserID,
		&recorded.NodeID,
		&recorded.StorageBackend,
		&recorded.StoragePath,
		&recorded.Status,
		&recorded.Attempts,
		&recorded.LastError,
		&recorded.CreatedAt,
		&recorded.UpdatedAt,
	)
	if err != nil {
		return ObjectCleanupFailure{}, fmt.Errorf("record drive object cleanup failure: %w", err)
	}
	return recorded, nil
}

func ValidateObjectCleanupFailure(failure ObjectCleanupFailure) (ObjectCleanupFailure, error) {
	userID, err := validateDriveID("user_id", failure.UserID, true)
	if err != nil {
		return ObjectCleanupFailure{}, err
	}
	nodeID, err := validateDriveID("node_id", failure.NodeID, false)
	if err != nil {
		return ObjectCleanupFailure{}, err
	}
	storageBackend, err := validateStorageBackend(failure.StorageBackend)
	if err != nil {
		return ObjectCleanupFailure{}, err
	}
	storagePath, err := storage.ValidateObjectPath(failure.StoragePath)
	if err != nil {
		return ObjectCleanupFailure{}, fmt.Errorf("unsafe storage path %q: %w", failure.StoragePath, err)
	}
	lastError := strings.TrimSpace(failure.LastError)
	if lastError == "" {
		return ObjectCleanupFailure{}, fmt.Errorf("last_error is required")
	}
	if strings.ContainsAny(lastError, "\r\n") {
		lastError = strings.NewReplacer("\r", " ", "\n", " ").Replace(lastError)
	}
	if len(lastError) > maxObjectCleanupErrorBytes {
		lastError = truncateDriveString(lastError, maxObjectCleanupErrorBytes)
	}
	return ObjectCleanupFailure{
		UserID:         userID,
		NodeID:         nodeID,
		StorageBackend: storageBackend,
		StoragePath:    storagePath,
		LastError:      lastError,
	}, nil
}

func truncateDriveString(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for len(value) > 0 && !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}
