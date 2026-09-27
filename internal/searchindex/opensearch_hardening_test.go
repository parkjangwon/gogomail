package searchindex

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestIndexer(t *testing.T, serverURL string, client *http.Client) OpenSearchIndexer {
	t.Helper()
	indexer, err := NewOpenSearchIndexer(OpenSearchOptions{
		Endpoint: serverURL + "/",
		Index:    "gogomail-messages",
		Client:   client,
	})
	if err != nil {
		t.Fatalf("NewOpenSearchIndexer returned error: %v", err)
	}
	return indexer
}

func TestOpenSearchPingSucceedsOnGreenCluster(t *testing.T) {
	t.Parallel()
	var gotPath, gotStatus string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotStatus = r.URL.Query().Get("wait_for_status")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"green"}`)
	}))
	defer server.Close()

	indexer := newTestIndexer(t, server.URL, server.Client())
	if err := indexer.Ping(context.Background()); err != nil {
		t.Fatalf("Ping returned error: %v", err)
	}
	if gotPath != "/_cluster/health" {
		t.Fatalf("path = %q, want /_cluster/health", gotPath)
	}
	if gotStatus != "yellow" {
		t.Fatalf("wait_for_status = %q, want yellow", gotStatus)
	}
}

func TestOpenSearchPingFailsOnRedCluster(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusRequestTimeout)
		_, _ = io.WriteString(w, `{"status":"red"}`)
	}))
	defer server.Close()

	indexer := newTestIndexer(t, server.URL, server.Client())
	if err := indexer.Ping(context.Background()); err == nil {
		t.Fatal("Ping error = nil, want failure for non-2xx health status")
	}
}

func TestOpenSearchMappingVersionReadback(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/_mapping") {
			t.Fatalf("unexpected mapping path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"gogomail-messages":{"mappings":{"_meta":{"gogomail_mapping_version":"`+OpenSearchMappingVersion+`"}}}}`)
	}))
	defer server.Close()

	indexer := newTestIndexer(t, server.URL, server.Client())
	version, err := indexer.MappingVersion(context.Background())
	if err != nil {
		t.Fatalf("MappingVersion returned error: %v", err)
	}
	if version != OpenSearchMappingVersion {
		t.Fatalf("version = %q, want %q", version, OpenSearchMappingVersion)
	}
}

func TestOpenSearchMappingVersionEmptyWhenUnstamped(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"gogomail-messages":{"mappings":{}}}`)
	}))
	defer server.Close()

	indexer := newTestIndexer(t, server.URL, server.Client())
	version, err := indexer.MappingVersion(context.Background())
	if err != nil {
		t.Fatalf("MappingVersion returned error: %v", err)
	}
	if version != "" {
		t.Fatalf("version = %q, want empty for unstamped index", version)
	}
}

func TestOpenSearchEnsureIndexStampsMappingVersion(t *testing.T) {
	t.Parallel()
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	indexer := newTestIndexer(t, server.URL, server.Client())
	if err := indexer.EnsureIndex(context.Background()); err != nil {
		t.Fatalf("EnsureIndex returned error: %v", err)
	}

	mappings, ok := payload["mappings"].(map[string]any)
	if !ok {
		t.Fatalf("mappings missing in index definition: %#v", payload)
	}
	meta, ok := mappings["_meta"].(map[string]any)
	if !ok {
		t.Fatalf("_meta missing in mappings: %#v", mappings)
	}
	if meta["gogomail_mapping_version"] != OpenSearchMappingVersion {
		t.Fatalf("mapping version = %v, want %q", meta["gogomail_mapping_version"], OpenSearchMappingVersion)
	}
}

func TestOpenSearchBulkIndexMessages(t *testing.T) {
	t.Parallel()
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/_bulk") {
			t.Fatalf("unexpected bulk path %q", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-ndjson" {
			t.Fatalf("content type = %q, want application/x-ndjson", ct)
		}
		raw, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(raw))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"errors":false,"items":[]}`)
	}))
	defer server.Close()

	indexer := newTestIndexer(t, server.URL, server.Client())
	docs := []Document{
		{MessageID: "msg-1", UserID: "user-1", StoragePath: "m/1.eml"},
		{MessageID: "msg-2", UserID: "user-1", StoragePath: "m/2.eml"},
	}
	if err := indexer.BulkIndexMessages(context.Background(), docs); err != nil {
		t.Fatalf("BulkIndexMessages returned error: %v", err)
	}
	if len(bodies) != 1 {
		t.Fatalf("expected 1 bulk request, got %d", len(bodies))
	}
	// Two docs => two action lines + two doc lines = 4 non-empty ndjson lines.
	lines := 0
	for _, line := range strings.Split(strings.TrimRight(bodies[0], "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			lines++
		}
	}
	if lines != 4 {
		t.Fatalf("expected 4 ndjson lines, got %d: %q", lines, bodies[0])
	}
}

func TestOpenSearchBulkIndexChunksByBatchSize(t *testing.T) {
	t.Parallel()
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"errors":false,"items":[]}`)
	}))
	defer server.Close()

	indexer := newTestIndexer(t, server.URL, server.Client())
	// maxOpenSearchBulkBatch + 1 docs must produce 2 requests (backpressure).
	docs := make([]Document, maxOpenSearchBulkBatch+1)
	for i := range docs {
		docs[i] = Document{MessageID: "msg-" + itoa(i), UserID: "u", StoragePath: "m/x.eml"}
	}
	if err := indexer.BulkIndexMessages(context.Background(), docs); err != nil {
		t.Fatalf("BulkIndexMessages returned error: %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("expected 2 chunked bulk requests, got %d", requestCount)
	}
}

func TestOpenSearchBulkIndexReturnsItemError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"errors":true,"items":[{"index":{"status":400,"error":"mapper_parsing_exception"}}]}`)
	}))
	defer server.Close()

	indexer := newTestIndexer(t, server.URL, server.Client())
	err := indexer.BulkIndexMessages(context.Background(), []Document{{MessageID: "msg-1", UserID: "u", StoragePath: "m/1.eml"}})
	if err == nil {
		t.Fatal("BulkIndexMessages error = nil, want per-item bulk error")
	}
	if !strings.Contains(err.Error(), "mapper_parsing_exception") {
		t.Fatalf("error = %v, want mapper_parsing_exception detail", err)
	}
}

func TestOpenSearchBulkIndexEmptyIsNoop(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("bulk endpoint should not be called for empty input")
	}))
	defer server.Close()

	indexer := newTestIndexer(t, server.URL, server.Client())
	if err := indexer.BulkIndexMessages(context.Background(), nil); err != nil {
		t.Fatalf("BulkIndexMessages(nil) returned error: %v", err)
	}
}

// itoa is a tiny int-to-string helper to avoid importing strconv in the test
// for a single use.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
