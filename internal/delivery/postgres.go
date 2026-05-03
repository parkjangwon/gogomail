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
	return nil
}
