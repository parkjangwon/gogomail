package maildb

import (
	"database/sql"
	"strings"
	"testing"
)

func TestMessageSearchQueryNormalizesLimit(t *testing.T) {
	t.Parallel()

	if got := (MessageSearchQuery{}).normalizedLimit(); got != MessageListDefaultLimit {
		t.Fatalf("default limit = %d", got)
	}
	if got := (MessageSearchQuery{Limit: 201}).normalizedLimit(); got != MessageListMaxLimit {
		t.Fatalf("max limit = %d", got)
	}
}

func TestMessageSearchQueryNormalizesSort(t *testing.T) {
	t.Parallel()

	if got := (MessageSearchQuery{}).normalizedSort(); got != MessageSearchSortDate {
		t.Fatalf("default sort = %q, want %q", got, MessageSearchSortDate)
	}
	if got := (MessageSearchQuery{Sort: " Relevance \n"}).normalizedSort(); got != MessageSearchSortRelevance {
		t.Fatalf("sort = %q, want %q", got, MessageSearchSortRelevance)
	}
}

func TestMessageSearchSQLOrdersByRankForRelevance(t *testing.T) {
	t.Parallel()

	query := messageSearchSQL(MessageSearchSortRelevance)
	if !strings.Contains(query, "search_rank DESC NULLS LAST") {
		t.Fatalf("query does not order by search rank:\n%s", query)
	}
}

func TestMessageSearchSQLWeightsMetadataAboveBody(t *testing.T) {
	t.Parallel()

	query := messageSearchSQL(MessageSearchSortRelevance)
	for _, want := range []string{
		"setweight(to_tsvector('simple', coalesce(messages.subject, '')), 'A')",
		"setweight(to_tsvector('simple', coalesce(messages.from_addr, '')), 'A')",
		"setweight(to_tsvector('simple', coalesce(messages.from_name, '')), 'B')",
		"setweight(to_tsvector('simple', coalesce(msd.body_text, '')), 'D')",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("query does not include weighted search vector %q:\n%s", want, query)
		}
	}
}

func TestMessageSearchSQLProjectsBoundedPreview(t *testing.T) {
	t.Parallel()

	query := messageSearchSQL(MessageSearchSortDate)
	for _, want := range []string{
		"left(btrim(regexp_replace(left(coalesce(msd.body_text, ''), 2000), '[[:space:]]+', ' ', 'g')), 280) AS preview",
		"preview,",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("query does not include preview projection %q:\n%s", want, query)
		}
	}
}

func TestMessageSearchSQLExcludesDraftRows(t *testing.T) {
	t.Parallel()

	query := messageSearchSQL(MessageSearchSortRelevance)
	if !strings.Contains(query, "messages.status = 'active'") {
		t.Fatalf("query does not restrict search to active messages:\n%s", query)
	}
	if strings.Contains(query, "draft_text_body") {
		t.Fatalf("query includes draft text despite active-message search contract:\n%s", query)
	}
}

func TestMessageSearchSQLUsesSargableFolderFilter(t *testing.T) {
	t.Parallel()

	query := buildMessageSearchSQL(MessageSearchSortDate, MessageSearchQuery{FolderID: "folder-1"}, "", "")
	if !strings.Contains(query, "AND messages.folder_id = $3::uuid") {
		t.Fatalf("message search query missing sargable folder filter:\n%s", query)
	}
	if strings.Contains(query, "$3 = '' OR folder_id::text = $3") {
		t.Fatalf("message search query contains non-sargable folder filter:\n%s", query)
	}

	query = buildMessageSearchSQL(MessageSearchSortRelevance, MessageSearchQuery{}, "", "")
	if strings.Contains(query, "AND folder_id") {
		t.Fatalf("folderless message search query unexpectedly includes folder predicate:\n%s", query)
	}
	if !strings.Contains(query, "search_rank DESC NULLS LAST") {
		t.Fatalf("folderless relevance search lost relevance ordering:\n%s", query)
	}
}

func TestMessageSearchSQLUsesSargableAttachmentFilter(t *testing.T) {
	t.Parallel()

	query := buildMessageSearchSQL(MessageSearchSortDate, MessageSearchQuery{}, "true", "")
	if !strings.Contains(query, "AND messages.has_attachment = $9::boolean") {
		t.Fatalf("message search query missing sargable attachment filter:\n%s", query)
	}
	if strings.Contains(query, "$9 = '' OR has_attachment = $9::boolean") {
		t.Fatalf("message search query contains optional attachment OR:\n%s", query)
	}

	query = buildMessageSearchSQL(MessageSearchSortDate, MessageSearchQuery{}, "", "")
	if strings.Contains(query, "AND has_attachment") {
		t.Fatalf("attachment-agnostic message search query unexpectedly includes attachment predicate:\n%s", query)
	}
}

