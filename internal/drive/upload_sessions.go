package drive

import (
	"fmt"
	"strings"
	"time"
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
