package apimeter

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/eventstream"
)

func TestDecodeUsageEventNormalizesDailyBucket(t *testing.T) {
	t.Parallel()

	payload := json.RawMessage(`{
		"event":" api.usage ",
		"schema_version":" 2026-05-04.api-usage.v1 ",
		"event_id":" usage-1 ",
		"method":" GET ",
		"route":" GET /api/v1/messages ",
		"status":200,
		"request_bytes":12,
		"response_bytes":34,
		"latency_ms":25,
		"timestamp":"2026-05-04T15:30:00+09:00",
		"tenant_id":" tenant-1 ",
		"company_id":" company-1 ",
		"domain_id":" domain-1 ",
		"user_id":" user-1 ",
		"api_key_id":" api-key-1 ",
		"principal_id":" principal-1 ",
		"auth_source":"bearer"
	}`)
	event, err := DecodeUsageEvent(payload)
	if err != nil {
		t.Fatalf("DecodeUsageEvent returned error: %v", err)
	}

	wantDay := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	if !event.Day.Equal(wantDay) {
		t.Fatalf("Day = %s, want %s", event.Day, wantDay)
	}
	wantMonth := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if !event.Month.Equal(wantMonth) {
		t.Fatalf("Month = %s, want %s", event.Month, wantMonth)
	}
	if event.Route != "GET /api/v1/messages" || event.UserID != "user-1" {
		t.Fatalf("event = %+v", event)
	}
	if event.EventID != "usage-1" {
		t.Fatalf("EventID = %q, want usage-1", event.EventID)
	}
	if event.SchemaVersion != APIUsageSchemaV1 {
		t.Fatalf("SchemaVersion = %q, want %s", event.SchemaVersion, APIUsageSchemaV1)
	}
	if len(event.RawPayload) == 0 {
		t.Fatal("RawPayload is empty")
	}
	if event.AuthSource != "bearer" {
		t.Fatalf("AuthSource = %q, want bearer", event.AuthSource)
	}
	if event.TenantID != "tenant-1" || event.CompanyID != "company-1" || event.DomainID != "domain-1" {
		t.Fatalf("identity dimensions = %+v", event)
	}
	if event.APIKeyID != "api-key-1" || event.PrincipalID != "principal-1" {
		t.Fatalf("identity principals = %+v", event)
	}
}

func TestDecodeUsageEventAcceptsV2Schema(t *testing.T) {
	t.Parallel()

	payload := json.RawMessage(`{
		"event":"api.usage",
		"schema_version":"2026-05-04.api-usage.v2",
		"method":"GET",
		"route":"GET /api/v1/messages",
		"status":200,
		"timestamp":"2026-05-04T00:00:00Z",
		"tenant_id":"tenant-1",
		"principal_id":"principal-1",
		"auth_source":"bearer"
	}`)
	event, err := DecodeUsageEvent(payload)
	if err != nil {
		t.Fatalf("DecodeUsageEvent returned error: %v", err)
	}
	if event.TenantID != "tenant-1" || event.PrincipalID != "principal-1" || event.AuthSource != "bearer" {
		t.Fatalf("event = %+v", event)
	}
}

func TestDecodeUsageEventDefaultsMissingAuthSource(t *testing.T) {
	t.Parallel()

	payload := json.RawMessage(`{
		"event":"api.usage",
		"method":"GET",
		"route":"GET /api/v1/messages",
		"status":200,
		"timestamp":"2026-05-04T00:00:00Z"
	}`)
	event, err := DecodeUsageEvent(payload)
	if err != nil {
		t.Fatalf("DecodeUsageEvent returned error: %v", err)
	}
	if event.AuthSource != "unknown" {
		t.Fatalf("AuthSource = %q, want unknown", event.AuthSource)
	}
}

