package caldavgw

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

type CreateCalendarRequest struct {
	UserID      string
	ActorUserID string
	Name        string
	Color       string
	Description string
}

type CreateCalendarAtPathRequest struct {
	UserID      string
	ActorUserID string
	CalendarID  string
	Name        string
	Color       string
	Description string
}

type ListCalendarsRequest struct {
	UserID string
	Status string
	Limit  int
}

type GetCalendarRequest struct {
	UserID     string
	CalendarID string
	Status     string
}

type UpsertObjectRequest struct {
	UserID       string
	ActorUserID  string
	CalendarID   string
	ObjectName   string
	UID          string
	Component    string
	ICS          []byte
	ObservedETag string
}

type ListObjectsRequest struct {
	UserID     string
	CalendarID string
	Status     string
	Limit      int
}

type GetObjectRequest struct {
	UserID     string
	CalendarID string
	ObjectName string
	Status     string
}

type DeleteObjectRequest struct {
	UserID      string
	ActorUserID string
	CalendarID  string
	ObjectName  string
}

type DeleteCalendarRequest struct {
	UserID      string
	ActorUserID string
	CalendarID  string
}

type UpdateCalendarRequest struct {
	UserID      string
	ActorUserID string
	CalendarID  string
	Name        *string
	Color       *string
	Description *string
}

type ListChangesSinceRequest struct {
	UserID     string
	CalendarID string
	SyncToken  string
	Limit      int
}

type PruneCalendarSyncChangesRequest struct {
	Cutoff     time.Time
	UserID     string
	CalendarID string
	Limit      int
	DryRun     bool
}

type CalendarSyncChangePruneResult struct {
	Cutoff         time.Time
	UserID         string
	CalendarID     string
	Limit          int
	DryRun         bool
	CandidateCount int64
	DeletedCount   int64
}

func (r *Repository) CreateCalendar(ctx context.Context, req CreateCalendarRequest) (Calendar, error) {
	if r == nil || r.db == nil {
		return Calendar{}, fmt.Errorf("database handle is required")
	}
	req, normalizedName, syncToken, err := ValidateCreateCalendarRequest(req)
	if err != nil {
		return Calendar{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Calendar{}, fmt.Errorf("begin CalDAV calendar create: %w", err)
	}
	defer tx.Rollback()
	const query = `
WITH active_user AS (
  SELECT u.id AS user_id, d.id AS domain_id, c.id AS company_id
  FROM users u
  JOIN domains d ON d.id = u.domain_id
  JOIN companies c ON c.id = d.company_id
  WHERE u.id = $1::uuid
    AND u.status = 'active'
    AND d.status = 'active'
    AND c.status = 'active'
)
INSERT INTO caldav_calendars (
  company_id, domain_id, user_id, name, normalized_name, color, description, sync_token
)
SELECT company_id, domain_id, user_id, $2, $3, $4, $5, $6
FROM active_user
RETURNING id::text, user_id::text, name, color, description, sync_token, created_at, updated_at`
	var calendar Calendar
	err = tx.QueryRowContext(ctx, query,
		req.UserID,
		req.Name,
		normalizedName,
		req.Color,
		req.Description,
		syncToken,
	).Scan(
		&calendar.ID,
		&calendar.UserID,
		&calendar.Name,
		&calendar.Color,
		&calendar.Description,
		&calendar.SyncToken,
		&calendar.CreatedAt,
		&calendar.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Calendar{}, fmt.Errorf("active user not found")
		}
		return Calendar{}, fmt.Errorf("create CalDAV calendar: %w", err)
	}
	if err := insertCalendarSyncChange(ctx, tx, calendar.UserID, req.ActorUserID, calendar.ID, calendar.SyncToken, "collection-created", "", ""); err != nil {
		return Calendar{}, err
	}
	if err := tx.Commit(); err != nil {
		return Calendar{}, fmt.Errorf("commit CalDAV calendar create: %w", err)
	}
	return calendar, nil
}

