package maildb

import "testing"

func TestAttachmentUploadStoragePathUsesFallbackFilename(t *testing.T) {
	t.Parallel()

	path := attachmentUploadStoragePath("user-1", "upload-1", " ")
	if path != "uploads/user-1/upload-1/attachment" {
		t.Fatalf("path = %q", path)
	}
}

func TestAttachmentUploadStoragePathSanitizesSeparators(t *testing.T) {
	t.Parallel()

	path := attachmentUploadStoragePath("user-1", "upload-1", `a/b\c.txt`)
	if path != "uploads/user-1/upload-1/a_b_c.txt" {
		t.Fatalf("path = %q", path)
	}
}
