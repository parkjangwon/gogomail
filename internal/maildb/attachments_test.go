package maildb

import (
	"strings"
	"testing"
)

func TestAttachmentsByIDsSQLUsesUuidOrdinality(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"unnest($2::uuid[]) WITH ORDINALITY",
		"JOIN requested ON requested.id = attachments.id",
		"ORDER BY requested.ordinality",
	} {
		if !strings.Contains(attachmentsByIDsSQL, want) {
			t.Fatalf("attachmentsByIDsSQL does not include %q:\n%s", want, attachmentsByIDsSQL)
		}
	}
	if strings.Contains(attachmentsByIDsSQL, "array_position") {
		t.Fatalf("attachmentsByIDsSQL still asks PostgreSQL to rescan attachment arrays:\n%s", attachmentsByIDsSQL)
	}
}
