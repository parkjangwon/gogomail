package maildb

import (
	"strings"
	"testing"
)

func TestMessageListPageSQLProjectsBoundedPreview(t *testing.T) {
	t.Parallel()

	for name, query := range map[string]string{
		"newest": messageListPageNewestSQL,
		"oldest": messageListPageOldestSQL,
	} {
		name := name
		query := query
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			for _, want := range []string{
				"LEFT JOIN message_search_documents msd",
				"left(btrim(regexp_replace(left(coalesce(msd.body_text, ''), 2000), '[[:space:]]+', ' ', 'g')), 280) AS preview",
			} {
				if !strings.Contains(query, want) {
					t.Fatalf("message list query does not include %q:\n%s", want, query)
				}
			}
		})
	}
}