func TestDecodeUsageEventClampsNegativeMetrics(t *testing.T) {
	t.Parallel()

	payload := json.RawMessage(`{
		"event":"api.usage",
		"method":"GET",
		"route":"GET /api/v1/messages",
		"status":200,
		"request_bytes":-12,
		"response_bytes":-34,
		"latency_ms":-25,
		"timestamp":"2026-05-04T00:00:00Z"
	}`)
	event, err := DecodeUsageEvent(payload)
	if err != nil {
		t.Fatalf("DecodeUsageEvent returned error: %v", err)
	}
	if event.RequestBytes != 0 || event.ResponseBytes != 0 || event.LatencyMS != 0 {
		t.Fatalf("metrics = request:%d response:%d latency:%d", event.RequestBytes, event.ResponseBytes, event.LatencyMS)
	}
}

func TestDecodeUsageEventKeepsV1Compatibility(t *testing.T) {
	t.Parallel()

	payload := json.RawMessage(`{
		"event":"api.usage",
		"schema_version":"2026-05-04.api-usage.v1",
		"event_id":"usage-v1",
		"method":"GET",
		"route":"GET /api/v1/messages",
		"status":200,
		"timestamp":"2026-05-04T00:00:00Z",
		"user_id":"user-1"
	}`)
	event, err := DecodeUsageEvent(payload)
	if err != nil {
		t.Fatalf("DecodeUsageEvent returned error: %v", err)
	}
	if event.EventID != "usage-v1" || event.UserID != "user-1" {
		t.Fatalf("event = %+v", event)
	}
	if event.TenantID != "" || event.PrincipalID != "" {
		t.Fatalf("v1 event should not invent tenant/principal dimensions: %+v", event)
	}
	if event.AuthSource != AuthSourceUnknown {
		t.Fatalf("AuthSource = %q, want unknown", event.AuthSource)
	}
}

func TestDecodeUsageEventRejectsUnsupportedSchemaVersion(t *testing.T) {
	t.Parallel()

	payload := json.RawMessage(`{
		"event":"api.usage",
		"schema_version":"2099-01-01.api-usage.v9",
		"method":"GET",
		"route":"GET /api/v1/messages",
		"status":200,
		"timestamp":"2026-05-04T00:00:00Z"
	}`)
	if _, err := DecodeUsageEvent(payload); err == nil {
		t.Fatal("DecodeUsageEvent accepted unsupported schema version")
	}
}

func TestDecodeUsageEventRejectsMissingRouteKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload json.RawMessage
	}{
		{
			name: "method",
			payload: json.RawMessage(`{
				"event":"api.usage",
				"route":"GET /api/v1/messages",
				"status":200,
				"timestamp":"2026-05-04T00:00:00Z"
			}`),
		},
		{
			name: "route",
			payload: json.RawMessage(`{
				"event":"api.usage",
				"method":"GET",
				"status":200,
				"timestamp":"2026-05-04T00:00:00Z"
			}`),
		},
		{
			name: "status",
			payload: json.RawMessage(`{
				"event":"api.usage",
				"method":"GET",
				"route":"GET /api/v1/messages",
				"status":99,
				"timestamp":"2026-05-04T00:00:00Z"
			}`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := DecodeUsageEvent(tc.payload); err == nil {
				t.Fatal("DecodeUsageEvent accepted invalid route key")
			}
		})
	}
}

func TestDecodeUsageEventRejectsInvalidRouteKeyOrIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload json.RawMessage
	}{
		{
			name: "method line break",
			payload: json.RawMessage(`{
				"event":"api.usage",
				"method":"GET\nPOST",
				"route":"GET /api/v1/messages",
				"status":200,
				"timestamp":"2026-05-04T00:00:00Z"
			}`),
		},
		{
			name: "route line break",
			payload: json.RawMessage(`{
				"event":"api.usage",
				"method":"GET",
				"route":"GET /api/v1/messages\r\nPOST /api/v1/send",
				"status":200,
				"timestamp":"2026-05-04T00:00:00Z"
			}`),
		},
		{
			name: "identity line break",
			payload: json.RawMessage(`{
				"event":"api.usage",
				"method":"GET",
				"route":"GET /api/v1/messages",
				"status":200,
				"timestamp":"2026-05-04T00:00:00Z",
				"tenant_id":"tenant-1\nother"
			}`),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := DecodeUsageEvent(tc.payload); err == nil {
				t.Fatal("DecodeUsageEvent accepted invalid route key or identity")
			}
		})
	}
}

