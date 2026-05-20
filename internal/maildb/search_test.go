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

	query := buildMessageSearchSQL(MessageSearchSortDate, "folder-1", "")
	if !strings.Contains(query, "AND folder_id = $3::uuid") {
		t.Fatalf("message search query missing sargable folder filter:\n%s", query)
	}
	if strings.Contains(query, "$3 = '' OR folder_id::text = $3") {
		t.Fatalf("message search query contains non-sargable folder filter:\n%s", query)
	}

	query = buildMessageSearchSQL(MessageSearchSortRelevance, "", "")
	if strings.Contains(query, "AND folder_id") {
		t.Fatalf("folderless message search query unexpectedly includes folder predicate:\n%s", query)
	}
	if !strings.Contains(query, "search_rank DESC NULLS LAST") {
		t.Fatalf("folderless relevance search lost relevance ordering:\n%s", query)
	}
}

func TestMessageSearchSQLUsesSargableAttachmentFilter(t *testing.T) {
	t.Parallel()

	query := buildMessageSearchSQL(MessageSearchSortDate, "", "true")
	if !strings.Contains(query, "AND has_attachment = $9::boolean") {
		t.Fatalf("message search query missing sargable attachment filter:\n%s", query)
	}
	if strings.Contains(query, "$9 = '' OR has_attachment = $9::boolean") {
		t.Fatalf("message search query contains optional attachment OR:\n%s", query)
	}

	query = buildMessageSearchSQL(MessageSearchSortDate, "", "")
	if strings.Contains(query, "AND has_attachment") {
		t.Fatalf("attachment-agnostic message search query unexpectedly includes attachment predicate:\n%s", query)
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
}

func TestDraftSearchSQLUsesSargableAttachmentFilter(t *testing.T) {
	t.Parallel()

	query := buildDraftSearchSQL("false")
	if !strings.Contains(query, "AND has_attachment = $8::boolean") {
		t.Fatalf("draft search query missing sargable attachment filter:\n%s", query)
	}
	if strings.Contains(query, "$8 = '' OR has_attachment = $8::boolean") {
		t.Fatalf("draft search query contains optional attachment OR:\n%s", query)
	}

	query = buildDraftSearchSQL("")
	if strings.Contains(query, "AND has_attachment") {
		t.Fatalf("attachment-agnostic draft search query unexpectedly includes attachment predicate:\n%s", query)
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