func (r *Repository) CreateCalendarAtPath(ctx context.Context, req CreateCalendarAtPathRequest) (Calendar, error) {
	if r == nil || r.db == nil {
		return Calendar{}, fmt.Errorf("database handle is required")
	}
	req, normalizedName, syncToken, err := ValidateCreateCalendarAtPathRequest(req)
	if err != nil {
		return Calendar{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Calendar{}, fmt.Errorf("begin CalDAV calendar create: %w", err)
	}
	defer tx.Rollback()
	const query = `
WITH active_user AS (
  SELECT u.id AS user_id, d.id AS domain_id, c.id AS company_id
  FROM users u
  JOIN domains d ON d.id = u.domain_id
  JOIN companies c ON c.id = d.company_id
  WHERE u.id = $1::uuid
    AND u.status = 'active'
    AND d.status = 'active'
    AND c.status = 'active'
)
INSERT INTO caldav_calendars (
  id, company_id, domain_id, user_id, name, normalized_name, color, description, sync_token
)
SELECT $2::uuid, company_id, domain_id, user_id, $3, $4, $5, $6, $7
FROM active_user
RETURNING id::text, user_id::text, name, color, description, sync_token, created_at, updated_at`
	var calendar Calendar
	err = tx.QueryRowContext(ctx, query,
		req.UserID,
		req.CalendarID,
		req.Name,
		normalizedName,
		req.Color,
		req.Description,
		syncToken,
	).Scan(
		&calendar.ID,
		&calendar.UserID,
		&calendar.Name,
		&calendar.Color,
		&calendar.Description,
		&calendar.SyncToken,
		&calendar.CreatedAt,
		&calendar.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Calendar{}, fmt.Errorf("active user not found")
		}
		return Calendar{}, fmt.Errorf("create CalDAV calendar at path: %w", err)
	}
	if err := insertCalendarSyncChange(ctx, tx, calendar.UserID, req.ActorUserID, calendar.ID, calendar.SyncToken, "collection-created", "", ""); err != nil {
		return Calendar{}, err
	}
	if err := tx.Commit(); err != nil {
		return Calendar{}, fmt.Errorf("commit CalDAV calendar create: %w", err)
	}
	return calendar, nil
}

func (r *Repository) ListCalendars(ctx context.Context, req ListCalendarsRequest) ([]Calendar, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateListCalendarsRequest(req)
	if err != nil {
		return nil, err
	}
	const query = `
SELECT id::text, user_id::text, name, color, description, sync_token, created_at, updated_at
FROM caldav_calendars
WHERE user_id = $1::uuid
  AND status = $2
ORDER BY updated_at DESC, id DESC
LIMIT $3`
	rows, err := r.db.QueryContext(ctx, query, req.UserID, req.Status, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list CalDAV calendars: %w", err)
	}
	defer rows.Close()
	var calendars []Calendar
	for rows.Next() {
		var calendar Calendar
		if err := rows.Scan(&calendar.ID, &calendar.UserID, &calendar.Name, &calendar.Color, &calendar.Description, &calendar.SyncToken, &calendar.CreatedAt, &calendar.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan CalDAV calendar: %w", err)
		}
		calendars = append(calendars, calendar)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate CalDAV calendars: %w", err)
	}
	return calendars, nil
}

