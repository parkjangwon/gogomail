package searchindex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMailFlowDailyStatsAggregationPayloadIncludesDailyBuckets(t *testing.T) {
	t.Parallel()

	payload := mailFlowDailyStatsAggregationPayload(MailFlowStatsQuery{
		Direction: "outbound",
		CompanyID: "company-1",
		DomainID:  "domain-1",
		UserID:    "user-1",
		Since:     "2026-05-01T00:00:00Z",
		Until:     "2026-05-08T00:00:00Z",
	})

	dateHistogram := payload["aggs"].(map[string]any)["date_histogram"].(map[string]any)
	aggs := dateHistogram["aggs"].(map[string]any)
	for _, name := range []string{"inbound", "outbound", "delivered", "failed", "bounced", "filtered", "rejected"} {
		if _, ok := aggs[name]; !ok {
			t.Fatalf("missing %q bucket in daily aggregation payload: %#v", name, aggs)
		}
	}
}

func TestMailFlowStatsSearcherReturnsDailyFilteredAndRejectedCounts(t *testing.T) {
	t.Parallel()

	var request map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/mail_flow/_search" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"aggregations": {
				"date_histogram": {
					"buckets": [
						{
							"key": 1778025600,
							"inbound": {"doc_count": 3, "size": {"value": 120}},
							"outbound": {"doc_count": 5, "size": {"value": 240}},
							"delivered": {"doc_count": 6},
							"failed": {"doc_count": 1},
							"bounced": {"doc_count": 1},
							"filtered": {"doc_count": 2},
							"rejected": {"doc_count": 1}
						}
					]
				}
			}
		}`))
	}))
	defer server.Close()

	searcher, err := NewMailFlowStatsSearcher(OpenSearchOptions{
		Endpoint: server.URL,
		Index:    "mail-flow",
		Client:   server.Client(),
	})
	if err != nil {
		t.Fatalf("NewMailFlowStatsSearcher returned error: %v", err)
	}

	stats, err := searcher.GetDailyStats(context.Background(), MailFlowStatsQuery{
		CompanyID: "company-1",
		Since:     "2026-05-01T00:00:00Z",
		Until:     "2026-05-08T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("GetDailyStats returned error: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("stats len = %d, want 1", len(stats))
	}
	got := stats[0]
	if got.Date != time.Unix(1778025600, 0).UTC() {
		t.Fatalf("Date = %s", got.Date)
	}
	if got.InboundMessages != 3 || got.OutboundMessages != 5 || got.InboundSize != 120 || got.OutboundSize != 240 {
		t.Fatalf("daily sizes/messages = %#v", got)
	}
	if got.Delivered != 6 || got.Failed != 1 || got.Bounced != 1 || got.Filtered != 2 || got.Rejected != 1 {
		t.Fatalf("daily status counts = %#v", got)
	}
	if request["size"].(float64) != 0 {
		t.Fatalf("request size = %#v", request["size"])
	}
	dateHistogram := request["aggs"].(map[string]any)["date_histogram"].(map[string]any)
	aggs := dateHistogram["aggs"].(map[string]any)
	if _, ok := aggs["filtered"]; !ok {
		t.Fatalf("request missing filtered bucket: %#v", aggs)
	}
	if _, ok := aggs["rejected"]; !ok {
		t.Fatalf("request missing rejected bucket: %#v", aggs)
	}
	bounds, ok := dateHistogram["extended_bounds"].(map[string]any)
	if !ok || bounds["min"] != "2026-05-01T00:00:00Z" || bounds["max"] != "2026-05-08T00:00:00Z" {
		t.Fatalf("extended_bounds = %#v", dateHistogram["extended_bounds"])
	}
}
