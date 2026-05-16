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
	"time"

	"github.com/gogomail/gogomail/internal/webhook"
)

const (
	maxMailFlowStatsResponseBytes = int64(4 << 20)
)

type MailFlowStatsQuery struct {
	Direction string
	CompanyID string
	DomainID  string
	UserID    string
	Since     string
	Until     string
}

type MailFlowStatsResult struct {
	TotalMessages    int64   `json:"total_messages"`
	UniqueSenders    int64   `json:"unique_senders"`
	UniqueDomains    int64   `json:"unique_domains"`
	TotalSizeBytes   int64   `json:"total_size_bytes"`
	AverageSizeBytes float64 `json:"average_size_bytes"`
	MaxSizeBytes     int64   `json:"max_size_bytes"`
	Delivered        int64   `json:"delivered"`
	Failed           int64   `json:"failed"`
	Bounced          int64   `json:"bounced"`
	Filtered         int64   `json:"filtered"`
	Rejected         int64   `json:"rejected"`
	DeliveryRate     float64 `json:"delivery_rate"`
}

type MailFlowDailyStatsResult struct {
	Date             time.Time `json:"date"`
	InboundMessages  int64     `json:"inbound_messages"`
	OutboundMessages int64     `json:"outbound_messages"`
	InboundSize      int64     `json:"inbound_size_bytes"`
	OutboundSize     int64     `json:"outbound_size_bytes"`
	Delivered        int64     `json:"delivered"`
	Failed           int64     `json:"failed"`
	Bounced          int64     `json:"bounced"`
	Filtered         int64     `json:"filtered"`
	Rejected         int64     `json:"rejected"`
}

type MailFlowStatsSearcher struct {
	indexer MailFlowIndexer
}

func NewMailFlowStatsSearcher(opts OpenSearchOptions) (MailFlowStatsSearcher, error) {
	indexer, err := NewMailFlowIndexer(opts)
	if err != nil {
		return MailFlowStatsSearcher{}, err
	}
	return MailFlowStatsSearcher{indexer: indexer}, nil
}

func (s MailFlowStatsSearcher) GetStats(ctx context.Context, query MailFlowStatsQuery) (MailFlowStatsResult, error) {
	payload := mailFlowStatsAggregationPayload(query)
	return s.executeStatsQuery(ctx, payload)
}

func (s MailFlowStatsSearcher) GetDailyStats(ctx context.Context, query MailFlowStatsQuery) ([]MailFlowDailyStatsResult, error) {
	payload := mailFlowDailyStatsAggregationPayload(query)
	return s.executeDailyStatsQuery(ctx, payload)
}

func (s MailFlowStatsSearcher) executeStatsQuery(ctx context.Context, payload map[string]any) (MailFlowStatsResult, error) {
	body, err := s.executeSearch(ctx, payload)
	if err != nil {
		return MailFlowStatsResult{}, err
	}
	defer body.Close()

	raw, err := io.ReadAll(io.LimitReader(body, maxMailFlowStatsResponseBytes+1))
	if err != nil {
		return MailFlowStatsResult{}, fmt.Errorf("read mail flow stats response: %w", err)
	}
	if int64(len(raw)) > maxMailFlowStatsResponseBytes {
		return MailFlowStatsResult{}, fmt.Errorf("mail flow stats response is too large")
	}

	var resp mailFlowStatsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return MailFlowStatsResult{}, fmt.Errorf("decode mail flow stats response: %w", err)
	}

	result := MailFlowStatsResult{
		TotalMessages:    resp.Aggregations.TotalMessages.Value,
		UniqueSenders:    resp.Aggregations.UniqueSenders.Value,
		UniqueDomains:    resp.Aggregations.UniqueDomains.Value,
		TotalSizeBytes:   int64(resp.Aggregations.TotalSize.Value),
		AverageSizeBytes: resp.Aggregations.AverageSize.Value,
		MaxSizeBytes:     int64(resp.Aggregations.MaxSize.Value),
		Delivered:        resp.Aggregations.Delivered.DocCount,
		Failed:           resp.Aggregations.Failed.DocCount,
		Bounced:          resp.Aggregations.Bounced.DocCount,
		Filtered:         resp.Aggregations.Filtered.DocCount,
		Rejected:         resp.Aggregations.Rejected.DocCount,
	}
	if result.TotalMessages > 0 {
		result.DeliveryRate = float64(result.Delivered) / float64(result.TotalMessages)
	}

	return result, nil
}