func (r *Repository) GetCalendar(ctx context.Context, req GetCalendarRequest) (Calendar, error) {
	if r == nil || r.db == nil {
		return Calendar{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateGetCalendarRequest(req)
	if err != nil {
		return Calendar{}, err
	}
	const query = `
SELECT id::text, user_id::text, name, color, description, sync_token, created_at, updated_at
FROM caldav_calendars
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = $3`
	var calendar Calendar
	err = r.db.QueryRowContext(ctx, query, req.UserID, req.CalendarID, req.Status).Scan(
		&calendar.ID,
		&calendar.UserID,
		&calendar.Name,
		&calendar.Color,
		&calendar.Description,
		&calendar.SyncToken,
		&calendar.CreatedAt,
		&calendar.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Calendar{}, fmt.Errorf("CalDAV calendar not found")
		}
		return Calendar{}, fmt.Errorf("get CalDAV calendar: %w", err)
	}
	return calendar, nil
}

func (r *Repository) UpsertObject(ctx context.Context, req UpsertObjectRequest) (CalendarObject, error) {
	if r == nil || r.db == nil {
		return CalendarObject{}, fmt.Errorf("database handle is required")
	}
	req, etag, syncToken, err := ValidateUpsertObjectRequest(req)
	if err != nil {
		return CalendarObject{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return CalendarObject{}, fmt.Errorf("begin CalDAV object upsert: %w", err)
	}
	defer tx.Rollback()
	if err := lockActiveCalendar(ctx, tx, req.UserID, req.CalendarID); err != nil {
		return CalendarObject{}, err
	}
	if err := ensureCalendarSyncMarker(ctx, tx, req.UserID, req.CalendarID); err != nil {
		return CalendarObject{}, err
	}
	if req.ObservedETag != "" {
		if err := ensureObjectETag(ctx, tx, req.UserID, req.CalendarID, req.ObjectName, req.ObservedETag); err != nil {
			return CalendarObject{}, err
		}
	}
	if err := ensureCalendarObjectUIDAvailable(ctx, tx, req.UserID, req.CalendarID, req.ObjectName, req.UID); err != nil {
		return CalendarObject{}, err
	}
	const query = `
INSERT INTO caldav_calendar_objects (
  user_id, calendar_id, object_name, uid, component_type, etag, size, ics
) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)
ON CONFLICT (calendar_id, object_name) WHERE status = 'active'
DO UPDATE SET
  uid = EXCLUDED.uid,
  component_type = EXCLUDED.component_type,
  etag = EXCLUDED.etag,
  size = EXCLUDED.size,
  ics = EXCLUDED.ics,
  updated_at = now()
RETURNING id::text, user_id::text, calendar_id::text, object_name, uid, component_type, etag, size, ics, created_at, updated_at`
	var object CalendarObject
	err = tx.QueryRowContext(ctx, query,
		req.UserID,
		req.CalendarID,
		req.ObjectName,
		req.UID,
		req.Component,
		etag,
		len(req.ICS),
		string(req.ICS),
	).Scan(
		&object.ID,
		&object.UserID,
		&object.CalendarID,
		&object.ObjectName,
		&object.UID,
		&object.Component,
		&object.ETag,
		&object.Size,
		&object.ICS,
		&object.CreatedAt,
		&object.UpdatedAt,
	)
	if err != nil {
		return CalendarObject{}, mapCalendarObjectUpsertError(err)
	}
	if err := updateCalendarSyncToken(ctx, tx, req.UserID, req.CalendarID, syncToken); err != nil {
		return CalendarObject{}, err
	}
	if err := insertCalendarSyncChange(ctx, tx, req.UserID, req.ActorUserID, req.CalendarID, syncToken, "object-upserted", req.ObjectName, etag); err != nil {
		return CalendarObject{}, err
	}
	if err := tx.Commit(); err != nil {
		return CalendarObject{}, fmt.Errorf("commit CalDAV object upsert: %w", err)
	}
	return object, nil
}

func (r *Repository) ListObjects(ctx context.Context, req ListObjectsRequest) ([]CalendarObject, error) {
	req, err := ValidateListObjectsRequest(req)
	if err != nil {
		return nil, err
	}
	return r.listObjects(ctx, req)
}

func (r *Repository) listObjectsForSync(ctx context.Context, req ListObjectsRequest) ([]CalendarObject, error) {
	req, err := ValidateListObjectsForSyncRequest(req)
	if err != nil {
		return nil, err
	}
	return r.listObjects(ctx, req)
}

func (r *Repository) listObjects(ctx context.Context, req ListObjectsRequest) ([]CalendarObject, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	const query = `
SELECT id::text, user_id::text, calendar_id::text, object_name, uid, component_type, etag, size, ics, created_at, updated_at
FROM caldav_calendar_objects
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND status = $3
ORDER BY updated_at DESC, id DESC
LIMIT $4`
	rows, err := r.db.QueryContext(ctx, query, req.UserID, req.CalendarID, req.Status, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list CalDAV objects: %w", err)
	}
	defer rows.Close()
	var objects []CalendarObject
	for rows.Next() {
		var object CalendarObject
		if err := rows.Scan(&object.ID, &object.UserID, &object.CalendarID, &object.ObjectName, &object.UID, &object.Component, &object.ETag, &object.Size, &object.ICS, &object.CreatedAt, &object.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan CalDAV object: %w", err)
		}
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate CalDAV objects: %w", err)
	}
	return objects, nil
}

func (r *Repository) GetObject(ctx context.Context, req GetObjectRequest) (CalendarObject, error) {
	if r == nil || r.db == nil {
		return CalendarObject{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateGetObjectRequest(req)
	if err != nil {
		return CalendarObject{}, err
	}
	const query = `
SELECT id::text, user_id::text, calendar_id::text, object_name, uid, component_type, etag, size, ics, created_at, updated_at
FROM caldav_calendar_objects
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND object_name = $3
  AND status = $4`
	var object CalendarObject
	err = r.db.QueryRowContext(ctx, query, req.UserID, req.CalendarID, req.ObjectName, req.Status).Scan(
		&object.ID,
		&object.UserID,
		&object.CalendarID,
		&object.ObjectName,
		&object.UID,
		&object.Component,
		&object.ETag,
		&object.Size,
		&object.ICS,
		&object.CreatedAt,
		&object.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CalendarObject{}, fmt.Errorf("CalDAV object not found")
		}
		return CalendarObject{}, fmt.Errorf("get CalDAV object: %w", err)
	}
	return object, nil
}

func (r *Repository) DeleteObject(ctx context.Context, req DeleteObjectRequest) (CalendarObject, error) {
	if r == nil || r.db == nil {
		return CalendarObject{}, fmt.Errorf("database handle is required")
	}
	req, syncToken, err := ValidateDeleteObjectRequest(req)
	if err != nil {
		return CalendarObject{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return CalendarObject{}, fmt.Errorf("begin CalDAV object delete: %w", err)
	}
	defer tx.Rollback()
	if err := lockActiveCalendar(ctx, tx, req.UserID, req.CalendarID); err != nil {
		return CalendarObject{}, err
	}
	if err := ensureCalendarSyncMarker(ctx, tx, req.UserID, req.CalendarID); err != nil {
		return CalendarObject{}, err
	}
	const query = `
UPDATE caldav_calendar_objects
SET status = 'deleted', deleted_at = now(), updated_at = now()
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND object_name = $3
  AND status = 'active'
RETURNING id::text, user_id::text, calendar_id::text, object_name, uid, component_type, etag, size, ics, created_at, updated_at`
	var object CalendarObject
	err = tx.QueryRowContext(ctx, query, req.UserID, req.CalendarID, req.ObjectName).Scan(
		&object.ID,
		&object.UserID,
		&object.CalendarID,
		&object.ObjectName,
		&object.UID,
		&object.Component,
		&object.ETag,
		&object.Size,
		&object.ICS,
		&object.CreatedAt,
		&object.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CalendarObject{}, fmt.Errorf("CalDAV object not found")
		}
		return CalendarObject{}, fmt.Errorf("delete CalDAV object: %w", err)
	}
	if err := updateCalendarSyncToken(ctx, tx, req.UserID, req.CalendarID, syncToken); err != nil {
		return CalendarObject{}, err
	}
	if err := insertCalendarSyncChange(ctx, tx, req.UserID, req.ActorUserID, req.CalendarID, syncToken, "object-deleted", req.ObjectName, object.ETag); err != nil {
		return CalendarObject{}, err
	}
	if err := tx.Commit(); err != nil {
		return CalendarObject{}, fmt.Errorf("commit CalDAV object delete: %w", err)
	}
	return object, nil
}

func (r *Repository) DeleteCalendar(ctx context.Context, req DeleteCalendarRequest) (Calendar, error) {
	if r == nil || r.db == nil {
		return Calendar{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateDeleteCalendarRequest(req)
	if err != nil {
		return Calendar{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Calendar{}, fmt.Errorf("begin CalDAV calendar delete: %w", err)
	}
	defer tx.Rollback()
	const query = `
UPDATE caldav_calendars
SET status = 'deleted', deleted_at = now(), updated_at = now()
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'
RETURNING id::text, user_id::text, name, color, description, sync_token, created_at, updated_at`
	var calendar Calendar
	err = tx.QueryRowContext(ctx, query, req.UserID, req.CalendarID).Scan(
		&calendar.ID,
		&calendar.UserID,
		&calendar.Name,
		&calendar.Color,
		&calendar.Description,
		&calendar.SyncToken,
		&calendar.CreatedAt,
		&calendar.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Calendar{}, fmt.Errorf("CalDAV calendar not found")
		}
		return Calendar{}, fmt.Errorf("delete CalDAV calendar: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE caldav_calendar_objects
SET status = 'deleted', deleted_at = COALESCE(deleted_at, now()), updated_at = now()
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND status = 'active'`, req.UserID, req.CalendarID); err != nil {
		return Calendar{}, fmt.Errorf("delete CalDAV calendar objects: %w", err)
	}
	syncToken := CalendarSyncToken(req.UserID, req.CalendarID, "collection-delete", time.Now().UTC().Format(time.RFC3339Nano))
	if err := insertCalendarSyncChange(ctx, tx, req.UserID, req.ActorUserID, req.CalendarID, syncToken, "collection-deleted", "", ""); err != nil {
		return Calendar{}, err
	}
	if err := tx.Commit(); err != nil {
		return Calendar{}, fmt.Errorf("commit CalDAV calendar delete: %w", err)
	}
	return calendar, nil
}

func (r *Repository) UpdateCalendarProperties(ctx context.Context, req UpdateCalendarRequest) (Calendar, error) {
	if r == nil || r.db == nil {
		return Calendar{}, fmt.Errorf("database handle is required")
	}
	req, normalizedName, syncToken, err := ValidateUpdateCalendarRequest(req)
	if err != nil {
		return Calendar{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Calendar{}, fmt.Errorf("begin CalDAV calendar update: %w", err)
	}
	defer tx.Rollback()
	if err := lockActiveCalendar(ctx, tx, req.UserID, req.CalendarID); err != nil {
		return Calendar{}, err
	}
	if err := ensureCalendarSyncMarker(ctx, tx, req.UserID, req.CalendarID); err != nil {
		return Calendar{}, err
	}
	nameValue, nameSet := optionalStringArg(req.Name)
	colorValue, colorSet := optionalStringArg(req.Color)
	descriptionValue, descriptionSet := optionalStringArg(req.Description)
	const query = `
UPDATE caldav_calendars
SET
  name = CASE WHEN $3 THEN $4 ELSE name END,
  normalized_name = CASE WHEN $3 THEN $5 ELSE normalized_name END,
  color = CASE WHEN $6 THEN $7 ELSE color END,
  description = CASE WHEN $8 THEN $9 ELSE description END,
  sync_token = $10,
  updated_at = now()
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'
RETURNING id::text, user_id::text, name, color, description, sync_token, created_at, updated_at`
	var calendar Calendar
	err = tx.QueryRowContext(ctx, query,
		req.UserID,
		req.CalendarID,
		nameSet,
		nameValue,
		normalizedName,
		colorSet,
		colorValue,
		descriptionSet,
		descriptionValue,
		syncToken,
	).Scan(
		&calendar.ID,
		&calendar.UserID,
		&calendar.Name,
		&calendar.Color,
		&calendar.Description,
		&calendar.SyncToken,
		&calendar.CreatedAt,
		&calendar.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Calendar{}, fmt.Errorf("CalDAV calendar not found")
		}
		return Calendar{}, fmt.Errorf("update CalDAV calendar properties: %w", err)
	}
	if err := insertCalendarSyncChange(ctx, tx, req.UserID, req.ActorUserID, req.CalendarID, syncToken, "collection-updated", "", ""); err != nil {
		return Calendar{}, err
	}
	if err := tx.Commit(); err != nil {
		return Calendar{}, fmt.Errorf("commit CalDAV calendar update: %w", err)
	}
	return calendar, nil
}

func (r *Repository) ListCalendarChangesSince(ctx context.Context, req ListChangesSinceRequest) ([]CalendarChange, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateListChangesSinceRequest(req)
	if err != nil {
		return nil, err
	}
	const query = `
WITH marker AS (
  SELECT id
  FROM caldav_calendar_sync_changes
  WHERE user_id = $1::uuid
    AND calendar_id = $2::uuid
    AND sync_token = $3
)
SELECT c.id, c.user_id::text, c.calendar_id::text, c.object_name, c.etag, c.action, c.sync_token, c.changed_at
FROM caldav_calendar_sync_changes c
JOIN marker m ON c.id > m.id
WHERE c.user_id = $1::uuid
  AND c.calendar_id = $2::uuid
ORDER BY c.id ASC
LIMIT $4`
	rows, err := r.db.QueryContext(ctx, query, req.UserID, req.CalendarID, req.SyncToken, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list CalDAV sync changes: %w", err)
	}
	defer rows.Close()
	var changes []CalendarChange
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
	if len(changes) == 0 {
		var markerExists bool
		err := r.db.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM caldav_calendar_sync_changes
  WHERE user_id = $1::uuid
    AND calendar_id = $2::uuid
    AND sync_token = $3
)`, req.UserID, req.CalendarID, req.SyncToken).Scan(&markerExists)
		if err != nil {
			return nil, fmt.Errorf("check CalDAV sync marker: %w", err)
		}
		if !markerExists {
			return nil, InvalidSyncTokenError{Token: req.SyncToken}
		}
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
		const query = `
WITH candidates AS (
  SELECT c.id
  FROM caldav_calendar_sync_changes c
  WHERE c.changed_at < $1
    AND ($2 = '' OR c.user_id = NULLIF($2, '')::uuid)
    AND ($3 = '' OR c.calendar_id = NULLIF($3, '')::uuid)
    AND EXISTS (
      SELECT 1
      FROM caldav_calendar_sync_changes newer
      WHERE newer.calendar_id = c.calendar_id
        AND newer.id > c.id
    )
  ORDER BY c.id ASC
  LIMIT $4
)
SELECT count(*)::bigint FROM candidates`
		if err := r.db.QueryRowContext(ctx, query, req.Cutoff, req.UserID, req.CalendarID, req.Limit).Scan(&result.CandidateCount); err != nil {
			return CalendarSyncChangePruneResult{}, fmt.Errorf("check CalDAV sync change prune candidates: %w", err)
		}
		return result, nil
	}
	const query = `
WITH candidates AS (
  SELECT c.id
  FROM caldav_calendar_sync_changes c
  WHERE c.changed_at < $1
    AND ($2 = '' OR c.user_id = NULLIF($2, '')::uuid)
    AND ($3 = '' OR c.calendar_id = NULLIF($3, '')::uuid)
    AND EXISTS (
      SELECT 1
      FROM caldav_calendar_sync_changes newer
      WHERE newer.calendar_id = c.calendar_id
        AND newer.id > c.id
    )
  ORDER BY c.id ASC
  LIMIT $4
),
deleted AS (
  DELETE FROM caldav_calendar_sync_changes c
  USING candidates
  WHERE c.id = candidates.id
  RETURNING c.id
)
SELECT (SELECT count(*)::bigint FROM candidates), (SELECT count(*)::bigint FROM deleted)`
	if err := r.db.QueryRowContext(ctx, query, req.Cutoff, req.UserID, req.CalendarID, req.Limit).Scan(&result.CandidateCount, &result.DeletedCount); err != nil {
		return CalendarSyncChangePruneResult{}, fmt.Errorf("prune CalDAV sync changes: %w", err)
	}
	return result, nil
}

func ValidateCreateCalendarRequest(req CreateCalendarRequest) (CreateCalendarRequest, string, string, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return CreateCalendarRequest{}, "", "", err
	}
	actorUserID, err := validateCalDAVActorUserID(req.ActorUserID, userID)
	if err != nil {
		return CreateCalendarRequest{}, "", "", err
	}
	name, err := ValidateCalendarName(req.Name)
	if err != nil {
		return CreateCalendarRequest{}, "", "", err
	}
	normalizedName, err := NormalizeCalendarName(name)
	if err != nil {
		return CreateCalendarRequest{}, "", "", err
	}
	color, err := ValidateCalendarColor(req.Color)
	if err != nil {
		return CreateCalendarRequest{}, "", "", err
	}
	description, err := ValidateCalendarDescription(req.Description)
	if err != nil {
		return CreateCalendarRequest{}, "", "", err
	}
	syncToken := CalendarSyncToken(userID, normalizedName, time.Now().UTC().Format(time.RFC3339Nano))
	return CreateCalendarRequest{UserID: userID, ActorUserID: actorUserID, Name: name, Color: color, Description: description}, normalizedName, syncToken, nil
}

func ValidateCreateCalendarAtPathRequest(req CreateCalendarAtPathRequest) (CreateCalendarAtPathRequest, string, string, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return CreateCalendarAtPathRequest{}, "", "", err
	}
	calendarID, err := ValidateCalendarPathID(req.CalendarID)
	if err != nil {
		return CreateCalendarAtPathRequest{}, "", "", err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = calendarID
	}
	create, normalizedName, syncToken, err := ValidateCreateCalendarRequest(CreateCalendarRequest{
		UserID:      userID,
		ActorUserID: req.ActorUserID,
		Name:        name,
		Color:       req.Color,
		Description: req.Description,
	})
	if err != nil {
		return CreateCalendarAtPathRequest{}, "", "", err
	}
	return CreateCalendarAtPathRequest{
		UserID:      create.UserID,
		ActorUserID: create.ActorUserID,
		CalendarID:  calendarID,
		Name:        create.Name,
		Color:       create.Color,
		Description: create.Description,
	}, normalizedName, syncToken, nil
}

func ValidateListCalendarsRequest(req ListCalendarsRequest) (ListCalendarsRequest, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return ListCalendarsRequest{}, err
	}
	status, err := ValidateCalendarStatus(req.Status)
	if err != nil {
		return ListCalendarsRequest{}, err
	}
	limit := normalizeCalDAVLimit(req.Limit)
	return ListCalendarsRequest{UserID: userID, Status: status, Limit: limit}, nil
}

func ValidateGetCalendarRequest(req GetCalendarRequest) (GetCalendarRequest, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return GetCalendarRequest{}, err
	}
	calendarID, err := validateCalDAVID("calendar_id", req.CalendarID, true)
	if err != nil {
		return GetCalendarRequest{}, err
	}
	status, err := ValidateCalendarStatus(req.Status)
	if err != nil {
		return GetCalendarRequest{}, err
	}
	return GetCalendarRequest{UserID: userID, CalendarID: calendarID, Status: status}, nil
}

func ValidateUpsertObjectRequest(req UpsertObjectRequest) (UpsertObjectRequest, string, string, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return UpsertObjectRequest{}, "", "", err
	}
	actorUserID, err := validateCalDAVActorUserID(req.ActorUserID, userID)
	if err != nil {
		return UpsertObjectRequest{}, "", "", err
	}
	calendarID, err := validateCalDAVID("calendar_id", req.CalendarID, true)
	if err != nil {
		return UpsertObjectRequest{}, "", "", err
	}
	objectName, err := ValidateCalendarObjectName(req.ObjectName)
	if err != nil {
		return UpsertObjectRequest{}, "", "", err
	}
	parsed, err := ParseICalendarObject(req.ICS)
	if err != nil {
		return UpsertObjectRequest{}, "", "", err
	}
	uid := parsed.UID
	if strings.TrimSpace(req.UID) != "" {
		uid, err = ValidateCalendarObjectUID(req.UID)
		if err != nil {
			return UpsertObjectRequest{}, "", "", err
		}
		if uid != parsed.UID {
			return UpsertObjectRequest{}, "", "", fmt.Errorf("calendar object UID does not match iCalendar body")
		}
	}
	component := parsed.Component
	if strings.TrimSpace(req.Component) != "" {
		component, err = ValidateCalendarComponent(req.Component)
		if err != nil {
			return UpsertObjectRequest{}, "", "", err
		}
		if component != parsed.Component {
			return UpsertObjectRequest{}, "", "", fmt.Errorf("calendar object component does not match iCalendar body")
		}
	}
	etag, _ := StrongETag(req.ICS)
	observedETag, err := validateOptionalETag(req.ObservedETag)
	if err != nil {
		return UpsertObjectRequest{}, "", "", err
	}
	syncToken := CalendarSyncToken(userID, calendarID, objectName, etag, time.Now().UTC().Format(time.RFC3339Nano))
	return UpsertObjectRequest{UserID: userID, ActorUserID: actorUserID, CalendarID: calendarID, ObjectName: objectName, UID: uid, Component: component, ICS: req.ICS, ObservedETag: observedETag}, etag, syncToken, nil
}

