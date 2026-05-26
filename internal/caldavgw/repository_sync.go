package caldavgw

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (r *Repository) ListCalendarChangesSince(ctx context.Context, req ListChangesSinceRequest) ([]CalendarChange, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateListChangesSinceRequest(req)
	if err != nil {
		return nil, err
	}
	const markerQuery = `
SELECT id
FROM caldav_calendar_sync_changes
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND sync_token = $3`
	var markerID int64
	if err := r.db.QueryRowContext(ctx, markerQuery, req.UserID, req.CalendarID, req.SyncToken).Scan(&markerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, InvalidSyncTokenError{Token: req.SyncToken}
		}
		return nil, fmt.Errorf("get CalDAV sync marker: %w", err)
	}
	const query = `
SELECT c.id, c.user_id::text, c.calendar_id::text, c.object_name, c.etag, c.action, c.sync_token, c.changed_at
FROM caldav_calendar_sync_changes c
WHERE c.user_id = $1::uuid
  AND c.calendar_id = $2::uuid
  AND c.id > $3
ORDER BY c.id ASC
LIMIT $4`
	rows, err := r.db.QueryContext(ctx, query, req.UserID, req.CalendarID, markerID, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list CalDAV sync changes: %w", err)
	}
	defer rows.Close()
	changes := make([]CalendarChange, 0, req.Limit)
	for rows.Next() {
		var change CalendarChange
		if err := rows.Scan(&change.ID, &change.UserID, &change.CalendarID, &change.ObjectName, &change.ETag, &change.Action, &change.SyncToken, &change.ChangedAt); err != nil {
			return nil, fmt.Errorf("scan CalDAV sync change: %w", err)
		}
		changes = append(changes, change)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate CalDAV sync changes: %w", err)
	}
	return changes, nil
}

func (r *Repository) ListCalendarChangesWithObjectsSince(ctx context.Context, req ListChangesSinceRequest, includeICS bool) ([]CalendarChangeWithObject, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateListChangesSinceRequest(req)
	if err != nil {
		return nil, err
	}
	const markerQuery = `
SELECT id
FROM caldav_calendar_sync_changes
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND sync_token = $3`
	var markerID int64
	if err := r.db.QueryRowContext(ctx, markerQuery, req.UserID, req.CalendarID, req.SyncToken).Scan(&markerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, InvalidSyncTokenError{Token: req.SyncToken}
		}
		return nil, fmt.Errorf("get CalDAV sync marker: %w", err)
	}

	var (
		query         string
		objectICSExpr string
	)
	if includeICS {
		objectICSExpr = "o.ics AS object_ics"
	} else {
		objectICSExpr = "NULL::text AS object_ics"
	}
	query = `
SELECT
  c.id,
  c.user_id::text,
  c.calendar_id::text,
  c.object_name,
  c.etag,
  c.action,
  c.sync_token,
  c.changed_at,
  o.id::text AS object_id,
  o.user_id::text AS object_user_id,
  o.calendar_id::text AS object_calendar_id,
  o.object_name AS object_object_name,
  o.uid AS object_uid,
  o.component_type AS object_component_type,
  o.etag AS object_etag,
  o.size AS object_size,
  ` + objectICSExpr + `,
  o.created_at AS object_created_at,
  o.updated_at AS object_updated_at
FROM caldav_calendar_sync_changes c
LEFT JOIN caldav_calendar_objects o
  ON o.user_id = c.user_id
 AND o.calendar_id = c.calendar_id
 AND o.object_name = c.object_name
 AND o.status = 'active'
WHERE c.user_id = $1::uuid
  AND c.calendar_id = $2::uuid
  AND c.id > $3
ORDER BY c.id ASC
LIMIT $4`
	rows, err := r.db.QueryContext(ctx, query, req.UserID, req.CalendarID, markerID, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("get CalDAV sync changes with objects: %w", err)
	}
	defer rows.Close()

	changes := make([]CalendarChangeWithObject, 0, req.Limit)
	for rows.Next() {
		var item CalendarChangeWithObject
		var (
			objectID         sql.NullString
			objectUserID     sql.NullString
			objectCalendarID sql.NullString
			objectObjectName sql.NullString
			objectUID        sql.NullString
			objectComponent  sql.NullString
			objectETag       sql.NullString
			objectSize       sql.NullInt64
			objectICS        sql.NullString
			objectCreatedAt  sql.NullTime
			objectUpdatedAt  sql.NullTime
		)
		if err := rows.Scan(
			&item.Change.ID,
			&item.Change.UserID,
			&item.Change.CalendarID,
			&item.Change.ObjectName,
			&item.Change.ETag,
			&item.Change.Action,
			&item.Change.SyncToken,
			&item.Change.ChangedAt,
			&objectID,
			&objectUserID,
			&objectCalendarID,
			&objectObjectName,
			&objectUID,
			&objectComponent,
			&objectETag,
			&objectSize,
			&objectICS,
			&objectCreatedAt,
			&objectUpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan CalDAV sync change object: %w", err)
		}
		if objectID.Valid {
			item.HasObject = true
			item.Object.ID = objectID.String
			item.Object.UserID = objectUserID.String
			item.Object.CalendarID = objectCalendarID.String
			item.Object.ObjectName = objectObjectName.String
			item.Object.UID = objectUID.String
			item.Object.Component = objectComponent.String
			item.Object.ETag = objectETag.String
			if objectSize.Valid {
				item.Object.Size = objectSize.Int64
			}
			if objectICS.Valid {
				item.Object.ICS = []byte(objectICS.String)
			}
			if objectCreatedAt.Valid {
				item.Object.CreatedAt = objectCreatedAt.Time
			}
			if objectUpdatedAt.Valid {
				item.Object.UpdatedAt = objectUpdatedAt.Time
			}
		}
		changes = append(changes, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate CalDAV sync changes with objects: %w", err)
	}
	return changes, nil
}

