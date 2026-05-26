package maildb

import (
	"fmt"
	"path"
	"strings"
)

func ValidateCreateAttachmentUploadSessionRequest(req CreateAttachmentUploadSessionRequest) error {
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(req.Filename) == "" {
		return fmt.Errorf("filename is required")
	}
	if strings.ContainsAny(req.Filename, "\r\n") {
		return fmt.Errorf("filename must not contain newlines")
	}
	if len(strings.TrimSpace(req.Filename)) > 255 {
		return fmt.Errorf("filename is too long")
	}
	if req.DeclaredSize < 0 {
		return fmt.Errorf("declared_size must not be negative")
	}
	if strings.TrimSpace(req.MIMEType) == "" {
		return fmt.Errorf("mime_type is required")
	}
	if strings.ContainsAny(req.MIMEType, "\r\n") {
		return fmt.Errorf("mime_type must not contain newlines")
	}
	if len(strings.TrimSpace(req.MIMEType)) > 255 {
		return fmt.Errorf("mime_type is too long")
	}
	if req.ExpiresAt.IsZero() {
		return fmt.Errorf("expires_at is required")
	}
	return nil
}

func ValidateCancelAttachmentUploadSessionRequest(req CancelAttachmentUploadSessionRequest) error {
	return validateAttachmentUploadSessionIdentity(req.UserID, req.SessionID)
}

func ValidateGetAttachmentUploadSessionRequest(req GetAttachmentUploadSessionRequest) error {
	return validateAttachmentUploadSessionIdentity(req.UserID, req.SessionID)
}

func ValidateStoreAttachmentUploadSessionBodyRequest(req StoreAttachmentUploadSessionBodyRequest) error {
	if err := validateAttachmentUploadSessionIdentity(req.UserID, req.SessionID); err != nil {
		return err
	}
	if req.ReceivedSize < 0 {
		return fmt.Errorf("received_size must not be negative")
	}
	if strings.TrimSpace(req.StoragePath) == "" {
		return fmt.Errorf("storage_path is required")
	}
	if err := validateUploadSessionStoragePath(req.StoragePath); err != nil {
		return err
	}
	if strings.TrimSpace(req.ChecksumSHA256) == "" {
		return fmt.Errorf("checksum_sha256 is required")
	}
	checksum := strings.TrimSpace(req.ChecksumSHA256)
	if len(checksum) != 64 {
		return fmt.Errorf("checksum_sha256 must be a lowercase SHA-256 hex digest")
	}
	for _, r := range checksum {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return fmt.Errorf("checksum_sha256 must be a lowercase SHA-256 hex digest")
		}
	}
	return nil
}

func ValidateStoreAttachmentUploadSessionChunkRequest(req StoreAttachmentUploadSessionChunkRequest) error {
	if err := validateAttachmentUploadSessionIdentity(req.UserID, req.SessionID); err != nil {
		return err
	}
	if req.ContentRange.FirstByte < 0 {
		return fmt.Errorf("content-range first-byte must not be negative")
	}
	if req.ContentRange.LastByte < req.ContentRange.FirstByte {
		return fmt.Errorf("content-range last-byte must be >= first-byte")
	}
	if req.ContentRange.TotalSize <= 0 {
		return fmt.Errorf("content-range total-size must be positive")
	}
	if req.ContentRange.LastByte >= req.ContentRange.TotalSize {
		return fmt.Errorf("content-range last-byte must be < total-size")
	}
	if strings.TrimSpace(req.StoragePath) == "" {
		return fmt.Errorf("storage_path is required")
	}
	if err := validateUploadSessionStoragePath(req.StoragePath); err != nil {
		return err
	}
	return nil
}

func validateUploadSessionStoragePath(storagePath string) error {
	storagePath = strings.TrimSpace(storagePath)
	if storagePath == "" {
		return fmt.Errorf("storage_path is required")
	}
	if len(storagePath) > 2048 {
		return fmt.Errorf("storage_path is too long")
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
	cleaned := path.Clean(storagePath)
	if cleaned == "." || cleaned != storagePath {
		return fmt.Errorf("storage_path contains an invalid segment")
	}
	for _, segment := range strings.Split(cleaned, "/") {
		if segment == "." || segment == ".." || strings.TrimSpace(segment) == "" {
			return fmt.Errorf("storage_path contains an invalid segment")
		}
	}
	if !strings.HasPrefix(cleaned, "upload-sessions/") {
		return fmt.Errorf("storage_path must use upload-sessions prefix")
	}
	return nil
}

func ValidateFinalizeAttachmentUploadSessionRequest(req FinalizeAttachmentUploadSessionRequest) error {
	return validateAttachmentUploadSessionIdentity(req.UserID, req.SessionID)
}

func validateAttachmentUploadSessionIdentity(userID string, sessionID string) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session_id is required")
	}
	if strings.ContainsAny(userID, "\r\n") {
		return fmt.Errorf("user_id must not contain newlines")
	}
	if strings.ContainsAny(sessionID, "\r\n") {
		return fmt.Errorf("session_id must not contain newlines")
	}
	if len(strings.TrimSpace(userID)) > 200 {
		return fmt.Errorf("user_id is too long")
	}
	if len(strings.TrimSpace(sessionID)) > 200 {
		return fmt.Errorf("session_id is too long")
	}
	return nil
}

func ValidateExpireAttachmentUploadSessionsRequest(req ExpireAttachmentUploadSessionsRequest) error {
	if req.Before.IsZero() {
		return fmt.Errorf("before is required")
	}
	if req.Limit < 0 {
		return fmt.Errorf("limit must not be negative")
	}
	return nil
}

func ValidateAttachmentUploadSessionListRequest(req AttachmentUploadSessionListRequest) error {
	for field, value := range map[string]string{
		"user_id":  strings.TrimSpace(req.UserID),
		"draft_id": strings.TrimSpace(req.DraftID),
	} {
		if value == "" {
			continue
		}
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%s must not contain newlines", field)
		}
		if len(value) > 200 {
			return fmt.Errorf("%s is too long", field)
		}
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "" && !isAttachmentUploadSessionStatus(status) {
		return fmt.Errorf("unsupported attachment upload session status %q", req.Status)
	}
	return nil
}

func isAttachmentUploadSessionStatus(status string) bool {
	switch status {
	case "pending", "uploading", "finalized", "canceled", "expired":
		return true
	default:
		return false
	}
}
