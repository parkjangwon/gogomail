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
