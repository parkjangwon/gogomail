package maildb

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
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
	ID           uuid.UUID  `json:"id"`
	DomainID     uuid.UUID  `json:"domain_id"`
	SyncRunID    uuid.UUID  `json:"sync_run_id"`
	ObjectType   string     `json:"object_type"` // 'user', 'group'
	ObjectID     *uuid.UUID `json:"object_id"`
	LDAPDN       string     `json:"ldap_dn"`
	ConflictType string     `json:"conflict_type"` // 'duplicate_key', 'missing_mapping', 'attr_mismatch'
	LocalValue   *string    `json:"local_value"`
	LDAPValue    *string    `json:"ldap_value"`
	Resolution   *string    `json:"resolution"`
	ResolvedAt   *time.Time `json:"resolved_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

// LDAPSyncConflictListRequest filters LDAP sync conflicts.
type LDAPSyncConflictListRequest struct {
	DomainID       string
	SyncRunID      string
	UnresolvedOnly bool
	Limit          int
	Offset         int
	Cursor         LDAPSyncConflictCursor
}

// LDAPSyncConflictCursor is an opaque seek-pagination cursor for LDAP sync conflicts.
type LDAPSyncConflictCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

// LDAPSyncConflictResolutionRequest resolves a conflict.
type LDAPSyncConflictResolutionRequest struct {
	ConflictID string // UUID
	Resolution string // 'prefer_local', 'prefer_ldap', 'manual'
}

func (c LDAPSyncConflictCursor) IsZero() bool {
	return c.CreatedAt.IsZero() || c.ID == uuid.Nil
}

func EncodeLDAPSyncConflictCursor(conflict LDAPSyncConflictView) (string, error) {
	if conflict.CreatedAt.IsZero() {
		return "", fmt.Errorf("ldap sync conflict cursor timestamp is required")
	}
	if conflict.ID == uuid.Nil {
		return "", fmt.Errorf("ldap sync conflict cursor id is required")
	}
	raw, err := json.Marshal(LDAPSyncConflictCursor{CreatedAt: conflict.CreatedAt, ID: conflict.ID})
	if err != nil {
		return "", fmt.Errorf("marshal ldap sync conflict cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeLDAPSyncConflictCursor(value string) (LDAPSyncConflictCursor, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return LDAPSyncConflictCursor{}, nil
	}
	if len(value) > MessageListCursorMaxBytes {
		return LDAPSyncConflictCursor{}, fmt.Errorf("ldap sync conflict cursor is too long")
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return LDAPSyncConflictCursor{}, fmt.Errorf("decode ldap sync conflict cursor: %w", err)
	}
	var cursor LDAPSyncConflictCursor
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return LDAPSyncConflictCursor{}, fmt.Errorf("unmarshal ldap sync conflict cursor: %w", err)
	}
	if cursor.CreatedAt.IsZero() {
		return LDAPSyncConflictCursor{}, fmt.Errorf("ldap sync conflict cursor timestamp is required")
	}
	if cursor.ID == uuid.Nil {
		return LDAPSyncConflictCursor{}, fmt.Errorf("ldap sync conflict cursor id is required")
	}
	return cursor, nil
}
