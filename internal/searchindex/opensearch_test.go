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
		FolderID:      "folder-1",
		Subject:       "Hello",
		FromAddr:      "sender@example.com",
		FromName:      "Sender",
		StoragePath:   "messages/msg-1.eml",
		HasAttachment: true,
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
	if payload["subject"] != "Hello" || payload["subject_lc"] != "hello" {
		t.Fatalf("payload subject = %#v", payload)
	}
	if payload["from_addr"] != "sender@example.com" || payload["from_addr_lc"] != "sender@example.com" || payload["has_attachment"] != true {
		t.Fatalf("payload sender/attachment = %#v", payload)
	}
	if payload["folder_id"] != "folder-1" {
		t.Fatalf("payload folder_id = %#v", payload)
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

func TestOpenSearchDocumentBoundsMetadata(t *testing.T) {
	t.Parallel()

	refs := make([]string, 0, maxEventReferences+10)
	for i := 0; i < maxEventReferences+10; i++ {
		refs = append(refs, strings.Repeat("참", 400))
	}
	doc := openSearchDocument(Document{
		MessageID:  "msg-1",
		Subject:    strings.Repeat("제", 400),
		FromName:   strings.Repeat("보", 400),
		References: append(refs, "bad\nref"),
	})

	subject := doc["subject"].(string)
	if len(subject) > maxOpenSearchMetadataBytes || !utf8.ValidString(subject) {
		t.Fatalf("subject length/utf8 = %d/%v", len(subject), utf8.ValidString(subject))
	}
	fromName := doc["from_name"].(string)
	if len(fromName) > maxOpenSearchMetadataBytes || !utf8.ValidString(fromName) {
		t.Fatalf("from_name length/utf8 = %d/%v", len(fromName), utf8.ValidString(fromName))
	}
	references := doc["references"].([]string)
	if len(references) != maxEventReferences {
		t.Fatalf("references = %d, want %d", len(references), maxEventReferences)
	}
	if len(references[0]) > maxOpenSearchMetadataBytes || !utf8.ValidString(references[0]) {
		t.Fatalf("reference length/utf8 = %d/%v", len(references[0]), utf8.ValidString(references[0]))
	}
}

func TestOpenSearchIndexerEnsuresIndexMapping(t *testing.T) {
	t.Parallel()

	var method, path string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	indexer, err := NewOpenSearchIndexer(OpenSearchOptions{
		Endpoint: server.URL,
		Index:    "gogomail-messages",
		Client:   server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenSearchIndexer returned error: %v", err)
	}
	if err := indexer.EnsureIndex(context.Background()); err != nil {
		t.Fatalf("EnsureIndex returned error: %v", err)
	}

	if method != http.MethodPut || path != "/gogomail-messages" {
		t.Fatalf("request = %s %s, want PUT /gogomail-messages", method, path)
	}
	mappings := payload["mappings"].(map[string]any)
	properties := mappings["properties"].(map[string]any)
	if properties["body_text"].(map[string]any)["type"] != "text" {
		t.Fatalf("body_text mapping = %#v", properties["body_text"])
	}
	if properties["message_id"].(map[string]any)["type"] != "keyword" {
		t.Fatalf("message_id mapping = %#v", properties["message_id"])
	}
	if properties["folder_id"].(map[string]any)["type"] != "keyword" {
		t.Fatalf("folder_id mapping = %#v", properties["folder_id"])
	}
	if properties["from_addr_lc"].(map[string]any)["type"] != "keyword" {
		t.Fatalf("from_addr_lc mapping = %#v", properties["from_addr_lc"])
	}
	if properties["subject_lc"].(map[string]any)["type"] != "keyword" {
		t.Fatalf("subject_lc mapping = %#v", properties["subject_lc"])
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
