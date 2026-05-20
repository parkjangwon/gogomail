package maildb

import (
	"strings"
	"testing"
)

func TestCreateTrackingPixelsSQLUsesBatchUnnest(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"FROM unnest($1::text[], $2::uuid[], $3::uuid[], $4::text[])",
		"AS input(pixel_id, message_id, sender_user_id, recipient_email)",
		"ON CONFLICT (pixel_id) DO NOTHING",
	} {
		if !strings.Contains(createTrackingPixelsSQL, want) {
			t.Fatalf("create tracking pixels SQL missing %q:\n%s", want, createTrackingPixelsSQL)
		}
	}
	if strings.Contains(createTrackingPixelsSQL, "VALUES ($1") {
		t.Fatalf("create tracking pixels SQL still uses single-row VALUES:\n%s", createTrackingPixelsSQL)
	}
}
