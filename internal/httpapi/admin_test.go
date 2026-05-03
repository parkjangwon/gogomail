package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/maildb"
)

func TestAdminQueueHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		queueStats: []maildb.QueueStat{{Topic: "mail.outbound.general", Status: "pending", Count: 2}},
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
	if len(body.Queues) != 1 || body.Queues[0].Count != 2 {
		t.Fatalf("queues = %+v", body.Queues)
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

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-attempts?limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastLimit != 10 {
		t.Fatalf("lastLimit = %d, want 10", service.lastLimit)
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
	queueStats              []maildb.QueueStat
	attempts                []maildb.DeliveryAttemptView
	suppression             []maildb.SuppressionEntry
	lastLimit               int
	lastRetryOutboxID       string
	lastDeleteSuppressionID string
}

func (f *fakeAdminService) ListQueueStats(context.Context) ([]maildb.QueueStat, error) {
	return f.queueStats, nil
}

func (f *fakeAdminService) ListDeliveryAttempts(_ context.Context, limit int) ([]maildb.DeliveryAttemptView, error) {
	f.lastLimit = limit
	return f.attempts, nil
}

func (f *fakeAdminService) ListSuppressionEntries(_ context.Context, limit int) ([]maildb.SuppressionEntry, error) {
	f.lastLimit = limit
	return f.suppression, nil
}

func (f *fakeAdminService) RetryOutbox(_ context.Context, id string) error {
	f.lastRetryOutboxID = id
	return nil
}

func (f *fakeAdminService) DeleteSuppressionEntry(_ context.Context, id string) error {
	f.lastDeleteSuppressionID = id
	return nil
}
