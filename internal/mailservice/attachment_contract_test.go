package mailservice

import (
	"strings"
	"testing"
	"time"
)

func TestValidateCreateAttachmentUploadRequestRejectsPathFilename(t *testing.T) {
	t.Parallel()

	err := ValidateCreateAttachmentUploadRequest(CreateAttachmentUploadRequest{
		UserID:   "user-1",
		Filename: "../secret.txt",
		Size:     1,
		MIMEType: "text/plain",
	})
	if err == nil {
		t.Fatal("ValidateCreateAttachmentUploadRequest accepted path filename")
	}
}

func TestValidateCreateAttachmentUploadRequestRejectsUnsafeDraftID(t *testing.T) {
	t.Parallel()

	tests := []string{
		"draft-1\r\nbad",
		strings.Repeat("x", maxServiceResourceIDBytes+1),
	}
	for _, draftID := range tests {
		err := ValidateCreateAttachmentUploadRequest(CreateAttachmentUploadRequest{
			UserID:   "user-1",
			DraftID:  draftID,
			Filename: "report.pdf",
			Size:     10,
			MIMEType: "application/pdf",
		})
		if err == nil {
			t.Fatalf("ValidateCreateAttachmentUploadRequest accepted draft_id %q", draftID)
		}
	}
}

func TestValidateCreateAttachmentUploadRequestRejectsUnsafeUserID(t *testing.T) {
	t.Parallel()

	for _, userID := range []string{
		"user-1\r\nbad",
		strings.Repeat("u", maxServiceResourceIDBytes+1),
	} {
		err := ValidateCreateAttachmentUploadRequest(CreateAttachmentUploadRequest{
			UserID:   userID,
			Filename: "report.pdf",
			Size:     10,
			MIMEType: "application/pdf",
		})
		if err == nil {
			t.Fatalf("ValidateCreateAttachmentUploadRequest accepted user_id %q", userID)
		}
	}
}

func TestValidateCreateAttachmentUploadRequestAcceptsMetadata(t *testing.T) {
	t.Parallel()

	err := ValidateCreateAttachmentUploadRequest(CreateAttachmentUploadRequest{
		UserID:   "user-1",
		DraftID:  "draft-1",
		Filename: "report.pdf",
		Size:     42,
		MIMEType: "application/pdf",
	})
	if err != nil {
		t.Fatalf("ValidateCreateAttachmentUploadRequest returned error: %v", err)
	}
}

func TestValidateCreateAttachmentUploadRequestRejectsNewlineMIMEType(t *testing.T) {
	t.Parallel()

	err := ValidateCreateAttachmentUploadRequest(CreateAttachmentUploadRequest{
		UserID:   "user-1",
		Filename: "report.pdf",
		Size:     42,
		MIMEType: "application/pdf\r\nx-bad: yes",
	})
	if err == nil {
		t.Fatal("ValidateCreateAttachmentUploadRequest accepted newline MIME type")
	}
}

func TestValidateCreateAttachmentUploadRequestRejectsNewlineFilename(t *testing.T) {
	t.Parallel()

	err := ValidateCreateAttachmentUploadRequest(CreateAttachmentUploadRequest{
		UserID:   "user-1",
		Filename: "report\r\nbad.pdf",
		Size:     42,
		MIMEType: "application/pdf",
	})
	if err == nil {
		t.Fatal("ValidateCreateAttachmentUploadRequest accepted newline filename")
	}
}

func TestValidateCreateAttachmentUploadRequestRejectsUnsafeStoragePath(t *testing.T) {
	t.Parallel()

	for _, storagePath := range []string{
		"../secret.eml",
		"/var/mail/secret.eml",
		`uploads\user\secret.eml`,
		"uploads/user\nsecret.eml",
		"uploads/../secret.eml",
		"uploads//secret.eml",
	} {
		err := ValidateCreateAttachmentUploadRequest(CreateAttachmentUploadRequest{
			UserID:      "user-1",
			Filename:    "report.pdf",
			Size:        42,
			MIMEType:    "application/pdf",
			StoragePath: storagePath,
		})
		if err == nil {
			t.Fatalf("ValidateCreateAttachmentUploadRequest accepted unsafe storage_path %q", storagePath)
		}
	}
}

