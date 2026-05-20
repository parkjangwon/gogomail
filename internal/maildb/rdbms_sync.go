package maildb

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RDBMSSyncRunView represents a single RDBMS sync run in the database.
type RDBMSSyncRunView struct {
	ID            uuid.UUID  `json:"id"`
	DomainID      uuid.UUID  `json:"domain_id"`
	SyncType      string     `json:"sync_type"`
	Status        string     `json:"status"` // pending, running, success, failed, partial
	UsersCreated  int        `json:"users_created"`
	UsersUpdated  int        `json:"users_updated"`
	UsersDeleted  int        `json:"users_deleted"`
	GroupsCreated int        `json:"groups_created"`
	GroupsUpdated int        `json:"groups_updated"`
	GroupsDeleted int        `json:"groups_deleted"`
	ConflictCount int        `json:"conflict_count"`
	ErrorCount    int        `json:"error_count"`
	ErrorMessage  *string    `json:"error_message"`
	StartedAt     time.Time  `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

// RDBMSSyncConflictView represents an unresolved RDBMS sync conflict.
type RDBMSSyncConflictView struct {
	ID           uuid.UUID  `json:"id"`
	DomainID     uuid.UUID  `json:"domain_id"`
	SyncRunID    uuid.UUID  `json:"sync_run_id"`
	ConflictType string     `json:"conflict_type"` // duplicate_key, schema_mismatch, etc.
	LocalData    string     `json:"local_data"`    // JSON representation of local entity
	RemoteData   string     `json:"remote_data"`   // JSON representation of RDBMS entity
	Resolution   *string    `json:"resolution"`    // prefer_local, prefer_rdbms, manual
	ResolvedAt   *time.Time `json:"resolved_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

// RDBMSSyncRunListRequest represents pagination parameters for listing RDBMS sync runs.
type RDBMSSyncRunListRequest struct {
	DomainID string
	Limit    int
	Offset   int
	Cursor   RDBMSSyncRunCursor
}

// RDBMSSyncRunCursor is an opaque seek-pagination cursor for sync history.
type RDBMSSyncRunCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

// RDBMSSyncConflictListRequest represents filter parameters for listing RDBMS sync conflicts.
type RDBMSSyncConflictListRequest struct {
	DomainID       string
	UnresolvedOnly bool
	Limit          int
	Offset         int
}

func (c RDBMSSyncRunCursor) IsZero() bool {
	return c.CreatedAt.IsZero() || c.ID == uuid.Nil
}

func EncodeRDBMSSyncRunCursor(run RDBMSSyncRunView) (string, error) {
	if run.CreatedAt.IsZero() {
		return "", fmt.Errorf("sync run cursor timestamp is required")
	}
	if run.ID == uuid.Nil {
		return "", fmt.Errorf("sync run cursor id is required")
	}
	raw, err := json.Marshal(RDBMSSyncRunCursor{CreatedAt: run.CreatedAt, ID: run.ID})
	if err != nil {
		return "", fmt.Errorf("marshal sync run cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeRDBMSSyncRunCursor(value string) (RDBMSSyncRunCursor, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return RDBMSSyncRunCursor{}, nil
	}
	if len(value) > MessageListCursorMaxBytes {
		return RDBMSSyncRunCursor{}, fmt.Errorf("sync run cursor is too long")
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return RDBMSSyncRunCursor{}, fmt.Errorf("decode sync run cursor: %w", err)
	}
	var cursor RDBMSSyncRunCursor
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return RDBMSSyncRunCursor{}, fmt.Errorf("unmarshal sync run cursor: %w", err)
	}
	if cursor.CreatedAt.IsZero() {
		return RDBMSSyncRunCursor{}, fmt.Errorf("sync run cursor timestamp is required")
	}
	if cursor.ID == uuid.Nil {
		return RDBMSSyncRunCursor{}, fmt.Errorf("sync run cursor id is required")
	}
	return cursor, nil
}
