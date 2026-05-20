package maildb

import (
	"strings"
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

func TestNormalizeAttachmentCleanupLimit(t *testing.T) {
	t.Parallel()

	tests := map[int]int{
		0:                                  AttachmentCleanupDefaultLimit,
		-1:                                 AttachmentCleanupDefaultLimit,
		25:                                 25,
		AttachmentCleanupMaxLimit:          AttachmentCleanupMaxLimit,
		AttachmentCleanupMaxLimit + 1:      AttachmentCleanupMaxLimit,
		MessageListMaxLimit + 100:          MessageListMaxLimit + 100,
		AttachmentCleanupDefaultLimit + 25: AttachmentCleanupDefaultLimit + 25,
	}
	for input, want := range tests {
		if got := NormalizeAttachmentCleanupLimit(input); got != want {
			t.Fatalf("NormalizeAttachmentCleanupLimit(%d) = %d, want %d", input, got, want)
		}
	}
}

func TestExpireStaleAttachmentUploadsSQLUsesBatchUpdates(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"unnest($1::uuid[])",
		"UPDATE attachments a",
		"FROM input",
	} {
		if !strings.Contains(expireStaleAttachmentUploadsSQL, want) {
			t.Fatalf("expireStaleAttachmentUploadsSQL missing %q:\n%s", want, expireStaleAttachmentUploadsSQL)
		}
	}
	for _, want := range []string{
		"unnest($1::uuid[], $2::bigint[])",
		"user_usage AS",
		"domain_usage AS",
		"company_usage AS",
		"UPDATE users u",
		"UPDATE domains d",
		"UPDATE companies c",
	} {
		if !strings.Contains(decrementUserQuotasSQL, want) {
			t.Fatalf("decrementUserQuotasSQL missing %q:\n%s", want, decrementUserQuotasSQL)
		}
	}
}