func TestUsageHandlerAggregatesAPIUsageEvent(t *testing.T) {
	t.Parallel()

	store := &fakeUsageAggregateStore{}
	handler := NewUsageHandler(store)
	payload := json.RawMessage(`{
		"event":"api.usage",
		"method":"POST",
		"route":"POST /api/v1/messages/send",
		"status":202,
		"request_bytes":100,
		"response_bytes":50,
		"latency_ms":75,
		"timestamp":"2026-05-04T00:00:00Z",
		"user_id":"user-1",
		"auth_source":"query_user_id"
	}`)

	err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: payload})
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if store.event.Route != "POST /api/v1/messages/send" || store.event.RequestCount != 1 {
		t.Fatalf("stored event = %+v", store.event)
	}
	if store.event.AuthSource != "query_user_id" {
		t.Fatalf("stored auth source = %q, want query_user_id", store.event.AuthSource)
	}
}

func TestPostgresAggregateStoreUpsertsDailyUsage(t *testing.T) {
	t.Parallel()

	db := &fakeUsageSQL{}
	store := NewPostgresAggregateStore(db)
	err := store.AddUsage(context.Background(), UsageEvent{
		Day:           time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Method:        "GET",
		Route:         "GET /api/v1/messages",
		Status:        200,
		UserID:        "user-1",
		RequestBytes:  12,
		ResponseBytes: 34,
		LatencyMS:     25,
		RequestCount:  1,
	})
	if err != nil {
		t.Fatalf("AddUsage returned error: %v", err)
	}
	if len(db.queries) == 0 || !strings.Contains(db.queries[0], "INSERT INTO api_usage_daily") {
		t.Fatalf("queries = %+v, want api_usage_daily upsert", db.queries)
	}
	if !strings.Contains(db.queries[0], "tenant_id, company_id, domain_id, user_id, api_key_id, principal_id, auth_source") {
		t.Fatalf("query = %s", db.queries[0])
	}
	if db.argSets[0][1] != "GET" || db.argSets[0][2] != "GET /api/v1/messages" {
		t.Fatalf("args = %+v", db.argSets[0])
	}
}

func TestPostgresAggregateStoreUpsertsIdentityDimensions(t *testing.T) {
	t.Parallel()

	db := &fakeUsageSQL{}
	store := NewPostgresAggregateStore(db)
	err := store.AddUsage(context.Background(), UsageEvent{
		Day:          time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Method:       "GET",
		Route:        "GET /api/v1/messages",
		Status:       200,
		TenantID:     " tenant-1 ",
		CompanyID:    " company-1 ",
		DomainID:     " domain-1 ",
		UserID:       " user-1 ",
		APIKeyID:     " api-key-1 ",
		PrincipalID:  " principal-1 ",
		AuthSource:   " BEARER ",
		RequestCount: 1,
	})
	if err != nil {
		t.Fatalf("AddUsage returned error: %v", err)
	}
	args := db.argSets[0]
	if args[4] != "tenant-1" || args[5] != "company-1" || args[6] != "domain-1" {
		t.Fatalf("aggregate dimension args = %+v", args)
	}
	if args[7] != "user-1" || args[8] != "api-key-1" || args[9] != "principal-1" || args[10] != "bearer" {
		t.Fatalf("aggregate principal args = %+v", args)
	}
	if !strings.Contains(db.queries[0], "ON CONFLICT (day, method, route, status, tenant_id, company_id, domain_id, user_id, api_key_id, principal_id, auth_source)") {
		t.Fatalf("query = %s", db.queries[0])
	}
}

