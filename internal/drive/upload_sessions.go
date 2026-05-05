package drive

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/storage"
)

const (
	UploadSessionStatusPending   = "pending"
	UploadSessionStatusUploading = "uploading"
	UploadSessionStatusFinalized = "finalized"
	UploadSessionStatusCanceled  = "canceled"
	UploadSessionStatusExpired   = "expired"
	UploadSessionStatusFailed    = "failed"

	DefaultUploadSessionTTL = 24 * time.Hour
	MaxUploadSessionTTL     = 7 * 24 * time.Hour
	MaxUploadSessionBytes   = 5 << 30

	UploadSessionCleanupDefaultLimit = 100
	UploadSessionCleanupMaxLimit     = 1000
)

type UploadSession struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	ParentID       string    `json:"parent_id,omitempty"`
	UploadID       string    `json:"upload_id"`
	Name           string    `json:"name"`
	DeclaredSize   int64     `json:"declared_size"`
	ReceivedSize   int64     `json:"received_size"`
	MIMEType       string    `json:"mime_type"`
	Status         string    `json:"status"`
	StorageBackend string    `json:"storage_backend"`
	StoragePath    string    `json:"storage_path,omitempty"`
	ChecksumSHA256 string    `json:"checksum_sha256,omitempty"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	FinalizedAt    time.Time `json:"finalized_at,omitempty"`
	CanceledAt     time.Time `json:"canceled_at,omitempty"`
}

type CreateUploadSessionRequest struct {
	UserID         string
	ParentID       string
	UploadID       string
	Name           string
	DeclaredSize   int64
	MIMEType       string
	StorageBackend string
	ExpiresAt      time.Time
}

type GetUploadSessionRequest struct {
	UserID    string
	SessionID string
}

type CancelUploadSessionRequest struct {
	UserID    string
	SessionID string
}

type StoreUploadSessionBodyRequest struct {
	UserID                 string
	SessionID              string
	ExpectedChecksumSHA256 string
	Body                   io.Reader
}

type FinalizeUploadSessionRequest struct {
	UserID    string
	SessionID string
}

type ExpireUploadSessionsRequest struct {
	Before time.Time
	Limit  int
}

type RecordUploadSessionBodyRequest struct {
	UserID         string
	SessionID      string
	ReceivedSize   int64
	StoragePath    string
	ChecksumSHA256 string
}

func NewUploadID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate drive upload id: %w", err)
	}
	return "upload-" + hex.EncodeToString(random[:]), nil
}

func ValidateCreateUploadSessionRequest(req CreateUploadSessionRequest, now time.Time) (CreateUploadSessionRequest, error) {
	userID, err := validateDriveID("user_id", req.UserID, true)
	if err != nil {
		return CreateUploadSessionRequest{}, err
	}
	parentID, err := validateDriveID("parent_id", req.ParentID, false)
	if err != nil {
		return CreateUploadSessionRequest{}, err
	}
	uploadID, err := validateDriveObjectPathID("upload_id", req.UploadID)
	if err != nil {
		return CreateUploadSessionRequest{}, err
	}
	name, err := ValidateNodeName(req.Name)
	if err != nil {
		return CreateUploadSessionRequest{}, err
	}
	if req.DeclaredSize < 0 {
		return CreateUploadSessionRequest{}, fmt.Errorf("declared_size must not be negative")
	}
	if req.DeclaredSize > MaxUploadSessionBytes {
		return CreateUploadSessionRequest{}, fmt.Errorf("declared_size exceeds %d bytes", MaxUploadSessionBytes)
	}
	mimeType, err := validateDriveMIMEType(req.MIMEType)
	if err != nil {
		return CreateUploadSessionRequest{}, err
	}
	storageBackend, err := validateStorageBackend(req.StorageBackend)
	if err != nil {
		return CreateUploadSessionRequest{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	expiresAt := req.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = now.Add(DefaultUploadSessionTTL)
	}
	if !expiresAt.After(now) {
		return CreateUploadSessionRequest{}, fmt.Errorf("expires_at must be in the future")
	}
	if expiresAt.After(now.Add(MaxUploadSessionTTL)) {
		return CreateUploadSessionRequest{}, fmt.Errorf("expires_at exceeds maximum upload session TTL")
	}
	return CreateUploadSessionRequest{
		UserID:         userID,
		ParentID:       parentID,
		UploadID:       uploadID,
		Name:           name,
		DeclaredSize:   req.DeclaredSize,
		MIMEType:       mimeType,
		StorageBackend: storageBackend,
		ExpiresAt:      expiresAt.UTC(),
	}, nil
}

func ValidateGetUploadSessionRequest(req GetUploadSessionRequest) (GetUploadSessionRequest, error) {
	userID, sessionID, err := validateUploadSessionIdentity(req.UserID, req.SessionID)
	if err != nil {
		return GetUploadSessionRequest{}, err
	}
	return GetUploadSessionRequest{UserID: userID, SessionID: sessionID}, nil
}

func ValidateCancelUploadSessionRequest(req CancelUploadSessionRequest) (CancelUploadSessionRequest, error) {
	userID, sessionID, err := validateUploadSessionIdentity(req.UserID, req.SessionID)
	if err != nil {
		return CancelUploadSessionRequest{}, err
	}
	return CancelUploadSessionRequest{UserID: userID, SessionID: sessionID}, nil
}

func ValidateStoreUploadSessionBodyRequest(req StoreUploadSessionBodyRequest) (StoreUploadSessionBodyRequest, error) {
	userID, sessionID, err := validateUploadSessionIdentity(req.UserID, req.SessionID)
	if err != nil {
		return StoreUploadSessionBodyRequest{}, err
	}
	expectedChecksum := strings.TrimSpace(strings.ToLower(req.ExpectedChecksumSHA256))
	if expectedChecksum != "" {
		expectedChecksum, err = validateRequiredDriveChecksum("expected_checksum_sha256", expectedChecksum)
		if err != nil {
			return StoreUploadSessionBodyRequest{}, err
		}
	}
	if req.Body == nil {
		return StoreUploadSessionBodyRequest{}, fmt.Errorf("drive upload session body is required")
	}
	return StoreUploadSessionBodyRequest{UserID: userID, SessionID: sessionID, ExpectedChecksumSHA256: expectedChecksum, Body: req.Body}, nil
}

func ValidateFinalizeUploadSessionRequest(req FinalizeUploadSessionRequest) (FinalizeUploadSessionRequest, error) {
	userID, sessionID, err := validateUploadSessionIdentity(req.UserID, req.SessionID)
	if err != nil {
		return FinalizeUploadSessionRequest{}, err
	}
	return FinalizeUploadSessionRequest{UserID: userID, SessionID: sessionID}, nil
}

func ValidateExpireUploadSessionsRequest(req ExpireUploadSessionsRequest) (ExpireUploadSessionsRequest, error) {
	if req.Before.IsZero() {
		return ExpireUploadSessionsRequest{}, fmt.Errorf("before is required")
	}
	if req.Limit < 0 {
		return ExpireUploadSessionsRequest{}, fmt.Errorf("limit must not be negative")
	}
	return ExpireUploadSessionsRequest{
		Before: req.Before.UTC(),
		Limit:  NormalizeUploadSessionCleanupLimit(req.Limit),
	}, nil
}

func NormalizeUploadSessionCleanupLimit(limit int) int {
	if limit <= 0 {
		return UploadSessionCleanupDefaultLimit
	}
	if limit > UploadSessionCleanupMaxLimit {
		return UploadSessionCleanupMaxLimit
	}
	return limit
}

func ValidateRecordUploadSessionBodyRequest(req RecordUploadSessionBodyRequest) (RecordUploadSessionBodyRequest, error) {
	userID, sessionID, err := validateUploadSessionIdentity(req.UserID, req.SessionID)
	if err != nil {
		return RecordUploadSessionBodyRequest{}, err
	}
	if req.ReceivedSize < 0 {
		return RecordUploadSessionBodyRequest{}, fmt.Errorf("received_size must not be negative")
	}
	storagePath := strings.TrimSpace(req.StoragePath)
	if storagePath == "" {
		return RecordUploadSessionBodyRequest{}, fmt.Errorf("storage_path is required")
	}
	if _, err := storage.ValidateObjectPath(storagePath); err != nil {
		return RecordUploadSessionBodyRequest{}, fmt.Errorf("storage_path is invalid: %w", err)
	}
	checksum, err := validateRequiredDriveChecksum("checksum_sha256", req.ChecksumSHA256)
	if err != nil {
		return RecordUploadSessionBodyRequest{}, err
	}
	return RecordUploadSessionBodyRequest{
		UserID:         userID,
		SessionID:      sessionID,
		ReceivedSize:   req.ReceivedSize,
		StoragePath:    storagePath,
		ChecksumSHA256: checksum,
	}, nil
}

func validateUploadSessionIdentity(userIDValue string, sessionIDValue string) (string, string, error) {
	userID, err := validateDriveID("user_id", userIDValue, true)
	if err != nil {
		return "", "", err
	}
	sessionID, err := validateDriveID("session_id", sessionIDValue, true)
	if err != nil {
		return "", "", err
	}
	return userID, sessionID, nil
}

func validateRequiredDriveChecksum(field string, value string) (string, error) {
	value, err := validateDriveChecksum(value)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	return value, nil
}

func ValidateUploadSessionStatus(status string) (string, error) {
	status = strings.TrimSpace(strings.ToLower(status))
	switch status {
	case UploadSessionStatusPending,
		UploadSessionStatusUploading,
		UploadSessionStatusFinalized,
		UploadSessionStatusCanceled,
		UploadSessionStatusExpired,
		UploadSessionStatusFailed:
		return status, nil
	default:
		return "", fmt.Errorf("unsupported drive upload session status %q", status)
	}
}
