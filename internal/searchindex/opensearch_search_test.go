package searchindex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestOpenSearchSearcherReturnsMessageIDs(t *testing.T) {
	t.Parallel()

	var request map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/gogomail-messages/_search" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"hits": {
				"hits": [
					{"_id":"msg-1","_score":1.5,"_source":{"message_id":"msg-1"},"highlight":{"subject":["<mark>hello</mark>"],"body_text":["body <mark>hello</mark>"]}},
					{"_id":"msg-2","_score":0.75,"_source":{"message_id":"msg-2"}}
				]
			}
		}`))
	}))
	defer server.Close()

	searcher, err := NewOpenSearchSearcher(OpenSearchOptions{
		Endpoint: server.URL,
		Index:    "gogomail-messages",
		Client:   server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenSearchSearcher returned error: %v", err)
	}

	hits, err := searcher.SearchMessageIDs(context.Background(), OpenSearchSearchQuery{
		UserID:            "user-1",
		FolderID:          "folder-1",
		Query:             "hello",
		From:              "sender@example.com",
		Subject:           "hello",
		HasAttachment:     boolSearchPtr(true),
		IncludeHighlights: true,
		Limit:             2,
	})
	if err != nil {
		t.Fatalf("SearchMessageIDs returned error: %v", err)
	}
	if len(hits) != 2 || hits[0].MessageID != "msg-1" || hits[0].Score != 1.5 {
		t.Fatalf("hits = %#v", hits)
	}
	if request["size"].(float64) != 2 {
		t.Fatalf("request size = %#v", request["size"])
	}
	if len(request["query"].(map[string]any)["bool"].(map[string]any)["must"].([]any)) < 5 {
		t.Fatalf("request query did not include filters: %#v", request["query"])
	}
	if !queryMustContainsWildcard(request, "from_addr_lc") {
		t.Fatalf("request query did not include lowercase sender wildcard: %#v", request["query"])
	}
	if !queryMustContainsWildcard(request, "subject_lc") {
		t.Fatalf("request query did not include lowercase subject wildcard: %#v", request["query"])
	}
	if len(hits[0].Highlights.Subject) != 1 || len(hits[0].Highlights.Body) != 1 {
		t.Fatalf("highlights = %#v", hits[0].Highlights)
	}
	if request["highlight"] == nil {
		t.Fatalf("request did not include highlighter: %#v", request)
	}
	if !queryMustContainsMultiMatchField(request, "subject^4") || !queryMustContainsMultiMatchField(request, "from_addr^4") || !queryMustContainsMultiMatchField(request, "from_name^2") {
		t.Fatalf("request query did not include expected relevance boosts: %#v", request["query"])
	}
}

func TestOpenSearchSearchPayloadIncludesToCcBccWildcards(t *testing.T) {
	t.Parallel()

	payload := openSearchSearchPayload(OpenSearchSearchQuery{
		UserID: "user-1",
		To:     "alice@example.com",
		Cc:     "BOB@example.com",
		Bcc:    "carol@example.com",
	}, "user-1", 50)
	must := payload["query"].(map[string]any)["bool"].(map[string]any)["must"].([]map[string]any)

	if got := wildcardValue(t, must, "to_addrs_lc"); got != `*alice@example.com*` {
		t.Fatalf("to wildcard = %q", got)
	}
	if got := wildcardValue(t, must, "cc_addrs_lc"); got != `*bob@example.com*` {
		t.Fatalf("cc wildcard = %q", got)
	}
	if got := wildcardValue(t, must, "bcc_addrs_lc"); got != `*carol@example.com*` {
		t.Fatalf("bcc wildcard = %q", got)
	}
}

func TestOpenSearchSearchPayloadEscapesWildcardFilters(t *testing.T) {
	t.Parallel()

	payload := openSearchSearchPayload(OpenSearchSearchQuery{
		UserID:  "user-1",
		From:    `sender*?\@example.com`,
		Subject: `quarterly*?`,
		Query:   strings.Repeat("한", 400),
	}, "user-1", 50)
	must := payload["query"].(map[string]any)["bool"].(map[string]any)["must"].([]map[string]any)

	if got := wildcardValue(t, must, "from_addr_lc"); got != `*sender\*\?\\@example.com*` {
		t.Fatalf("from wildcard = %q", got)
	}
	if got := wildcardValue(t, must, "subject_lc"); got != `*quarterly\*\?*` {
		t.Fatalf("subject wildcard = %q", got)
	}
	multiMatch := must[1]["multi_match"].(map[string]any)
	if len(multiMatch["query"].(string)) > maxOpenSearchSearchTextBytes || !utf8.ValidString(multiMatch["query"].(string)) {
		t.Fatalf("query length/utf8 = %d/%v", len(multiMatch["query"].(string)), utf8.ValidString(multiMatch["query"].(string)))
	}
}

func TestOpenSearchSearcherRequiresUserID(t *testing.T) {
	t.Parallel()

	searcher, err := NewOpenSearchSearcher(OpenSearchOptions{
		Endpoint: "http://localhost:9200",
		Index:    "messages",
	})
	if err != nil {
		t.Fatalf("NewOpenSearchSearcher returned error: %v", err)
	}
	if _, err := searcher.SearchMessageIDs(context.Background(), OpenSearchSearchQuery{}); err == nil {
		t.Fatal("SearchMessageIDs accepted missing user id")
	}
}

func TestOpenSearchSearcherBoundsResponseBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"hits":[`))
		_, _ = w.Write([]byte(strings.Repeat(" ", int(maxOpenSearchSearchResponseBytes)+1)))
	}))
	defer server.Close()

	searcher, err := NewOpenSearchSearcher(OpenSearchOptions{
		Endpoint: server.URL,
		Index:    "gogomail-messages",
		Client:   server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenSearchSearcher returned error: %v", err)
	}
	_, err = searcher.SearchMessageIDs(context.Background(), OpenSearchSearchQuery{
		UserID: "user-1",
		Query:  "hello",
	})
	if err == nil || !strings.Contains(err.Error(), "opensearch search response is too large") {
		t.Fatalf("error = %v, want oversized response error", err)
	}
}

func TestOpenSearchSearcherSanitizesServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "search failed\ntrace-id: 123", http.StatusBadGateway)
	}))
	defer server.Close()

	searcher, err := NewOpenSearchSearcher(OpenSearchOptions{
		Endpoint: server.URL,
		Index:    "gogomail-messages",
		Client:   server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenSearchSearcher returned error: %v", err)
	}
	_, err = searcher.SearchMessageIDs(context.Background(), OpenSearchSearchQuery{
		UserID: "user-1",
		Query:  "hello",
	})
	if err == nil || !strings.Contains(err.Error(), "502") || !strings.Contains(err.Error(), "search failed trace-id: 123") || strings.ContainsAny(err.Error(), "\r\n") {
		t.Fatalf("error = %q, want sanitized status error", err)
	}
}

func TestOpenSearchSearcherRejectsTrailingResponseTokens(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"hits":[]}}{"extra":true}`))
	}))
	defer server.Close()

	searcher, err := NewOpenSearchSearcher(OpenSearchOptions{
		Endpoint: server.URL,
		Index:    "gogomail-messages",
		Client:   server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenSearchSearcher returned error: %v", err)
	}
	_, err = searcher.SearchMessageIDs(context.Background(), OpenSearchSearchQuery{
		UserID: "user-1",
		Query:  "hello",
	})
	if err == nil || !strings.Contains(err.Error(), "single JSON value") {
		t.Fatalf("error = %v, want trailing token error", err)
	}
}

func TestOpenSearchSearcherCleansHitMessageIDs(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"hits": {
				"hits": [
					{"_id":"fallback%2D1","_score":1,"_source":{"message_id":"msg-1\r\nbad"}},
					{"_id":"bad%0Aid","_score":1,"_source":{}},
					{"_id":"fallback%2D2","_score":1,"_source":{}}
				]
			}
		}`))
	}))
	defer server.Close()

	searcher, err := NewOpenSearchSearcher(OpenSearchOptions{
		Endpoint: server.URL,
		Index:    "gogomail-messages",
		Client:   server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenSearchSearcher returned error: %v", err)
	}
	hits, err := searcher.SearchMessageIDs(context.Background(), OpenSearchSearchQuery{
		UserID: "user-1",
		Query:  "hello",
	})
	if err != nil {
		t.Fatalf("SearchMessageIDs returned error: %v", err)
	}
	if len(hits) != 2 || hits[0].MessageID != "fallback-1" || hits[1].MessageID != "fallback-2" {
		t.Fatalf("hits = %#v", hits)
	}
}

func TestOpenSearchHighlightsAreBounded(t *testing.T) {
	t.Parallel()

	long := "<mark>" + strings.Repeat("한", 300) + "</mark>"
	highlights := openSearchHighlightsFromResponse(map[string][]string{
		"subject": {
			"plain",
			"<mark>one</mark>",
			"<mark>two</mark>",
			"<mark>three</mark>",
			"<mark>four</mark>",
		},
		"body_text": {long},
	})
	if len(highlights.Subject) != maxOpenSearchHighlightFragments {
		t.Fatalf("subject highlights = %#v", highlights.Subject)
	}
	if len(highlights.Body) != 1 || len(highlights.Body[0]) > maxOpenSearchHighlightFragmentBytes {
		t.Fatalf("body highlight = %#v", highlights.Body)
	}
	if !utf8.ValidString(highlights.Body[0]) {
		t.Fatalf("body highlight is invalid UTF-8: %q", highlights.Body[0])
	}
}

func boolSearchPtr(value bool) *bool {
	return &value
}

func wildcardValue(t *testing.T, must []map[string]any, field string) string {
	t.Helper()
	for _, clause := range must {
		wildcard, ok := clause["wildcard"].(map[string]any)
		if !ok {
			continue
		}
		if value, ok := wildcard[field].(string); ok {
			return value
		}
	}
	t.Fatalf("wildcard field %q not found in %#v", field, must)
	return ""
}

func queryMustContainsWildcard(request map[string]any, field string) bool {
	must := request["query"].(map[string]any)["bool"].(map[string]any)["must"].([]any)
	for _, clause := range must {
		item, ok := clause.(map[string]any)
		if !ok {
			continue
		}
		wildcard, ok := item["wildcard"].(map[string]any)
		if !ok {
			continue
		}
		if _, ok := wildcard[field]; ok {
			return true
		}
	}
	return false
}

func queryMustContainsMultiMatchField(request map[string]any, field string) bool {
	must := request["query"].(map[string]any)["bool"].(map[string]any)["must"].([]any)
	for _, clause := range must {
		item, ok := clause.(map[string]any)
		if !ok {
			continue
		}
		multiMatch, ok := item["multi_match"].(map[string]any)
		if !ok {
			continue
		}
		fields, ok := multiMatch["fields"].([]any)
		if !ok {
			continue
		}
		for _, got := range fields {
			if got == field {
				return true
			}
		}
	}
	return false
}