func TestPostgresAggregateStoreRejectsInvalidUsageDimensions(t *testing.T) {
	t.Parallel()

	valid := UsageEvent{
		Day:          time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Method:       "GET",
		Route:        "GET /api/v1/messages",
		Status:       200,
		TenantID:     "tenant-1",
		UserID:       "user-1",
		RequestCount: 1,
	}
	tests := []struct {
		name  string
		patch func(*UsageEvent)
	}{
		{
			name: "missing method",
			patch: func(event *UsageEvent) {
				event.Method = " "
			},
		},
		{
			name: "method line break",
			patch: func(event *UsageEvent) {
				event.Method = "GET\r\nX-Bad: 1"
			},
		},
		{
			name: "route line break",
			patch: func(event *UsageEvent) {
				event.Route = "GET /api/v1/messages\nX-Bad: 1"
			},
		},
		{
			name: "invalid status",
			patch: func(event *UsageEvent) {
				event.Status = 99
			},
		},
		{
			name: "event id line break",
			patch: func(event *UsageEvent) {
				event.EventID = "usage-1\r\nX-Bad: 1"
			},
		},
		{
			name: "identity line break",
			patch: func(event *UsageEvent) {
				event.TenantID = "tenant-1\nX-Bad: 1"
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			event := valid
			tc.patch(&event)
			db := &fakeUsageSQL{}
			err := NewPostgresAggregateStore(db).AddUsage(context.Background(), event)
			if err == nil {
				t.Fatal("AddUsage accepted invalid usage dimensions")
			}
			if len(db.queries) != 0 {
				t.Fatalf("queries = %+v, want no writes", db.queries)
			}
		})
	}
}

func TestPostgresAggregateStoreUpsertsMonthlyUsage(t *testing.T) {
	t.Parallel()

	db := &fakeUsageSQL{}
	store := NewPostgresAggregateStore(db)
	err := store.AddUsage(context.Background(), UsageEvent{
		Day:           time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Month:         time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Method:        "GET",
		Route:         "GET /api/v1/messages",
		Status:        200,
		UserID:        "user-1",
		RequestBytes:  12,
		ResponseBytes: 34,
		LatencyMS:     25,
		RequestCount:  1,
	})
	if err != nil {
		t.Fatalf("AddUsage returned error: %v", err)
	}
	if len(db.queries) != 2 || !strings.Contains(db.queries[1], "INSERT INTO api_usage_monthly") {
		t.Fatalf("queries = %+v, want monthly upsert after daily", db.queries)
	}
	if db.argSets[1][0] != time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("monthly args = %+v", db.argSets[1])
	}
}

func TestPostgresAggregateStoreClampsNegativeMetrics(t *testing.T) {
	t.Parallel()

	db := &fakeUsageSQL{}
	store := NewPostgresAggregateStore(db)
	err := store.AddUsage(context.Background(), UsageEvent{
		Day:           time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Method:        "GET",
		Route:         "GET /api/v1/messages",
		Status:        200,
		RequestBytes:  -12,
		ResponseBytes: -34,
		LatencyMS:     -25,
		RequestCount:  0,
	})
	if err != nil {
		t.Fatalf("AddUsage returned error: %v", err)
	}
	args := db.argSets[0]
	if args[11] != int64(1) || args[12] != int64(0) || args[13] != int64(0) || args[14] != int64(0) || args[15] != int64(0) {
		t.Fatalf("aggregate metric args = %+v", args)
	}
}

func TestPostgresAggregateStoreSkipsDuplicateEventID(t *testing.T) {
	t.Parallel()

	db := &fakeUsageSQL{rowsAffected: []int64{0}}
	store := NewPostgresAggregateStore(db)
	err := store.AddUsage(context.Background(), UsageEvent{
		EventID:       "usage-1",
		Day:           time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Method:        "GET",
		Route:         "GET /api/v1/messages",
		Status:        200,
		UserID:        "user-1",
		RequestBytes:  12,
		ResponseBytes: 34,
		LatencyMS:     25,
		RequestCount:  1,
	})
	if err != nil {
		t.Fatalf("AddUsage returned error: %v", err)
	}
	if len(db.queries) != 1 || !strings.Contains(db.queries[0], "INSERT INTO api_usage_events") {
		t.Fatalf("queries = %+v, want only idempotency insert", db.queries)
	}
}

