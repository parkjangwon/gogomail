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
	payload, err := json.Marshal(apiUsagePayload(event))
	if err != nil {
		return fmt.Errorf("marshal api usage event: %w", err)
	}
	partitionKey := strings.TrimSpace(event.UserID)
	if partitionKey == "" {
		partitionKey = strings.TrimSpace(event.Identity.PrincipalID)
	}
	if partitionKey == "" {
		partitionKey = strings.TrimSpace(event.Identity.TenantID)
	}
	if partitionKey == "" {
		partitionKey = strings.TrimSpace(event.RoutePattern)
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

func apiUsagePayload(event Event) map[string]any {
	timestamp := event.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	identity := event.Identity.Normalize()
	if identity.UserID == "" && event.UserID != "" {
		identity.UserID = strings.TrimSpace(event.UserID)
	}
	if identity.AuthSource == AuthSourceUnknown && event.AuthSource != "" {
		identity.AuthSource = normalizeAuthSource(event.AuthSource)
	}
	identity = identity.Normalize()
	return map[string]any{
		"schema_version": APIUsageSchemaCurrent,
		"event":          EventAPIUsage,
		"event_id":       apiUsageEventID(event, timestamp, identity),
		"method":         strings.TrimSpace(event.Method),
		"route":          strings.TrimSpace(event.RoutePattern),
		"status":         event.Status,
		"request_bytes":  event.RequestBytes,
		"response_bytes": event.ResponseBytes,
		"latency_ms":     event.Latency.Milliseconds(),
		"timestamp":      timestamp.UTC().Format(time.RFC3339Nano),
		"tenant_id":      identity.TenantID,
		"company_id":     identity.CompanyID,
		"domain_id":      identity.DomainID,
		"user_id":        identity.UserID,
		"api_key_id":     identity.APIKeyID,
		"principal_id":   identity.PrincipalID,
		"auth_source":    identity.AuthSource,
	}
}

func apiUsageEventID(event Event, timestamp time.Time, identity Identity) string {
	if id := strings.TrimSpace(event.ID); id != "" {
		return id
	}
	identity = identity.Normalize()
	parts := []string{
		timestamp.UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(event.Method),
		strings.TrimSpace(event.RoutePattern),
		fmt.Sprint(event.Status),
		identity.TenantID,
		identity.CompanyID,
		identity.DomainID,
		identity.UserID,
		identity.APIKeyID,
		identity.PrincipalID,
		identity.AuthSource,
		fmt.Sprint(event.RequestBytes),
		fmt.Sprint(event.ResponseBytes),
		fmt.Sprint(event.Latency.Milliseconds()),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "api-usage-" + hex.EncodeToString(sum[:16])
}
