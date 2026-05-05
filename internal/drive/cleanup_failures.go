package drive

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogomail/gogomail/internal/storage"
)

const maxObjectCleanupErrorBytes = 2048

const (
	ObjectCleanupFailureStatusPending  = "pending"
	ObjectCleanupFailureStatusResolved = "resolved"

	DefaultObjectCleanupFailureListLimit = 50
	MaxObjectCleanupFailureListLimit     = 200
)

type ObjectCleanupFailure struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	NodeID         string    `json:"node_id,omitempty"`
	StorageBackend string    `json:"storage_backend"`
	StoragePath    string    `json:"storage_path"`
	Status         string    `json:"status"`
	Attempts       int       `json:"attempts"`
	LastError      string    `json:"last_error"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ListObjectCleanupFailuresRequest struct {
	UserID string
	Status string
	Limit  int
}

type ResolveObjectCleanupFailureRequest struct {
	ID string
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

func (r *Repository) ListObjectCleanupFailures(ctx context.Context, req ListObjectCleanupFailuresRequest) ([]ObjectCleanupFailure, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateListObjectCleanupFailuresRequest(req)
	if err != nil {
		return nil, err
	}
	const query = `
SELECT
  id::text,
  user_id::text,
  COALESCE(node_id::text, ''),
  storage_backend,
  storage_path,
  status,
  attempts,
  last_error,
  created_at,
  updated_at
FROM drive_object_cleanup_failures
WHERE status = $1
  AND (NULLIF($2, '') IS NULL OR user_id = $2::uuid)
ORDER BY updated_at ASC, id ASC
LIMIT $3`
	rows, err := r.db.QueryContext(ctx, query, req.Status, req.UserID, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list drive object cleanup failures: %w", err)
	}
	defer rows.Close()

	failures := make([]ObjectCleanupFailure, 0, req.Limit)
	for rows.Next() {
		var failure ObjectCleanupFailure
		if err := rows.Scan(
			&failure.ID,
			&failure.UserID,
			&failure.NodeID,
			&failure.StorageBackend,
			&failure.StoragePath,
			&failure.Status,
			&failure.Attempts,
			&failure.LastError,
			&failure.CreatedAt,
			&failure.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan drive object cleanup failure: %w", err)
		}
		failures = append(failures, failure)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate drive object cleanup failures: %w", err)
	}
	return failures, nil
}

func (r *Repository) ResolveObjectCleanupFailure(ctx context.Context, req ResolveObjectCleanupFailureRequest) (ObjectCleanupFailure, error) {
	if r == nil || r.db == nil {
		return ObjectCleanupFailure{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateResolveObjectCleanupFailureRequest(req)
	if err != nil {
		return ObjectCleanupFailure{}, err
	}
	const query = `
UPDATE drive_object_cleanup_failures
SET status = 'resolved',
    resolved_at = COALESCE(resolved_at, now()),
    updated_at = now()
WHERE id = $1::uuid
  AND status = 'pending'
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
	var failure ObjectCleanupFailure
	err = r.db.QueryRowContext(ctx, query, req.ID).Scan(
		&failure.ID,
		&failure.UserID,
		&failure.NodeID,
		&failure.StorageBackend,
		&failure.StoragePath,
		&failure.Status,
		&failure.Attempts,
		&failure.LastError,
		&failure.CreatedAt,
		&failure.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return ObjectCleanupFailure{}, fmt.Errorf("pending drive object cleanup failure not found")
		}
		return ObjectCleanupFailure{}, fmt.Errorf("resolve drive object cleanup failure: %w", err)
	}
	return failure, nil
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

func ValidateListObjectCleanupFailuresRequest(req ListObjectCleanupFailuresRequest) (ListObjectCleanupFailuresRequest, error) {
	userID, err := validateDriveID("user_id", req.UserID, false)
	if err != nil {
		return ListObjectCleanupFailuresRequest{}, err
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = ObjectCleanupFailureStatusPending
	}
	status, err = ValidateObjectCleanupFailureStatus(status)
	if err != nil {
		return ListObjectCleanupFailuresRequest{}, err
	}
	limit := req.Limit
	if limit <= 0 {
		limit = DefaultObjectCleanupFailureListLimit
	}
	if limit > MaxObjectCleanupFailureListLimit {
		limit = MaxObjectCleanupFailureListLimit
	}
	return ListObjectCleanupFailuresRequest{UserID: userID, Status: status, Limit: limit}, nil
}

func ValidateResolveObjectCleanupFailureRequest(req ResolveObjectCleanupFailureRequest) (ResolveObjectCleanupFailureRequest, error) {
	id, err := validateDriveID("id", req.ID, true)
	if err != nil {
		return ResolveObjectCleanupFailureRequest{}, err
	}
	return ResolveObjectCleanupFailureRequest{ID: id}, nil
}

func ValidateObjectCleanupFailureStatus(status string) (string, error) {
	status = strings.TrimSpace(strings.ToLower(status))
	switch status {
	case ObjectCleanupFailureStatusPending, ObjectCleanupFailureStatusResolved:
		return status, nil
	default:
		return "", fmt.Errorf("unsupported drive object cleanup failure status %q", status)
	}
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