func (r *Repository) PruneCalendarSyncChanges(ctx context.Context, req PruneCalendarSyncChangesRequest) (CalendarSyncChangePruneResult, error) {
	if r == nil || r.db == nil {
		return CalendarSyncChangePruneResult{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidatePruneCalendarSyncChangesRequest(req)
	if err != nil {
		return CalendarSyncChangePruneResult{}, err
	}
	result := CalendarSyncChangePruneResult{
		Cutoff:     req.Cutoff,
		UserID:     req.UserID,
		CalendarID: req.CalendarID,
		Limit:      req.Limit,
		DryRun:     req.DryRun,
	}
	if req.DryRun {
		query, args := buildPruneCalendarSyncChangesSQL(req, true)
		if err := r.db.QueryRowContext(ctx, query, args...).Scan(&result.CandidateCount); err != nil {
			return CalendarSyncChangePruneResult{}, fmt.Errorf("check CalDAV sync change prune candidates: %w", err)
		}
		return result, nil
	}
	query, args := buildPruneCalendarSyncChangesSQL(req, false)
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&result.CandidateCount, &result.DeletedCount); err != nil {
		return CalendarSyncChangePruneResult{}, fmt.Errorf("prune CalDAV sync changes: %w", err)
	}
	return result, nil
}

func buildPruneCalendarSyncChangesSQL(req PruneCalendarSyncChangesRequest, dryRun bool) (string, []any) {
	args := []any{req.Cutoff}
	where := []string{"c.changed_at < $1"}
	if req.UserID != "" {
		args = append(args, req.UserID)
		where = append(where, fmt.Sprintf("c.user_id = $%d::uuid", len(args)))
	}
	if req.CalendarID != "" {
		args = append(args, req.CalendarID)
		where = append(where, fmt.Sprintf("c.calendar_id = $%d::uuid", len(args)))
	}
	args = append(args, req.Limit)
	limitParam := len(args)

	query := fmt.Sprintf(`
WITH candidates AS (
  SELECT c.id
  FROM caldav_calendar_sync_changes c
  WHERE %s
    AND EXISTS (
      SELECT 1
      FROM caldav_calendar_sync_changes newer
      WHERE newer.calendar_id = c.calendar_id
        AND newer.id > c.id
    )
    AND NOT EXISTS (
      SELECT 1
      FROM caldav_calendars cal
      WHERE cal.id = c.calendar_id
        AND cal.sync_token = c.sync_token
    )
  ORDER BY c.id ASC
  LIMIT $%d
)`, strings.Join(where, "\n    AND "), limitParam)
	if dryRun {
		return query + `
SELECT count(*)::bigint FROM candidates`, args
	}
	return query + `,
deleted AS (
  DELETE FROM caldav_calendar_sync_changes c
  USING candidates
  WHERE c.id = candidates.id
  RETURNING c.id
)
SELECT (SELECT count(*)::bigint FROM candidates), (SELECT count(*)::bigint FROM deleted)`, args
}

func insertCalendarSyncChange(ctx context.Context, execer syncChangeExecer, userID string, actorUserID string, calendarID string, syncToken string, action string, objectName string, etag string) error {
	_, err := execer.ExecContext(ctx, `
INSERT INTO caldav_calendar_sync_changes (
  user_id, calendar_id, sync_token, action, object_name, etag
) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)`, userID, calendarID, syncToken, action, objectName, etag)
	if err != nil {
		return fmt.Errorf("insert CalDAV sync change: %w", err)
	}
	if err := insertCalendarChangeOutbox(ctx, execer, userID, actorUserID, calendarID, syncToken, action, objectName, etag); err != nil {
		return err
	}
	return nil
}

func insertCalendarChangeOutbox(ctx context.Context, execer syncChangeExecer, userID string, actorUserID string, calendarID string, syncToken string, action string, objectName string, etag string) error {
	ownerUserID := strings.TrimSpace(userID)
	actorUserID = strings.TrimSpace(actorUserID)
	if actorUserID == "" {
		actorUserID = ownerUserID
	}
	payload, err := json.Marshal(map[string]any{
		"event":          calendarChangedEvent,
		"schema_version": davChangeSchemaVersion,
		"dav_kind":       davChangeKindCalDAV,
		"action":         action,
		"user_id":        ownerUserID,
		"owner_user_id":  ownerUserID,
		"actor_user_id":  actorUserID,
		"delegated":      actorUserID != "" && actorUserID != ownerUserID,
		"collection_id":  calendarID,
		"object_name":    objectName,
		"etag":           etag,
		"sync_token":     syncToken,
		"changed_at":     time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("marshal CalDAV change event: %w", err)
	}
	partitionKey := ownerUserID
	if partitionKey == "" {
		partitionKey = davChangePartitionFallback
	}
	_, err = execer.ExecContext(ctx, `
INSERT INTO outbox (topic, partition_key, payload, status)
VALUES ($1, $2, $3::jsonb, 'pending')`, davChangeOutboxTopic, partitionKey, string(payload))
	if err != nil {
		return fmt.Errorf("insert CalDAV change outbox event: %w", err)
	}
	return nil
}

func ValidateListChangesSinceRequest(req ListChangesSinceRequest) (ListChangesSinceRequest, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return ListChangesSinceRequest{}, err
	}
	calendarID, err := validateCalDAVID("calendar_id", req.CalendarID, true)
	if err != nil {
		return ListChangesSinceRequest{}, err
	}
	syncToken := strings.TrimSpace(req.SyncToken)
	if syncToken == "" {
		return ListChangesSinceRequest{}, fmt.Errorf("sync token is required")
	}
	if len(syncToken) > 128 || strings.ContainsAny(syncToken, "\r\n") {
		return ListChangesSinceRequest{}, fmt.Errorf("sync token is invalid")
	}
	return ListChangesSinceRequest{UserID: userID, CalendarID: calendarID, SyncToken: syncToken, Limit: normalizeCalDAVChangeLimit(req.Limit)}, nil
}

func ValidatePruneCalendarSyncChangesRequest(req PruneCalendarSyncChangesRequest) (PruneCalendarSyncChangesRequest, error) {
	if req.Cutoff.IsZero() {
		return PruneCalendarSyncChangesRequest{}, fmt.Errorf("cutoff is required")
	}
	cutoff := req.Cutoff.UTC()
	if cutoff.After(time.Now().UTC()) {
		return PruneCalendarSyncChangesRequest{}, fmt.Errorf("cutoff must not be in the future")
	}
	userID, err := validateCalDAVID("user_id", req.UserID, false)
	if err != nil {
		return PruneCalendarSyncChangesRequest{}, err
	}
	calendarID, err := validateCalDAVID("calendar_id", req.CalendarID, false)
	if err != nil {
		return PruneCalendarSyncChangesRequest{}, err
	}
	return PruneCalendarSyncChangesRequest{
		Cutoff:     cutoff,
		UserID:     userID,
		CalendarID: calendarID,
		Limit:      normalizeCalDAVChangeLimit(req.Limit),
		DryRun:     req.DryRun,
	}, nil
}
