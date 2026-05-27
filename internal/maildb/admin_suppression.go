package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/audit"
)

func (r *Repository) ListSuppressionEntries(ctx context.Context, req SuppressionEntryListRequest) ([]SuppressionEntry, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateSuppressionEntryListRequest(req); err != nil {
		return nil, err
	}
	limit := normalizeLimit(req.Limit)

	query, args := buildListSuppressionEntriesQuery(req, limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list suppression entries: %w", err)
	}
	defer rows.Close()

	var entries []SuppressionEntry
	for rows.Next() {
		var entry SuppressionEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.DomainID,
			&entry.Email,
			&entry.Reason,
			&entry.SourceMessageID,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan suppression entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate suppression entries: %w", err)
	}
	return entries, nil
}

func buildListSuppressionEntriesQuery(req SuppressionEntryListRequest, limit int) (string, []any) {
	args := make([]any, 0, 4)
	conditions := make([]string, 0, 3)
	if domainID := strings.TrimSpace(req.DomainID); domainID != "" {
		args = append(args, domainID)
		conditions = append(conditions, fmt.Sprintf("domain_id = $%d::uuid", len(args)))
	}
	if email := strings.TrimSpace(req.Email); email != "" {
		args = append(args, email)
		conditions = append(conditions, fmt.Sprintf("email = $%d", len(args)))
	}
	if reason := strings.TrimSpace(req.Reason); reason != "" {
		args = append(args, reason)
		conditions = append(conditions, fmt.Sprintf("reason = $%d", len(args)))
	}
	args = append(args, limit)
	limitPlaceholder := fmt.Sprintf("$%d", len(args))

	where := ""
	if len(conditions) > 0 {
		where = "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}

	query := `
SELECT
  id::text,
  COALESCE(domain_id::text, ''),
  email,
  reason,
  COALESCE(source_message_id::text, ''),
  created_at
FROM suppression_list
` + strings.TrimPrefix(where, "\n") + `
ORDER BY created_at DESC, id DESC
LIMIT ` + limitPlaceholder
	return query, args
}

func ValidateSuppressionEntryListRequest(req SuppressionEntryListRequest) error {
	for field, value := range map[string]string{
		"domain_id": req.DomainID,
		"email":     req.Email,
		"reason":    req.Reason,
	} {
		if err := validatePushNotificationFilter(field, strings.TrimSpace(value)); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) GetSuppressionEntry(ctx context.Context, id string) (SuppressionEntry, error) {
	if r.db == nil {
		return SuppressionEntry{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return SuppressionEntry{}, fmt.Errorf("suppression entry id is required")
	}
	const query = `
SELECT
  id::text,
  COALESCE(domain_id::text, ''),
  email,
  reason,
  COALESCE(source_message_id::text, ''),
  created_at
FROM suppression_list
WHERE id = $1
LIMIT 1`
	var entry SuppressionEntry
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&entry.ID,
		&entry.DomainID,
		&entry.Email,
		&entry.Reason,
		&entry.SourceMessageID,
		&entry.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SuppressionEntry{}, fmt.Errorf("suppression entry %q not found", id)
		}
		return SuppressionEntry{}, fmt.Errorf("get suppression entry: %w", err)
	}
	return entry, nil
}

func (r *Repository) DeleteSuppressionEntry(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("suppression entry id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin suppression delete transaction: %w", err)
	}
	defer tx.Rollback()

	var entry SuppressionEntry
	if err := tx.QueryRowContext(ctx, `
SELECT
  id::text,
  COALESCE(domain_id::text, ''),
  email,
  reason,
  COALESCE(source_message_id::text, ''),
  created_at
FROM suppression_list
WHERE id = $1
FOR UPDATE`, id).Scan(
		&entry.ID,
		&entry.DomainID,
		&entry.Email,
		&entry.Reason,
		&entry.SourceMessageID,
		&entry.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("suppression entry %q not found", id)
		}
		return fmt.Errorf("read suppression entry for deletion: %w", err)
	}

	result, err := tx.ExecContext(ctx, `DELETE FROM suppression_list WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete suppression entry: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("suppression entry %q not found", id)
	}
	detail, err := suppressionEntryAuditDetail(entry)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		DomainID:   entry.DomainID,
		Category:   "admin",
		Action:     "suppression.delete",
		TargetType: "suppression_entry",
		TargetID:   entry.ID,
		Result:     "deleted",
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record suppression delete audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit suppression delete transaction: %w", err)
	}
	return nil
}

func suppressionEntryAuditDetail(entry SuppressionEntry) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"suppression_entry_id": entry.ID,
		"domain_id":            entry.DomainID,
		"email":                entry.Email,
		"reason":               entry.Reason,
		"source_message_id":    entry.SourceMessageID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal suppression audit detail: %w", err)
	}
	return detail, nil
}
