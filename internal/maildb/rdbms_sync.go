package maildb

import (
	"time"

	"github.com/google/uuid"
)

// RDBMSSyncRunView represents a single RDBMS sync run in the database.
type RDBMSSyncRunView struct {
	ID             uuid.UUID  `json:"id"`
	DomainID       uuid.UUID  `json:"domain_id"`
	SyncType       string     `json:"sync_type"`
	Status         string     `json:"status"` // pending, running, success, failed, partial
	UsersCreated   int        `json:"users_created"`
	UsersUpdated   int        `json:"users_updated"`
	UsersDeleted   int        `json:"users_deleted"`
	GroupsCreated  int        `json:"groups_created"`
	GroupsUpdated  int        `json:"groups_updated"`
	GroupsDeleted  int        `json:"groups_deleted"`
	ConflictCount  int        `json:"conflict_count"`
	ErrorCount     int        `json:"error_count"`
	ErrorMessage   *string    `json:"error_message"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at"`
	CreatedAt      time.Time  `json:"created_at"`
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
}

// RDBMSSyncConflictListRequest represents filter parameters for listing RDBMS sync conflicts.
type RDBMSSyncConflictListRequest struct {
	DomainID       string
	UnresolvedOnly bool
	Limit          int
	Offset         int
}
