package maildb

import (
	"time"

	"github.com/google/uuid"
)

// LDAPSyncRunView represents a single LDAP sync operation.
type LDAPSyncRunView struct {
	ID                 uuid.UUID  `json:"id"`
	DomainID           uuid.UUID  `json:"domain_id"`
	SyncType           string     `json:"sync_type"` // 'users', 'groups', 'memberships'
	Status             string     `json:"status"`    // 'running', 'success', 'failed', 'partial'
	CreatedCount       int        `json:"created_count"`
	UpdatedCount       int        `json:"updated_count"`
	DeletedCount       int        `json:"deleted_count"`
	ConflictCount      int        `json:"conflict_count"`
	ErrorCount         int        `json:"error_count"`
	ResolutionStrategy *string    `json:"resolution_strategy"`
	StartedAt          time.Time  `json:"started_at"`
	CompletedAt        *time.Time `json:"completed_at"`
	LastSuccessAt      *time.Time `json:"last_success_at"`
	DurationMs         *int       `json:"duration_ms"`
	ErrorMessage       *string    `json:"error_message"`
}

// LDAPSyncRunListRequest filters LDAP sync runs.
type LDAPSyncRunListRequest struct {
	DomainID string
	Status   string
	Limit    int
	Offset   int
}

// LDAPSyncConflictView represents a conflict detected during sync.
type LDAPSyncConflictView struct {
	ID              uuid.UUID  `json:"id"`
	DomainID        uuid.UUID  `json:"domain_id"`
	SyncRunID       uuid.UUID  `json:"sync_run_id"`
	ObjectType      string     `json:"object_type"` // 'user', 'group'
	ObjectID        *uuid.UUID `json:"object_id"`
	LDAPDN          string     `json:"ldap_dn"`
	ConflictType    string     `json:"conflict_type"` // 'duplicate_key', 'missing_mapping', 'attr_mismatch'
	LocalValue      *string    `json:"local_value"`
	LDAPValue       *string    `json:"ldap_value"`
	Resolution      *string    `json:"resolution"`
	ResolvedAt      *time.Time `json:"resolved_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

// LDAPSyncConflictListRequest filters LDAP sync conflicts.
type LDAPSyncConflictListRequest struct {
	DomainID      string
	SyncRunID     string
	UnresolvedOnly bool
	Limit         int
	Offset        int
}

// LDAPSyncConflictResolutionRequest resolves a conflict.
type LDAPSyncConflictResolutionRequest struct {
	ConflictID string // UUID
	Resolution string // 'prefer_local', 'prefer_ldap', 'manual'
}
