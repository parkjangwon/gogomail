package searchindex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/gogomail/gogomail/internal/webhook"
)

// maxOpenSearchBulkBatch bounds how many documents a single BulkIndexMessages
// call sends to OpenSearch in one _bulk request. It provides backpressure:
// callers with a larger slice are chunked into fixed-size requests so a single
// request never grows unbounded and overwhelms the cluster.
const maxOpenSearchBulkBatch = 500

// Ping performs a lightweight cluster health check against the OpenSearch
// endpoint. It is used by readiness probes and the operator health path to
// confirm the cluster is reachable before the search backend is marked ready.
// Any non-2xx status or transport error is returned so callers can degrade
// gracefully (e.g. fall back to the Postgres search backend).
func (i OpenSearchIndexer) Ping(ctx context.Context) error {
	target := *i.endpoint
	target.Path = path.Join(target.Path, "_cluster", "health")
	q := target.Query()
	// wait_for_status=yellow tolerates a single-node cluster (no replicas
	// assigned) while still rejecting a red cluster with unassigned primaries.
	q.Set("wait_for_status", "yellow")
	q.Set("timeout", "5s")
	target.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return fmt.Errorf("create opensearch health request: %w", err)
	}
	if i.username != "" || i.password != "" {
		req.SetBasicAuth(i.username, i.password)
	}

	resp, err := i.client.Do(req)
	if err != nil {
		return fmt.Errorf("opensearch health check: %w", err)
	}
	defer func() { _ = webhook.DrainAndClose(resp.Body, webhook.DefaultDrainBytes) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("opensearch health check: status %d: %s", resp.StatusCode, webhook.ErrorBodyPreview(resp.Body, 512))
	}
	return nil
}

// MappingVersion reads the gogomail_mapping_version stamped into the index
// _meta at creation time. Operators can compare the returned value against
// OpenSearchMappingVersion to detect a stale index that predates the current
// mapping and needs a rollover/reindex. An empty string is returned when the
// index has no version stamp (created before versioning was introduced).
func (i OpenSearchIndexer) MappingVersion(ctx context.Context) (string, error) {
	target := *i.endpoint
	target.Path = path.Join(target.Path, i.index, "_mapping")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return "", fmt.Errorf("create opensearch mapping request: %w", err)
	}
	if i.username != "" || i.password != "" {
		req.SetBasicAuth(i.username, i.password)
	}

	resp, err := i.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("read opensearch mapping: %w", err)
	}
	defer func() { _ = webhook.DrainAndClose(resp.Body, webhook.DefaultDrainBytes) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("read opensearch mapping: status %d: %s", resp.StatusCode, webhook.ErrorBodyPreview(resp.Body, 512))
	}

	// Response shape: {"<index>":{"mappings":{"_meta":{"gogomail_mapping_version":"..."}}}}
	var decoded map[string]struct {
		Mappings struct {
			Meta struct {
				MappingVersion string `json:"gogomail_mapping_version"`
			} `json:"_meta"`
		} `json:"mappings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", fmt.Errorf("decode opensearch mapping: %w", err)
	}
	for _, entry := range decoded {
		if v := strings.TrimSpace(entry.Mappings.Meta.MappingVersion); v != "" {
			return v, nil
		}
	}
	return "", nil
}

// BulkIndexMessages indexes multiple documents using the OpenSearch _bulk API.
// It chunks the input into batches of at most maxOpenSearchBulkBatch documents
// to bound request size and apply backpressure. Each batch is a single HTTP
// round-trip; if any item in a batch fails, the whole call returns an error so
// the caller (e.g. the outbox-relay / search-index worker) can retry via the
// existing at-least-once delivery path.
func (i OpenSearchIndexer) BulkIndexMessages(ctx context.Context, docs []Document) error {
	if len(docs) == 0 {
		return nil
	}
	for start := 0; start < len(docs); start += maxOpenSearchBulkBatch {
		end := start + maxOpenSearchBulkBatch
		if end > len(docs) {
			end = len(docs)
		}
		if err := i.bulkIndexBatch(ctx, docs[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func (i OpenSearchIndexer) bulkIndexBatch(ctx context.Context, docs []Document) error {
	var body bytes.Buffer
	for idx := range docs {
		messageID, err := cleanOpenSearchDocumentID(docs[idx].MessageID)
		if err != nil {
			return err
		}
		action := map[string]any{
			"index": map[string]any{
				"_index": i.index,
				"_id":    messageID,
			},
		}
		actionLine, err := json.Marshal(action)
		if err != nil {
			return fmt.Errorf("marshal opensearch bulk action: %w", err)
		}
		docLine, err := json.Marshal(openSearchDocument(docs[idx]))
		if err != nil {
			return fmt.Errorf("marshal opensearch bulk document: %w", err)
		}
		body.Write(actionLine)
		body.WriteByte('\n')
		body.Write(docLine)
		body.WriteByte('\n')
	}

	target := *i.endpoint
	target.Path = path.Join(target.Path, "_bulk")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(body.Bytes()))
	if err != nil {
		return fmt.Errorf("create opensearch bulk request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	if i.username != "" || i.password != "" {
		req.SetBasicAuth(i.username, i.password)
	}

	resp, err := i.client.Do(req)
	if err != nil {
		return fmt.Errorf("opensearch bulk index: %w", err)
	}
	defer func() { _ = webhook.DrainAndClose(resp.Body, webhook.DefaultDrainBytes) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("opensearch bulk index: status %d: %s", resp.StatusCode, webhook.ErrorBodyPreview(resp.Body, 512))
	}

	return checkOpenSearchBulkErrors(resp.Body)
}

// checkOpenSearchBulkErrors inspects the top-level "errors" flag of a _bulk
// response and returns an error describing the first failed item, if any.
func checkOpenSearchBulkErrors(body io.Reader) error {
	// Bounded read to avoid unbounded memory use on a pathological response.
	limited := io.LimitReader(body, maxOpenSearchSearchResponseBytes+1)
	var resp struct {
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			Status int             `json:"status"`
			Error  json.RawMessage `json:"error"`
		} `json:"items"`
	}
	if err := json.NewDecoder(limited).Decode(&resp); err != nil {
		return fmt.Errorf("decode opensearch bulk response: %w", err)
	}
	if !resp.Errors {
		return nil
	}
	for _, item := range resp.Items {
		for action, result := range item {
			if result.Status < 200 || result.Status >= 300 {
				return fmt.Errorf("opensearch bulk %s failed: status %d: %s", action, result.Status, string(result.Error))
			}
		}
	}
	return fmt.Errorf("opensearch bulk index reported errors")
}
