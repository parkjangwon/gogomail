package maildb

import (
	"testing"
	"time"
)

func TestValidateExpireStaleAttachmentUploadsRequestRequiresBefore(t *testing.T) {
	t.Parallel()

	if err := ValidateExpireStaleAttachmentUploadsRequest(ExpireStaleAttachmentUploadsRequest{}); err == nil {
		t.Fatal("ValidateExpireStaleAttachmentUploadsRequest accepted zero before")
	}
}

func TestValidateExpireStaleAttachmentUploadsRequestRejectsNegativeLimit(t *testing.T) {
	t.Parallel()

	err := ValidateExpireStaleAttachmentUploadsRequest(ExpireStaleAttachmentUploadsRequest{
		Before: time.Now(),
		Limit:  -1,
	})
	if err == nil {
		t.Fatal("ValidateExpireStaleAttachmentUploadsRequest accepted negative limit")
	}
}
