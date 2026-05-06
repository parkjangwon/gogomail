package davsyncretention

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	maxRunErrorMessageBytes = 1024
	maxRunIDBytes           = 128
	MaxRunListLimit         = 100
)

type RunStatus string

const (
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
)

type RunRecord struct {
	ID                string    `json:"id"`
	CreatedAt         time.Time `json:"created_at"`
	Cutoff            time.Time `json:"cutoff"`
	Limit             int       `json:"limit"`
	DryRun            bool      `json:"dry_run"`
	ConfirmReady      bool      `json:"confirm_ready"`
	Status            RunStatus `json:"status"`
	ErrorMessage      string    `json:"error_message,omitempty"`
	CalDAVCandidates  int64     `json:"caldav_candidate_count"`
	CalDAVDeleted     int64     `json:"caldav_deleted_count"`
	CardDAVCandidates int64     `json:"carddav_candidate_count"`
	CardDAVDeleted    int64     `json:"carddav_deleted_count"`
}

type RunListRequest struct {
	Limit       int
	Status      RunStatus
	CreatedFrom time.Time
	CreatedTo   time.Time
}

type Repository struct {
	db  *sql.DB
	now func() time.Time
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, now: time.Now}
}

func (r *Repository) RecordRun(ctx context.Context, record RunRecord) (RunRecord, error) {
	if r.db == nil {
		return RunRecord{}, fmt.Errorf("database handle is required")
	}
	if r.now == nil {
		r.now = time.Now
	}
	record, err := normalizeRunRecord(record, r.now)
	if err != nil {
		return RunRecord{}, err
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO dav_sync_retention_runs (
  id, created_at, cutoff, limit_count, dry_run, confirm_ready, status, error_message,
  caldav_candidate_count, caldav_deleted_count, carddav_candidate_count, carddav_deleted_count
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		record.ID,
		record.CreatedAt,
		record.Cutoff,
		record.Limit,
		record.DryRun,
		record.ConfirmReady,
		string(record.Status),
		record.ErrorMessage,
		record.CalDAVCandidates,
		record.CalDAVDeleted,
		record.CardDAVCandidates,
		record.CardDAVDeleted,
	)
	if err != nil {
		return RunRecord{}, fmt.Errorf("record DAV sync retention run: %w", err)
	}
	return record, nil
}

func (r *Repository) ListRuns(ctx context.Context, req RunListRequest) ([]RunRecord, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := normalizeRunListRequest(req)
	if err != nil {
		return nil, err
	}
	query := `
SELECT id, created_at, cutoff, limit_count, dry_run, confirm_ready, status, error_message,
  caldav_candidate_count, caldav_deleted_count, carddav_candidate_count, carddav_deleted_count
FROM dav_sync_retention_runs`
	var conditions []string
	var args []any
	if req.Status != "" {
		args = append(args, string(req.Status))
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if !req.CreatedFrom.IsZero() {
		args = append(args, req.CreatedFrom.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if !req.CreatedTo.IsZero() {
		args = append(args, req.CreatedTo.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at < $%d", len(args)))
	}
	if len(conditions) > 0 {
		query += "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}
	args = append(args, req.Limit)
	query += fmt.Sprintf(`
ORDER BY created_at DESC, id DESC
LIMIT $%d`, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list DAV sync retention runs: %w", err)
	}
	defer rows.Close()

	var runs []RunRecord
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate DAV sync retention runs: %w", err)
	}
	return runs, nil
}

func (r *Repository) GetRun(ctx context.Context, id string) (RunRecord, error) {
	if r.db == nil {
		return RunRecord{}, fmt.Errorf("database handle is required")
	}
	id, err := validateRunID(id)
	if err != nil {
		return RunRecord{}, err
	}
	const query = `
SELECT id, created_at, cutoff, limit_count, dry_run, confirm_ready, status, error_message,
  caldav_candidate_count, caldav_deleted_count, carddav_candidate_count, carddav_deleted_count
FROM dav_sync_retention_runs
WHERE id = $1`
	run, err := scanRun(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RunRecord{}, fmt.Errorf("DAV sync retention run not found")
		}
		return RunRecord{}, fmt.Errorf("get DAV sync retention run: %w", err)
	}
	return run, nil
}

func normalizeRunRecord(record RunRecord, now func() time.Time) (RunRecord, error) {
	if record.Cutoff.IsZero() {
		return RunRecord{}, fmt.Errorf("cutoff is required")
	}
	record.Cutoff = record.Cutoff.UTC()
	if record.Limit <= 0 {
		return RunRecord{}, fmt.Errorf("limit must be positive")
	}
	if record.CalDAVCandidates < 0 || record.CalDAVDeleted < 0 || record.CardDAVCandidates < 0 || record.CardDAVDeleted < 0 {
		return RunRecord{}, fmt.Errorf("DAV sync retention counts must not be negative")
	}
	if record.Status == "" {
		record.Status = RunStatusCompleted
	}
	if record.Status != RunStatusCompleted && record.Status != RunStatusFailed {
		return RunRecord{}, fmt.Errorf("DAV sync retention status is unsupported")
	}
	record.ErrorMessage = cleanRunErrorMessage(record.ErrorMessage)
	if record.Status == RunStatusCompleted {
		record.ErrorMessage = ""
	}
	if record.ID == "" {
		id, err := newRunID()
		if err != nil {
			return RunRecord{}, err
		}
		record.ID = id
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now().UTC()
	} else {
		record.CreatedAt = record.CreatedAt.UTC()
	}
	return record, nil
}

func normalizeRunListRequest(req RunListRequest) (RunListRequest, error) {
	if req.Limit < 0 {
		return RunListRequest{}, fmt.Errorf("limit must not be negative")
	}
	if req.Limit == 0 || req.Limit > MaxRunListLimit {
		req.Limit = MaxRunListLimit
	}
	if req.Status != "" && req.Status != RunStatusCompleted && req.Status != RunStatusFailed {
		return RunListRequest{}, fmt.Errorf("DAV sync retention status is unsupported")
	}
	if !req.CreatedFrom.IsZero() {
		req.CreatedFrom = req.CreatedFrom.UTC()
	}
	if !req.CreatedTo.IsZero() {
		req.CreatedTo = req.CreatedTo.UTC()
	}
	if !req.CreatedFrom.IsZero() && !req.CreatedTo.IsZero() && !req.CreatedFrom.Before(req.CreatedTo) {
		return RunListRequest{}, fmt.Errorf("created_from must be before created_to")
	}
	return req, nil
}

func validateRunID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("DAV sync retention run id is required")
	}
	if len(id) > maxRunIDBytes {
		return "", fmt.Errorf("DAV sync retention run id is too long")
	}
	if strings.ContainsAny(id, "\r\n\t") {
		return "", fmt.Errorf("DAV sync retention run id must not contain control whitespace")
	}
	return id, nil
}

func cleanRunErrorMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	message = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, message)
	message = strings.Join(strings.Fields(message), " ")
	if len(message) <= maxRunErrorMessageBytes {
		return message
	}
	var b strings.Builder
	for _, r := range message {
		if b.Len()+len(string(r)) > maxRunErrorMessageBytes {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

type runScanner interface {
	Scan(...any) error
}

func scanRun(scanner runScanner) (RunRecord, error) {
	var run RunRecord
	var status string
	if err := scanner.Scan(
		&run.ID,
		&run.CreatedAt,
		&run.Cutoff,
		&run.Limit,
		&run.DryRun,
		&run.ConfirmReady,
		&status,
		&run.ErrorMessage,
		&run.CalDAVCandidates,
		&run.CalDAVDeleted,
		&run.CardDAVCandidates,
		&run.CardDAVDeleted,
	); err != nil {
		return RunRecord{}, fmt.Errorf("scan DAV sync retention run: %w", err)
	}
	run.Status = RunStatus(status)
	if run.Status != RunStatusCompleted && run.Status != RunStatusFailed {
		return RunRecord{}, fmt.Errorf("scan DAV sync retention run: unsupported status")
	}
	run.CreatedAt = run.CreatedAt.UTC()
	run.Cutoff = run.Cutoff.UTC()
	return run, nil
}

func newRunID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate DAV sync retention run id: %w", err)
	}
	return "dav-sync-retention-" + hex.EncodeToString(random[:]), nil
}
