package searchindex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type OpenSearchSearchQuery struct {
	UserID string
	Query  string
	Limit  int
}

type OpenSearchHit struct {
	MessageID string
	Score     float64
}

type OpenSearchSearcher struct {
	indexer OpenSearchIndexer
}

func NewOpenSearchSearcher(opts OpenSearchOptions) (OpenSearchSearcher, error) {
	indexer, err := NewOpenSearchIndexer(opts)
	if err != nil {
		return OpenSearchSearcher{}, err
	}
	return OpenSearchSearcher{indexer: indexer}, nil
}

func (s OpenSearchSearcher) SearchMessageIDs(ctx context.Context, query OpenSearchSearchQuery) ([]OpenSearchHit, error) {
	userID := strings.TrimSpace(query.UserID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	payload, err := json.Marshal(openSearchSearchPayload(userID, strings.TrimSpace(query.Query), limit))
	if err != nil {
		return nil, fmt.Errorf("marshal opensearch search request: %w", err)
	}
	target := *s.indexer.endpoint
	target.Path = path.Join(target.Path, s.indexer.index, "_search")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create opensearch search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.indexer.username != "" || s.indexer.password != "" {
		req.SetBasicAuth(s.indexer.username, s.indexer.password)
	}

	resp, err := s.indexer.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search opensearch messages: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("search opensearch messages: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Hits struct {
			Hits []struct {
				ID     string  `json:"_id"`
				Score  float64 `json:"_score"`
				Source struct {
					MessageID string `json:"message_id"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode opensearch search response: %w", err)
	}
	hits := make([]OpenSearchHit, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		messageID := strings.TrimSpace(hit.Source.MessageID)
		if messageID == "" {
			messageID = strings.TrimSpace(hit.ID)
			if unescaped, err := url.PathUnescape(messageID); err == nil {
				messageID = unescaped
			}
		}
		if messageID == "" {
			continue
		}
		hits = append(hits, OpenSearchHit{MessageID: messageID, Score: hit.Score})
	}
	return hits, nil
}

func openSearchSearchPayload(userID string, query string, limit int) map[string]any {
	must := []map[string]any{
		{"term": map[string]any{"user_id": userID}},
	}
	if query != "" {
		must = append(must, map[string]any{
			"multi_match": map[string]any{
				"query":  query,
				"fields": []string{"subject^2", "body_text"},
			},
		})
	}
	return map[string]any{
		"size": limit,
		"_source": []string{
			"message_id",
		},
		"query": map[string]any{
			"bool": map[string]any{
				"must": must,
			},
		},
	}
}
