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
		"setweight(to_tsvector('simple', coalesce(messages.draft_text_body, '')), 'C')",
		"setweight(to_tsvector('simple', coalesce(msd.body_text, '')), 'D')",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("query does not include weighted search vector %q:\n%s", want, query)
		}
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
