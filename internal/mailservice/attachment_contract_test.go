package mailservice

import "testing"

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