func TestMessageSearchSQLUsesSargableCursorFilter(t *testing.T) {
	t.Parallel()

	query := buildMessageSearchSQL(MessageSearchSortDate, MessageSearchQuery{}, "", "cursor-1")
	if !strings.Contains(query, "AND (COALESCE(messages.received_at, messages.sent_at, messages.draft_updated_at, messages.created_at), messages.id) < ($13::timestamptz, $14::uuid)") {
		t.Fatalf("message search query missing direct cursor predicate:\n%s", query)
	}
	if strings.Contains(query, "$14 = '' OR") {
		t.Fatalf("message search query contains optional cursor OR:\n%s", query)
	}

	query = buildMessageSearchSQL(MessageSearchSortDate, MessageSearchQuery{}, "", "")
	if strings.Contains(query, "$14::uuid") {
		t.Fatalf("cursorless message search query unexpectedly includes cursor predicate:\n%s", query)
	}
}

func TestMessageSearchSQLUsesDirectTextFilters(t *testing.T) {
	t.Parallel()

	query := buildMessageSearchSQL(MessageSearchSortDate, MessageSearchQuery{
		Query:   "quarterly",
		From:    "alice@example.com",
		To:      "bob@example.net",
		Cc:      "carol@example.net",
		Bcc:     "dave@example.net",
		Subject: "report",
	}, "", "")
	for _, want := range []string{
		"messages.id IN (SELECT id FROM query_matches)",
		"query_matches AS (",
		"UNION",
		"to_tsvector('simple', msd.body_text) @@ search_input.tsq",
		"messages.subject ILIKE '%' || $2 || '%'",
		"messages.from_name ILIKE '%' || $2 || '%'",
		"msd.body_text ILIKE '%' || $2 || '%'",
		"messages.from_addr ILIKE '%' || $4 || '%'",
		"messages.to_addrs::text ILIKE '%' || $5 || '%'",
		"messages.cc_addrs::text ILIKE '%' || $6 || '%'",
		"messages.bcc_addrs::text ILIKE '%' || $7 || '%'",
		"messages.subject ILIKE '%' || $8 || '%'",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("message search query missing text filter %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{"$2 = '' OR", "$4 = '' OR", "$5 = '' OR", "$6 = '' OR", "$7 = '' OR", "$8 = '' OR"} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("message search query contains optional OR %q:\n%s", forbidden, query)
		}
	}
	for _, forbidden := range []string{
		") @@ plainto_tsquery('simple', $2)\n    OR",
		"OR messages.subject ILIKE '%' || $2 || '%'",
		"OR messages.from_addr ILIKE '%' || $2 || '%'",
		"OR msd.body_text ILIKE '%' || $2 || '%'",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("message search query contains broad query OR %q:\n%s", forbidden, query)
		}
	}

	query = buildMessageSearchSQL(MessageSearchSortDate, MessageSearchQuery{}, "", "")
	for _, absent := range []string{"@@ plainto_tsquery", "ILIKE '%' || $4", "ILIKE '%' || $5", "ILIKE '%' || $6", "ILIKE '%' || $7", "ILIKE '%' || $8"} {
		if strings.Contains(query, absent) {
			t.Fatalf("filterless message search query contains %q:\n%s", absent, query)
		}
	}
	if strings.Contains(query, "query_matches AS") {
		t.Fatalf("filterless message search query unexpectedly includes query_matches CTE:\n%s", query)
	}
}

func TestDraftSearchQueryNormalizesLimit(t *testing.T) {
	t.Parallel()

	if got := (DraftSearchQuery{}).normalizedLimit(); got != MessageListDefaultLimit {
		t.Fatalf("default limit = %d", got)
	}
	if got := (DraftSearchQuery{Limit: 201}).normalizedLimit(); got != MessageListMaxLimit {
		t.Fatalf("max limit = %d", got)
	}
}

