package pushnotify

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type PostgresRecorder struct {
	db *sql.DB
}

func NewPostgresRecorder(db *sql.DB) *PostgresRecorder {
	return &PostgresRecorder{db: db}
}

func (r *PostgresRecorder) RecordCandidate(ctx context.Context, record CandidateRecord) (CandidateRecordResult, error) {
	if r == nil || r.db == nil {
		return CandidateRecordResult{}, fmt.Errorf("database handle is required")
	}
	record = normalizeCandidateRecord(record)
	if record.MessageID == "" {
		return CandidateRecordResult{}, fmt.Errorf("message_id is required")
	}
	if record.UserID == "" {
		return CandidateRecordResult{}, fmt.Errorf("user_id is required")
	}
	if record.DeviceID == "" {
		return CandidateRecordResult{}, fmt.Errorf("device_id is required")
	}
	if record.Status == "" {
		record.Status = "candidate"
	}

	const query = `
INSERT INTO push_notification_attempts (
  message_id, rfc_message_id, company_id, domain_id, user_id, recipient,
  subject, device_id, platform, token_suffix, status, error_message
) VALUES (
  $1::uuid, $2, nullif($3, '')::uuid, nullif($4, '')::uuid, $5::uuid, $6,
  $7, $8::uuid, $9, $10, $11, $12
)
RETURNING id::text`
	var result CandidateRecordResult
	if err := r.db.QueryRowContext(
		ctx,
		query,
		record.MessageID,
		record.RFCMessageID,
		record.CompanyID,
		record.DomainID,
		record.UserID,
		record.Recipient,
		record.Subject,
		record.DeviceID,
		record.Platform,
		record.TokenSuffix,
		record.Status,
		record.ErrorMessage,
	).Scan(&result.ID); err != nil {
		return CandidateRecordResult{}, fmt.Errorf("record push notification candidate: %w", err)
	}
	return result, nil
}

func normalizeCandidateRecord(record CandidateRecord) CandidateRecord {
	record.MessageID = strings.TrimSpace(record.MessageID)
	record.RFCMessageID = strings.TrimSpace(record.RFCMessageID)
	record.CompanyID = strings.TrimSpace(record.CompanyID)
	record.DomainID = strings.TrimSpace(record.DomainID)
	record.UserID = strings.TrimSpace(record.UserID)
	record.Recipient = strings.TrimSpace(record.Recipient)
	record.Subject = strings.TrimSpace(record.Subject)
	record.DeviceID = strings.TrimSpace(record.DeviceID)
	record.Platform = strings.ToLower(strings.TrimSpace(record.Platform))
	record.TokenSuffix = strings.TrimSpace(record.TokenSuffix)
	record.Status = strings.ToLower(strings.TrimSpace(record.Status))
	record.ErrorMessage = strings.TrimSpace(record.ErrorMessage)
	if len(record.Subject) > 500 {
		record.Subject = record.Subject[:500]
	}
	if len(record.ErrorMessage) > 2000 {
		record.ErrorMessage = record.ErrorMessage[:2000]
	}
	return record
}
