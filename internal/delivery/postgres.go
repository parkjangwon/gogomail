package delivery

import (
	"context"
	"database/sql"
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

	if _, err := r.db.ExecContext(
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
		if err := r.suppressBouncedRecipient(ctx, attempt); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresRecorder) suppressBouncedRecipient(ctx context.Context, attempt Attempt) error {
	const query = `
INSERT INTO suppression_list (domain_id, email, reason, source_message_id)
VALUES ($1, $2, 'hard_bounce', $3)
ON CONFLICT DO NOTHING`

	if _, err := r.db.ExecContext(ctx, query, uuidOrNil(attempt.DomainID), attempt.Recipient, uuidOrNil(attempt.MessageID)); err != nil {
		return fmt.Errorf("insert suppression list entry: %w", err)
	}
	return nil
}

func uuidOrNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}
