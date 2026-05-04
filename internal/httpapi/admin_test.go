package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/backpressure"
	"github.com/gogomail/gogomail/internal/dnscheck"
	"github.com/gogomail/gogomail/internal/maildb"
)

func TestAdminQueueHandler(t *testing.T) {
	t.Parallel()

	oldestReadyAt := time.Date(2026, 5, 4, 8, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		queueStats: []maildb.QueueStat{{
			Topic:         "mail.outbound.general",
			Status:        "pending",
			Count:         2,
			ReadyCount:    1,
			DelayedCount:  1,
			OldestReadyAt: &oldestReadyAt,
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/queue", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Queues []maildb.QueueStat `json:"queues"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Queues) != 1 || body.Queues[0].Count != 2 || body.Queues[0].ReadyCount != 1 || body.Queues[0].DelayedCount != 1 || body.Queues[0].OldestReadyAt == nil {
		t.Fatalf("queues = %+v", body.Queues)
	}
}

func TestAdminOutboxEventsHandler(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		outboxEvents: []maildb.OutboxEventView{{
			ID:           "outbox-1",
			Topic:        "mail.event",
			PartitionKey: "msg-1",
			Status:       "pending",
			Attempts:     1,
			CreatedAt:    now,
			AvailableAt:  now,
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/outbox-events?limit=10&topic=mail.event&partition_key=msg-1&status=pending&since=2026-05-04T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Events []maildb.OutboxEventView `json:"outbox_events"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Events) != 1 || body.Events[0].ID != "outbox-1" {
		t.Fatalf("outbox_events = %+v", body.Events)
	}
	if service.lastOutboxEventList.Limit != 10 || service.lastOutboxEventList.Topic != "mail.event" || service.lastOutboxEventList.PartitionKey != "msg-1" || service.lastOutboxEventList.Status != "pending" || service.lastOutboxEventList.Since.IsZero() {
		t.Fatalf("lastOutboxEventList = %+v", service.lastOutboxEventList)
	}
}

func TestAdminOutboxEventsHandlerRejectsInvalidSince(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/outbox-events?since=not-a-time", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "since must be RFC3339 timestamp") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminOutboxEventsHandlerRejectsInvalidStatus(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/outbox-events?status=stuck", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "unsupported outbox status") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminOutboxEventDetailHandler(t *testing.T) {
	t.Parallel()

	longError := strings.Repeat("redis down ", 80)
	now := time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		outboxEvent: maildb.OutboxEventView{
			ID:           "outbox-1",
			Topic:        "mail.event",
			PartitionKey: "msg-1",
			Status:       "failed",
			Attempts:     10,
			LastError:    longError,
			CreatedAt:    now,
			AvailableAt:  now,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/outbox-events/outbox-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Event maildb.OutboxEventView `json:"outbox_event"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Event.ID != "outbox-1" || body.Event.LastError != longError {
		t.Fatalf("outbox_event = %+v", body.Event)
	}
	if service.lastOutboxEventID != "outbox-1" {
		t.Fatalf("lastOutboxEventID = %q", service.lastOutboxEventID)
	}
}

func TestAdminBackpressureHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		backpressureState: backpressure.State{Level: "warning", Reason: "queue lag"},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/backpressure", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Backpressure backpressure.State `json:"backpressure"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Backpressure.Level != "warning" || body.Backpressure.Reason != "queue lag" {
		t.Fatalf("backpressure = %+v", body.Backpressure)
	}
}

func TestAdminUpdateBackpressureHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/backpressure", strings.NewReader(`{
		"level": "danger",
		"reason": "queue lag above threshold"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastBackpressureUpdate.Level != "danger" || service.lastBackpressureUpdate.Reason != "queue lag above threshold" {
		t.Fatalf("lastBackpressureUpdate = %+v", service.lastBackpressureUpdate)
	}
	if !strings.Contains(rec.Body.String(), `"backpressure"`) {
		t.Fatalf("response missing backpressure envelope: %s", rec.Body.String())
	}
}

func TestAdminQuotaUsageHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		quotaUsage: []maildb.QuotaUsageView{{
			Scope:            "domain",
			ID:               "domain-1",
			DomainID:         "domain-1",
			Name:             "example.com",
			QuotaUsed:        900,
			QuotaLimit:       1000,
			QuotaRemaining:   100,
			AllocatedQuota:   700,
			AllocatableQuota: 300,
			UsageRatio:       0.9,
			AllocationRatio:  0.7,
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/quota-usage?limit=5", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		QuotaUsage []maildb.QuotaUsageView `json:"quota_usage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.QuotaUsage) != 1 || body.QuotaUsage[0].Name != "example.com" {
		t.Fatalf("quota_usage = %+v", body.QuotaUsage)
	}
	if body.QuotaUsage[0].QuotaRemaining != 100 || body.QuotaUsage[0].AllocatableQuota != 300 {
		t.Fatalf("quota capacity fields = %+v", body.QuotaUsage[0])
	}
	if service.lastLimit != 5 {
		t.Fatalf("lastLimit = %d, want 5", service.lastLimit)
	}
}

func TestAdminAPIUsageDailyHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageDaily: []maildb.APIUsageDailyView{{
			Day:              time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
			Method:           "GET",
			Route:            "GET /api/v1/messages",
			Status:           200,
			TenantID:         "tenant-1",
			CompanyID:        "company-1",
			DomainID:         "domain-1",
			UserID:           "user-1",
			APIKeyID:         "api-key-1",
			PrincipalID:      "principal-1",
			AuthSource:       "bearer",
			RequestCount:     4,
			RequestBytes:     40,
			ResponseBytes:    400,
			LatencyMSTotal:   100,
			LatencyMSMax:     40,
			LatencyMSAverage: 25,
			FirstSeenAt:      time.Date(2026, 5, 4, 1, 0, 0, 0, time.UTC),
			LastSeenAt:       time.Date(2026, 5, 4, 2, 0, 0, 0, time.UTC),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/daily?limit=5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		APIUsageDaily []maildb.APIUsageDailyView `json:"api_usage_daily"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.APIUsageDaily) != 1 || body.APIUsageDaily[0].LatencyMSAverage != 25 {
		t.Fatalf("api_usage_daily = %+v", body.APIUsageDaily)
	}
	if body.APIUsageDaily[0].TenantID != "tenant-1" || body.APIUsageDaily[0].PrincipalID != "principal-1" {
		t.Fatalf("api_usage_daily identity = %+v", body.APIUsageDaily[0])
	}
	if service.lastLimit != 5 {
		t.Fatalf("lastLimit = %d, want 5", service.lastLimit)
	}
}

func TestAdminAPIUsageMonthlyHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageMonthly: []maildb.APIUsageMonthlyView{{
			Month:            time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
			Method:           "GET",
			Route:            "GET /api/v1/messages",
			Status:           200,
			TenantID:         "tenant-1",
			PrincipalID:      "principal-1",
			AuthSource:       "bearer",
			RequestCount:     4,
			LatencyMSTotal:   100,
			LatencyMSAverage: 25,
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/monthly?limit=5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		APIUsageMonthly []maildb.APIUsageMonthlyView `json:"api_usage_monthly"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.APIUsageMonthly) != 1 || body.APIUsageMonthly[0].LatencyMSAverage != 25 {
		t.Fatalf("api_usage_monthly = %+v", body.APIUsageMonthly)
	}
	if body.APIUsageMonthly[0].TenantID != "tenant-1" || body.APIUsageMonthly[0].PrincipalID != "principal-1" {
		t.Fatalf("api_usage_monthly identity = %+v", body.APIUsageMonthly[0])
	}
}

func TestAdminAPIUsageLedgerHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageLedger: []maildb.APIUsageLedgerView{{
			EventID:       "usage-1",
			SchemaVersion: "2026-05-04.api-usage.v2",
			EventTime:     time.Date(2026, 5, 4, 1, 0, 0, 0, time.UTC),
			RecordedAt:    time.Date(2026, 5, 4, 1, 0, 1, 0, time.UTC),
			Method:        "GET",
			Route:         "GET /api/v1/messages",
			Status:        200,
			TenantID:      "tenant-1",
			PrincipalID:   "principal-1",
			AuthSource:    "bearer",
			RequestCount:  1,
			Payload:       json.RawMessage(`{"event":"api.usage"}`),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/ledger?limit=5&tenant_id=tenant-1&principal_id=principal-1&from=2026-05-04T00:00:00Z&to=2026-05-05T00:00:00Z", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		APIUsageLedger []maildb.APIUsageLedgerView `json:"api_usage_ledger"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.APIUsageLedger) != 1 || body.APIUsageLedger[0].EventID != "usage-1" {
		t.Fatalf("api_usage_ledger = %+v", body.APIUsageLedger)
	}
	if body.APIUsageLedger[0].TenantID != "tenant-1" || body.APIUsageLedger[0].PrincipalID != "principal-1" {
		t.Fatalf("api_usage_ledger identity = %+v", body.APIUsageLedger[0])
	}
	if service.lastLimit != 5 {
		t.Fatalf("lastLimit = %d, want 5", service.lastLimit)
	}
	if service.lastAPIUsageLedgerList.TenantID != "tenant-1" || service.lastAPIUsageLedgerList.PrincipalID != "principal-1" {
		t.Fatalf("lastAPIUsageLedgerList = %+v", service.lastAPIUsageLedgerList)
	}
	if service.lastAPIUsageLedgerList.From.IsZero() || service.lastAPIUsageLedgerList.To.IsZero() {
		t.Fatalf("lastAPIUsageLedgerList timestamps = %+v", service.lastAPIUsageLedgerList)
	}
}

func TestAdminAPIUsageLedgerRejectsInvalidTimeRange(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/ledger?from=2026-05-05T00:00:00Z&to=2026-05-04T00:00:00Z", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAdminAPIUsageLedgerExportHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageLedger: []maildb.APIUsageLedgerView{{
			EventID:       "usage-1",
			SchemaVersion: "2026-05-04.api-usage.v2",
			EventTime:     time.Date(2026, 5, 4, 1, 0, 0, 0, time.UTC),
			RecordedAt:    time.Date(2026, 5, 4, 1, 0, 1, 0, time.UTC),
			Method:        "GET",
			Route:         "GET /api/v1/messages",
			Status:        200,
			RequestCount:  1,
			Payload:       json.RawMessage(`{"event":"api.usage"}`),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/ledger/export?limit=5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/x-ndjson" {
		t.Fatalf("content type = %q", got)
	}
	lines := strings.Split(strings.TrimSpace(rr.Body.String()), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], `"event_id":"usage-1"`) {
		t.Fatalf("ndjson = %q", rr.Body.String())
	}
}

