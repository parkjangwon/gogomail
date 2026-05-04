package searchindex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
	if len(hits[0].Highlights.Subject) != 1 || len(hits[0].Highlights.Body) != 1 {
		t.Fatalf("highlights = %#v", hits[0].Highlights)
	}
	if request["highlight"] == nil {
		t.Fatalf("request did not include highlighter: %#v", request)
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

func boolSearchPtr(value bool) *bool {
	return &value
}
