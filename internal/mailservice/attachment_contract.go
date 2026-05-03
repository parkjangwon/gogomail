package mailservice

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const MaxAttachmentFilenameBytes = 255
const MaxAttachmentUploadBytes int64 = 25 << 20

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

func ValidateCreateAttachmentUploadRequest(req CreateAttachmentUploadRequest) error {
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user_id is required")
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
