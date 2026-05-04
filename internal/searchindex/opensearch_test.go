package searchindex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenSearchIndexerIndexesDocumentByMessageID(t *testing.T) {
	t.Parallel()

	var method, path string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	indexer, err := NewOpenSearchIndexer(OpenSearchOptions{
		Endpoint: server.URL + "/",
		Index:    "gogomail-messages",
		Client:   server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenSearchIndexer returned error: %v", err)
	}

	err = indexer.IndexMessage(context.Background(), Document{
		MessageID:     "msg-1",
		RFCMessageID:  "<msg-1@example.com>",
		UserID:        "user-1",
		DomainID:      "domain-1",
		Subject:       "Hello",
		StoragePath:   "messages/msg-1.eml",
		BodyText:      "search me",
		BodyTruncated: true,
		BodyMaxBytes:  1024,
	})
	if err != nil {
		t.Fatalf("IndexMessage returned error: %v", err)
	}

	if method != http.MethodPut {
		t.Fatalf("method = %q, want PUT", method)
	}
	if path != "/gogomail-messages/_doc/msg-1" {
		t.Fatalf("path = %q", path)
	}
	if payload["message_id"] != "msg-1" || payload["body_text"] != "search me" {
		t.Fatalf("payload = %#v", payload)
	}
	if payload["body_truncated"] != true || payload["body_max_bytes"].(float64) != 1024 {
		t.Fatalf("payload truncation fields = %#v", payload)
	}
}

func TestOpenSearchIndexerReportsServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad index", http.StatusBadGateway)
	}))
	defer server.Close()

	indexer, err := NewOpenSearchIndexer(OpenSearchOptions{
		Endpoint: server.URL,
		Index:    "messages",
		Client:   server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenSearchIndexer returned error: %v", err)
	}

	err = indexer.IndexMessage(context.Background(), Document{MessageID: "msg-1"})
	if err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("error = %v, want status error", err)
	}
}

func TestOpenSearchIndexerRequiresOptions(t *testing.T) {
	t.Parallel()

	if _, err := NewOpenSearchIndexer(OpenSearchOptions{}); err == nil {
		t.Fatal("NewOpenSearchIndexer accepted empty options")
	}
	if _, err := NewOpenSearchIndexer(OpenSearchOptions{Endpoint: "http://localhost:9200", Index: "../bad"}); err == nil {
		t.Fatal("NewOpenSearchIndexer accepted unsafe index")
	}
}