func TestDraftSearchSQLUsesComposeFocusedDraftFields(t *testing.T) {
	t.Parallel()

	query := draftSearchSQL()
	for _, want := range []string{
		"WITH draft_matches AS (",
		"id IN (SELECT id FROM draft_matches)",
		"UNION",
		"status = 'draft'",
		"draft_text_body ILIKE",
		"to_addrs::text ILIKE",
		"cc_addrs::text ILIKE",
		"bcc_addrs::text ILIKE",
		"ORDER BY draft_at DESC, id DESC",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("draft search query does not include %q:\n%s", want, query)
		}
	}
	if strings.Contains(query, "message_search_documents") {
		t.Fatalf("draft search query should not depend on active-message search index:\n%s", query)
	}
	for _, forbidden := range []string{
		"OR from_addr ILIKE '%' || $2 || '%'",
		"OR from_name ILIKE '%' || $2 || '%'",
		"OR to_addrs::text ILIKE '%' || $2 || '%'",
		"OR draft_text_body ILIKE '%' || $2 || '%'",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("draft search query contains broad query OR %q:\n%s", forbidden, query)
		}
	}
}

func TestDraftSearchSQLUsesSargableAttachmentFilter(t *testing.T) {
	t.Parallel()

	query := buildDraftSearchSQL(DraftSearchQuery{}, "false", "")
	if !strings.Contains(query, "AND has_attachment = $8::boolean") {
		t.Fatalf("draft search query missing sargable attachment filter:\n%s", query)
	}
	if strings.Contains(query, "$8 = '' OR has_attachment = $8::boolean") {
		t.Fatalf("draft search query contains optional attachment OR:\n%s", query)
	}

	query = buildDraftSearchSQL(DraftSearchQuery{}, "", "")
	if strings.Contains(query, "AND has_attachment") {
		t.Fatalf("attachment-agnostic draft search query unexpectedly includes attachment predicate:\n%s", query)
	}
}

func TestDraftSearchSQLUsesSargableCursorFilter(t *testing.T) {
	t.Parallel()

	query := buildDraftSearchSQL(DraftSearchQuery{}, "", "cursor-1")
	if !strings.Contains(query, "AND (COALESCE(draft_updated_at, updated_at, created_at), id) < ($9::timestamptz, $10::uuid)") {
		t.Fatalf("draft search query missing direct cursor predicate:\n%s", query)
	}
	if strings.Contains(query, "$10 = ''") {
		t.Fatalf("draft search query contains optional cursor OR:\n%s", query)
	}

	query = buildDraftSearchSQL(DraftSearchQuery{}, "", "")
	if strings.Contains(query, "$10::uuid") {
		t.Fatalf("cursorless draft search query unexpectedly includes cursor predicate:\n%s", query)
	}
}

func TestDraftSearchSQLUsesDirectTextFilters(t *testing.T) {
	t.Parallel()

	query := buildDraftSearchSQL(DraftSearchQuery{
		Query:   "quarterly",
		From:    "alice@example.com",
		To:      "bob@example.net",
		Cc:      "carol@example.net",
		Bcc:     "dave@example.net",
		Subject: "report",
	}, "", "")
	for _, want := range []string{
		"subject ILIKE '%' || $2 || '%'",
		"from_addr ILIKE '%' || $3 || '%'",
		"to_addrs::text ILIKE '%' || $4 || '%'",
		"cc_addrs::text ILIKE '%' || $5 || '%'",
		"bcc_addrs::text ILIKE '%' || $6 || '%'",
		"subject ILIKE '%' || $7 || '%'",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("draft search query missing text filter %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{"$2 = '' OR", "$3 = '' OR", "$4 = '' OR", "$5 = '' OR", "$6 = '' OR", "$7 = '' OR"} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("draft search query contains optional OR %q:\n%s", forbidden, query)
		}
	}

	query = buildDraftSearchSQL(DraftSearchQuery{}, "", "")
	for _, absent := range []string{"ILIKE '%' || $2", "ILIKE '%' || $3", "ILIKE '%' || $4", "ILIKE '%' || $5", "ILIKE '%' || $6", "ILIKE '%' || $7"} {
		if strings.Contains(query, absent) {
			t.Fatalf("filterless draft search query contains %q:\n%s", absent, query)
		}
	}
	if strings.Contains(query, "draft_matches AS") {
		t.Fatalf("filterless draft search query unexpectedly includes draft_matches CTE:\n%s", query)
	}
}

func TestHighlightFragmentsDropsUnmarkedText(t *testing.T) {
	t.Parallel()

	if got := highlightFragments(sql.NullString{String: "plain text", Valid: true}); len(got) != 0 {
		t.Fatalf("highlightFragments returned %#v, want no unmarked fragment", got)
	}
	got := highlightFragments(sql.NullString{String: "<mark>hello</mark>", Valid: true})
	if len(got) != 1 || got[0] != "<mark>hello</mark>" {
		t.Fatalf("highlightFragments returned %#v", got)
	}
}
