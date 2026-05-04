package pushnotify

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"
)

type PostgresRecorder struct {
	db *sql.DB
}

type AttemptOutcome struct {
	AttemptID         string
	Status            string
	ErrorMessage      string
	ProviderMessageID string
	ProviderStatus    string
}

const maxPushAttemptIDBytes = 200

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

func (r *PostgresRecorder) RecordOutcome(ctx context.Context, outcome AttemptOutcome) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	normalized, err := normalizeAttemptOutcome(outcome)
	if err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin push notification outcome transaction: %w", err)
	}
	defer tx.Rollback()

	var deviceID string
	var userID string
	if err := tx.QueryRowContext(
		ctx,
		`UPDATE push_notification_attempts
SET status = $2,
    error_message = $3,
    provider_message_id = $4,
    provider_status = $5,
    attempted_at = now()
WHERE id = $1::uuid
RETURNING COALESCE(device_id::text, ''), user_id::text`,
		normalized.AttemptID,
		normalized.Status,
		normalized.ErrorMessage,
		normalized.ProviderMessageID,
		normalized.ProviderStatus,
	).Scan(&deviceID, &userID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("push notification attempt %q not found", normalized.AttemptID)
		}
		return fmt.Errorf("record push notification outcome: %w", err)
	}

	if normalized.Status == "invalid_token" && strings.TrimSpace(deviceID) != "" {
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE push_devices SET status = 'deleted', updated_at = now() WHERE id = $1::uuid AND user_id = $2::uuid`,
			deviceID,
			userID,
		); err != nil {
			return fmt.Errorf("delete invalid push device: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit push notification outcome: %w", err)
	}
	return nil
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
	record.Subject = cleanBoundedText(record.Subject, 500)
	record.ErrorMessage = cleanBoundedText(record.ErrorMessage, 2000)
	return record
}

func normalizeAttemptOutcome(outcome AttemptOutcome) (AttemptOutcome, error) {
	outcome.AttemptID = strings.TrimSpace(outcome.AttemptID)
	outcome.Status = strings.ToLower(strings.TrimSpace(outcome.Status))
	outcome.ErrorMessage = cleanBoundedText(outcome.ErrorMessage, 2000)
	outcome.ProviderMessageID = cleanBoundedText(outcome.ProviderMessageID, 500)
	outcome.ProviderStatus = cleanBoundedText(outcome.ProviderStatus, 500)
	if outcome.AttemptID == "" {
		return AttemptOutcome{}, fmt.Errorf("attempt_id is required")
	}
	if strings.ContainsAny(outcome.AttemptID, "\r\n") || len(outcome.AttemptID) > maxPushAttemptIDBytes || !utf8.ValidString(outcome.AttemptID) {
		return AttemptOutcome{}, fmt.Errorf("attempt_id is invalid")
	}
	if !allowedOutcomeStatus(outcome.Status) {
		return AttemptOutcome{}, fmt.Errorf("unsupported push notification outcome status")
	}
	return outcome, nil
}

func cleanBoundedText(value string, maxBytes int) string {
	value = strings.ToValidUTF8(strings.TrimSpace(value), "")
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	cut := 0
	for i := range value {
		if i > maxBytes {
			return value[:cut]
		}
		cut = i
	}
	return value[:cut]
}

func allowedOutcomeStatus(status string) bool {
	switch status {
	case "queued", "delivered", "failed", "invalid_token":
		return true
	default:
		return false
	}
}
