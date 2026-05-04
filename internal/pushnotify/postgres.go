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

type AttemptOutcome struct {
	AttemptID         string
	Status            string
	ErrorMessage      string
	ProviderMessageID string
	ProviderStatus    string
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
	record.ErrorMessage = strings.TrimSpace(record.ErrorMessage)
	if len(record.Subject) > 500 {
		record.Subject = record.Subject[:500]
	}
	if len(record.ErrorMessage) > 2000 {
		record.ErrorMessage = record.ErrorMessage[:2000]
	}
	return record
}

func normalizeAttemptOutcome(outcome AttemptOutcome) (AttemptOutcome, error) {
	outcome.AttemptID = strings.TrimSpace(outcome.AttemptID)
	outcome.Status = strings.ToLower(strings.TrimSpace(outcome.Status))
	outcome.ErrorMessage = strings.TrimSpace(outcome.ErrorMessage)
	outcome.ProviderMessageID = strings.TrimSpace(outcome.ProviderMessageID)
	outcome.ProviderStatus = strings.TrimSpace(outcome.ProviderStatus)
	if outcome.AttemptID == "" {
		return AttemptOutcome{}, fmt.Errorf("attempt_id is required")
	}
	if !allowedOutcomeStatus(outcome.Status) {
		return AttemptOutcome{}, fmt.Errorf("unsupported push notification outcome status")
	}
	if len(outcome.ErrorMessage) > 2000 {
		outcome.ErrorMessage = outcome.ErrorMessage[:2000]
	}
	if len(outcome.ProviderMessageID) > 500 {
		outcome.ProviderMessageID = outcome.ProviderMessageID[:500]
	}
	if len(outcome.ProviderStatus) > 500 {
		outcome.ProviderStatus = outcome.ProviderStatus[:500]
	}
	return outcome, nil
}

func allowedOutcomeStatus(status string) bool {
	switch status {
	case "queued", "delivered", "failed", "invalid_token":
		return true
	default:
		return false
	}
}
