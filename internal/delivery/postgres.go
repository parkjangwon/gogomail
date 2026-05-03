package delivery

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type PostgresRecorder struct {
	db *sql.DB
}

func NewPostgresRecorder(db *sql.DB) *PostgresRecorder {
	return &PostgresRecorder{db: db}
}

func (r *PostgresRecorder) RecordAttempt(ctx context.Context, attempt Attempt) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delivery attempt transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
INSERT INTO delivery_attempts (
  message_id, rfc_message_id, farm,
  recipient, recipient_domain,
  status, error_message, attempted_at
) VALUES (
  $1, $2, $3,
  $4, $5,
  $6, $7, $8
)`

	if _, err := tx.ExecContext(
		ctx,
		query,
		attempt.MessageID,
		attempt.RFCMessageID,
		attempt.Farm,
		attempt.Recipient,
		attempt.RecipientDomain,
		string(attempt.Status),
		attempt.ErrorMessage,
		attempt.AttemptedAt,
	); err != nil {
		return fmt.Errorf("insert delivery attempt: %w", err)
	}
	if attempt.Status == AttemptBounced {
		if err := suppressBouncedRecipient(ctx, tx, attempt); err != nil {
			return err
		}
	}
	if err := insertDeliveryAttemptEvent(ctx, tx, attempt); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delivery attempt transaction: %w", err)
	}
	return nil
}

func suppressBouncedRecipient(ctx context.Context, tx *sql.Tx, attempt Attempt) error {
	const query = `
INSERT INTO suppression_list (domain_id, email, reason, source_message_id)
VALUES ($1, $2, 'hard_bounce', $3)
ON CONFLICT DO NOTHING`

	if _, err := tx.ExecContext(ctx, query, uuidOrNil(attempt.DomainID), attempt.Recipient, uuidOrNil(attempt.MessageID)); err != nil {
		return fmt.Errorf("insert suppression list entry: %w", err)
	}
	return nil
}

func insertDeliveryAttemptEvent(ctx context.Context, tx *sql.Tx, attempt Attempt) error {
	payload, err := deliveryAttemptEventPayload(attempt)
	if err != nil {
		return err
	}

	const query = `
INSERT INTO outbox (topic, partition_key, payload, status)
VALUES ('mail.event', $1, $2::jsonb, 'pending')`

	if _, err := tx.ExecContext(ctx, query, attempt.MessageID, string(payload)); err != nil {
		return fmt.Errorf("insert delivery attempt event: %w", err)
	}
	return nil
}

func deliveryAttemptEventPayload(attempt Attempt) ([]byte, error) {
	event := "mail.delivery_failed"
	switch attempt.Status {
	case AttemptDelivered:
		event = "mail.delivered"
	case AttemptBounced:
		event = "mail.bounced"
	}

	payload := map[string]any{
		"event":            event,
		"message_id":       attempt.MessageID,
		"rfc_message_id":   attempt.RFCMessageID,
		"company_id":       attempt.CompanyID,
		"domain_id":        attempt.DomainID,
		"farm":             attempt.Farm,
		"recipient":        attempt.Recipient,
		"recipient_domain": attempt.RecipientDomain,
		"status":           attempt.Status,
		"enhanced_status":  attempt.EnhancedStatus,
		"error_message":    attempt.ErrorMessage,
		"attempted_at":     attempt.AttemptedAt,
	}
	if attempt.DSNReturn != "" || attempt.DSNEnvelopeID != "" || len(attempt.DSNNotify) > 0 || attempt.OriginalRecipient != "" {
		payload["dsn"] = map[string]any{
			"return":             attempt.DSNReturn,
			"envelope_id":        attempt.DSNEnvelopeID,
			"notify":             append([]string(nil), attempt.DSNNotify...),
			"original_recipient": attempt.OriginalRecipient,
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal delivery attempt event: %w", err)
	}
	return raw, nil
}

func uuidOrNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}
