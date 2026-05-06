package davsyncretention

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const maxRunErrorMessageBytes = 1024

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

func newRunID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate DAV sync retention run id: %w", err)
	}
	return "dav-sync-retention-" + hex.EncodeToString(random[:]), nil
}
