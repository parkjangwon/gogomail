// Package auditretention implements per-company audit log retention policies
// and the batched purge machinery that enforces them.
//
// A company with no policy row keeps its audit logs forever (the default,
// preserving existing behavior). When a policy row exists, audit log entries
// older than retention_days are purged in bounded batches by a background
// worker, and each purge run is recorded for operator visibility.
package auditretention

import (
	"fmt"
	"strings"
	"time"
)

// AllowedRetentionDays is the sane set of retention windows an admin may
// configure. 0 is also accepted at the API boundary and means "keep forever"
// (delete the policy row); it is intentionally not part of this set because a
// stored policy row always carries a positive retention window.
var AllowedRetentionDays = []int{30, 90, 180, 365, 730}

// RetentionForever is the sentinel a caller supplies to disable retention
// (removing any stored policy so logs are kept indefinitely).
const RetentionForever = 0

// Policy is a per-company audit log retention policy.
type Policy struct {
	CompanyID     string    `json:"company_id"`
	RetentionDays int       `json:"retention_days"`
	UpdatedBy     string    `json:"updated_by,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ValidateRetentionDays reports whether days is an accepted retention window.
// RetentionForever (0) is accepted and signals policy removal; any positive
// value must be a member of AllowedRetentionDays.
func ValidateRetentionDays(days int) error {
	if days == RetentionForever {
		return nil
	}
	for _, allowed := range AllowedRetentionDays {
		if days == allowed {
			return nil
		}
	}
	return fmt.Errorf("retention_days must be one of %v or 0 (forever), got %d", AllowedRetentionDays, days)
}

// RunStatus is the terminal state of a purge run.
type RunStatus string

const (
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
)

const (
	maxRunErrorMessageBytes = 1024
	maxRunIDBytes           = 128
	// MaxRunListLimit bounds how many runs a single list call returns.
	MaxRunListLimit = 100
	// DefaultBatchSize is the number of rows deleted per batch when none is set.
	DefaultBatchSize = 10000
)

// RunRecord captures the outcome of a single per-company purge run.
type RunRecord struct {
	ID            string    `json:"id"`
	CompanyID     string    `json:"company_id"`
	RetentionDays int       `json:"retention_days"`
	Cutoff        time.Time `json:"cutoff"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at"`
	DryRun        bool      `json:"dry_run"`
	Status        RunStatus `json:"status"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	RowsDeleted   int64     `json:"rows_deleted"`
}

// RunListRequest filters a run listing for a single company.
type RunListRequest struct {
	CompanyID string
	Limit     int
	Status    RunStatus
}

func normalizeRunListRequest(req RunListRequest) (RunListRequest, error) {
	req.CompanyID = strings.TrimSpace(req.CompanyID)
	if req.CompanyID == "" {
		return RunListRequest{}, fmt.Errorf("company id is required")
	}
	if req.Limit < 0 {
		return RunListRequest{}, fmt.Errorf("limit must not be negative")
	}
	if req.Limit == 0 || req.Limit > MaxRunListLimit {
		req.Limit = MaxRunListLimit
	}
	if req.Status != "" && req.Status != RunStatusCompleted && req.Status != RunStatusFailed {
		return RunListRequest{}, fmt.Errorf("audit retention status is unsupported")
	}
	return req, nil
}

func normalizeRunRecord(record RunRecord, now func() time.Time) (RunRecord, error) {
	if now == nil {
		now = time.Now
	}
	record.CompanyID = strings.TrimSpace(record.CompanyID)
	if record.CompanyID == "" {
		return RunRecord{}, fmt.Errorf("company id is required")
	}
	if record.RetentionDays <= 0 {
		return RunRecord{}, fmt.Errorf("retention days must be positive")
	}
	if record.Cutoff.IsZero() {
		return RunRecord{}, fmt.Errorf("cutoff is required")
	}
	record.Cutoff = record.Cutoff.UTC()
	if record.RowsDeleted < 0 {
		return RunRecord{}, fmt.Errorf("rows deleted must not be negative")
	}
	if record.Status == "" {
		record.Status = RunStatusCompleted
	}
	if record.Status != RunStatusCompleted && record.Status != RunStatusFailed {
		return RunRecord{}, fmt.Errorf("audit retention status is unsupported")
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
	if record.StartedAt.IsZero() {
		record.StartedAt = now().UTC()
	} else {
		record.StartedAt = record.StartedAt.UTC()
	}
	if record.FinishedAt.IsZero() {
		record.FinishedAt = now().UTC()
	} else {
		record.FinishedAt = record.FinishedAt.UTC()
	}
	return record, nil
}

func validateRunID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("audit retention run id is required")
	}
	if len(id) > maxRunIDBytes {
		return "", fmt.Errorf("audit retention run id is too long")
	}
	if strings.ContainsAny(id, "\r\n\t") {
		return "", fmt.Errorf("audit retention run id must not contain control whitespace")
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
