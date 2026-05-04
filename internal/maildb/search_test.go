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