func ValidateListObjectsRequest(req ListObjectsRequest) (ListObjectsRequest, error) {
	return validateListObjectsRequest(req, normalizeCalDAVLimit)
}

func ValidateListObjectsForSyncRequest(req ListObjectsRequest) (ListObjectsRequest, error) {
	return validateListObjectsRequest(req, normalizeCalDAVChangeLimit)
}

func validateListObjectsRequest(req ListObjectsRequest, normalizeLimit func(int) int) (ListObjectsRequest, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return ListObjectsRequest{}, err
	}
	calendarID, err := validateCalDAVID("calendar_id", req.CalendarID, true)
	if err != nil {
		return ListObjectsRequest{}, err
	}
	status, err := ValidateCalendarStatus(req.Status)
	if err != nil {
		return ListObjectsRequest{}, err
	}
	return ListObjectsRequest{UserID: userID, CalendarID: calendarID, Status: status, Limit: normalizeLimit(req.Limit)}, nil
}

func ValidateGetObjectRequest(req GetObjectRequest) (GetObjectRequest, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return GetObjectRequest{}, err
	}
	calendarID, err := validateCalDAVID("calendar_id", req.CalendarID, true)
	if err != nil {
		return GetObjectRequest{}, err
	}
	objectName, err := ValidateCalendarObjectName(req.ObjectName)
	if err != nil {
		return GetObjectRequest{}, err
	}
	status, err := ValidateCalendarStatus(req.Status)
	if err != nil {
		return GetObjectRequest{}, err
	}
	return GetObjectRequest{UserID: userID, CalendarID: calendarID, ObjectName: objectName, Status: status}, nil
}