func (s MailFlowStatsSearcher) executeDailyStatsQuery(ctx context.Context, payload map[string]any) ([]MailFlowDailyStatsResult, error) {
	body, err := s.executeSearch(ctx, payload)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	raw, err := io.ReadAll(io.LimitReader(body, maxMailFlowStatsResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read mail flow daily stats response: %w", err)
	}
	if int64(len(raw)) > maxMailFlowStatsResponseBytes {
		return nil, fmt.Errorf("mail flow daily stats response is too large")
	}

	var resp mailFlowDailyStatsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode mail flow daily stats response: %w", err)
	}

	results := make([]MailFlowDailyStatsResult, 0, len(resp.Aggregations.DateHistogram.Buckets))
	for _, bucket := range resp.Aggregations.DateHistogram.Buckets {
		r := MailFlowDailyStatsResult{
			Date:             time.Unix(bucket.Key, 0).UTC(),
			InboundMessages:  bucket.Inbound.DocCount,
			OutboundMessages: bucket.Outbound.DocCount,
			InboundSize:      int64(bucket.Inbound.Size.Value),
			OutboundSize:     int64(bucket.Outbound.Size.Value),
			Delivered:        bucket.Delivered.DocCount,
			Failed:           bucket.Failed.DocCount,
			Bounced:          bucket.Bounced.DocCount,
			Filtered:         bucket.Filtered.DocCount,
			Rejected:         bucket.Rejected.DocCount,
		}
		results = append(results, r)
	}

	return results, nil
}

func (s MailFlowStatsSearcher) executeSearch(ctx context.Context, payload map[string]any) (io.ReadCloser, error) {
	reqPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal mail flow stats query: %w", err)
	}
	target := *s.indexer.indexer.endpoint
	target.Path = path.Join(target.Path, s.indexer.indexer.index, "_search")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(reqPayload))
	if err != nil {
		return nil, fmt.Errorf("create mail flow stats request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.indexer.indexer.username != "" || s.indexer.indexer.password != "" {
		req.SetBasicAuth(s.indexer.indexer.username, s.indexer.indexer.password)
	}

	resp, err := s.indexer.indexer.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search mail flow stats: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer func() { _ = webhook.DrainAndClose(resp.Body, webhook.DefaultDrainBytes) }()
		return nil, fmt.Errorf("search mail flow stats: status %d: %s", resp.StatusCode, webhook.ErrorBodyPreview(resp.Body, 512))
	}
	return resp.Body, nil
}

func mailFlowStatsAggregationPayload(query MailFlowStatsQuery) map[string]any {
	must := buildMailFlowMustClauses(query)

	return map[string]any{
		"size": 0,
		"query": map[string]any{
			"bool": map[string]any{"must": must},
		},
		"aggs": map[string]any{
			"unique_senders": map[string]any{"cardinality": map[string]any{"field": "from_addr"}},
			"unique_domains": map[string]any{
				"cardinality": map[string]any{
					"script": map[string]any{
						"source": "def parts = doc['from_addr'].value.splitOnToken('@'); return parts.length > 1 ? parts[1] : doc['from_addr'].value",
					},
				},
			},
			"total_messages": map[string]any{"value_count": map[string]any{"field": "message_id"}},
			"total_size":     map[string]any{"sum": map[string]any{"field": "size"}},
			"average_size":   map[string]any{"avg": map[string]any{"field": "size"}},
			"max_size":       map[string]any{"max": map[string]any{"field": "size"}},
			"delivered":      map[string]any{"filter": map[string]any{"term": map[string]any{"flow_status": "delivered"}}},
			"failed":         map[string]any{"filter": map[string]any{"term": map[string]any{"flow_status": "failed"}}},
			"bounced":        map[string]any{"filter": map[string]any{"term": map[string]any{"flow_status": "bounced"}}},
			"filtered":       map[string]any{"filter": map[string]any{"term": map[string]any{"flow_status": "filtered"}}},
			"rejected":       map[string]any{"filter": map[string]any{"term": map[string]any{"flow_status": "rejected"}}},
		},
	}
}

