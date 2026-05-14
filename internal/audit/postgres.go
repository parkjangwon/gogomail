package audit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type PostgresRepository struct {
	db *sql.DB
}

type ListFilters struct {
	CompanyID  string // filter by company_id
	DomainID   string // filter by domain_id
	UserID     string // filter by user_id
	Category   string // filter by category
	Action     string // filter by action
	TargetType string // filter by target_type
	FromDate   *time.Time
	ToDate     *time.Time
	Limit      int
	Offset     int
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

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (*Log, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	const query = `
SELECT company_id, domain_id, user_id, actor_id,
       category, action, target_type, target_id,
       ip_address, user_agent, result, detail,
       created_at
FROM audit_logs
WHERE id = $1`

	var log Log
	var companyID, domainID, userID, actorID, targetID, ipAddress sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&companyID, &domainID, &userID, &actorID,
		&log.Category, &log.Action, &log.TargetType, &targetID,
		&ipAddress, &log.UserAgent, &log.Result, &log.Detail,
		&log.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query audit log: %w", err)
	}

	log.CompanyID = companyID.String
	log.DomainID = domainID.String
	log.UserID = userID.String
	log.ActorID = actorID.String
	log.IPAddress = ipAddress.String
	if targetID.Valid {
		log.TargetID = targetID.String
	}

	return &log, nil
}

func (r *PostgresRepository) ListWithFilters(ctx context.Context, filters ListFilters) ([]Log, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	if filters.Limit == 0 {
		filters.Limit = 50
	}
	if filters.Limit > 1000 {
		filters.Limit = 1000
	}

	query := `
SELECT company_id, domain_id, user_id, actor_id,
       category, action, target_type, target_id,
       ip_address, user_agent, result, detail,
       created_at
FROM audit_logs
WHERE 1=1`

	args := []any{}
	argNum := 1

	if filters.CompanyID != "" {
		query += fmt.Sprintf(" AND company_id = $%d", argNum)
		args = append(args, filters.CompanyID)
		argNum++
	}
	if filters.DomainID != "" {
		query += fmt.Sprintf(" AND domain_id = $%d", argNum)
		args = append(args, filters.DomainID)
		argNum++
	}
	if filters.UserID != "" {
		query += fmt.Sprintf(" AND user_id = $%d", argNum)
		args = append(args, filters.UserID)
		argNum++
	}
	if filters.Category != "" {
		query += fmt.Sprintf(" AND category = $%d", argNum)
		args = append(args, filters.Category)
		argNum++
	}
	if filters.Action != "" {
		query += fmt.Sprintf(" AND action = $%d", argNum)
		args = append(args, filters.Action)
		argNum++
	}
	if filters.TargetType != "" {
		query += fmt.Sprintf(" AND target_type = $%d", argNum)
		args = append(args, filters.TargetType)
		argNum++
	}
	if filters.FromDate != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argNum)
		args = append(args, filters.FromDate)
		argNum++
	}
	if filters.ToDate != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argNum)
		args = append(args, filters.ToDate)
		argNum++
	}

	query += " ORDER BY created_at DESC, id DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argNum, argNum+1)
	args = append(args, filters.Limit, filters.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []Log
	for rows.Next() {
		var log Log
		var companyID, domainID, userID, actorID, targetID, ipAddress sql.NullString

		err := rows.Scan(
			&companyID, &domainID, &userID, &actorID,
			&log.Category, &log.Action, &log.TargetType, &targetID,
			&ipAddress, &log.UserAgent, &log.Result, &log.Detail,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}

		log.CompanyID = companyID.String
		log.DomainID = domainID.String
		log.UserID = userID.String
		log.ActorID = actorID.String
		log.IPAddress = ipAddress.String
		if targetID.Valid {
			log.TargetID = targetID.String
		}

		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit logs: %w", err)
	}

	return logs, nil
}
