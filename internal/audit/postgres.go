package audit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Insert(ctx context.Context, log Log) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin audit log transaction: %w", err)
	}
	defer tx.Rollback()

	if err := InsertTx(ctx, tx, log); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit audit log transaction: %w", err)
	}
	return nil
}

func InsertTx(ctx context.Context, tx *sql.Tx, log Log) error {
	if tx == nil {
		return fmt.Errorf("audit transaction is required")
	}

	normalized, err := log.normalized()
	if err != nil {
		return err
	}

	prevHash, err := latestAuditHash(ctx, tx)
	if err != nil {
		return err
	}
	hash, err := ComputeHash(prevHash, normalized)
	if err != nil {
		return err
	}

	const query = `
INSERT INTO audit_logs (
  company_id, domain_id, user_id, actor_id,
  category, action, target_type, target_id,
  ip_address, user_agent, result, detail,
  prev_hash, hash, created_at
) VALUES (
  $1, $2, $3, $4,
  $5, $6, $7, $8,
  $9, $10, $11, $12::jsonb,
  $13, $14, $15
)`

	_, err = tx.ExecContext(
		ctx,
		query,
		uuidOrNil(normalized.CompanyID),
		uuidOrNil(normalized.DomainID),
		uuidOrNil(normalized.UserID),
		uuidOrNil(normalized.ActorID),
		normalized.Category,
		normalized.Action,
		normalized.TargetType,
		uuidOrNil(normalized.TargetID),
		textOrNil(normalized.IPAddress),
		normalized.UserAgent,
		normalized.Result,
		string(normalized.Detail),
		prevHash,
		hash,
		normalized.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

func latestAuditHash(ctx context.Context, tx *sql.Tx) (string, error) {
	const query = `
SELECT hash
FROM audit_logs
ORDER BY created_at DESC, id DESC
LIMIT 1
FOR UPDATE`

	var hash string
	if err := tx.QueryRowContext(ctx, query).Scan(&hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("read latest audit hash: %w", err)
	}
	return hash, nil
}

func uuidOrNil(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func textOrNil(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
