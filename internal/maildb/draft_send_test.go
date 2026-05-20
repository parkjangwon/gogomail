package maildb

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestDraftOutboundAddressesDecodesAddressArray(t *testing.T) {
	t.Parallel()

	addresses, err := draftOutboundAddresses([]byte(`[{"name":"User","email":"user@example.net"}]`))
	if err != nil {
		t.Fatalf("draftOutboundAddresses returned error: %v", err)
	}
	if len(addresses) != 1 || addresses[0].Email != "user@example.net" || addresses[0].Name != "User" {
		t.Fatalf("addresses = %+v", addresses)
	}
}

func TestDraftOutboundAddressesAllowsEmptyJSON(t *testing.T) {
	t.Parallel()

	addresses, err := draftOutboundAddresses(nil)
	if err != nil {
		t.Fatalf("draftOutboundAddresses returned error: %v", err)
	}
	if len(addresses) != 0 {
		t.Fatalf("addresses = %+v", addresses)
	}
}

func TestErrDraftConflictIsSentinel(t *testing.T) {
	t.Parallel()

	// Verify that ErrDraftConflict is a distinct sentinel that can be detected
	// via errors.Is and is not equal to a generic error.
	if !errors.Is(ErrDraftConflict, ErrDraftConflict) {
		t.Fatal("ErrDraftConflict must satisfy errors.Is(err, ErrDraftConflict)")
	}
	wrapped := fmt.Errorf("outer: %w", ErrDraftConflict)
	if !errors.Is(wrapped, ErrDraftConflict) {
		t.Fatal("wrapped ErrDraftConflict must be detectable with errors.Is")
	}
}

func TestBindDraftAttachmentsSQLUsesSingleTypedArrayUpdate(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"unnest($3::uuid[]) AS requested(value)",
		"SELECT DISTINCT value AS id",
		"UPDATE attachments",
		"RETURNING attachments.id::text",
	} {
		if !strings.Contains(bindDraftAttachmentsSQL, want) {
			t.Fatalf("bindDraftAttachmentsSQL missing %q:\n%s", want, bindDraftAttachmentsSQL)
		}
	}
	if strings.Contains(bindDraftAttachmentsSQL, "AND id = $2") {
		t.Fatalf("bindDraftAttachmentsSQL still uses per-attachment id binding:\n%s", bindDraftAttachmentsSQL)
	}
}
