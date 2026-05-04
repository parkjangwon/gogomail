package apimeter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/eventstream"
)

type UsageEvent struct {
	EventID       string
	SchemaVersion string
	RawPayload    json.RawMessage
	Day           time.Time
	Month         time.Time
	Method        string
	Route         string
	Status        int
	TenantID      string
	CompanyID     string
	DomainID      string
	UserID        string
	APIKeyID      string
	PrincipalID   string
	AuthSource    string
	RequestBytes  int64
	ResponseBytes int64
	LatencyMS     int64
	RequestCount  int64
}

type UsageAggregateStore interface {
	AddUsage(ctx context.Context, event UsageEvent) error
}

type PostgresAggregateStore struct {
	db SQLExecer
}

func NewPostgresAggregateStore(db SQLExecer) PostgresAggregateStore {
	return PostgresAggregateStore{db: db}
}

func (s PostgresAggregateStore) AddUsage(ctx context.Context, event UsageEvent) error {
	if s.db == nil {
		return fmt.Errorf("database handle is required")
	}
	event = normalizeUsageEventMetrics(event)
	claimed, err := s.claimEvent(ctx, event)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	if err := s.recordLedger(ctx, event); err != nil {
		return err
	}
	if event.RequestCount <= 0 {
		event.RequestCount = 1
	}
	if event.Month.IsZero() {
		event.Month = time.Date(event.Day.Year(), event.Day.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	if err := s.upsert(ctx, "api_usage_daily", "day", event.Day, event); err != nil {
		return err
	}
	if err := s.upsert(ctx, "api_usage_monthly", "month", event.Month, event); err != nil {
		return err
	}
	return nil
}

func normalizeUsageEventMetrics(event UsageEvent) UsageEvent {
	if event.RequestBytes < 0 {
		event.RequestBytes = 0
	}
	if event.ResponseBytes < 0 {
		event.ResponseBytes = 0
	}
	if event.LatencyMS < 0 {
		event.LatencyMS = 0
	}
	if event.RequestCount <= 0 {
		event.RequestCount = 1
	}
	return event
}

func (s PostgresAggregateStore) recordLedger(ctx context.Context, event UsageEvent) error {
	eventID := strings.TrimSpace(event.EventID)
	if eventID == "" {
		return nil
	}
	rawPayload := event.RawPayload
	if len(rawPayload) == 0 {
		rawPayload = json.RawMessage(`{}`)
	}
	schemaVersion := strings.TrimSpace(event.SchemaVersion)
	if schemaVersion == "" {
		schemaVersion = APIUsageSchemaV1
	}
	eventTime := event.Day
	if eventTime.IsZero() {
		eventTime = time.Now().UTC()
	}
	identity := Identity{
		TenantID:    event.TenantID,
		CompanyID:   event.CompanyID,
		DomainID:    event.DomainID,
		UserID:      event.UserID,
		APIKeyID:    event.APIKeyID,
		PrincipalID: event.PrincipalID,
		AuthSource:  event.AuthSource,
	}.Normalize()
	requestCount := event.RequestCount
	if requestCount <= 0 {
		requestCount = 1
	}
	const query = `
INSERT INTO api_usage_ledger (
  event_id,
  schema_version,
  event_timestamp,
  method,
  route,
  status,
  tenant_id,
  company_id,
  domain_id,
  user_id,
  api_key_id,
  principal_id,
  auth_source,
  request_count,
  request_bytes,
  response_bytes,
  latency_ms,
  payload
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
ON CONFLICT (event_id) DO NOTHING`
	if _, err := s.db.ExecContext(
		ctx,
		query,
		eventID,
		schemaVersion,
		eventTime,
		event.Method,
		event.Route,
		event.Status,
		identity.TenantID,
		identity.CompanyID,
		identity.DomainID,
		identity.UserID,
		identity.APIKeyID,
		identity.PrincipalID,
		identity.AuthSource,
		requestCount,
		event.RequestBytes,
		event.ResponseBytes,
		event.LatencyMS,
		[]byte(rawPayload),
	); err != nil {
		return fmt.Errorf("record api usage ledger: %w", err)
	}
	return nil
}

func (s PostgresAggregateStore) claimEvent(ctx context.Context, event UsageEvent) (bool, error) {
	eventID := strings.TrimSpace(event.EventID)
	if eventID == "" {
		return true, nil
	}
	eventTime := event.Day
	if eventTime.IsZero() {
		eventTime = time.Now().UTC()
	}
	const query = `
INSERT INTO api_usage_events (
  event_id,
  event_timestamp,
  method,
  route,
  status,
  user_id,
  tenant_id,
  company_id,
  domain_id,
  api_key_id,
  principal_id,
  auth_source
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (event_id) DO NOTHING`
	identity := Identity{
		TenantID:    event.TenantID,
		CompanyID:   event.CompanyID,
		DomainID:    event.DomainID,
		UserID:      event.UserID,
		APIKeyID:    event.APIKeyID,
		PrincipalID: event.PrincipalID,
		AuthSource:  event.AuthSource,
	}.Normalize()
	result, err := s.db.ExecContext(
		ctx,
		query,
		eventID,
		eventTime,
		event.Method,
		event.Route,
		event.Status,
		identity.UserID,
		identity.TenantID,
		identity.CompanyID,
		identity.DomainID,
		identity.APIKeyID,
		identity.PrincipalID,
		identity.AuthSource,
	)
	if err != nil {
		return false, fmt.Errorf("claim api usage event: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("inspect api usage event claim: %w", err)
	}
	return affected > 0, nil
}

func (s PostgresAggregateStore) upsert(ctx context.Context, table string, bucketColumn string, bucket time.Time, event UsageEvent) error {
	query := fmt.Sprintf(`
INSERT INTO %s (
  %s,
  method,
  route,
  status,
  tenant_id,
  company_id,
  domain_id,
  user_id,
  api_key_id,
  principal_id,
  auth_source,
  request_count,
  request_bytes,
  response_bytes,
  latency_ms_total,
  latency_ms_max,
  first_seen_at,
  last_seen_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, now(), now())
ON CONFLICT (%s, method, route, status, tenant_id, company_id, domain_id, user_id, api_key_id, principal_id, auth_source)
DO UPDATE SET
  request_count = %[1]s.request_count + EXCLUDED.request_count,
  request_bytes = %[1]s.request_bytes + EXCLUDED.request_bytes,
  response_bytes = %[1]s.response_bytes + EXCLUDED.response_bytes,
  latency_ms_total = %[1]s.latency_ms_total + EXCLUDED.latency_ms_total,
  latency_ms_max = GREATEST(%[1]s.latency_ms_max, EXCLUDED.latency_ms_max),
  last_seen_at = GREATEST(%[1]s.last_seen_at, EXCLUDED.last_seen_at)`, table, bucketColumn, bucketColumn)
	identity := Identity{
		TenantID:    event.TenantID,
		CompanyID:   event.CompanyID,
		DomainID:    event.DomainID,
		UserID:      event.UserID,
		APIKeyID:    event.APIKeyID,
		PrincipalID: event.PrincipalID,
		AuthSource:  event.AuthSource,
	}.Normalize()
	if _, err := s.db.ExecContext(
		ctx,
		query,
		bucket,
		event.Method,
		event.Route,
		event.Status,
		identity.TenantID,
		identity.CompanyID,
		identity.DomainID,
		identity.UserID,
		identity.APIKeyID,
		identity.PrincipalID,
		identity.AuthSource,
		event.RequestCount,
		event.RequestBytes,
		event.ResponseBytes,
		event.LatencyMS,
		event.LatencyMS,
	); err != nil {
		return fmt.Errorf("upsert api usage aggregate %s: %w", table, err)
	}
	return nil
}

type UsageHandler struct {
	store UsageAggregateStore
}

func NewUsageHandler(store UsageAggregateStore) UsageHandler {
	return UsageHandler{store: store}
}

func (h UsageHandler) HandleEvent(ctx context.Context, msg eventstream.Message) error {
	if h.store == nil {
		return fmt.Errorf("api usage aggregate store is required")
	}
	event, err := DecodeUsageEvent(msg.Payload)
	if err != nil {
		return err
	}
	return h.store.AddUsage(ctx, event)
}

func DecodeUsageEvent(payload json.RawMessage) (UsageEvent, error) {
	var raw struct {
		Event         string `json:"event"`
		SchemaVersion string `json:"schema_version"`
		EventID       string `json:"event_id"`
		Method        string `json:"method"`
		Route         string `json:"route"`
		Status        int    `json:"status"`
		RequestBytes  int64  `json:"request_bytes"`
		ResponseBytes int64  `json:"response_bytes"`
		LatencyMS     int64  `json:"latency_ms"`
		Timestamp     string `json:"timestamp"`
		TenantID      string `json:"tenant_id"`
		CompanyID     string `json:"company_id"`
		DomainID      string `json:"domain_id"`
		UserID        string `json:"user_id"`
		APIKeyID      string `json:"api_key_id"`
		PrincipalID   string `json:"principal_id"`
		AuthSource    string `json:"auth_source"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return UsageEvent{}, fmt.Errorf("decode api usage event: %w", err)
	}
	if strings.TrimSpace(raw.Event) != EventAPIUsage {
		return UsageEvent{}, fmt.Errorf("unexpected api metering event %q", raw.Event)
	}
	if schemaVersion := strings.TrimSpace(raw.SchemaVersion); schemaVersion != "" && schemaVersion != APIUsageSchemaV1 && schemaVersion != APIUsageSchemaV2 {
		return UsageEvent{}, fmt.Errorf("unsupported api usage schema_version %q", schemaVersion)
	}
	method := strings.TrimSpace(raw.Method)
	if method == "" {
		return UsageEvent{}, fmt.Errorf("api usage method is required")
	}
	route := strings.TrimSpace(raw.Route)
	if route == "" {
		return UsageEvent{}, fmt.Errorf("api usage route is required")
	}
	if raw.Status < 100 || raw.Status > 999 {
		return UsageEvent{}, fmt.Errorf("api usage status must be between 100 and 999")
	}
	timestamp, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw.Timestamp))
	if err != nil {
		return UsageEvent{}, fmt.Errorf("parse api usage timestamp: %w", err)
	}
	day := timestamp.UTC().Truncate(24 * time.Hour)
	month := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.UTC)
	return normalizeUsageEventMetrics(UsageEvent{
		EventID:       strings.TrimSpace(raw.EventID),
		SchemaVersion: strings.TrimSpace(raw.SchemaVersion),
		RawPayload:    append(json.RawMessage(nil), payload...),
		Day:           day,
		Month:         month,
		Method:        method,
		Route:         route,
		Status:        raw.Status,
		TenantID:      strings.TrimSpace(raw.TenantID),
		CompanyID:     strings.TrimSpace(raw.CompanyID),
		DomainID:      strings.TrimSpace(raw.DomainID),
		UserID:        strings.TrimSpace(raw.UserID),
		APIKeyID:      strings.TrimSpace(raw.APIKeyID),
		PrincipalID:   strings.TrimSpace(raw.PrincipalID),
		AuthSource:    normalizeAuthSource(raw.AuthSource),
		RequestBytes:  raw.RequestBytes,
		ResponseBytes: raw.ResponseBytes,
		LatencyMS:     raw.LatencyMS,
		RequestCount:  1,
	}), nil
}