func TestAdminAPIUsageLedgerStatsHandler(t *testing.T) {
	t.Parallel()

	first := time.Date(2026, 5, 4, 1, 0, 0, 0, time.UTC)
	last := time.Date(2026, 5, 4, 2, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		apiUsageLedgerStats: maildb.APIUsageLedgerStatsView{
			EventCount:       2,
			RequestCount:     4,
			RequestBytes:     40,
			ResponseBytes:    400,
			LatencyMSTotal:   100,
			LatencyMSMax:     40,
			LatencyMSAverage: 25,
			FirstEventAt:     &first,
			LastEventAt:      &last,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/ledger/stats?tenant_id=tenant-1&from=2026-05-04T00:00:00Z&to=2026-05-05T00:00:00Z", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Stats maildb.APIUsageLedgerStatsView `json:"api_usage_ledger_stats"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Stats.EventCount != 2 || body.Stats.LatencyMSAverage != 25 {
		t.Fatalf("stats = %+v", body.Stats)
	}
	if service.lastAPIUsageLedgerList.TenantID != "tenant-1" || service.lastAPIUsageLedgerList.From.IsZero() {
		t.Fatalf("lastAPIUsageLedgerList = %+v", service.lastAPIUsageLedgerList)
	}
}

func TestAdminAPIUsageLedgerRetentionReadinessHandler(t *testing.T) {
	t.Parallel()

	cutoff := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		apiUsageLedgerRetentionReadiness: maildb.APIUsageLedgerRetentionReadinessView{
			Cutoff:                cutoff,
			TenantID:              "tenant-1",
			PrincipalID:           "principal-1",
			CandidateEventCount:   10,
			CandidateRequestCount: 10,
			CoveringExportBatchID: "api-usage-export-1",
			Ready:                 true,
			BlockingReasons:       []string{},
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/ledger/retention-readiness?cutoff=2026-05-05T00:00:00Z&tenant_id=tenant-1&principal_id=principal-1", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Readiness maildb.APIUsageLedgerRetentionReadinessView `json:"api_usage_ledger_retention_readiness"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Readiness.Ready || body.Readiness.CoveringExportBatchID != "api-usage-export-1" {
		t.Fatalf("readiness = %+v", body.Readiness)
	}
	if service.lastAPIUsageLedgerRetention.TenantID != "tenant-1" || service.lastAPIUsageLedgerRetention.PrincipalID != "principal-1" || service.lastAPIUsageLedgerRetention.Cutoff.IsZero() {
		t.Fatalf("lastAPIUsageLedgerRetention = %+v", service.lastAPIUsageLedgerRetention)
	}
}

func TestAdminAPIUsageLedgerRetentionReadinessRequiresCutoff(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/ledger/retention-readiness", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAdminCreateAPIUsageExportBatchHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportBatch: maildb.APIUsageExportBatchView{
			ID:           "api-usage-export-1",
			Status:       "completed",
			ExportFormat: "ndjson",
			TenantID:     "tenant-1",
			EventCount:   2,
			Manifest:     json.RawMessage(`{"version":"2026-05-04.api-usage-export.v1"}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/api-usage/export-batches?tenant_id=tenant-1&from=2026-05-04T00:00:00Z&to=2026-05-05T00:00:00Z", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Batch maildb.APIUsageExportBatchView `json:"api_usage_export_batch"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Batch.ID != "api-usage-export-1" || body.Batch.EventCount != 2 {
		t.Fatalf("batch = %+v", body.Batch)
	}
	if service.lastAPIUsageLedgerList.TenantID != "tenant-1" || service.lastAPIUsageLedgerList.From.IsZero() {
		t.Fatalf("lastAPIUsageLedgerList = %+v", service.lastAPIUsageLedgerList)
	}
}

func TestAdminListAPIUsageExportBatchesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportBatches: []maildb.APIUsageExportBatchView{{
			ID:           "api-usage-export-1",
			Status:       "completed",
			ExportFormat: "ndjson",
			EventCount:   2,
			Manifest:     json.RawMessage(`{}`),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches?limit=5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Batches []maildb.APIUsageExportBatchView `json:"api_usage_export_batches"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Batches) != 1 || body.Batches[0].ID != "api-usage-export-1" {
		t.Fatalf("batches = %+v", body.Batches)
	}
	if service.lastLimit != 5 {
		t.Fatalf("lastLimit = %d, want 5", service.lastLimit)
	}
}

func TestAdminGetAPIUsageExportBatchHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportBatch: maildb.APIUsageExportBatchView{
			ID:           "api-usage-export-1",
			Status:       "completed",
			ExportFormat: "ndjson",
			EventCount:   2,
			Manifest:     json.RawMessage(`{}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Batch maildb.APIUsageExportBatchView `json:"api_usage_export_batch"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Batch.ID != "api-usage-export-1" {
		t.Fatalf("batch = %+v", body.Batch)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" {
		t.Fatalf("lastAPIUsageExportBatchID = %q", service.lastAPIUsageExportBatchID)
	}
}

func TestAdminGetAPIUsageExportCapabilitiesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportCapabilities: maildb.APIUsageExportCapabilityView{
			ExportFormat:                  "ndjson",
			ArtifactContentType:           "application/x-ndjson",
			ManifestDigestAlgorithm:       "sha256",
			SignerBackend:                 "local-hmac",
			SignerConfigured:              true,
			SignerKeyID:                   "key-1",
			VerifierConfigured:            true,
			ProductionSignatureReady:      false,
			BillingReadySupported:         false,
			VerifiedBillingReadySupported: false,
			BlockingReasons:               []string{"production_manifest_signer_required"},
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-capabilities", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Capabilities maildb.APIUsageExportCapabilityView `json:"api_usage_export_capabilities"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Capabilities.SignerBackend != "local-hmac" || body.Capabilities.ProductionSignatureReady {
		t.Fatalf("capabilities = %+v", body.Capabilities)
	}
	if !service.lastAPIUsageExportCapabilities {
		t.Fatal("GetAPIUsageExportCapabilities was not called")
	}
}

func TestAdminGetAPIUsageExportHandoffReadinessHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportHandoff: maildb.APIUsageExportHandoffView{
			BatchID:                    "api-usage-export-1",
			BatchStatus:                "completed",
			BatchCompleted:             true,
			EventCount:                 2,
			ArtifactCount:              1,
			ArtifactEventCount:         2,
			ManifestDigestCount:        1,
			LatestManifestDigestID:     "api-usage-manifest-1",
			LatestDigestSignatureCount: 1,
			LatestSignatureID:          "api-usage-signature-1",
			LatestSignatureSigner:      "local-hmac",
			Ready:                      true,
			ReadinessGrade:             "operational",
			BillingReady:               false,
			BillingBlockingReasons:     []string{"production_manifest_signer_required"},
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/handoff-readiness", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Handoff maildb.APIUsageExportHandoffView `json:"api_usage_export_handoff_readiness"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Handoff.Ready || body.Handoff.BillingReady || body.Handoff.ReadinessGrade != "operational" {
		t.Fatalf("handoff = %+v", body.Handoff)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" {
		t.Fatalf("lastAPIUsageExportBatchID = %q", service.lastAPIUsageExportBatchID)
	}
	if service.lastAPIUsageExportHandoffDeep {
		t.Fatal("lastAPIUsageExportHandoffDeep = true, want false")
	}
}

func TestAdminGetAPIUsageExportHandoffReadinessHandlerDeep(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportHandoff: maildb.APIUsageExportHandoffView{
			BatchID:                    "api-usage-export-1",
			BatchStatus:                "completed",
			BatchCompleted:             true,
			EventCount:                 2,
			ArtifactCount:              1,
			ArtifactEventCount:         2,
			ManifestDigestCount:        1,
			LatestManifestDigestID:     "api-usage-manifest-1",
			LatestDigestSignatureCount: 1,
			LatestSignatureID:          "api-usage-signature-1",
			Ready:                      true,
			ReadinessGrade:             "billing_candidate",
			BillingReady:               true,
			DeepVerification:           true,
			DeepReady:                  true,
			ArtifactVerifications: []maildb.APIUsageExportArtifactVerificationView{{
				BatchID:    "api-usage-export-1",
				ArtifactID: "api-usage-artifact-1",
				Valid:      true,
			}},
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/handoff-readiness?deep=true", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Handoff maildb.APIUsageExportHandoffView `json:"api_usage_export_handoff_readiness"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Handoff.DeepVerification || !body.Handoff.DeepReady || len(body.Handoff.ArtifactVerifications) != 1 {
		t.Fatalf("handoff = %+v", body.Handoff)
	}
	if !service.lastAPIUsageExportHandoffDeep {
		t.Fatal("lastAPIUsageExportHandoffDeep = false, want true")
	}
}

func TestAdminGetAPIUsageExportHandoffReadinessHandlerRejectsBadDeepQuery(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/handoff-readiness?deep=sure", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if service.lastAPIUsageExportBatchID != "" {
		t.Fatalf("lastAPIUsageExportBatchID = %q", service.lastAPIUsageExportBatchID)
	}
}

func TestAdminExportAPIUsageExportBatchHandler(t *testing.T) {
	t.Parallel()

	windowStart := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		apiUsageExportBatch: maildb.APIUsageExportBatchView{
			ID:           "api-usage-export-1",
			Status:       "completed",
			ExportFormat: "ndjson",
			TenantID:     "tenant-1",
			PrincipalID:  "principal-1",
			WindowStart:  &windowStart,
			WindowEnd:    &windowEnd,
			EventCount:   1,
			Manifest:     json.RawMessage(`{}`),
		},
		apiUsageLedger: []maildb.APIUsageLedgerView{{
			EventID:       "usage-1",
			SchemaVersion: "2026-05-04.api-usage.v2",
			EventTime:     windowStart,
			RecordedAt:    windowStart,
			Method:        "GET",
			Route:         "GET /api/v1/messages",
			Status:        200,
			RequestCount:  1,
			Payload:       json.RawMessage(`{"event":"api.usage"}`),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/export?limit=5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/x-ndjson" {
		t.Fatalf("content type = %q", got)
	}
	if !strings.Contains(rr.Body.String(), `"event_id":"usage-1"`) {
		t.Fatalf("ndjson = %q", rr.Body.String())
	}
	if service.lastAPIUsageLedgerList.TenantID != "tenant-1" || service.lastAPIUsageLedgerList.PrincipalID != "principal-1" {
		t.Fatalf("lastAPIUsageLedgerList = %+v", service.lastAPIUsageLedgerList)
	}
	if !service.lastAPIUsageLedgerList.From.Equal(windowStart) || !service.lastAPIUsageLedgerList.To.Equal(windowEnd) {
		t.Fatalf("lastAPIUsageLedgerList timestamps = %+v", service.lastAPIUsageLedgerList)
	}
}

func TestAdminCreateAPIUsageExportArtifactHandler(t *testing.T) {
	t.Parallel()

	hash := strings.Repeat("a", 64)
	service := &fakeAdminService{
		apiUsageExportArtifact: maildb.APIUsageExportArtifactView{
			ID:             "api-usage-artifact-1",
			BatchID:        "api-usage-export-1",
			StorageBackend: "s3",
			ObjectKey:      "exports/api-usage-export-1.ndjson",
			ContentType:    "application/x-ndjson",
			ByteCount:      12,
			SHA256Hex:      hash,
			EventCount:     2,
			Metadata:       json.RawMessage(`{"bucket":"billing"}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	body := strings.NewReader(`{"storage_backend":"s3","object_key":"exports/api-usage-export-1.ndjson","byte_count":12,"sha256_hex":"` + hash + `","event_count":2,"metadata":{"bucket":"billing"}}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts", body)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Artifact maildb.APIUsageExportArtifactView `json:"api_usage_export_artifact"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Artifact.ID != "api-usage-artifact-1" || service.lastCreateAPIUsageExportArtifact.BatchID != "api-usage-export-1" {
		t.Fatalf("artifact = %+v last=%+v", response.Artifact, service.lastCreateAPIUsageExportArtifact)
	}
}

func TestAdminWriteAPIUsageExportArtifactHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportArtifact: maildb.APIUsageExportArtifactView{
			ID:             "api-usage-artifact-1",
			BatchID:        "api-usage-export-1",
			StorageBackend: "local",
			ObjectKey:      "exports/api-usage/api-usage-export-1.ndjson",
			ContentType:    "application/x-ndjson",
			ByteCount:      12,
			SHA256Hex:      strings.Repeat("a", 64),
			EventCount:     2,
			Metadata:       json.RawMessage(`{"writer":"gogomail-admin-api"}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	body := strings.NewReader(`{"object_key":"exports/api-usage/api-usage-export-1.ndjson","metadata":{"purpose":"billing"}}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts/write", body)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Artifact maildb.APIUsageExportArtifactView `json:"api_usage_export_artifact"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Artifact.ID != "api-usage-artifact-1" || service.lastAPIUsageExportBatchID != "api-usage-export-1" {
		t.Fatalf("artifact = %+v lastBatch=%q", response.Artifact, service.lastAPIUsageExportBatchID)
	}
	if service.lastWriteAPIUsageExportArtifact.ObjectKey != "exports/api-usage/api-usage-export-1.ndjson" {
		t.Fatalf("last write request = %+v", service.lastWriteAPIUsageExportArtifact)
	}
}

func TestAdminWriteAPIUsageExportArtifactHandlerAcceptsEmptyBody(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportArtifact: maildb.APIUsageExportArtifactView{
			ID:          "api-usage-artifact-1",
			BatchID:     "api-usage-export-1",
			ContentType: "application/x-ndjson",
			SHA256Hex:   strings.Repeat("a", 64),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts/write", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" {
		t.Fatalf("last batch = %q", service.lastAPIUsageExportBatchID)
	}
	if service.lastWriteAPIUsageExportArtifact.ObjectKey != "" || len(service.lastWriteAPIUsageExportArtifact.Metadata) != 0 {
		t.Fatalf("last write request = %+v", service.lastWriteAPIUsageExportArtifact)
	}
}

func TestAdminListAPIUsageExportArtifactsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportArtifacts: []maildb.APIUsageExportArtifactView{{
			ID:             "api-usage-artifact-1",
			BatchID:        "api-usage-export-1",
			StorageBackend: "s3",
			ObjectKey:      "exports/api-usage-export-1.ndjson",
			ContentType:    "application/x-ndjson",
			SHA256Hex:      strings.Repeat("a", 64),
			Metadata:       json.RawMessage(`{}`),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts?limit=5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Artifacts []maildb.APIUsageExportArtifactView `json:"api_usage_export_artifacts"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Artifacts) != 1 || response.Artifacts[0].ID != "api-usage-artifact-1" {
		t.Fatalf("artifacts = %+v", response.Artifacts)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastLimit != 5 {
		t.Fatalf("last batch/limit = %q/%d", service.lastAPIUsageExportBatchID, service.lastLimit)
	}
}

func TestAdminGetAPIUsageExportArtifactHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportArtifact: maildb.APIUsageExportArtifactView{
			ID:             "api-usage-artifact-1",
			BatchID:        "api-usage-export-1",
			StorageBackend: "s3",
			ObjectKey:      "exports/api-usage-export-1.ndjson",
			ContentType:    "application/x-ndjson",
			SHA256Hex:      strings.Repeat("a", 64),
			Metadata:       json.RawMessage(`{}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts/api-usage-artifact-1", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Artifact maildb.APIUsageExportArtifactView `json:"api_usage_export_artifact"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Artifact.ID != "api-usage-artifact-1" {
		t.Fatalf("artifact = %+v", response.Artifact)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastAPIUsageExportArtifactID != "api-usage-artifact-1" {
		t.Fatalf("last ids = %q/%q", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportArtifactID)
	}
}

func TestAdminDownloadAPIUsageExportArtifactHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportArtifact: maildb.APIUsageExportArtifactView{
			ID:          "api-usage-artifact-1",
			BatchID:     "api-usage-export-1",
			ContentType: "application/x-ndjson",
			SHA256Hex:   strings.Repeat("a", 64),
		},
		apiUsageExportArtifactBody: " {\"event_id\":\"usage-1\"}\n",
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts/api-usage-artifact-1/download", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/x-ndjson" {
		t.Fatalf("content type = %q", got)
	}
	if got := rr.Header().Get("X-Gogomail-Artifact-SHA256"); got != strings.Repeat("a", 64) {
		t.Fatalf("sha header = %q", got)
	}
	if !strings.Contains(rr.Body.String(), `"event_id":"usage-1"`) {
		t.Fatalf("body = %q", rr.Body.String())
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastAPIUsageExportArtifactID != "api-usage-artifact-1" {
		t.Fatalf("last ids = %q/%q", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportArtifactID)
	}
}

func TestAdminVerifyAPIUsageExportArtifactHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportArtifactVerification: maildb.APIUsageExportArtifactVerificationView{
			BatchID:           "api-usage-export-1",
			ArtifactID:        "api-usage-artifact-1",
			ObjectKey:         "exports/api-usage/api-usage-export-1.ndjson",
			ExpectedByteCount: 12,
			ActualByteCount:   12,
			ExpectedSHA256Hex: strings.Repeat("a", 64),
			ActualSHA256Hex:   strings.Repeat("a", 64),
			Valid:             true,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/artifacts/api-usage-artifact-1/verification", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Verification maildb.APIUsageExportArtifactVerificationView `json:"api_usage_export_artifact_verification"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Verification.Valid {
		t.Fatalf("verification = %+v", response.Verification)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastAPIUsageExportArtifactID != "api-usage-artifact-1" {
		t.Fatalf("last ids = %q/%q", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportArtifactID)
	}
}

func TestAdminCreateAPIUsageExportManifestDigestHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportManifestDigest: maildb.APIUsageExportManifestDigestView{
			ID:              "api-usage-manifest-1",
			BatchID:         "api-usage-export-1",
			SchemaVersion:   "2026-05-04.api-usage-export-manifest.v1",
			DigestAlgorithm: "sha256",
			DigestHex:       strings.Repeat("a", 64),
			Manifest:        json.RawMessage(`{"schema_version":"2026-05-04.api-usage-export-manifest.v1"}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Digest maildb.APIUsageExportManifestDigestView `json:"api_usage_export_manifest_digest"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Digest.ID != "api-usage-manifest-1" || service.lastAPIUsageExportBatchID != "api-usage-export-1" {
		t.Fatalf("digest = %+v lastBatch=%q", response.Digest, service.lastAPIUsageExportBatchID)
	}
}

func TestAdminListAPIUsageExportManifestDigestsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportManifestDigests: []maildb.APIUsageExportManifestDigestView{{
			ID:              "api-usage-manifest-1",
			BatchID:         "api-usage-export-1",
			SchemaVersion:   "2026-05-04.api-usage-export-manifest.v1",
			DigestAlgorithm: "sha256",
			DigestHex:       strings.Repeat("a", 64),
			Manifest:        json.RawMessage(`{}`),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests?limit=5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Digests []maildb.APIUsageExportManifestDigestView `json:"api_usage_export_manifest_digests"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Digests) != 1 || response.Digests[0].ID != "api-usage-manifest-1" {
		t.Fatalf("digests = %+v", response.Digests)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastLimit != 5 {
		t.Fatalf("last batch/limit = %q/%d", service.lastAPIUsageExportBatchID, service.lastLimit)
	}
}

func TestAdminGetAPIUsageExportManifestDigestHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportManifestDigest: maildb.APIUsageExportManifestDigestView{
			ID:              "api-usage-manifest-1",
			BatchID:         "api-usage-export-1",
			SchemaVersion:   "2026-05-04.api-usage-export-manifest.v1",
			DigestAlgorithm: "sha256",
			DigestHex:       strings.Repeat("a", 64),
			Manifest:        json.RawMessage(`{}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Digest maildb.APIUsageExportManifestDigestView `json:"api_usage_export_manifest_digest"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Digest.ID != "api-usage-manifest-1" {
		t.Fatalf("digest = %+v", response.Digest)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastAPIUsageExportManifestDigestID != "api-usage-manifest-1" {
		t.Fatalf("last ids = %q/%q", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportManifestDigestID)
	}
}

func TestAdminVerifyAPIUsageExportManifestDigestHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportManifestDigestVerification: maildb.APIUsageExportManifestDigestVerificationView{
			BatchID:           "api-usage-export-1",
			DigestID:          "api-usage-manifest-1",
			SchemaVersion:     "2026-05-04.api-usage-export-manifest.v1",
			DigestAlgorithm:   "sha256",
			ExpectedDigestHex: strings.Repeat("a", 64),
			ActualDigestHex:   strings.Repeat("a", 64),
			Valid:             true,
			CanonicalManifest: json.RawMessage(`{}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1/verification", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Verification maildb.APIUsageExportManifestDigestVerificationView `json:"api_usage_export_manifest_digest_verification"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Verification.Valid {
		t.Fatalf("verification = %+v", response.Verification)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastAPIUsageExportManifestDigestID != "api-usage-manifest-1" {
		t.Fatalf("last ids = %q/%q", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportManifestDigestID)
	}
}

func TestAdminCreateAPIUsageExportManifestSignatureHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportManifestSignature: maildb.APIUsageExportManifestSignatureView{
			ID:                 "api-usage-signature-1",
			DigestID:           "api-usage-manifest-1",
			BatchID:            "api-usage-export-1",
			SignerBackend:      "local-hmac",
			KeyID:              "key-1",
			SignatureAlgorithm: "hmac-sha256",
			SignedDigestHex:    strings.Repeat("a", 64),
			SignatureHex:       strings.Repeat("b", 64),
			Metadata:           json.RawMessage(`{}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1/signatures", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Signature maildb.APIUsageExportManifestSignatureView `json:"api_usage_export_manifest_signature"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Signature.ID != "api-usage-signature-1" {
		t.Fatalf("signature = %+v", response.Signature)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastAPIUsageExportManifestDigestID != "api-usage-manifest-1" {
		t.Fatalf("last ids = %q/%q", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportManifestDigestID)
	}
}

func TestAdminListAPIUsageExportManifestSignaturesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportManifestSignatures: []maildb.APIUsageExportManifestSignatureView{{
			ID:                 "api-usage-signature-1",
			DigestID:           "api-usage-manifest-1",
			BatchID:            "api-usage-export-1",
			SignerBackend:      "local-hmac",
			KeyID:              "key-1",
			SignatureAlgorithm: "hmac-sha256",
			SignedDigestHex:    strings.Repeat("a", 64),
			SignatureHex:       strings.Repeat("b", 64),
			Metadata:           json.RawMessage(`{}`),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1/signatures?limit=5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Signatures []maildb.APIUsageExportManifestSignatureView `json:"api_usage_export_manifest_signatures"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Signatures) != 1 || response.Signatures[0].ID != "api-usage-signature-1" {
		t.Fatalf("signatures = %+v", response.Signatures)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastAPIUsageExportManifestDigestID != "api-usage-manifest-1" || service.lastLimit != 5 {
		t.Fatalf("last = %q/%q/%d", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportManifestDigestID, service.lastLimit)
	}
}

func TestAdminGetAPIUsageExportManifestSignatureHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportManifestSignature: maildb.APIUsageExportManifestSignatureView{
			ID:                 "api-usage-signature-1",
			DigestID:           "api-usage-manifest-1",
			BatchID:            "api-usage-export-1",
			SignerBackend:      "local-hmac",
			KeyID:              "key-1",
			SignatureAlgorithm: "hmac-sha256",
			SignedDigestHex:    strings.Repeat("a", 64),
			SignatureHex:       strings.Repeat("b", 64),
			Metadata:           json.RawMessage(`{}`),
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1/signatures/api-usage-signature-1", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Signature maildb.APIUsageExportManifestSignatureView `json:"api_usage_export_manifest_signature"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Signature.ID != "api-usage-signature-1" {
		t.Fatalf("signature = %+v", response.Signature)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastAPIUsageExportManifestDigestID != "api-usage-manifest-1" || service.lastAPIUsageExportManifestSignatureID != "api-usage-signature-1" {
		t.Fatalf("last ids = %q/%q/%q", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportManifestDigestID, service.lastAPIUsageExportManifestSignatureID)
	}
}

func TestAdminVerifyAPIUsageExportManifestSignatureHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		apiUsageExportManifestSignatureVerification: maildb.APIUsageExportManifestSignatureVerificationView{
			BatchID:            "api-usage-export-1",
			DigestID:           "api-usage-manifest-1",
			SignatureID:        "api-usage-signature-1",
			SignerBackend:      "local-hmac",
			KeyID:              "key-1",
			SignatureAlgorithm: "hmac-sha256",
			SignedDigestHex:    strings.Repeat("a", 64),
			ExpectedDigestHex:  strings.Repeat("a", 64),
			Valid:              true,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/export-batches/api-usage-export-1/manifest-digests/api-usage-manifest-1/signatures/api-usage-signature-1/verification", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Verification maildb.APIUsageExportManifestSignatureVerificationView `json:"api_usage_export_manifest_signature_verification"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Verification.Valid {
		t.Fatalf("verification = %+v", response.Verification)
	}
	if service.lastAPIUsageExportBatchID != "api-usage-export-1" || service.lastAPIUsageExportManifestDigestID != "api-usage-manifest-1" || service.lastAPIUsageExportManifestSignatureID != "api-usage-signature-1" {
		t.Fatalf("last ids = %q/%q/%q", service.lastAPIUsageExportBatchID, service.lastAPIUsageExportManifestDigestID, service.lastAPIUsageExportManifestSignatureID)
	}
}

func TestAdminQuotaReconciliationHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		quotaReconciliation: []maildb.QuotaReconciliationView{{
			Scope:      "user",
			ID:         "user-1",
			DomainID:   "domain-1",
			Name:       "admin@example.com",
			LedgerUsed: 1200,
			ActualUsed: 1000,
			Delta:      200,
			InSync:     false,
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/quota-reconciliation?limit=7", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		QuotaReconciliation []maildb.QuotaReconciliationView `json:"quota_reconciliation"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.QuotaReconciliation) != 1 || body.QuotaReconciliation[0].Delta != 200 {
		t.Fatalf("quota_reconciliation = %+v", body.QuotaReconciliation)
	}
	if service.lastLimit != 7 {
		t.Fatalf("lastLimit = %d, want 7", service.lastLimit)
	}
}

func TestAdminQuotaCorrectionHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		quotaCorrection: maildb.QuotaCorrectionResult{
			DryRun: true,
			Corrected: []maildb.QuotaReconciliationView{{
				Scope:      "domain",
				ID:         "domain-1",
				LedgerUsed: 10,
				ActualUsed: 20,
				Delta:      -10,
			}},
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/quota-reconciliation/corrections", strings.NewReader(`{"scope":"domain","id":"domain-1","dry_run":true}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastQuotaCorrection.Scope != "domain" || service.lastQuotaCorrection.ID != "domain-1" || !service.lastQuotaCorrection.DryRun {
		t.Fatalf("lastQuotaCorrection = %+v", service.lastQuotaCorrection)
	}
	var body struct {
		QuotaCorrection maildb.QuotaCorrectionResult `json:"quota_correction"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.QuotaCorrection.Corrected) != 1 || body.QuotaCorrection.Corrected[0].Delta != -10 {
		t.Fatalf("quota_correction = %+v", body.QuotaCorrection)
	}
}

func TestAdminCompaniesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		companies: []maildb.CompanyView{{ID: "company-1", Name: "Gogo Co", Status: "active", QuotaLimit: 10_000}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/companies?limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Companies []maildb.CompanyView `json:"companies"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Companies) != 1 || body.Companies[0].Name != "Gogo Co" {
		t.Fatalf("companies = %+v", body.Companies)
	}
	if service.lastLimit != 10 {
		t.Fatalf("lastLimit = %d, want 10", service.lastLimit)
	}
}

func TestAdminGetCompanyHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		companies: []maildb.CompanyView{{ID: "company-1", Name: "Gogo Co", Status: "active"}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/companies/company-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Company maildb.CompanyView `json:"company"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Company.ID != "company-1" || service.lastCompanyID != "company-1" {
		t.Fatalf("company = %+v lastCompanyID=%q", body.Company, service.lastCompanyID)
	}
}

func TestAdminUpdateCompanyQuotaHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/companies/company-1/quota", bytes.NewReader([]byte(`{"quota_limit":8192}`)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCompanyQuota.ID != "company-1" || service.lastCompanyQuota.QuotaLimit != 8192 {
		t.Fatalf("lastCompanyQuota = %+v", service.lastCompanyQuota)
	}
}

func TestAdminAuthAcceptsBearerToken(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		queueStats: []maildb.QueueStat{{Topic: "mail.outbound.general", Status: "pending", Count: 1}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "secret")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/queue", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminAuthRejectsWrongLengthToken(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "secret")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/queue", nil)
	req.Header.Set("Authorization", "Bearer secrets")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminDomainsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		domains: []maildb.DomainView{{ID: "domain-1", Name: "example.com", Status: "active"}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains?limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Domains []maildb.DomainView `json:"domains"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Domains) != 1 || body.Domains[0].Name != "example.com" {
		t.Fatalf("domains = %+v", body.Domains)
	}
	if service.lastLimit != 10 {
		t.Fatalf("lastLimit = %d", service.lastLimit)
	}
}

func TestAdminListHandlerRejectsInvalidLimit(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains?limit=bad", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminListHandlerRejectsTooLargeLimit(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains?limit=201", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "limit must be at most 200") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminGetDomainHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		domains: []maildb.DomainView{{ID: "domain-1", Name: "example.com", Status: "active"}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains/domain-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Domain maildb.DomainView `json:"domain"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Domain.ID != "domain-1" || service.lastDomainID != "domain-1" {
		t.Fatalf("domain = %+v lastDomainID=%q", body.Domain, service.lastDomainID)
	}
}

func TestAdminDomainDNSCheckHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		dnsReport: dnscheck.DomainReport{
			Domain: "example.com",
			MX:     dnscheck.RecordCheck{Name: "mx", Host: "example.com", Status: dnscheck.StatusOK, Found: []string{"mx.example.com"}},
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains/domain-1/dns-check", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		DNSCheck dnscheck.DomainReport `json:"dns_check"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.DNSCheck.Domain != "example.com" || service.lastDomainID != "domain-1" {
		t.Fatalf("dns_check = %+v lastDomainID=%q", body.DNSCheck, service.lastDomainID)
	}
}

func TestAdminDomainDNSCheckHistoryHandler(t *testing.T) {
	t.Parallel()

	checkedAt := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	service := &fakeAdminService{
		dnsChecks: []maildb.DomainDNSCheckView{{
			ID:        "check-1",
			DomainID:  "domain-1",
			Status:    "ok",
			CheckedAt: checkedAt,
			Report: dnscheck.DomainReport{
				Domain: "example.com",
				MX:     dnscheck.RecordCheck{Name: "mx", Host: "example.com", Status: dnscheck.StatusOK},
			},
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains/domain-1/dns-checks?limit=5", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		DNSChecks []maildb.DomainDNSCheckView `json:"dns_checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.DNSChecks) != 1 || body.DNSChecks[0].ID != "check-1" {
		t.Fatalf("dns_checks = %+v", body.DNSChecks)
	}
	if service.lastDomainID != "domain-1" || service.lastLimit != 5 {
		t.Fatalf("lastDomainID=%q lastLimit=%d", service.lastDomainID, service.lastLimit)
	}
}

func TestAdminCreateDomainHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	body := []byte(`{"company_id":"company-1","name":"Example.COM","quota_limit":1024}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/domains", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCreateDomain.CompanyID != "company-1" || service.lastCreateDomain.Name != "Example.COM" {
		t.Fatalf("lastCreateDomain = %+v", service.lastCreateDomain)
	}
}

func TestAdminUpdateDomainStatusHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/domains/domain-1/status", bytes.NewReader([]byte(`{"status":"suspended"}`)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDomainStatus.ID != "domain-1" || service.lastDomainStatus.Status != "suspended" {
		t.Fatalf("lastDomainStatus = %+v", service.lastDomainStatus)
	}
}

func TestAdminUpdateDomainQuotaHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/domains/domain-1/quota", bytes.NewReader([]byte(`{"quota_limit":2048}`)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDomainQuota.ID != "domain-1" || service.lastDomainQuota.QuotaLimit != 2048 {
		t.Fatalf("lastDomainQuota = %+v", service.lastDomainQuota)
	}
}

func TestAdminUpdateDomainPolicyHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/domains/domain-1/policy", bytes.NewReader([]byte(`{
		"inbound_mode": "monitor",
		"outbound_mode": "enforce",
		"max_recipients_per_message": 50,
		"max_message_bytes": 1048576,
		"max_attachment_bytes": 524288
	}`)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDomainPolicy.ID != "domain-1" || service.lastDomainPolicy.InboundMode != "monitor" {
		t.Fatalf("lastDomainPolicy = %+v", service.lastDomainPolicy)
	}
	if service.lastDomainPolicy.MaxAttachmentBytes != 524288 {
		t.Fatalf("MaxAttachmentBytes = %d, want 524288", service.lastDomainPolicy.MaxAttachmentBytes)
	}
	if !strings.Contains(rec.Body.String(), `"domain_policy"`) {
		t.Fatalf("response missing domain_policy envelope: %s", rec.Body.String())
	}
}

func TestAdminUsersHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		users: []maildb.UserView{{ID: "user-1", DomainID: "domain-1", Username: "admin", Status: "active"}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/users?domain_id=domain-1&limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Users []maildb.UserView `json:"users"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Users) != 1 || body.Users[0].Username != "admin" {
		t.Fatalf("users = %+v", body.Users)
	}
	if service.lastDomainID != "domain-1" || service.lastLimit != 10 {
		t.Fatalf("domain/limit = %q/%d", service.lastDomainID, service.lastLimit)
	}
}

func TestAdminGetUserHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		users: []maildb.UserView{{ID: "user-1", DomainID: "domain-1", Username: "admin", Status: "active"}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/users/user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		User maildb.UserView `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.User.ID != "user-1" || service.lastUserID != "user-1" {
		t.Fatalf("user = %+v lastUserID=%q", body.User, service.lastUserID)
	}
}

func TestAdminCreateUserHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	body := []byte(`{"domain_id":"domain-1","username":"admin","display_name":"Admin","address":"admin@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/users", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCreateUser.Username != "admin" || service.lastCreateUser.Address != "admin@example.com" {
		t.Fatalf("lastCreateUser = %+v", service.lastCreateUser)
	}
}

func TestAdminUpdateUserStatusHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/users/user-1/status", bytes.NewReader([]byte(`{"status":"disabled"}`)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserStatus.ID != "user-1" || service.lastUserStatus.Status != "disabled" {
		t.Fatalf("lastUserStatus = %+v", service.lastUserStatus)
	}
}

func TestAdminUpdateUserQuotaHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/users/user-1/quota", bytes.NewReader([]byte(`{"quota_limit":4096}`)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserQuota.ID != "user-1" || service.lastUserQuota.QuotaLimit != 4096 {
		t.Fatalf("lastUserQuota = %+v", service.lastUserQuota)
	}
}

func TestAdminDeliveryAttemptsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		attempts: []maildb.DeliveryAttemptView{{
			ID:          "attempt-1",
			MessageID:   "msg-1",
			Recipient:   "user@example.net",
			Status:      "bounced",
			AttemptedAt: time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-attempts?limit=10&status=bounced&recipient_domain=example.net&since=2026-05-04T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeliveryAttemptList.Limit != 10 || service.lastDeliveryAttemptList.Status != "bounced" || service.lastDeliveryAttemptList.RecipientDomain != "example.net" || service.lastDeliveryAttemptList.Since.IsZero() {
		t.Fatalf("lastDeliveryAttemptList = %+v", service.lastDeliveryAttemptList)
	}
}

func TestAdminDeliveryAttemptsHandlerRejectsInvalidSince(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-attempts?since=not-a-time", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "since must be RFC3339 timestamp") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminDeliveryAttemptsHandlerRejectsInvalidStatus(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-attempts?status=retrying", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "unsupported delivery attempt status") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminDeliveryAttemptStatsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		deliveryAttemptStats: maildb.DeliveryAttemptStatsView{
			TotalAttempts:    4,
			UniqueMessages:   2,
			UniqueRecipients: 3,
			Delivered:        1,
			Failed:           1,
			Bounced:          1,
			Exhausted:        1,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-attempts/stats?status=failed&recipient_domain=example.net&since=2026-05-04T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Stats maildb.DeliveryAttemptStatsView `json:"delivery_attempt_stats"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Stats.TotalAttempts != 4 || body.Stats.UniqueRecipients != 3 || body.Stats.Exhausted != 1 {
		t.Fatalf("delivery_attempt_stats = %+v", body.Stats)
	}
	if service.lastDeliveryAttemptStats.Status != "failed" || service.lastDeliveryAttemptStats.RecipientDomain != "example.net" || service.lastDeliveryAttemptStats.Since.IsZero() {
		t.Fatalf("lastDeliveryAttemptStats = %+v", service.lastDeliveryAttemptStats)
	}
}

func TestAdminDeliveryAttemptStatsHandlerRejectsInvalidSince(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-attempts/stats?since=not-a-time", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "since must be RFC3339 timestamp") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminDeliveryAttemptStatsHandlerRejectsInvalidStatus(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-attempts/stats?status=retrying", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "unsupported delivery attempt status") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminExhaustedAttemptsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		attempts: []maildb.DeliveryAttemptView{{
			ID:              "attempt-1",
			MessageID:       "msg-1",
			Recipient:       "user@example.net",
			RecipientDomain: "example.net",
			Status:          "exhausted",
			AttemptedAt:     time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-attempts/exhausted?limit=10&recipient_domain=example.net&since=2026-05-04T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastExhaustedAttemptList.Limit != 10 || service.lastExhaustedAttemptList.RecipientDomain != "example.net" || service.lastExhaustedAttemptList.Since.IsZero() {
		t.Fatalf("lastExhaustedAttemptList = %+v", service.lastExhaustedAttemptList)
	}
}

func TestAdminExhaustedAttemptsHandlerRejectsInvalidSince(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-attempts/exhausted?since=not-a-time", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "since must be RFC3339 timestamp") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminPushNotificationAttemptsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		pushNotificationAttempts: []maildb.PushNotificationAttemptView{{
			ID:          "push-attempt-1",
			MessageID:   "msg-1",
			UserID:      "user-1",
			DeviceID:    "device-1",
			Platform:    "fcm",
			TokenSuffix: "token-1",
			Status:      "candidate",
			AttemptedAt: time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/push-notification-attempts?limit=10&status=candidate&user_id=user-1&since=2026-05-04T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastPushAttemptList.Limit != 10 || service.lastPushAttemptList.Status != "candidate" || service.lastPushAttemptList.UserID != "user-1" || service.lastPushAttemptList.Since.IsZero() {
		t.Fatalf("lastPushAttemptList = %+v", service.lastPushAttemptList)
	}
	if !strings.Contains(rec.Body.String(), "push_notification_attempts") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminPushNotificationAttemptsHandlerRejectsInvalidSince(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/push-notification-attempts?since=not-a-time", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "since must be RFC3339 timestamp") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminPushNotificationStatsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		pushNotificationStats: maildb.PushNotificationStatsView{
			ActiveDevices: 3,
			TotalAttempts: 9,
			Candidate:     4,
			Delivered:     2,
			Failed:        1,
			InvalidToken:  2,
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/push-notification-stats?user_id=user-1&since=2026-05-04T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastPushNotificationStats.UserID != "user-1" || service.lastPushNotificationStats.Since.IsZero() {
		t.Fatalf("lastPushNotificationStats = %+v", service.lastPushNotificationStats)
	}
	if !strings.Contains(rec.Body.String(), "push_notification_stats") || !strings.Contains(rec.Body.String(), `"active_devices":3`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminPushNotificationStatsHandlerRejectsInvalidSince(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, &fakeAdminService{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/push-notification-stats?since=not-a-time", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "since must be RFC3339 timestamp") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestAdminSuppressionListHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		suppression: []maildb.SuppressionEntry{{
			ID:        "suppression-1",
			Email:     "user@example.net",
			Reason:    "hard_bounce",
			CreatedAt: time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/suppression-list?limit=5", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastLimit != 5 {
		t.Fatalf("lastLimit = %d, want 5", service.lastLimit)
	}
}

func TestAdminDKIMKeysHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		dkimKeys: []maildb.DKIMKeyView{{
			ID:       "dkim-1",
			DomainID: "domain-1",
			Selector: "s1",
			Status:   "active",
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/dkim-keys?domain_id=domain-1&limit=5", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDomainID != "domain-1" {
		t.Fatalf("lastDomainID = %q, want domain-1", service.lastDomainID)
	}
	if service.lastLimit != 5 {
		t.Fatalf("lastLimit = %d, want 5", service.lastLimit)
	}
}

func TestAdminCreateDKIMKeyHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{createdDKIMKeyID: "dkim-1"}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	body := []byte(`{"domain_id":"domain-1","selector":"s1","private_key_pem":"private","public_key_dns":"v=DKIM1; p=public"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/dkim-keys", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCreateDKIMKey.Selector != "s1" {
		t.Fatalf("lastCreateDKIMKey = %+v", service.lastCreateDKIMKey)
	}
}

func TestAdminDeactivateDKIMKeyHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/dkim-keys/dkim-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeactivateDKIMKeyID != "dkim-1" {
		t.Fatalf("lastDeactivateDKIMKeyID = %q", service.lastDeactivateDKIMKeyID)
	}
}

func TestAdminRetryOutboxHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/outbox/outbox-1/retry", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastRetryOutboxID != "outbox-1" {
		t.Fatalf("lastRetryOutboxID = %q", service.lastRetryOutboxID)
	}
}

func TestAdminDeleteSuppressionHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/suppression-list/suppression-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeleteSuppressionID != "suppression-1" {
		t.Fatalf("lastDeleteSuppressionID = %q", service.lastDeleteSuppressionID)
	}
}

func TestAdminTrustedRelaysHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		trustedRelays: []maildb.TrustedRelayView{{
			ID:          "relay-1",
			CIDR:        "192.0.2.0/24",
			Description: "spam relay",
			CreatedAt:   time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/trusted-relays?limit=5", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		TrustedRelays []maildb.TrustedRelayView `json:"trusted_relays"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.TrustedRelays) != 1 || body.TrustedRelays[0].CIDR != "192.0.2.0/24" {
		t.Fatalf("trusted_relays = %+v", body.TrustedRelays)
	}
	if service.lastLimit != 5 {
		t.Fatalf("lastLimit = %d, want 5", service.lastLimit)
	}
}

func TestAdminCreateTrustedRelayHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/trusted-relays", bytes.NewReader([]byte(`{
		"cidr": "192.0.2.1",
		"description": "edge relay"
	}`)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCreateTrustedRelay.CIDR != "192.0.2.1" || service.lastCreateTrustedRelay.Description != "edge relay" {
		t.Fatalf("lastCreateTrustedRelay = %+v", service.lastCreateTrustedRelay)
	}
}

func TestAdminDeliveryRoutesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		deliveryRoutes: []maildb.DeliveryRouteView{{
			ID:            "route-1",
			DomainPattern: "*.example.net",
			Hosts:         []string{"relay.example.net"},
			Port:          587,
			TLSMode:       "require",
			Status:        "active",
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-routes?limit=5", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		DeliveryRoutes []maildb.DeliveryRouteView `json:"delivery_routes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.DeliveryRoutes) != 1 || body.DeliveryRoutes[0].DomainPattern != "*.example.net" {
		t.Fatalf("delivery_routes = %+v", body.DeliveryRoutes)
	}
	if service.lastLimit != 5 {
		t.Fatalf("lastLimit = %d, want 5", service.lastLimit)
	}
}

func TestAdminCreateDeliveryRouteHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/delivery-routes", bytes.NewReader([]byte(`{
		"domain_pattern": "*.example.net",
		"farm": "transactional",
		"hosts": ["relay.example.net"],
		"port": 587,
		"tls_mode": "require",
		"auth_username": "relay-user",
		"auth_password": "secret"
	}`)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCreateDeliveryRoute.DomainPattern != "*.example.net" || service.lastCreateDeliveryRoute.AuthPassword != "secret" {
		t.Fatalf("lastCreateDeliveryRoute = %+v", service.lastCreateDeliveryRoute)
	}
	if !strings.Contains(rec.Body.String(), `"delivery_route"`) {
		t.Fatalf("response missing delivery_route envelope: %s", rec.Body.String())
	}
}

func TestAdminResolveDeliveryRouteHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		deliveryRouteResolution: maildb.DeliveryRouteResolveView{
			Domain:  "mail.example.net",
			Matched: true,
			Route:   &maildb.DeliveryRouteView{ID: "route-1", DomainPattern: "*.example.net"},
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-routes/resolve?domain=mail.example.net", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastResolveDeliveryRouteDomain != "mail.example.net" {
		t.Fatalf("lastResolveDeliveryRouteDomain = %q", service.lastResolveDeliveryRouteDomain)
	}
	if !strings.Contains(rec.Body.String(), `"delivery_route_resolution"`) {
		t.Fatalf("response missing delivery_route_resolution envelope: %s", rec.Body.String())
	}
}

func TestAdminUpdateDeliveryRouteStatusHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/delivery-routes/route-1/status", bytes.NewReader([]byte(`{"status":"disabled"}`)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeliveryRouteStatus.ID != "route-1" || service.lastDeliveryRouteStatus.Status != "disabled" {
		t.Fatalf("lastDeliveryRouteStatus = %+v", service.lastDeliveryRouteStatus)
	}
}

func TestAdminDeleteDeliveryRouteHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/delivery-routes/route-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeleteDeliveryRouteID != "route-1" {
		t.Fatalf("lastDeleteDeliveryRouteID = %q", service.lastDeleteDeliveryRouteID)
	}
}

func TestAdminDeleteTrustedRelayHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/trusted-relays/relay-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeleteTrustedRelayID != "relay-1" {
		t.Fatalf("lastDeleteTrustedRelayID = %q", service.lastDeleteTrustedRelayID)
	}
}

func TestAdminRoutesRequireTokenWhenConfigured(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "secret")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/queue", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/v1/queue", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

type fakeAdminService struct {
	companies                                   []maildb.CompanyView
	domains                                     []maildb.DomainView
	dnsReport                                   dnscheck.DomainReport
	dnsChecks                                   []maildb.DomainDNSCheckView
	users                                       []maildb.UserView
	queueStats                                  []maildb.QueueStat
	outboxEvents                                []maildb.OutboxEventView
	outboxEvent                                 maildb.OutboxEventView
	quotaUsage                                  []maildb.QuotaUsageView
	apiUsageDaily                               []maildb.APIUsageDailyView
	apiUsageMonthly                             []maildb.APIUsageMonthlyView
	apiUsageLedger                              []maildb.APIUsageLedgerView
	apiUsageLedgerStats                         maildb.APIUsageLedgerStatsView
	apiUsageLedgerRetentionReadiness            maildb.APIUsageLedgerRetentionReadinessView
	apiUsageExportCapabilities                  maildb.APIUsageExportCapabilityView
	apiUsageExportBatch                         maildb.APIUsageExportBatchView
	apiUsageExportBatches                       []maildb.APIUsageExportBatchView
	apiUsageExportHandoff                       maildb.APIUsageExportHandoffView
	apiUsageExportArtifact                      maildb.APIUsageExportArtifactView
	apiUsageExportArtifacts                     []maildb.APIUsageExportArtifactView
	apiUsageExportArtifactBody                  string
	apiUsageExportArtifactVerification          maildb.APIUsageExportArtifactVerificationView
	apiUsageExportManifestDigest                maildb.APIUsageExportManifestDigestView
	apiUsageExportManifestDigests               []maildb.APIUsageExportManifestDigestView
	apiUsageExportManifestDigestVerification    maildb.APIUsageExportManifestDigestVerificationView
	apiUsageExportManifestSignature             maildb.APIUsageExportManifestSignatureView
	apiUsageExportManifestSignatures            []maildb.APIUsageExportManifestSignatureView
	apiUsageExportManifestSignatureVerification maildb.APIUsageExportManifestSignatureVerificationView
	quotaReconciliation                         []maildb.QuotaReconciliationView
	quotaCorrection                             maildb.QuotaCorrectionResult
	attempts                                    []maildb.DeliveryAttemptView
	deliveryAttemptStats                        maildb.DeliveryAttemptStatsView
	lastDeliveryAttemptList                     maildb.DeliveryAttemptListRequest
	lastDeliveryAttemptStats                    maildb.DeliveryAttemptStatsRequest
	lastExhaustedAttemptList                    maildb.ExhaustedAttemptListRequest
	pushNotificationAttempts                    []maildb.PushNotificationAttemptView
	pushNotificationStats                       maildb.PushNotificationStatsView
	suppression                                 []maildb.SuppressionEntry
	trustedRelays                               []maildb.TrustedRelayView
	deliveryRoutes                              []maildb.DeliveryRouteView
	deliveryRouteResolution                     maildb.DeliveryRouteResolveView
	dkimKeys                                    []maildb.DKIMKeyView
	backpressureState                           backpressure.State
	createdDKIMKeyID                            string
	lastLimit                                   int
	lastOutboxEventList                         maildb.OutboxEventListRequest
	lastOutboxEventID                           string
	lastCompanyID                               string
	lastDomainID                                string
	lastUserID                                  string
	lastDomainStatus                            maildb.UpdateDomainStatusRequest
	lastCompanyQuota                            maildb.UpdateCompanyQuotaRequest
	lastDomainQuota                             maildb.UpdateDomainQuotaRequest
	lastDomainPolicy                            maildb.UpdateDomainPolicyRequest
	lastCreateDomain                            maildb.CreateDomainRequest
	lastUserStatus                              maildb.UpdateUserStatusRequest
	lastUserQuota                               maildb.UpdateUserQuotaRequest
	lastQuotaCorrection                         maildb.CorrectQuotaReconciliationRequest
	lastAPIUsageLedgerList                      maildb.APIUsageLedgerListRequest
	lastAPIUsageLedgerRetention                 maildb.APIUsageLedgerRetentionRequest
	lastAPIUsageExportCapabilities              bool
	lastAPIUsageExportBatchID                   string
	lastAPIUsageExportHandoffDeep               bool
	lastAPIUsageExportArtifactID                string
	lastAPIUsageExportManifestDigestID          string
	lastAPIUsageExportManifestSignatureID       string
	lastCreateAPIUsageExportArtifact            maildb.CreateAPIUsageExportArtifactRequest
	lastWriteAPIUsageExportArtifact             maildb.WriteAPIUsageExportArtifactRequest
	lastPushAttemptList                         maildb.PushNotificationAttemptListRequest
	lastPushNotificationStats                   maildb.PushNotificationStatsRequest
	lastCreateUser                              maildb.CreateUserRequest
	lastCreateDKIMKey                           maildb.CreateDKIMKeyInput
	lastCreateTrustedRelay                      maildb.CreateTrustedRelayRequest
	lastCreateDeliveryRoute                     maildb.CreateDeliveryRouteRequest
	lastResolveDeliveryRouteDomain              string
	lastDeliveryRouteStatus                     maildb.UpdateDeliveryRouteStatusRequest
	lastBackpressureUpdate                      backpressure.StateUpdate
	lastDeactivateDKIMKeyID                     string
	lastRetryOutboxID                           string
	lastDeleteSuppressionID                     string
	lastDeleteTrustedRelayID                    string
	lastDeleteDeliveryRouteID                   string
}

func (f *fakeAdminService) ListCompanies(_ context.Context, limit int) ([]maildb.CompanyView, error) {
	f.lastLimit = limit
	return f.companies, nil
}

func (f *fakeAdminService) GetCompany(_ context.Context, id string) (maildb.CompanyView, error) {
	f.lastCompanyID = id
	for _, company := range f.companies {
		if company.ID == id {
			return company, nil
		}
	}
	return maildb.CompanyView{}, nil
}

func (f *fakeAdminService) UpdateCompanyQuota(_ context.Context, req maildb.UpdateCompanyQuotaRequest) error {
	f.lastCompanyQuota = req
	return nil
}

func (f *fakeAdminService) ListDomains(_ context.Context, limit int) ([]maildb.DomainView, error) {
	f.lastLimit = limit
	return f.domains, nil
}

func (f *fakeAdminService) CreateDomain(_ context.Context, req maildb.CreateDomainRequest) (maildb.DomainView, error) {
	f.lastCreateDomain = req
	return maildb.DomainView{ID: "domain-new", CompanyID: req.CompanyID, Name: req.Name, NameACE: req.NameACE, Status: "active"}, nil
}

func (f *fakeAdminService) GetDomain(_ context.Context, id string) (maildb.DomainView, error) {
	f.lastDomainID = id
	for _, domain := range f.domains {
		if domain.ID == id {
			return domain, nil
		}
	}
	return maildb.DomainView{}, nil
}

func (f *fakeAdminService) GetDomainStats(_ context.Context, id string) (maildb.DomainStatsView, error) {
	f.lastDomainID = id
	return maildb.DomainStatsView{DomainID: id}, nil
}

func (f *fakeAdminService) VerifyDomainDNS(_ context.Context, id string) (dnscheck.DomainReport, error) {
	f.lastDomainID = id
	return f.dnsReport, nil
}

func (f *fakeAdminService) ListDomainDNSChecks(_ context.Context, id string, limit int) ([]maildb.DomainDNSCheckView, error) {
	f.lastDomainID = id
	f.lastLimit = limit
	return f.dnsChecks, nil
}

func (f *fakeAdminService) UpdateDomainStatus(_ context.Context, req maildb.UpdateDomainStatusRequest) error {
	f.lastDomainStatus = req
	return nil
}

func (f *fakeAdminService) UpdateDomainQuota(_ context.Context, req maildb.UpdateDomainQuotaRequest) error {
	f.lastDomainQuota = req
	return nil
}

func (f *fakeAdminService) UpdateDomainPolicy(_ context.Context, req maildb.UpdateDomainPolicyRequest) (maildb.DomainPolicyView, error) {
	f.lastDomainPolicy = req
	return maildb.DomainPolicyView{
		DomainID:                req.ID,
		InboundMode:             req.InboundMode,
		OutboundMode:            req.OutboundMode,
		MaxRecipientsPerMessage: req.MaxRecipientsPerMessage,
		MaxMessageBytes:         req.MaxMessageBytes,
		MaxAttachmentBytes:      req.MaxAttachmentBytes,
	}, nil
}

func (f *fakeAdminService) ListUsers(_ context.Context, domainID string, limit int) ([]maildb.UserView, error) {
	f.lastDomainID = domainID
	f.lastLimit = limit
	return f.users, nil
}

func (f *fakeAdminService) CreateUser(_ context.Context, req maildb.CreateUserRequest) (maildb.UserView, error) {
	f.lastCreateUser = req
	return maildb.UserView{ID: "user-new", DomainID: req.DomainID, Username: req.Username, DisplayName: req.DisplayName, Status: "active"}, nil
}

func (f *fakeAdminService) GetUser(_ context.Context, id string) (maildb.UserView, error) {
	f.lastUserID = id
	for _, user := range f.users {
		if user.ID == id {
			return user, nil
		}
	}
	return maildb.UserView{}, nil
}

func (f *fakeAdminService) UpdateUserStatus(_ context.Context, req maildb.UpdateUserStatusRequest) error {
	f.lastUserStatus = req
	return nil
}

func (f *fakeAdminService) UpdateUserQuota(_ context.Context, req maildb.UpdateUserQuotaRequest) error {
	f.lastUserQuota = req
	return nil
}

func (f *fakeAdminService) ListQueueStats(context.Context) ([]maildb.QueueStat, error) {
	return f.queueStats, nil
}

func (f *fakeAdminService) ListOutboxEvents(_ context.Context, req maildb.OutboxEventListRequest) ([]maildb.OutboxEventView, error) {
	f.lastOutboxEventList = req
	if req.Status != "" && req.Status != "pending" && req.Status != "processing" && req.Status != "done" && req.Status != "failed" {
		return nil, fmt.Errorf("unsupported outbox status")
	}
	return f.outboxEvents, nil
}

func (f *fakeAdminService) GetOutboxEvent(_ context.Context, id string) (maildb.OutboxEventView, error) {
	f.lastOutboxEventID = id
	if f.outboxEvent.ID == "" {
		return maildb.OutboxEventView{}, fmt.Errorf("outbox event %q not found", id)
	}
	return f.outboxEvent, nil
}

func (f *fakeAdminService) GetBackpressure(context.Context) (backpressure.State, error) {
	if f.backpressureState.Level == "" {
		return backpressure.State{Level: "normal"}, nil
	}
	return f.backpressureState, nil
}

func (f *fakeAdminService) UpdateBackpressure(_ context.Context, req backpressure.StateUpdate) (backpressure.State, error) {
	f.lastBackpressureUpdate = req
	return backpressure.State{Level: req.Level, Reason: req.Reason}, nil
}

func (f *fakeAdminService) ListQuotaUsage(_ context.Context, limit int) ([]maildb.QuotaUsageView, error) {
	f.lastLimit = limit
	return f.quotaUsage, nil
}

func (f *fakeAdminService) ListAPIUsageDaily(_ context.Context, limit int) ([]maildb.APIUsageDailyView, error) {
	f.lastLimit = limit
	return f.apiUsageDaily, nil
}

func (f *fakeAdminService) ListAPIUsageMonthly(_ context.Context, limit int) ([]maildb.APIUsageMonthlyView, error) {
	f.lastLimit = limit
	return f.apiUsageMonthly, nil
}

func (f *fakeAdminService) ListAPIUsageLedger(_ context.Context, req maildb.APIUsageLedgerListRequest) ([]maildb.APIUsageLedgerView, error) {
	f.lastLimit = req.Limit
	f.lastAPIUsageLedgerList = req
	return f.apiUsageLedger, nil
}

func (f *fakeAdminService) GetAPIUsageLedgerStats(_ context.Context, req maildb.APIUsageLedgerListRequest) (maildb.APIUsageLedgerStatsView, error) {
	f.lastAPIUsageLedgerList = req
	return f.apiUsageLedgerStats, nil
}

func (f *fakeAdminService) GetAPIUsageLedgerRetentionReadiness(_ context.Context, req maildb.APIUsageLedgerRetentionRequest) (maildb.APIUsageLedgerRetentionReadinessView, error) {
	f.lastAPIUsageLedgerRetention = req
	return f.apiUsageLedgerRetentionReadiness, nil
}

func (f *fakeAdminService) GetAPIUsageExportCapabilities(context.Context) (maildb.APIUsageExportCapabilityView, error) {
	f.lastAPIUsageExportCapabilities = true
	return f.apiUsageExportCapabilities, nil
}

func (f *fakeAdminService) CreateAPIUsageExportBatch(_ context.Context, req maildb.APIUsageLedgerListRequest) (maildb.APIUsageExportBatchView, error) {
	f.lastAPIUsageLedgerList = req
	return f.apiUsageExportBatch, nil
}

func (f *fakeAdminService) ListAPIUsageExportBatches(_ context.Context, limit int) ([]maildb.APIUsageExportBatchView, error) {
	f.lastLimit = limit
	return f.apiUsageExportBatches, nil
}

func (f *fakeAdminService) GetAPIUsageExportBatch(_ context.Context, id string) (maildb.APIUsageExportBatchView, error) {
	f.lastAPIUsageExportBatchID = id
	return f.apiUsageExportBatch, nil
}

func (f *fakeAdminService) GetAPIUsageExportHandoff(_ context.Context, batchID string, deep bool) (maildb.APIUsageExportHandoffView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportHandoffDeep = deep
	return f.apiUsageExportHandoff, nil
}

func (f *fakeAdminService) CreateAPIUsageExportArtifact(_ context.Context, req maildb.CreateAPIUsageExportArtifactRequest) (maildb.APIUsageExportArtifactView, error) {
	f.lastCreateAPIUsageExportArtifact = req
	return f.apiUsageExportArtifact, nil
}

func (f *fakeAdminService) WriteAPIUsageExportArtifact(_ context.Context, batchID string, req maildb.WriteAPIUsageExportArtifactRequest) (maildb.APIUsageExportArtifactView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastWriteAPIUsageExportArtifact = req
	return f.apiUsageExportArtifact, nil
}

func (f *fakeAdminService) ListAPIUsageExportArtifacts(_ context.Context, batchID string, limit int) ([]maildb.APIUsageExportArtifactView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastLimit = limit
	return f.apiUsageExportArtifacts, nil
}

func (f *fakeAdminService) GetAPIUsageExportArtifact(_ context.Context, batchID string, artifactID string) (maildb.APIUsageExportArtifactView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportArtifactID = artifactID
	return f.apiUsageExportArtifact, nil
}

func (f *fakeAdminService) OpenAPIUsageExportArtifact(_ context.Context, batchID string, artifactID string) (maildb.APIUsageExportArtifactView, io.ReadCloser, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportArtifactID = artifactID
	return f.apiUsageExportArtifact, io.NopCloser(strings.NewReader(f.apiUsageExportArtifactBody)), nil
}

func (f *fakeAdminService) VerifyAPIUsageExportArtifact(_ context.Context, batchID string, artifactID string) (maildb.APIUsageExportArtifactVerificationView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportArtifactID = artifactID
	return f.apiUsageExportArtifactVerification, nil
}

func (f *fakeAdminService) CreateAPIUsageExportManifestDigest(_ context.Context, batchID string) (maildb.APIUsageExportManifestDigestView, error) {
	f.lastAPIUsageExportBatchID = batchID
	return f.apiUsageExportManifestDigest, nil
}

func (f *fakeAdminService) ListAPIUsageExportManifestDigests(_ context.Context, batchID string, limit int) ([]maildb.APIUsageExportManifestDigestView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastLimit = limit
	return f.apiUsageExportManifestDigests, nil
}

func (f *fakeAdminService) GetAPIUsageExportManifestDigest(_ context.Context, batchID string, digestID string) (maildb.APIUsageExportManifestDigestView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportManifestDigestID = digestID
	return f.apiUsageExportManifestDigest, nil
}

func (f *fakeAdminService) VerifyAPIUsageExportManifestDigest(_ context.Context, batchID string, digestID string) (maildb.APIUsageExportManifestDigestVerificationView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportManifestDigestID = digestID
	return f.apiUsageExportManifestDigestVerification, nil
}

func (f *fakeAdminService) CreateAPIUsageExportManifestSignature(_ context.Context, batchID string, digestID string) (maildb.APIUsageExportManifestSignatureView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportManifestDigestID = digestID
	return f.apiUsageExportManifestSignature, nil
}

func (f *fakeAdminService) ListAPIUsageExportManifestSignatures(_ context.Context, batchID string, digestID string, limit int) ([]maildb.APIUsageExportManifestSignatureView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportManifestDigestID = digestID
	f.lastLimit = limit
	return f.apiUsageExportManifestSignatures, nil
}

func (f *fakeAdminService) GetAPIUsageExportManifestSignature(_ context.Context, batchID string, digestID string, signatureID string) (maildb.APIUsageExportManifestSignatureView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportManifestDigestID = digestID
	f.lastAPIUsageExportManifestSignatureID = signatureID
	return f.apiUsageExportManifestSignature, nil
}

func (f *fakeAdminService) VerifyAPIUsageExportManifestSignature(_ context.Context, batchID string, digestID string, signatureID string) (maildb.APIUsageExportManifestSignatureVerificationView, error) {
	f.lastAPIUsageExportBatchID = batchID
	f.lastAPIUsageExportManifestDigestID = digestID
	f.lastAPIUsageExportManifestSignatureID = signatureID
	return f.apiUsageExportManifestSignatureVerification, nil
}

func (f *fakeAdminService) ListQuotaReconciliation(_ context.Context, limit int) ([]maildb.QuotaReconciliationView, error) {
	f.lastLimit = limit
	return f.quotaReconciliation, nil
}

func (f *fakeAdminService) CorrectQuotaReconciliation(_ context.Context, req maildb.CorrectQuotaReconciliationRequest) (maildb.QuotaCorrectionResult, error) {
	f.lastQuotaCorrection = req
	return f.quotaCorrection, nil
}

func (f *fakeAdminService) ListDeliveryAttempts(_ context.Context, req maildb.DeliveryAttemptListRequest) ([]maildb.DeliveryAttemptView, error) {
	f.lastLimit = req.Limit
	f.lastDeliveryAttemptList = req
	if req.Status != "" && req.Status != "delivered" && req.Status != "failed" && req.Status != "bounced" && req.Status != "exhausted" {
		return nil, fmt.Errorf("unsupported delivery attempt status")
	}
	return f.attempts, nil
}

func (f *fakeAdminService) GetDeliveryAttemptStats(_ context.Context, req maildb.DeliveryAttemptStatsRequest) (maildb.DeliveryAttemptStatsView, error) {
	f.lastDeliveryAttemptStats = req
	if req.Status != "" && req.Status != "delivered" && req.Status != "failed" && req.Status != "bounced" && req.Status != "exhausted" {
		return maildb.DeliveryAttemptStatsView{}, fmt.Errorf("unsupported delivery attempt status")
	}
	return f.deliveryAttemptStats, nil
}

func (f *fakeAdminService) ListExhaustedAttempts(_ context.Context, req maildb.ExhaustedAttemptListRequest) ([]maildb.DeliveryAttemptView, error) {
	f.lastLimit = req.Limit
	f.lastExhaustedAttemptList = req
	return f.attempts, nil
}

func (f *fakeAdminService) ListPushNotificationAttempts(_ context.Context, req maildb.PushNotificationAttemptListRequest) ([]maildb.PushNotificationAttemptView, error) {
	f.lastPushAttemptList = req
	return f.pushNotificationAttempts, nil
}

func (f *fakeAdminService) GetPushNotificationStats(_ context.Context, req maildb.PushNotificationStatsRequest) (maildb.PushNotificationStatsView, error) {
	f.lastPushNotificationStats = req
	return f.pushNotificationStats, nil
}

func (f *fakeAdminService) ListSuppressionEntries(_ context.Context, limit int) ([]maildb.SuppressionEntry, error) {
	f.lastLimit = limit
	return f.suppression, nil
}

func (f *fakeAdminService) ListTrustedRelays(_ context.Context, limit int) ([]maildb.TrustedRelayView, error) {
	f.lastLimit = limit
	return f.trustedRelays, nil
}

func (f *fakeAdminService) CreateTrustedRelay(_ context.Context, req maildb.CreateTrustedRelayRequest) (maildb.TrustedRelayView, error) {
	f.lastCreateTrustedRelay = req
	return maildb.TrustedRelayView{ID: "relay-new", CIDR: req.CIDR, Description: req.Description}, nil
}

func (f *fakeAdminService) DeleteTrustedRelay(_ context.Context, id string) error {
	f.lastDeleteTrustedRelayID = id
	return nil
}

func (f *fakeAdminService) ListDeliveryRoutes(_ context.Context, limit int) ([]maildb.DeliveryRouteView, error) {
	f.lastLimit = limit
	return f.deliveryRoutes, nil
}

func (f *fakeAdminService) CreateDeliveryRoute(_ context.Context, req maildb.CreateDeliveryRouteRequest) (maildb.DeliveryRouteView, error) {
	f.lastCreateDeliveryRoute = req
	return maildb.DeliveryRouteView{
		ID:            "route-new",
		DomainPattern: req.DomainPattern,
		Farm:          req.Farm,
		Hosts:         req.Hosts,
		Port:          req.Port,
		TLSMode:       req.TLSMode,
		Status:        "active",
	}, nil
}

func (f *fakeAdminService) ResolveDeliveryRoute(_ context.Context, domain string) (maildb.DeliveryRouteResolveView, error) {
	f.lastResolveDeliveryRouteDomain = domain
	return f.deliveryRouteResolution, nil
}

func (f *fakeAdminService) UpdateDeliveryRouteStatus(_ context.Context, req maildb.UpdateDeliveryRouteStatusRequest) error {
	f.lastDeliveryRouteStatus = req
	return nil
}

func (f *fakeAdminService) DeleteDeliveryRoute(_ context.Context, id string) error {
	f.lastDeleteDeliveryRouteID = id
	return nil
}

func (f *fakeAdminService) ListDKIMKeys(_ context.Context, domainID string, limit int) ([]maildb.DKIMKeyView, error) {
	f.lastDomainID = domainID
	f.lastLimit = limit
	return f.dkimKeys, nil
}

func (f *fakeAdminService) CreateDKIMKey(_ context.Context, input maildb.CreateDKIMKeyInput) (string, error) {
	f.lastCreateDKIMKey = input
	if f.createdDKIMKeyID != "" {
		return f.createdDKIMKeyID, nil
	}
	return "dkim-1", nil
}

func (f *fakeAdminService) DeactivateDKIMKey(_ context.Context, id string) error {
	f.lastDeactivateDKIMKeyID = id
	return nil
}

func (f *fakeAdminService) VerifyDKIMKeyDNS(_ context.Context, keyID string) (maildb.DKIMKeyDNSVerificationResult, error) {
	return maildb.DKIMKeyDNSVerificationResult{KeyID: keyID, Selector: "default"}, nil
}

func (f *fakeAdminService) RetryOutbox(_ context.Context, id string) error {
	f.lastRetryOutboxID = id
	return nil
}

func (f *fakeAdminService) DeleteSuppressionEntry(_ context.Context, id string) error {
	f.lastDeleteSuppressionID = id
	return nil
}
