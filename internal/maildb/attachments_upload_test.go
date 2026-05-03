package maildb

import (
	"strings"
	"testing"
)

func TestNewUploadIDHasStablePrefix(t *testing.T) {
	t.Parallel()

	if got := newUploadID(); !strings.HasPrefix(got, "upload-") {
		t.Fatalf("upload id = %q", got)
	}
}

func TestAttachmentUploadStoragePathSanitizesFilename(t *testing.T) {
	t.Parallel()

	got := attachmentUploadStoragePath("user-1", "upload-1", `dir\report.pdf`)
	want := "uploads/user-1/upload-1/dir_report.pdf"
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}