func TestValidateCreateAttachmentUploadRequestAcceptsRelativeStoragePath(t *testing.T) {
	t.Parallel()

	err := ValidateCreateAttachmentUploadRequest(CreateAttachmentUploadRequest{
		UserID:      "user-1",
		Filename:    "report.pdf",
		Size:        42,
		MIMEType:    "application/pdf",
		StoragePath: "uploads/user-1/upload-1/report.pdf",
	})
	if err != nil {
		t.Fatalf("ValidateCreateAttachmentUploadRequest returned error: %v", err)
	}
}

func TestValidateCreateAttachmentUploadSessionRequest(t *testing.T) {
	t.Parallel()

	valid := CreateAttachmentUploadSessionRequest{
		UserID:       "user-1",
		DraftID:      "draft-1",
		Filename:     "large.bin",
		DeclaredSize: 42,
		MIMEType:     "application/octet-stream",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	if err := ValidateCreateAttachmentUploadSessionRequest(valid); err != nil {
		t.Fatalf("ValidateCreateAttachmentUploadSessionRequest returned error: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*CreateAttachmentUploadSessionRequest)
	}{
		{name: "missing expiry", mutate: func(req *CreateAttachmentUploadSessionRequest) { req.ExpiresAt = time.Time{} }},
		{name: "oversized declared size", mutate: func(req *CreateAttachmentUploadSessionRequest) { req.DeclaredSize = MaxAttachmentUploadBytes + 1 }},
		{name: "unsafe draft", mutate: func(req *CreateAttachmentUploadSessionRequest) { req.DraftID = "draft\nbad" }},
		{name: "unsafe filename", mutate: func(req *CreateAttachmentUploadSessionRequest) { req.Filename = "../large.bin" }},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := valid
			tt.mutate(&req)
			if err := ValidateCreateAttachmentUploadSessionRequest(req); err == nil {
				t.Fatal("ValidateCreateAttachmentUploadSessionRequest accepted invalid request")
			}
		})
	}
}

func TestValidateStoreAttachmentUploadSessionBodyRequest(t *testing.T) {
	t.Parallel()

	valid := StoreAttachmentUploadSessionBodyRequest{
		UserID:                 "user-1",
		SessionID:              "session-1",
		ExpectedChecksumSHA256: strings.Repeat("a", 64),
		Body:                   strings.NewReader("content"),
	}
	if err := ValidateStoreAttachmentUploadSessionBodyRequest(valid); err != nil {
		t.Fatalf("ValidateStoreAttachmentUploadSessionBodyRequest returned error: %v", err)
	}
	if err := ValidateStoreAttachmentUploadSessionBodyRequest(StoreAttachmentUploadSessionBodyRequest{
		UserID:                 "user-1",
		SessionID:              "session-1",
		ExpectedChecksumSHA256: strings.Repeat("A", 64),
		Body:                   strings.NewReader("content"),
	}); err == nil {
		t.Fatal("ValidateStoreAttachmentUploadSessionBodyRequest accepted uppercase checksum")
	}
	if err := ValidateStoreAttachmentUploadSessionBodyRequest(StoreAttachmentUploadSessionBodyRequest{
		UserID:    "user-1",
		SessionID: "session-1",
	}); err == nil {
		t.Fatal("ValidateStoreAttachmentUploadSessionBodyRequest accepted missing body")
	}
	if err := ValidateStoreAttachmentUploadSessionBodyRequest(StoreAttachmentUploadSessionBodyRequest{
		UserID:    "user-1\nbad",
		SessionID: "session-1",
		Body:      strings.NewReader("content"),
	}); err == nil {
		t.Fatal("ValidateStoreAttachmentUploadSessionBodyRequest accepted unsafe user_id")
	}
}