func ValidateDeleteObjectRequest(req DeleteObjectRequest) (DeleteObjectRequest, string, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return DeleteObjectRequest{}, "", err
	}
	actorUserID, err := validateCalDAVActorUserID(req.ActorUserID, userID)
	if err != nil {
		return DeleteObjectRequest{}, "", err
	}
	calendarID, err := validateCalDAVID("calendar_id", req.CalendarID, true)
	if err != nil {
		return DeleteObjectRequest{}, "", err
	}
	objectName, err := ValidateCalendarObjectName(req.ObjectName)
	if err != nil {
		return DeleteObjectRequest{}, "", err
	}
	syncToken := CalendarSyncToken(userID, calendarID, objectName, "delete", time.Now().UTC().Format(time.RFC3339Nano))
	return DeleteObjectRequest{UserID: userID, ActorUserID: actorUserID, CalendarID: calendarID, ObjectName: objectName}, syncToken, nil
}

func ValidateDeleteCalendarRequest(req DeleteCalendarRequest) (DeleteCalendarRequest, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return DeleteCalendarRequest{}, err
	}
	actorUserID, err := validateCalDAVActorUserID(req.ActorUserID, userID)
	if err != nil {
		return DeleteCalendarRequest{}, err
	}
	calendarID, err := validateCalDAVID("calendar_id", req.CalendarID, true)
	if err != nil {
		return DeleteCalendarRequest{}, err
	}
	return DeleteCalendarRequest{UserID: userID, ActorUserID: actorUserID, CalendarID: calendarID}, nil
}