func mailFlowDailyStatsAggregationPayload(query MailFlowStatsQuery) map[string]any {
	must := buildMailFlowMustClauses(query)
	dateHistogram := map[string]any{
		"field":             "created_at",
		"calendar_interval": "day",
		"time_zone":         "UTC",
		"min_doc_count":     0,
		"aggs": map[string]any{
			"inbound": map[string]any{
				"filter": map[string]any{"term": map[string]any{"direction": "inbound"}},
				"aggs": map[string]any{
					"size": map[string]any{"sum": map[string]any{"field": "size"}},
				},
			},
			"outbound": map[string]any{
				"filter": map[string]any{"term": map[string]any{"direction": "outbound"}},
				"aggs": map[string]any{
					"size": map[string]any{"sum": map[string]any{"field": "size"}},
				},
			},
			"delivered": map[string]any{"filter": map[string]any{"term": map[string]any{"flow_status": "delivered"}}},
			"failed":    map[string]any{"filter": map[string]any{"term": map[string]any{"flow_status": "failed"}}},
			"bounced":   map[string]any{"filter": map[string]any{"term": map[string]any{"flow_status": "bounced"}}},
			"filtered":  map[string]any{"filter": map[string]any{"term": map[string]any{"flow_status": "filtered"}}},
			"rejected":  map[string]any{"filter": map[string]any{"term": map[string]any{"flow_status": "rejected"}}},
		},
	}
	if query.Since != "" || query.Until != "" {
		bounds := map[string]any{}
		if query.Since != "" {
			bounds["min"] = query.Since
		}
		if query.Until != "" {
			bounds["max"] = query.Until
		}
		dateHistogram["extended_bounds"] = bounds
	}

	return map[string]any{
		"size":  0,
		"query": map[string]any{"bool": map[string]any{"must": must}},
		"aggs": map[string]any{
			"date_histogram": dateHistogram,
		},
	}
}

func buildMailFlowMustClauses(query MailFlowStatsQuery) []map[string]any {
	var must []map[string]any

	if query.Direction != "" {
		must = append(must, map[string]any{"term": map[string]any{"direction": strings.ToLower(strings.TrimSpace(query.Direction))}})
	}
	if query.CompanyID != "" {
		must = append(must, map[string]any{"term": map[string]any{"company_id": strings.TrimSpace(query.CompanyID)}})
	}
	if query.DomainID != "" {
		must = append(must, map[string]any{"term": map[string]any{"domain_id": strings.TrimSpace(query.DomainID)}})
	}
	if query.UserID != "" {
		must = append(must, map[string]any{"term": map[string]any{"user_id": strings.TrimSpace(query.UserID)}})
	}
	if query.Since != "" || query.Until != "" {
		rangeFilter := map[string]any{}
		if query.Since != "" {
			rangeFilter["gte"] = query.Since
		}
		if query.Until != "" {
			rangeFilter["lte"] = query.Until
		}
		must = append(must, map[string]any{"range": map[string]any{"created_at": rangeFilter}})
	}

	if must == nil {
		must = []map[string]any{{"match_all": map[string]any{}}}
	}
	return must
}

type mailFlowStatsResponse struct {
	Aggregations struct {
		TotalMessages struct {
			Value int64 `json:"value"`
		} `json:"total_messages"`
		UniqueSenders struct {
			Value int64 `json:"value"`
		} `json:"unique_senders"`
		UniqueDomains struct {
			Value int64 `json:"value"`
		} `json:"unique_domains"`
		TotalSize struct {
			Value float64 `json:"value"`
		} `json:"total_size"`
		AverageSize struct {
			Value float64 `json:"value"`
		} `json:"average_size"`
		MaxSize struct {
			Value float64 `json:"value"`
		} `json:"max_size"`
		Delivered struct {
			DocCount int64 `json:"doc_count"`
		} `json:"delivered"`
		Failed struct {
			DocCount int64 `json:"doc_count"`
		} `json:"failed"`
		Bounced struct {
			DocCount int64 `json:"doc_count"`
		} `json:"bounced"`
		Filtered struct {
			DocCount int64 `json:"doc_count"`
		} `json:"filtered"`
		Rejected struct {
			DocCount int64 `json:"doc_count"`
		} `json:"rejected"`
	} `json:"aggregations"`
}

type mailFlowDailyStatsResponse struct {
	Aggregations struct {
		DateHistogram struct {
			Buckets []struct {
				Key     int64 `json:"key"`
				Inbound struct {
					DocCount int64 `json:"doc_count"`
					Size     struct {
						Value float64 `json:"value"`
					} `json:"size"`
				} `json:"inbound"`
				Outbound struct {
					DocCount int64 `json:"doc_count"`
					Size     struct {
						Value float64 `json:"value"`
					} `json:"size"`
				} `json:"outbound"`
				Delivered struct {
					DocCount int64 `json:"doc_count"`
				} `json:"delivered"`
				Failed struct {
					DocCount int64 `json:"doc_count"`
				} `json:"failed"`
				Bounced struct {
					DocCount int64 `json:"doc_count"`
				} `json:"bounced"`
				Filtered struct {
					DocCount int64 `json:"doc_count"`
				} `json:"filtered"`
				Rejected struct {
					DocCount int64 `json:"doc_count"`
				} `json:"rejected"`
			} `json:"buckets"`
		} `json:"date_histogram"`
	} `json:"aggregations"`
}
