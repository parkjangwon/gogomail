package mailservice

import (
	"fmt"
	"path/filepath"
	"strings"
)

const MaxAttachmentFilenameBytes = 255

type CreateAttachmentUploadRequest struct {
	UserID      string `json:"user_id"`
	DraftID     string `json:"draft_id,omitempty"`
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	MIMEType    string `json:"mime_type"`
	StoragePath string `json:"storage_path,omitempty"`
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
	if len(filename) > MaxAttachmentFilenameBytes {
		return fmt.Errorf("filename is too long")
	}
	if req.Size < 0 {
		return fmt.Errorf("attachment size must not be negative")
	}
	if strings.TrimSpace(req.MIMEType) == "" {
		return fmt.Errorf("mime_type is required")
	}
	return nil
}
