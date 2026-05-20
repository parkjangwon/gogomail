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

func TestMessageListPageQueryUsesSargableFolderFilter(t *testing.T) {
	t.Parallel()

	query := buildMessageListPageSQL(ListSortNewest, "folder-1", MessageListFilter{})
	if !strings.Contains(query, "AND messages.folder_id = $2::uuid") {
		t.Fatalf("message list query missing sargable folder filter:\n%s", query)
	}
	if strings.Contains(query, "$2 = '' OR messages.folder_id::text = $2") {
		t.Fatalf("message list query contains non-sargable folder filter:\n%s", query)
	}

	query = buildMessageListPageSQL(ListSortOldest, "", MessageListFilter{})
	if strings.Contains(query, "AND messages.folder_id") {
		t.Fatalf("folderless message list query unexpectedly includes folder predicate:\n%s", query)
	}
	if !strings.Contains(query, "ORDER BY message_at ASC, id ASC") {
		t.Fatalf("oldest message list query lost oldest ordering:\n%s", query)
	}
}

func TestMessageListPageQueryUsesSargableBooleanFilters(t *testing.T) {
	t.Parallel()

	read := false
	starred := true
	hasAttachment := true
	query := buildMessageListPageSQL(ListSortNewest, "", MessageListFilter{
		Read:          &read,
		Starred:       &starred,
		HasAttachment: &hasAttachment,
	})
	for _, want := range []string{
		"AND COALESCE((messages.flags->>'read')::boolean, false) = $6::boolean",
		"AND COALESCE((messages.flags->>'starred')::boolean, false) = $7::boolean",
		"AND messages.has_attachment = $8::boolean",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("message list query missing sargable boolean filter %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{
		"$6::boolean IS NULL",
		"$7::boolean IS NULL",
		"$8::boolean IS NULL",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("message list query contains optional boolean filter %q:\n%s", forbidden, query)
		}
	}

	query = buildMessageListPageSQL(ListSortOldest, "", MessageListFilter{})
	for _, forbidden := range []string{
		"AND COALESCE((messages.flags->>'read')::boolean",
		"AND COALESCE((messages.flags->>'starred')::boolean",
		"AND messages.has_attachment",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("unfiltered message list query unexpectedly includes boolean filter %q:\n%s", forbidden, query)
		}
	}
	if !strings.Contains(query, "ORDER BY message_at ASC, id ASC") {
		t.Fatalf("oldest message list query lost oldest ordering:\n%s", query)
	}
}