func ValidateUpdateCalendarRequest(req UpdateCalendarRequest) (UpdateCalendarRequest, string, string, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return UpdateCalendarRequest{}, "", "", err
	}
	actorUserID, err := validateCalDAVActorUserID(req.ActorUserID, userID)
	if err != nil {
		return UpdateCalendarRequest{}, "", "", err
	}
	calendarID, err := validateCalDAVID("calendar_id", req.CalendarID, true)
	if err != nil {
		return UpdateCalendarRequest{}, "", "", err
	}
	if req.Name == nil && req.Color == nil && req.Description == nil {
		return UpdateCalendarRequest{}, "", "", fmt.Errorf("at least one calendar property is required")
	}
	var normalizedName string
	var name *string
	if req.Name != nil {
		value, err := ValidateCalendarName(*req.Name)
		if err != nil {
			return UpdateCalendarRequest{}, "", "", err
		}
		normalizedName, err = NormalizeCalendarName(value)
		if err != nil {
			return UpdateCalendarRequest{}, "", "", err
		}
		name = &value
	}
	var color *string
	if req.Color != nil {
		value, err := ValidateCalendarColor(*req.Color)
		if err != nil {
			return UpdateCalendarRequest{}, "", "", err
		}
		color = &value
	}
	var description *string
	if req.Description != nil {
		value, err := ValidateCalendarDescription(*req.Description)
		if err != nil {
			return UpdateCalendarRequest{}, "", "", err
		}
		description = &value
	}
	syncToken := CalendarSyncToken(userID, calendarID, "collection-update", time.Now().UTC().Format(time.RFC3339Nano))
	return UpdateCalendarRequest{UserID: userID, ActorUserID: actorUserID, CalendarID: calendarID, Name: name, Color: color, Description: description}, normalizedName, syncToken, nil
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

