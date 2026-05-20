package maildb

import (
	"strings"
	"testing"
)

func TestSuppressedRecipientsSQLUsesSingleOrdinalityBatchLookup(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"unnest($2::text[]) WITH ORDINALITY",
		"GROUP BY email",
		"ORDER BY requested.ordinality",
		"EXISTS (",
		"COALESCE(s.domain_id, '00000000-0000-0000-0000-000000000000'::uuid)",
	} {
		if !strings.Contains(suppressedRecipientsSQL, want) {
			t.Fatalf("suppressedRecipientsSQL does not include %q:\n%s", want, suppressedRecipientsSQL)
		}
	}
	for _, forbidden := range []string{
		"LIMIT 1",
		"lower($1)",
		" OR ",
	} {
		if strings.Contains(suppressedRecipientsSQL, forbidden) {
			t.Fatalf("suppressedRecipientsSQL still contains per-recipient lookup shape %q:\n%s", forbidden, suppressedRecipientsSQL)
		}
	}
}
