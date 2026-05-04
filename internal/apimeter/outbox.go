package apimeter

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	OutboxTopicAPIUsage   = "api.event"
	EventAPIUsage         = "api.usage"
	APIUsageSchemaV1      = "2026-05-04.api-usage.v1"
	APIUsageSchemaV2      = "2026-05-04.api-usage.v2"
	APIUsageSchemaCurrent = APIUsageSchemaV2
)

type SQLExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type PostgresOutboxSink struct {
	db SQLExecer
}

func NewPostgresOutboxSink(db SQLExecer) PostgresOutboxSink {
	return PostgresOutboxSink{db: db}
}

func (s PostgresOutboxSink) Record(ctx context.Context, event Event) error {
	if s.db == nil {
		return fmt.Errorf("database handle is required")
	}
	payloadMap, err := apiUsagePayload(event)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(payloadMap)
	if err != nil {
		return fmt.Errorf("marshal api usage event: %w", err)
	}
	partitionKey := strings.TrimSpace(payloadMap["user_id"].(string))
	if partitionKey == "" {
		partitionKey = strings.TrimSpace(payloadMap["principal_id"].(string))
	}
	if partitionKey == "" {
		partitionKey = strings.TrimSpace(payloadMap["tenant_id"].(string))
	}
	if partitionKey == "" {
		partitionKey = strings.TrimSpace(payloadMap["route"].(string))
	}
	if partitionKey == "" {
		partitionKey = "anonymous"
	}

	const query = `
INSERT INTO outbox (topic, partition_key, payload, status)
VALUES ($1, $2, $3, 'pending')`
	if _, err := s.db.ExecContext(ctx, query, OutboxTopicAPIUsage, partitionKey, payload); err != nil {
		return fmt.Errorf("insert api usage outbox event: %w", err)
	}
	return nil
}

func apiUsagePayload(event Event) (map[string]any, error) {
	timestamp := event.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	requestBytes, responseBytes, latencyMS := normalizedEventMetrics(event)
	identity := event.Identity.Normalize()
	if identity.UserID == "" && event.UserID != "" {
		identity.UserID = strings.TrimSpace(event.UserID)
	}
	if identity.AuthSource == AuthSourceUnknown && event.AuthSource != "" {
		identity.AuthSource = normalizeAuthSource(event.AuthSource)
	}
	identity = identity.Normalize()
	method, err := requiredUsageEventValue("method", event.Method)
	if err != nil {
		return nil, err
	}
	route, err := requiredUsageEventValue("route", event.RoutePattern)
	if err != nil {
		return nil, err
	}
	eventID, err := optionalUsageEventValue("event_id", event.ID)
	if err != nil {
		return nil, err
	}
	identity.TenantID, err = optionalUsageEventValue("tenant_id", identity.TenantID)
	if err != nil {
		return nil, err
	}
	identity.CompanyID, err = optionalUsageEventValue("company_id", identity.CompanyID)
	if err != nil {
		return nil, err
	}
	identity.DomainID, err = optionalUsageEventValue("domain_id", identity.DomainID)
	if err != nil {
		return nil, err
	}
	identity.UserID, err = optionalUsageEventValue("user_id", identity.UserID)
	if err != nil {
		return nil, err
	}
	identity.APIKeyID, err = optionalUsageEventValue("api_key_id", identity.APIKeyID)
	if err != nil {
		return nil, err
	}
	identity.PrincipalID, err = optionalUsageEventValue("principal_id", identity.PrincipalID)
	if err != nil {
		return nil, err
	}
	if eventID == "" {
		eventID = apiUsageEventID(event, timestamp, identity, method, route)
	}
	return map[string]any{
		"schema_version": APIUsageSchemaCurrent,
		"event":          EventAPIUsage,
		"event_id":       eventID,
		"method":         method,
		"route":          route,
		"status":         event.Status,
		"request_bytes":  requestBytes,
		"response_bytes": responseBytes,
		"latency_ms":     latencyMS,
		"timestamp":      timestamp.UTC().Format(time.RFC3339Nano),
		"tenant_id":      identity.TenantID,
		"company_id":     identity.CompanyID,
		"domain_id":      identity.DomainID,
		"user_id":        identity.UserID,
		"api_key_id":     identity.APIKeyID,
		"principal_id":   identity.PrincipalID,
		"auth_source":    identity.AuthSource,
	}, nil
}

func apiUsageEventID(event Event, timestamp time.Time, identity Identity, method string, route string) string {
	if id := strings.TrimSpace(event.ID); id != "" {
		return id
	}
	identity = identity.Normalize()
	requestBytes, responseBytes, latencyMS := normalizedEventMetrics(event)
	parts := []string{
		timestamp.UTC().Format(time.RFC3339Nano),
		method,
		route,
		fmt.Sprint(event.Status),
		identity.TenantID,
		identity.CompanyID,
		identity.DomainID,
		identity.UserID,
		identity.APIKeyID,
		identity.PrincipalID,
		identity.AuthSource,
		fmt.Sprint(requestBytes),
		fmt.Sprint(responseBytes),
		fmt.Sprint(latencyMS),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "api-usage-" + hex.EncodeToString(sum[:16])
}

func normalizedEventMetrics(event Event) (int64, int64, int64) {
	requestBytes := event.RequestBytes
	if requestBytes < 0 {
		requestBytes = 0
	}
	responseBytes := event.ResponseBytes
	if responseBytes < 0 {
		responseBytes = 0
	}
	latencyMS := event.Latency.Milliseconds()
	if latencyMS < 0 {
		latencyMS = 0
	}
	return requestBytes, responseBytes, latencyMS
}