func lockActiveCalendar(ctx context.Context, tx *sql.Tx, userID string, calendarID string) error {
	var id string
	err := tx.QueryRowContext(ctx, `
SELECT id::text
FROM caldav_calendars
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'
FOR UPDATE`, userID, calendarID).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("CalDAV calendar not found")
		}
		return fmt.Errorf("lock CalDAV calendar: %w", err)
	}
	return nil
}

func ensureCalendarObjectUIDAvailable(ctx context.Context, tx *sql.Tx, userID string, calendarID string, objectName string, uid string) error {
	var existingObject string
	err := tx.QueryRowContext(ctx, `
SELECT object_name
FROM caldav_calendar_objects
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND uid = $3
  AND object_name <> $4
  AND status = 'active'
LIMIT 1`, userID, calendarID, uid, objectName).Scan(&existingObject)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("read CalDAV calendar object UID: %w", err)
	}
	return fmt.Errorf("CalDAV calendar object UID %q already exists as %q", uid, existingObject)
}

func mapCalendarObjectUpsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "idx_caldav_calendar_objects_active_uid":
			return fmt.Errorf("CalDAV calendar object UID already exists")
		case "idx_caldav_calendar_objects_active_name":
			return fmt.Errorf("CalDAV calendar object already exists")
		}
	}
	return fmt.Errorf("upsert CalDAV object: %w", err)
}