func TestPostgresAggregateStoreRecordsUsageLedger(t *testing.T) {
	t.Parallel()

	db := &fakeUsageSQL{}
	store := NewPostgresAggregateStore(db)
	err := store.AddUsage(context.Background(), UsageEvent{
		EventID:       "usage-1",
		SchemaVersion: APIUsageSchemaV2,
		RawPayload:    json.RawMessage(`{"event":"api.usage","event_id":"usage-1"}`),
		Day:           time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Method:        "GET",
		Route:         "GET /api/v1/messages",
		Status:        200,
		TenantID:      "tenant-1",
		UserID:        "user-1",
		PrincipalID:   "principal-1",
		AuthSource:    "bearer",
		RequestBytes:  12,
		ResponseBytes: 34,
		LatencyMS:     25,
		RequestCount:  1,
	})
	if err != nil {
		t.Fatalf("AddUsage returned error: %v", err)
	}
	if len(db.queries) < 2 || !strings.Contains(db.queries[1], "INSERT INTO api_usage_ledger") {
		t.Fatalf("queries = %+v, want api_usage_ledger after claim", db.queries)
	}
	args := db.argSets[1]
	if args[0] != "usage-1" || args[1] != APIUsageSchemaV2 {
		t.Fatalf("ledger id/schema args = %+v", args)
	}
	if args[6] != "tenant-1" || args[10] != "" || args[11] != "principal-1" || args[12] != "bearer" {
		t.Fatalf("ledger identity args = %+v", args)
	}
	if string(args[17].([]byte)) != `{"event":"api.usage","event_id":"usage-1"}` {
		t.Fatalf("ledger payload arg = %s", args[17])
	}
}

func TestPostgresAggregateStoreClaimsEventDimensions(t *testing.T) {
	t.Parallel()

	db := &fakeUsageSQL{rowsAffected: []int64{0}}
	store := NewPostgresAggregateStore(db)
	err := store.AddUsage(context.Background(), UsageEvent{
		EventID:     "usage-1",
		Day:         time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Method:      "GET",
		Route:       "GET /api/v1/messages",
		Status:      200,
		TenantID:    "tenant-1",
		CompanyID:   "company-1",
		DomainID:    "domain-1",
		UserID:      "user-1",
		APIKeyID:    "api-key-1",
		PrincipalID: "principal-1",
		AuthSource:  "bearer",
	})
	if err != nil {
		t.Fatalf("AddUsage returned error: %v", err)
	}
	if len(db.argSets) != 1 {
		t.Fatalf("argSets = %+v", db.argSets)
	}
	args := db.argSets[0]
	if args[6] != "tenant-1" || args[7] != "company-1" || args[8] != "domain-1" {
		t.Fatalf("claim dimension args = %+v", args)
	}
	if args[9] != "api-key-1" || args[10] != "principal-1" || args[11] != "bearer" {
		t.Fatalf("claim principal args = %+v", args)
	}
}

type fakeUsageAggregateStore struct {
	event UsageEvent
}

func (s *fakeUsageAggregateStore) AddUsage(_ context.Context, event UsageEvent) error {
	s.event = event
	return nil
}

type fakeUsageSQL struct {
	query        string
	args         []any
	queries      []string
	argSets      [][]any
	rowsAffected []int64
}

func (f *fakeUsageSQL) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.query = query
	f.args = args
	f.queries = append(f.queries, query)
	f.argSets = append(f.argSets, args)
	rowsAffected := int64(1)
	if len(f.rowsAffected) > 0 {
		rowsAffected = f.rowsAffected[0]
		f.rowsAffected = f.rowsAffected[1:]
	}
	return fakeUsageSQLResult{rowsAffected: rowsAffected}, nil
}

type fakeUsageSQLResult struct {
	rowsAffected int64
}

func (r fakeUsageSQLResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeUsageSQLResult) RowsAffected() (int64, error) { return r.rowsAffected, nil }
