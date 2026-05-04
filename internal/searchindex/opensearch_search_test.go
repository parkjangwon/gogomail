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
					{"_id":"msg-1","_score":1.5,"_source":{"message_id":"msg-1"}},
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
		UserID: "user-1",
		Query:  "hello",
		Limit:  2,
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
