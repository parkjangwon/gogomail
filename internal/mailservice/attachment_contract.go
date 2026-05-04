package mailservice

import (
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const MaxAttachmentFilenameBytes = 255
const MaxAttachmentUploadBytes int64 = 25 << 20
const MaxAttachmentUploadSessionTTL = 24 * time.Hour

type CreateAttachmentUploadRequest struct {
	UserID      string `json:"user_id"`
	DraftID     string `json:"draft_id,omitempty"`
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	MIMEType    string `json:"mime_type"`
	StoragePath string `json:"storage_path,omitempty"`
}

type UploadAttachmentRequest struct {
	UserID   string
	DraftID  string
	Filename string
	Size     int64
	MIMEType string
	Body     io.Reader
}

type CreateAttachmentUploadSessionRequest struct {
	UserID       string
	DraftID      string
	Filename     string
	DeclaredSize int64
	MIMEType     string
	ExpiresAt    time.Time
}

type StoreAttachmentUploadSessionBodyRequest struct {
	UserID                 string
	SessionID              string
	ExpectedChecksumSHA256 string
	Body                   io.Reader
}

func ValidateCreateAttachmentUploadRequest(req CreateAttachmentUploadRequest) error {
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(req.DraftID) != "" {
		if err := validateServiceResourceID("draft_id", req.DraftID); err != nil {
			return err
		}
	}
	filename := strings.TrimSpace(req.Filename)
	if filename == "" {
		return fmt.Errorf("filename is required")
	}
	if filename != filepath.Base(filename) {
		return fmt.Errorf("filename must not contain path separators")
	}
	if strings.ContainsAny(filename, "\r\n") {
		return fmt.Errorf("filename must not contain newlines")
	}
	if len(filename) > MaxAttachmentFilenameBytes {
		return fmt.Errorf("filename is too long")
	}
	if req.Size < 0 {
		return fmt.Errorf("attachment size must not be negative")
	}
	if strings.TrimSpace(req.MIMEType) == "" {
		return fmt.Errorf("mime_type is required")
	}
	if strings.ContainsAny(req.MIMEType, "\r\n") {
		return fmt.Errorf("mime_type must not contain newlines")
	}
	if err := validateAttachmentStoragePath(req.StoragePath); err != nil {
		return err
	}
	return nil
}

func ValidateUploadAttachmentRequest(req UploadAttachmentRequest) error {
	if req.Body == nil {
		return fmt.Errorf("attachment body is required")
	}
	if req.Size > MaxAttachmentUploadBytes {
		return fmt.Errorf("attachment size exceeds %d bytes", MaxAttachmentUploadBytes)
	}
	return ValidateCreateAttachmentUploadRequest(CreateAttachmentUploadRequest{
		UserID:   req.UserID,
		DraftID:  req.DraftID,
		Filename: req.Filename,
		Size:     req.Size,
		MIMEType: req.MIMEType,
	})
}

func ValidateCreateAttachmentUploadSessionRequest(req CreateAttachmentUploadSessionRequest) error {
	if req.ExpiresAt.IsZero() {
		return fmt.Errorf("expires_at is required")
	}
	if req.DeclaredSize > MaxAttachmentUploadBytes {
		return fmt.Errorf("declared_size exceeds %d bytes", MaxAttachmentUploadBytes)
	}
	return ValidateCreateAttachmentUploadRequest(CreateAttachmentUploadRequest{
		UserID:   req.UserID,
		DraftID:  req.DraftID,
		Filename: req.Filename,
		Size:     req.DeclaredSize,
		MIMEType: req.MIMEType,
	})
}

func ValidateStoreAttachmentUploadSessionBodyRequest(req StoreAttachmentUploadSessionBodyRequest) error {
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if err := validateServiceResourceID("session_id", strings.TrimSpace(req.SessionID)); err != nil {
		return err
	}
	if strings.TrimSpace(req.ExpectedChecksumSHA256) != "" && !isLowerSHA256Hex(strings.TrimSpace(req.ExpectedChecksumSHA256)) {
		return fmt.Errorf("expected checksum must be a lowercase SHA-256 hex digest")
	}
	if req.Body == nil {
		return fmt.Errorf("attachment upload session body is required")
	}
	return nil
}

func isLowerSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func validateAttachmentStoragePath(storagePath string) error {
	storagePath = strings.TrimSpace(storagePath)
	if storagePath == "" {
		return nil
	}
	if strings.ContainsAny(storagePath, "\r\n") {
		return fmt.Errorf("storage_path must not contain newlines")
	}
	if strings.Contains(storagePath, `\`) {
		return fmt.Errorf("storage_path must use forward slash separators")
	}
	if strings.HasPrefix(storagePath, "/") {
		return fmt.Errorf("storage_path must be relative")
	}
	for _, segment := range strings.Split(storagePath, "/") {
		if segment == "." || segment == ".." || strings.TrimSpace(segment) == "" {
			return fmt.Errorf("storage_path contains an invalid segment")
		}
	}
	cleaned := path.Clean(storagePath)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return fmt.Errorf("storage_path must not escape the storage root")
	}
	return nil
}