func ensureObjectETag(ctx context.Context, tx *sql.Tx, userID string, calendarID string, objectName string, etag string) error {
	var current string
	err := tx.QueryRowContext(ctx, `
SELECT etag
FROM caldav_calendar_objects
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND object_name = $3
  AND status = 'active'
FOR UPDATE`, userID, calendarID, objectName).Scan(&current)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("CalDAV object not found")
		}
		return fmt.Errorf("read CalDAV object etag: %w", err)
	}
	if current != etag {
		return fmt.Errorf("CalDAV object etag mismatch")
	}
	return nil
}

func updateCalendarSyncToken(ctx context.Context, tx *sql.Tx, userID string, calendarID string, syncToken string) error {
	res, err := tx.ExecContext(ctx, `
UPDATE caldav_calendars
SET sync_token = $3, updated_at = now()
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'`, userID, calendarID, syncToken)
	if err != nil {
		return fmt.Errorf("update CalDAV sync token: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read CalDAV sync token update count: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("CalDAV calendar not found")
	}
	return nil
}

type syncChangeExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

const (
	calendarChangedEvent       = "calendar.changed"
	davChangeOutboxTopic       = "dav.event"
	davChangeSchemaVersion     = "2026-05-06.dav-change.v1"
	davChangeKindCalDAV        = "caldav"
	davChangePartitionFallback = "unknown"
)

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

func ensureCalendarSyncMarker(ctx context.Context, tx *sql.Tx, userID string, calendarID string) error {
	var token string
	err := tx.QueryRowContext(ctx, `
SELECT sync_token
FROM caldav_calendars
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'`, userID, calendarID).Scan(&token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("CalDAV calendar not found")
		}
		return fmt.Errorf("read CalDAV sync marker: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO caldav_calendar_sync_changes (
  user_id, calendar_id, sync_token, action
) VALUES ($1::uuid, $2::uuid, $3, 'collection-created')
ON CONFLICT (calendar_id, sync_token) DO NOTHING`, userID, calendarID, token)
	if err != nil {
		return fmt.Errorf("ensure CalDAV sync marker: %w", err)
	}
	return nil
}

func validateOptionalETag(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	return ValidateStrongETag(value)
}

func validateCalDAVActorUserID(actorUserID string, ownerUserID string) (string, error) {
	actorUserID = strings.TrimSpace(actorUserID)
	if actorUserID == "" {
		return ownerUserID, nil
	}
	return validateCalDAVID("actor_user_id", actorUserID, true)
}

func optionalStringArg(value *string) (string, bool) {
	if value == nil {
		return "", false
	}
	return *value, true
}

func validateCalDAVID(field string, value string, required bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return "", fmt.Errorf("%s is required", field)
		}
		return "", nil
	}
	if len(value) > 128 {
		return "", fmt.Errorf("%s is too long", field)
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("%s must not contain line breaks", field)
	}
	return value, nil
}

func normalizeCalDAVLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func normalizeCalDAVChangeLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	if limit > MaxWebDAVReportLimit+1 {
		return MaxWebDAVReportLimit + 1
	}
	return limit
}
