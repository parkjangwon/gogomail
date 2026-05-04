package searchindex

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestOpenSearchIntegrationIndexesAndSearches(t *testing.T) {
	endpoint := strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_OPENSEARCH_URL"))
	if endpoint == "" {
		t.Skip("set GOGOMAIL_TEST_OPENSEARCH_URL to run OpenSearch integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	opts := OpenSearchOptions{
		Endpoint: endpoint,
		Index:    "gogomail-test-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		Username: firstEnv("GOGOMAIL_TEST_OPENSEARCH_USERNAME", "GOGOMAIL_SEARCH_INDEX_OPENSEARCH_USERNAME"),
		Password: firstEnv("GOGOMAIL_TEST_OPENSEARCH_PASSWORD", "GOGOMAIL_SEARCH_INDEX_OPENSEARCH_PASSWORD"),
	}
	indexer, err := NewOpenSearchIndexer(opts)
	if err != nil {
		t.Fatalf("NewOpenSearchIndexer returned error: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = openSearchIntegrationRequest(cleanupCtx, indexer, http.MethodDelete, "")
	})

	if err := indexer.EnsureIndex(ctx); err != nil {
		t.Fatalf("EnsureIndex returned error: %v", err)
	}
	docs := []Document{
		{
			MessageID:     "msg-folder-1",
			UserID:        "user-1",
			FolderID:      "folder-1",
			Subject:       "Quarterly Launch",
			FromAddr:      "sender@example.com",
			HasAttachment: true,
			BodyText:      "quarterly launch budget attachment",
			ReceivedAt:    "2026-05-04T00:00:00Z",
		},
		{
			MessageID:     "msg-folder-2",
			UserID:        "user-1",
			FolderID:      "folder-2",
			Subject:       "Quarterly Launch",
			FromAddr:      "sender@example.com",
			HasAttachment: true,
			BodyText:      "quarterly launch budget attachment",
			ReceivedAt:    "2026-05-04T00:01:00Z",
		},
	}
	for _, doc := range docs {
		if err := indexer.IndexMessage(ctx, doc); err != nil {
			t.Fatalf("IndexMessage(%s) returned error: %v", doc.MessageID, err)
		}
	}
	if err := openSearchIntegrationRequest(ctx, indexer, http.MethodPost, "_refresh"); err != nil {
		t.Fatalf("refresh index: %v", err)
	}

	searcher, err := NewOpenSearchSearcher(opts)
	if err != nil {
		t.Fatalf("NewOpenSearchSearcher returned error: %v", err)
	}
	hits, err := searcher.SearchMessageIDs(ctx, OpenSearchSearchQuery{
		UserID:            "user-1",
		FolderID:          "folder-1",
		Query:             "launch",
		From:              "sender@example.com",
		Subject:           "Quarterly",
		HasAttachment:     boolSearchPtr(true),
		IncludeHighlights: true,
		Limit:             10,
	})
	if err != nil {
		t.Fatalf("SearchMessageIDs returned error: %v", err)
	}
	if len(hits) != 1 || hits[0].MessageID != "msg-folder-1" {
		t.Fatalf("hits = %#v, want only folder-1 document", hits)
	}
}

func openSearchIntegrationRequest(ctx context.Context, indexer OpenSearchIndexer, method string, suffix string) error {
	target := *indexer.endpoint
	parts := []string{target.Path, indexer.index}
	if strings.TrimSpace(suffix) != "" {
		parts = append(parts, suffix)
	}
	target.Path = path.Join(parts...)

	req, err := http.NewRequestWithContext(ctx, method, target.String(), nil)
	if err != nil {
		return fmt.Errorf("create opensearch integration request: %w", err)
	}
	if indexer.username != "" || indexer.password != "" {
		req.SetBasicAuth(indexer.username, indexer.password)
	}
	resp, err := indexer.client.Do(req)
	if err != nil {
		return fmt.Errorf("send opensearch integration request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s %s: status %d: %s", method, target.Path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}
