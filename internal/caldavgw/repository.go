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

const caldavCalendarObjectLookupBatchSize = 256
const caldavObjectWriteMaxAttempts = 4
const caldavObjectWriteBaseDelay = 5 * time.Millisecond
const caldavObjectWriteMaxDelay = 80 * time.Millisecond

type calendarObjectNameTuple struct {
	calendarID string
	objectName string
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

type CreateCalendarRequest struct {
	UserID          string
	ActorUserID     string
	Name            string
	NameLang        *string
	Color           string
	Description     string
	DescriptionLang *string
}

type CreateCalendarAtPathRequest struct {
	UserID          string
	ActorUserID     string
	CalendarID      string
	Name            string
	NameLang        *string
	Slug            *string
	Timezone        *string
	Color           string
	Description     string
	DescriptionLang *string
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
	UserID       string
	ActorUserID  string
	CalendarID   string
	ObjectName   string
	ObservedETag string
}

type DeleteCalendarRequest struct {
	UserID       string
	ActorUserID  string
	CalendarID   string
	ObservedETag string
}

type UpdateCalendarRequest struct {
	UserID          string
	ActorUserID     string
	CalendarID      string
	Name            *string
	NameLang        *string
	Slug            *string
	Timezone        *string
	Color           *string
	Description     *string
	DescriptionLang *string
	ObservedETag    string
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
  company_id, domain_id, user_id, name, normalized_name, displayname_lang, color, description, description_lang, sync_token
)
SELECT company_id, domain_id, user_id, $2, $3, $4, $5, $6, $7, $8
FROM active_user
RETURNING id::text, user_id::text, name, displayname_lang, color, description, description_lang, sync_token, created_at, updated_at`
	var calendar Calendar
	err = tx.QueryRowContext(ctx, query,
		req.UserID,
		req.Name,
		normalizedName,
		req.NameLang,
		req.Color,
		req.Description,
		req.DescriptionLang,
		syncToken,
	).Scan(
		&calendar.ID,
		&calendar.UserID,
		&calendar.Name,
		&calendar.NameLang,
		&calendar.Color,
		&calendar.Description,
		&calendar.DescriptionLang,
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
	req, normalizedName, syncToken, normalizedSlug, normalizedTimezone, err := ValidateCreateCalendarAtPathRequest(req)
	if err != nil {
		return Calendar{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Calendar{}, fmt.Errorf("begin CalDAV calendar create: %w", err)
	}
	defer tx.Rollback()
	var calendar Calendar
	if req.Slug != nil {
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
  id, company_id, domain_id, user_id, name, normalized_name, displayname_lang, slug, timezone, color, description, description_lang, sync_token
)
SELECT $2::uuid, company_id, domain_id, user_id, $3, $4, $5, $6, $7, $8, $9, $10, $11
FROM active_user
RETURNING id::text, user_id::text, name, displayname_lang, slug, timezone, color, description, description_lang, sync_token, created_at, updated_at`
		err = tx.QueryRowContext(ctx, query,
			req.UserID,
			req.CalendarID,
			req.Name,
			normalizedName,
			req.NameLang,
			normalizedSlug,
			normalizedTimezone,
			req.Color,
			req.Description,
			req.DescriptionLang,
			syncToken,
		).Scan(
			&calendar.ID,
			&calendar.UserID,
			&calendar.Name,
			&calendar.NameLang,
			&calendar.Slug,
			&calendar.Timezone,
			&calendar.Color,
			&calendar.Description,
			&calendar.DescriptionLang,
			&calendar.SyncToken,
			&calendar.CreatedAt,
			&calendar.UpdatedAt,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return Calendar{}, fmt.Errorf("active user not found")
			}
			if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
				return Calendar{}, fmt.Errorf("calendar slug already exists")
			}
			return Calendar{}, fmt.Errorf("create CalDAV calendar at path: %w", err)
		}
	} else {
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
  id, company_id, domain_id, user_id, name, normalized_name, displayname_lang, timezone, color, description, description_lang, sync_token
)
SELECT $2::uuid, company_id, domain_id, user_id, $3, $4, $5, $6, $7, $8, $9, $10
FROM active_user
RETURNING id::text, user_id::text, name, displayname_lang, timezone, color, description, description_lang, sync_token, created_at, updated_at`
		err = tx.QueryRowContext(ctx, query,
			req.UserID,
			req.CalendarID,
			req.Name,
			normalizedName,
			req.NameLang,
			normalizedTimezone,
			req.Color,
			req.Description,
			req.DescriptionLang,
			syncToken,
		).Scan(
			&calendar.ID,
			&calendar.UserID,
			&calendar.Name,
			&calendar.NameLang,
			&calendar.Timezone,
			&calendar.Color,
			&calendar.Description,
			&calendar.DescriptionLang,
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
SELECT id::text, user_id::text, name, displayname_lang, timezone, color, description, description_lang, sync_token, created_at, updated_at
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
		if err := rows.Scan(&calendar.ID, &calendar.UserID, &calendar.Name, &calendar.NameLang, &calendar.Timezone, &calendar.Color, &calendar.Description, &calendar.DescriptionLang, &calendar.SyncToken, &calendar.CreatedAt, &calendar.UpdatedAt); err != nil {
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
SELECT id::text, user_id::text, name, displayname_lang, timezone, color, description, description_lang, sync_token, created_at, updated_at
FROM caldav_calendars
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = $3`
	var calendar Calendar
	err = r.db.QueryRowContext(ctx, query, req.UserID, req.CalendarID, req.Status).Scan(
		&calendar.ID,
		&calendar.UserID,
		&calendar.Name,
		&calendar.NameLang,
		&calendar.Timezone,
		&calendar.Color,
		&calendar.Description,
		&calendar.DescriptionLang,
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

type GetCalendarBySlugRequest struct {
	UserID string
	Slug   string
	Status string
}

func (r *Repository) GetCalendarBySlug(ctx context.Context, req GetCalendarBySlugRequest) (Calendar, error) {
	if r == nil || r.db == nil {
		return Calendar{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateGetCalendarBySlugRequest(req)
	if err != nil {
		return Calendar{}, err
	}
	const query = `
SELECT id::text, user_id::text, name, displayname_lang, slug, timezone, color, description, description_lang, sync_token, created_at, updated_at
FROM caldav_calendars
WHERE user_id = $1::uuid
  AND lower(slug) = lower($2)
  AND status = $3`
	var calendar Calendar
	err = r.db.QueryRowContext(ctx, query, req.UserID, req.Slug, req.Status).Scan(
		&calendar.ID,
		&calendar.UserID,
		&calendar.Name,
		&calendar.NameLang,
		&calendar.Slug,
		&calendar.Timezone,
		&calendar.Color,
		&calendar.Description,
		&calendar.DescriptionLang,
		&calendar.SyncToken,
		&calendar.CreatedAt,
		&calendar.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Calendar{}, fmt.Errorf("CalDAV calendar not found")
		}
		return Calendar{}, fmt.Errorf("get CalDAV calendar by slug: %w", err)
	}
	return calendar, nil
}

func ValidateGetCalendarBySlugRequest(req GetCalendarBySlugRequest) (GetCalendarBySlugRequest, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return GetCalendarBySlugRequest{}, err
	}
	slug, err := ValidateSlug(req.Slug)
	if err != nil {
		return GetCalendarBySlugRequest{}, err
	}
	status, err := ValidateCalendarStatus(req.Status)
	if err != nil {
		return GetCalendarBySlugRequest{}, err
	}
	return GetCalendarBySlugRequest{
		UserID: userID,
		Slug:   slug,
		Status: status,
	}, nil
}

func (r *Repository) UpsertObject(ctx context.Context, req UpsertObjectRequest) (CalendarObject, error) {
	if r == nil || r.db == nil {
		return CalendarObject{}, fmt.Errorf("database handle is required")
	}
	req, etag, syncToken, err := ValidateUpsertObjectRequest(req)
	if err != nil {
		return CalendarObject{}, err
	}
	var object CalendarObject
	var objectICS sql.NullString
	if err := runCalDAVWriteWithRetry(ctx, func() error {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin CalDAV object upsert: %w", err)
		}
		defer tx.Rollback()
		if err := ensureCalendarSyncMarker(ctx, tx, req.UserID, req.CalendarID); err != nil {
			return err
		}
		var query string
		args := []interface{}{
			req.UserID,
			req.CalendarID,
			req.ObjectName,
			req.UID,
			req.Component,
			etag,
			len(req.ICS),
			string(req.ICS),
		}
		if req.ObservedETag != "" {
			query = `
INSERT INTO caldav_calendar_objects (
  user_id, calendar_id, object_name, uid, component_type, etag, size, ics
) SELECT $1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8
WHERE EXISTS (
  SELECT 1
  FROM caldav_calendar_objects
  WHERE user_id = $1::uuid
    AND calendar_id = $2::uuid
    AND object_name = $3
    AND etag = $9
    AND status = 'active'
)
ON CONFLICT (calendar_id, object_name) WHERE status = 'active'
DO UPDATE SET
  uid = EXCLUDED.uid,
  component_type = EXCLUDED.component_type,
  etag = EXCLUDED.etag,
  size = EXCLUDED.size,
  ics = EXCLUDED.ics,
  updated_at = now()
WHERE caldav_calendar_objects.etag = $9

RETURNING
  id::text,
  user_id::text,
  calendar_id::text,
  object_name,
  uid,
  component_type,
  etag,
  size,
  NULL::text AS ics,
  created_at,
  updated_at`
			args = append(args, req.ObservedETag)
		} else {
			query = `
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

RETURNING
  id::text,
  user_id::text,
  calendar_id::text,
  object_name,
  uid,
  component_type,
  etag,
  size,
  NULL::text AS ics,
  created_at,
  updated_at`
		}
		err = tx.QueryRowContext(ctx, query, args...).Scan(
			&object.ID,
			&object.UserID,
			&object.CalendarID,
			&object.ObjectName,
			&object.UID,
			&object.Component,
			&object.ETag,
			&object.Size,
			&objectICS,
			&object.CreatedAt,
			&object.UpdatedAt,
		)
		if err != nil {
			return mapCalendarObjectUpsertError(err)
		}
		if err := updateCalendarSyncToken(ctx, tx, req.UserID, req.CalendarID, syncToken); err != nil {
			return err
		}
		if err := insertCalendarSyncChange(ctx, tx, req.UserID, req.ActorUserID, req.CalendarID, syncToken, "object-upserted", req.ObjectName, etag); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit CalDAV object upsert: %w", err)
		}
		return nil
	}); err != nil {
		return CalendarObject{}, err
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

func (r *Repository) ListCalendarObjectsByNames(ctx context.Context, userID string, calendarID string, status string, objectNames []string, includeICS bool) ([]CalendarObject, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID, err := validateCalDAVID("user_id", userID, true)
	if err != nil {
		return nil, err
	}
	calendarID, err = validateCalDAVID("calendar_id", calendarID, true)
	if err != nil {
		return nil, err
	}
	status, err = ValidateCalendarStatus(status)
	if err != nil {
		return nil, err
	}
	objectNames, err = validateCalendarObjectNames(objectNames)
	if err != nil {
		return nil, err
	}
	if len(objectNames) == 0 {
		return []CalendarObject{}, nil
	}
	objects := make([]CalendarObject, 0, len(objectNames))
	for i := 0; i < len(objectNames); i += caldavCalendarObjectLookupBatchSize {
		end := i + caldavCalendarObjectLookupBatchSize
		if end > len(objectNames) {
			end = len(objectNames)
		}
		chunk := make([]calendarObjectNameTuple, 0, end-i)
		for _, objectName := range objectNames[i:end] {
			chunk = append(chunk, calendarObjectNameTuple{
				calendarID: calendarID,
				objectName: objectName,
			})
		}
		more, err := r.listCalendarObjectsByNameTuples(ctx, userID, status, chunk, includeICS)
		if err != nil {
			return nil, err
		}
		objects = append(objects, more...)
	}
	return objects, nil
}

func (r *Repository) ListCalendarObjectsByNameGroups(ctx context.Context, userID string, objectNamesByCalendar map[string][]string, status string, includeICS bool) ([]CalendarObject, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID, err := validateCalDAVID("user_id", userID, true)
	if err != nil {
		return nil, err
	}
	status, err = ValidateCalendarStatus(status)
	if err != nil {
		return nil, err
	}
	if len(objectNamesByCalendar) == 0 {
		return []CalendarObject{}, nil
	}

	normalizedByCalendar := make(map[string][]string, len(objectNamesByCalendar))
	calendarIDs := make([]string, 0, len(objectNamesByCalendar))
	pairCount := 0
	for calendarID, objectNames := range objectNamesByCalendar {
		calendarID, err = validateCalDAVID("calendar_id", calendarID, true)
		if err != nil {
			return nil, err
		}
		objectNames, err = validateCalendarObjectNames(objectNames)
		if err != nil {
			return nil, err
		}
		if len(objectNames) == 0 {
			continue
		}
		normalizedByCalendar[calendarID] = objectNames
		calendarIDs = append(calendarIDs, calendarID)
		pairCount += len(objectNames)
	}
	if len(normalizedByCalendar) == 0 || pairCount == 0 {
		return []CalendarObject{}, nil
	}

	tuples := make([]calendarObjectNameTuple, 0, pairCount)
	for _, calendarID := range calendarIDs {
		for _, objectName := range normalizedByCalendar[calendarID] {
			tuples = append(tuples, calendarObjectNameTuple{
				calendarID: calendarID,
				objectName: objectName,
			})
		}
	}
	objects := make([]CalendarObject, 0, pairCount)
	for i := 0; i < len(tuples); i += caldavCalendarObjectLookupBatchSize {
		end := i + caldavCalendarObjectLookupBatchSize
		if end > len(tuples) {
			end = len(tuples)
		}
		more, err := r.listCalendarObjectsByNameTuples(ctx, userID, status, tuples[i:end], includeICS)
		if err != nil {
			return nil, err
		}
		objects = append(objects, more...)
	}
	return objects, nil
}

func (r *Repository) ListCalendarObjectsByComponentLimit(ctx context.Context, userID string, calendarID string, status string, component string, limit int, includeICS bool) ([]CalendarObject, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateListObjectsForSyncRequest(ListObjectsRequest{
		UserID:     userID,
		CalendarID: calendarID,
		Status:     status,
		Limit:      limit,
	})
	if err != nil {
		return nil, err
	}
	component = strings.ToUpper(strings.TrimSpace(component))
	queryArgs := []interface{}{req.UserID, req.CalendarID, req.Status, component, req.Limit}
	var query string
	if includeICS {
		query = `
SELECT id::text, user_id::text, calendar_id::text, object_name, uid, component_type, etag, size, ics, created_at, updated_at
FROM caldav_calendar_objects
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND status = $3
  AND component_type = $4
ORDER BY updated_at DESC, id DESC
LIMIT $5`
	} else {
		query = `
SELECT id::text, user_id::text, calendar_id::text, object_name, uid, component_type, etag, size, NULL::text AS ics, created_at, updated_at
FROM caldav_calendar_objects
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND status = $3
  AND component_type = $4
ORDER BY updated_at DESC, id DESC
LIMIT $5`
	}
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("get CalDAV objects by component limit: %w", err)
	}
	defer rows.Close()
	objects := make([]CalendarObject, 0, req.Limit)
	for rows.Next() {
		var object CalendarObject
		if err := rows.Scan(&object.ID, &object.UserID, &object.CalendarID, &object.ObjectName, &object.UID, &object.Component, &object.ETag, &object.Size, &object.ICS, &object.CreatedAt, &object.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan CalDAV object: %w", err)
		}
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate CalDAV objects by component limit: %w", err)
	}
	return objects, nil
}

func (r *Repository) listCalendarObjectsByNameTuples(ctx context.Context, userID string, status string, tuples []calendarObjectNameTuple, includeICS bool) ([]CalendarObject, error) {
	if len(tuples) == 0 {
		return []CalendarObject{}, nil
	}
	valueRows := make([]string, 0, len(tuples))
	queryArgs := make([]interface{}, 0, 2+len(tuples)*2)
	queryArgs = append(queryArgs, userID, status)
	param := 3
	for _, tuple := range tuples {
		valueRows = append(valueRows, fmt.Sprintf("($%d::uuid, $%d)", param, param+1))
		queryArgs = append(queryArgs, tuple.calendarID, tuple.objectName)
		param += 2
	}
	queryValues := strings.Join(valueRows, ", ")
	var query string
	if includeICS {
		query = `
SELECT o.id::text, o.user_id::text, o.calendar_id::text, o.object_name, o.uid, o.component_type, o.etag, o.size, o.ics, o.created_at, o.updated_at
FROM caldav_calendar_objects o
JOIN (VALUES ` + queryValues + `) AS req(calendar_id, object_name)
  ON req.calendar_id = o.calendar_id
 AND req.object_name = o.object_name
WHERE o.user_id = $1::uuid
  AND o.status = $2`
	} else {
		query = `
SELECT o.id::text, o.user_id::text, o.calendar_id::text, o.object_name, o.uid, o.component_type, o.etag, o.size, NULL::text AS ics, o.created_at, o.updated_at
FROM caldav_calendar_objects o
JOIN (VALUES ` + queryValues + `) AS req(calendar_id, object_name)
  ON req.calendar_id = o.calendar_id
 AND req.object_name = o.object_name
WHERE o.user_id = $1::uuid
  AND o.status = $2`
	}
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("get CalDAV objects by grouped names: %w", err)
	}
	defer rows.Close()
	objects := make([]CalendarObject, 0, len(tuples))
	for rows.Next() {
		var object CalendarObject
		if err := rows.Scan(&object.ID, &object.UserID, &object.CalendarID, &object.ObjectName, &object.UID, &object.Component, &object.ETag, &object.Size, &object.ICS, &object.CreatedAt, &object.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan CalDAV object: %w", err)
		}
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate CalDAV objects by grouped names: %w", err)
	}
	return objects, nil
}

func (r *Repository) listObjectsForSync(ctx context.Context, req ListObjectsRequest) ([]CalendarObject, error) {
	req, err := ValidateListObjectsForSyncRequest(req)
	if err != nil {
		return nil, err
	}
	return r.listObjects(ctx, req)
}

func (r *Repository) listObjects(ctx context.Context, req ListObjectsRequest) ([]CalendarObject, error) {
	return r.listCalendarObjects(ctx, req, true)
}

func (r *Repository) listObjectsMetadata(ctx context.Context, req ListObjectsRequest) ([]CalendarObject, error) {
	return r.listCalendarObjects(ctx, req, false)
}

func (r *Repository) listCalendarObjects(ctx context.Context, req ListObjectsRequest, includeICS bool) ([]CalendarObject, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	query := `
SELECT id::text, user_id::text, calendar_id::text, object_name, uid, component_type, etag, size, ics, created_at, updated_at
FROM caldav_calendar_objects
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND status = $3
ORDER BY updated_at DESC, id DESC
LIMIT $4`
	if !includeICS {
		query = `
SELECT id::text, user_id::text, calendar_id::text, object_name, uid, component_type, etag, size, NULL::text AS ics, created_at, updated_at
FROM caldav_calendar_objects
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND status = $3
ORDER BY updated_at DESC, id DESC
LIMIT $4`
	}
	rows, err := r.db.QueryContext(ctx, query, req.UserID, req.CalendarID, req.Status, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list CalDAV objects: %w", err)
	}
	defer rows.Close()
	objects := make([]CalendarObject, 0, req.Limit)
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

func (r *Repository) ListCalendarObjectMetadataLimit(ctx context.Context, userID string, calendarID string, status string, limit int) ([]CalendarObject, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateListObjectsForSyncRequest(ListObjectsRequest{
		UserID:     userID,
		CalendarID: calendarID,
		Status:     status,
		Limit:      limit,
	})
	if err != nil {
		return nil, err
	}
	return r.listObjectsMetadata(ctx, req)
}

func (r *Repository) LookupCalendarObjectMetadata(ctx context.Context, userID string, calendarID string, objectName string) (CalendarObject, error) {
	if r == nil || r.db == nil {
		return CalendarObject{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateGetObjectRequest(GetObjectRequest{
		UserID:     userID,
		CalendarID: calendarID,
		ObjectName: objectName,
		Status:     CalendarStatusActive,
	})
	if err != nil {
		return CalendarObject{}, err
	}
	const query = `
SELECT id::text, user_id::text, calendar_id::text, object_name, uid, component_type, etag, size, NULL::text AS ics, created_at, updated_at
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
		return CalendarObject{}, fmt.Errorf("get CalDAV object metadata: %w", err)
	}
	return object, nil
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
	var object CalendarObject
	var objectICS sql.NullString
	if err := runCalDAVWriteWithRetry(ctx, func() error {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin CalDAV object delete: %w", err)
		}
		defer tx.Rollback()
		const query = `
UPDATE caldav_calendar_objects
SET status = 'deleted', deleted_at = now(), updated_at = now()
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND object_name = $3
  AND status = 'active'
  AND ($4 = '' OR etag = $4)

RETURNING
  id::text,
  user_id::text,
  calendar_id::text,
  object_name,
  uid,
  component_type,
  etag,
  size,
  NULL::text AS ics,
  created_at,
  updated_at`
		err = tx.QueryRowContext(ctx, query, req.UserID, req.CalendarID, req.ObjectName, req.ObservedETag).Scan(
			&object.ID,
			&object.UserID,
			&object.CalendarID,
			&object.ObjectName,
			&object.UID,
			&object.Component,
			&object.ETag,
			&object.Size,
			&objectICS,
			&object.CreatedAt,
			&object.UpdatedAt,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("CalDAV object not found")
			}
			return fmt.Errorf("delete CalDAV object: %w", err)
		}
		if err := ensureCalendarSyncMarker(ctx, tx, req.UserID, req.CalendarID); err != nil {
			return err
		}
		if err := updateCalendarSyncToken(ctx, tx, req.UserID, req.CalendarID, syncToken); err != nil {
			return err
		}
		if err := insertCalendarSyncChange(ctx, tx, req.UserID, req.ActorUserID, req.CalendarID, syncToken, "object-deleted", req.ObjectName, object.ETag); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit CalDAV object delete: %w", err)
		}
		return nil
	}); err != nil {
		return CalendarObject{}, err
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
	var calendar Calendar
	if err := runCalDAVWriteWithRetry(ctx, func() error {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin CalDAV calendar delete: %w", err)
		}
		defer tx.Rollback()
		if req.ObservedETag != "" {
			if err := ensureCalendarCollectionETag(ctx, tx, req.UserID, req.CalendarID, req.ObservedETag); err != nil {
				return err
			}
		}
		const query = `
UPDATE caldav_calendars
SET status = 'deleted', deleted_at = now(), updated_at = now()
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'
RETURNING id::text, user_id::text, name, displayname_lang, color, description, description_lang, sync_token, created_at, updated_at`
		if err := tx.QueryRowContext(ctx, query, req.UserID, req.CalendarID).Scan(
			&calendar.ID,
			&calendar.UserID,
			&calendar.Name,
			&calendar.NameLang,
			&calendar.Color,
			&calendar.Description,
			&calendar.DescriptionLang,
			&calendar.SyncToken,
			&calendar.CreatedAt,
			&calendar.UpdatedAt,
		); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("CalDAV calendar not found")
			}
			return fmt.Errorf("delete CalDAV calendar: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE caldav_calendar_objects
SET status = 'deleted', deleted_at = COALESCE(deleted_at, now()), updated_at = now()
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND status = 'active'`, req.UserID, req.CalendarID); err != nil {
			return fmt.Errorf("delete CalDAV calendar objects: %w", err)
		}
		syncToken := CalendarSyncToken(req.UserID, req.CalendarID, "collection-delete", time.Now().UTC().Format(time.RFC3339Nano))
		if err := insertCalendarSyncChange(ctx, tx, req.UserID, req.ActorUserID, req.CalendarID, syncToken, "collection-deleted", "", ""); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit CalDAV calendar delete: %w", err)
		}
		return nil
	}); err != nil {
		return Calendar{}, err
	}
	return calendar, nil
}

func (r *Repository) UpdateCalendarProperties(ctx context.Context, req UpdateCalendarRequest) (Calendar, error) {
	if r == nil || r.db == nil {
		return Calendar{}, fmt.Errorf("database handle is required")
	}
	req, normalizedName, syncToken, normalizedSlug, normalizedTimezone, err := ValidateUpdateCalendarRequest(req)
	if err != nil {
		return Calendar{}, err
	}
	var calendar Calendar
	if err := runCalDAVWriteWithRetry(ctx, func() error {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin CalDAV calendar update: %w", err)
		}
		defer tx.Rollback()
		if req.ObservedETag != "" {
			if err := ensureCalendarCollectionETag(ctx, tx, req.UserID, req.CalendarID, req.ObservedETag); err != nil {
				return err
			}
		}
		nameValue, nameSet := optionalStringArg(req.Name)
		slugValue, slugSet := optionalStringArg(normalizedSlug)
		timezoneValue, timezoneSet := optionalStringArg(normalizedTimezone)
		colorValue, colorSet := optionalStringArg(req.Color)
		descriptionValue, descriptionSet := optionalStringArg(req.Description)
		nameLangValue, nameLangSet := optionalStringArg(req.NameLang)
		descriptionLangValue, descriptionLangSet := optionalStringArg(req.DescriptionLang)
		const query = `
UPDATE caldav_calendars
SET
  name = CASE WHEN $3 THEN $4 ELSE name END,
  normalized_name = CASE WHEN $3 THEN $5 ELSE normalized_name END,
  displayname_lang = CASE WHEN $6 THEN $7 ELSE displayname_lang END,
  slug = CASE WHEN $8 THEN $9 ELSE slug END,
  timezone = CASE WHEN $10 THEN $11 ELSE timezone END,
  color = CASE WHEN $12 THEN $13 ELSE color END,
  description = CASE WHEN $14 THEN $15 ELSE description END,
  description_lang = CASE WHEN $16 THEN $17 ELSE description_lang END,
  sync_token = $18,
  updated_at = now()
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'
RETURNING id::text, user_id::text, name, displayname_lang, slug, timezone, color, description, description_lang, sync_token, created_at, updated_at`
		if err := tx.QueryRowContext(ctx, query,
			req.UserID,
			req.CalendarID,
			nameSet,
			nameValue,
			normalizedName,
			nameLangSet,
			nameLangValue,
			slugSet,
			slugValue,
			timezoneSet,
			timezoneValue,
			colorSet,
			colorValue,
			descriptionSet,
			descriptionValue,
			descriptionLangSet,
			descriptionLangValue,
			syncToken,
		).Scan(
			&calendar.ID,
			&calendar.UserID,
			&calendar.Name,
			&calendar.NameLang,
			&calendar.Slug,
			&calendar.Timezone,
			&calendar.Color,
			&calendar.Description,
			&calendar.DescriptionLang,
			&calendar.SyncToken,
			&calendar.CreatedAt,
			&calendar.UpdatedAt,
		); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("CalDAV calendar not found")
			}
			if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
				return fmt.Errorf("calendar slug already exists")
			}
			return fmt.Errorf("update CalDAV calendar properties: %w", err)
		}
		if err := ensureCalendarSyncMarker(ctx, tx, req.UserID, req.CalendarID); err != nil {
			return err
		}
		if err := insertCalendarSyncChange(ctx, tx, req.UserID, req.ActorUserID, req.CalendarID, syncToken, "collection-updated", "", ""); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit CalDAV calendar update: %w", err)
		}
		return nil
	}); err != nil {
		return Calendar{}, err
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

func (r *Repository) DeliverSchedulingMessage(ctx context.Context, req DeliverSchedulingMessageRequest) (SchedulingMessage, error) {
	if r == nil || r.db == nil {
		return SchedulingMessage{}, fmt.Errorf("database handle is required")
	}
	req, etag, err := ValidateDeliverSchedulingMessageRequest(req)
	if err != nil {
		return SchedulingMessage{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return SchedulingMessage{}, fmt.Errorf("begin scheduling deliver: %w", err)
	}
	defer tx.Rollback()
	inboxID, err := r.getSchedulingInboxID(ctx, tx, req.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SchedulingMessage{}, fmt.Errorf("scheduling inbox not found")
		}
		return SchedulingMessage{}, fmt.Errorf("lookup scheduling inbox: %w", err)
	}
	objectName := fmt.Sprintf("%s.ics", req.UID)
	var existingObject string
	err = tx.QueryRowContext(ctx, `
SELECT object_name
FROM caldav_calendar_objects
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND uid = $3
  AND status = 'active'`, req.UserID, inboxID, req.UID).Scan(&existingObject)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return SchedulingMessage{}, fmt.Errorf("check existing scheduling object: %w", err)
	}
	if existingObject != "" {
		objectName = existingObject
	}
	var msg SchedulingMessage
	if req.Method == ScheduleMethodCancel || req.Method == ScheduleMethodDeclineCounter {
		if existingObject != "" {
			_, err = tx.ExecContext(ctx, `
UPDATE caldav_calendar_objects
SET status = 'deleted', deleted_at = now(), updated_at = now()
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND object_name = $3
  AND status = 'active'`, req.UserID, inboxID, objectName)
			if err != nil {
				return SchedulingMessage{}, fmt.Errorf("cancel scheduling object: %w", err)
			}
		}
		msg = SchedulingMessage{
			UserID:       req.UserID,
			Recipient:    req.Recipient,
			Method:       req.Method,
			UID:          req.UID,
			ICSPayload:   req.ICSPayload,
			ETag:         etag,
			ProcessedAt:  time.Now().UTC(),
			ResponseCode: "2.0;success",
		}
	} else {
		_, err = tx.ExecContext(ctx, `
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
  updated_at = now()`, req.UserID, inboxID, objectName, req.UID, strings.ToUpper(string(req.Method)), etag, len(req.ICSPayload), string(req.ICSPayload))
		if err != nil {
			return SchedulingMessage{}, fmt.Errorf("store scheduling object: %w", err)
		}
		component, _ := parseICSScheduleMethod(req.ICSPayload)
		if component == "" {
			component = "VEVENT"
		}
		msg = SchedulingMessage{
			UserID:       req.UserID,
			Recipient:    req.Recipient,
			Method:       req.Method,
			UID:          req.UID,
			ICSPayload:   req.ICSPayload,
			ETag:         etag,
			ProcessedAt:  time.Now().UTC(),
			ResponseCode: "2.0;success",
		}
		_ = component
	}
	if err := tx.Commit(); err != nil {
		return SchedulingMessage{}, fmt.Errorf("commit scheduling deliver: %w", err)
	}
	return msg, nil
}

func (r *Repository) SendSchedulingMessage(ctx context.Context, req SendSchedulingMessageRequest) (SchedulingMessage, error) {
	if r == nil || r.db == nil {
		return SchedulingMessage{}, fmt.Errorf("database handle is required")
	}
	req, etag, err := ValidateSendSchedulingMessageRequest(req)
	if err != nil {
		return SchedulingMessage{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return SchedulingMessage{}, fmt.Errorf("begin scheduling send: %w", err)
	}
	defer tx.Rollback()
	outboxID, err := r.getSchedulingOutboxID(ctx, tx, req.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SchedulingMessage{}, fmt.Errorf("scheduling outbox not found")
		}
		return SchedulingMessage{}, fmt.Errorf("lookup scheduling outbox: %w", err)
	}
	objectName := fmt.Sprintf("%s.ics", req.UID)
	_, err = tx.ExecContext(ctx, `
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
  updated_at = now()`, req.UserID, outboxID, objectName, req.UID, strings.ToUpper(string(req.Method)), etag, len(req.ICSPayload), string(req.ICSPayload))
	if err != nil {
		return SchedulingMessage{}, fmt.Errorf("store outbox object: %w", err)
	}
	if err := r.insertSchedulingOutboxEvent(ctx, tx, req.UserID, req.UID, string(req.Method), req.ICSPayload); err != nil {
		return SchedulingMessage{}, err
	}
	msg := SchedulingMessage{
		UserID:      req.UserID,
		Method:      req.Method,
		UID:         req.UID,
		ICSPayload:  req.ICSPayload,
		ETag:        etag,
		ProcessedAt: time.Now().UTC(),
	}
	if err := tx.Commit(); err != nil {
		return SchedulingMessage{}, fmt.Errorf("commit scheduling send: %w", err)
	}
	return msg, nil
}

func (r *Repository) getSchedulingInboxID(ctx context.Context, tx *sql.Tx, userID string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `
SELECT id::text
FROM caldav_calendars
WHERE user_id = $1::uuid
  AND name = 'inbox'
  AND status = 'active'
LIMIT 1`, userID).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *Repository) getSchedulingOutboxID(ctx context.Context, tx *sql.Tx, userID string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `
SELECT id::text
FROM caldav_calendars
WHERE user_id = $1::uuid
  AND name = 'outbox'
  AND status = 'active'
LIMIT 1`, userID).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *Repository) insertSchedulingOutboxEvent(ctx context.Context, tx *sql.Tx, userID string, uid string, method string, payload []byte) error {
	partitionKey := strings.TrimSpace(userID)
	if partitionKey == "" {
		partitionKey = "unknown"
	}
	schedPayload, err := json.Marshal(map[string]any{
		"event":          "scheduling.outbox",
		"schema_version": "2026-05-08.scheduling.v1",
		"dav_kind":       "caldav-scheduling",
		"user_id":        userID,
		"uid":            uid,
		"method":         method,
		"payload":        string(payload),
		"created_at":     time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("marshal scheduling outbox event: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO outbox (topic, partition_key, payload, status)
VALUES ($1, $2, $3::jsonb, 'pending')`, "scheduling.outbox", partitionKey, string(schedPayload))
	if err != nil {
		return fmt.Errorf("insert scheduling outbox event: %w", err)
	}
	return nil
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
	nameLang, err := validateDAVPropertyLanguagePointer("displayname xml:lang", req.NameLang)
	if err != nil {
		return CreateCalendarRequest{}, "", "", err
	}
	descriptionLang, err := validateDAVPropertyLanguagePointer("calendar-description xml:lang", req.DescriptionLang)
	if err != nil {
		return CreateCalendarRequest{}, "", "", err
	}
	syncToken := CalendarSyncToken(userID, normalizedName, time.Now().UTC().Format(time.RFC3339Nano))
	return CreateCalendarRequest{UserID: userID, ActorUserID: actorUserID, Name: name, NameLang: nameLang, Color: color, Description: description, DescriptionLang: descriptionLang}, normalizedName, syncToken, nil
}

func ValidateCreateCalendarAtPathRequest(req CreateCalendarAtPathRequest) (CreateCalendarAtPathRequest, string, string, *string, *string, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return CreateCalendarAtPathRequest{}, "", "", nil, nil, err
	}
	calendarID, err := ValidateCalendarPathID(req.CalendarID)
	if err != nil {
		return CreateCalendarAtPathRequest{}, "", "", nil, nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = calendarID
	}
	var normalizedSlug *string
	if req.Slug != nil {
		ns, err := NormalizeSlug(*req.Slug)
		if err != nil {
			return CreateCalendarAtPathRequest{}, "", "", nil, nil, fmt.Errorf("slug: %w", err)
		}
		normalizedSlug = &ns
	}
	var timezone *string
	if req.Timezone != nil {
		tz, err := NormalizeTimezone(*req.Timezone)
		if err != nil {
			return CreateCalendarAtPathRequest{}, "", "", nil, nil, fmt.Errorf("timezone: %w", err)
		}
		timezone = &tz
	}
	create, normalizedName, syncToken, err := ValidateCreateCalendarRequest(CreateCalendarRequest{
		UserID:          userID,
		ActorUserID:     req.ActorUserID,
		Name:            name,
		NameLang:        req.NameLang,
		Color:           req.Color,
		Description:     req.Description,
		DescriptionLang: req.DescriptionLang,
	})
	if err != nil {
		return CreateCalendarAtPathRequest{}, "", "", nil, nil, err
	}
	return CreateCalendarAtPathRequest{
		UserID:          create.UserID,
		ActorUserID:     create.ActorUserID,
		CalendarID:      calendarID,
		Name:            create.Name,
		NameLang:        create.NameLang,
		Slug:            normalizedSlug,
		Timezone:        timezone,
		Color:           create.Color,
		Description:     create.Description,
		DescriptionLang: create.DescriptionLang,
	}, normalizedName, syncToken, normalizedSlug, timezone, nil
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

func validateCalendarObjectNames(objectNames []string) ([]string, error) {
	if len(objectNames) == 0 {
		return nil, nil
	}
	normalized := make([]string, 0, len(objectNames))
	seen := make(map[string]struct{}, len(objectNames))
	for _, objectName := range objectNames {
		normalizedName, err := ValidateCalendarObjectName(objectName)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[normalizedName]; ok {
			continue
		}
		seen[normalizedName] = struct{}{}
		normalized = append(normalized, normalizedName)
	}
	return normalized, nil
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
	observedETag, err := validateOptionalETag(req.ObservedETag)
	if err != nil {
		return DeleteObjectRequest{}, "", err
	}
	syncToken := CalendarSyncToken(userID, calendarID, objectName, "delete", time.Now().UTC().Format(time.RFC3339Nano))
	return DeleteObjectRequest{UserID: userID, ActorUserID: actorUserID, CalendarID: calendarID, ObjectName: objectName, ObservedETag: observedETag}, syncToken, nil
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
	observedETag, err := validateOptionalETag(req.ObservedETag)
	if err != nil {
		return DeleteCalendarRequest{}, err
	}
	return DeleteCalendarRequest{UserID: userID, ActorUserID: actorUserID, CalendarID: calendarID, ObservedETag: observedETag}, nil
}

func ValidateUpdateCalendarRequest(req UpdateCalendarRequest) (UpdateCalendarRequest, string, string, *string, *string, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return UpdateCalendarRequest{}, "", "", nil, nil, err
	}
	actorUserID, err := validateCalDAVActorUserID(req.ActorUserID, userID)
	if err != nil {
		return UpdateCalendarRequest{}, "", "", nil, nil, err
	}
	calendarID, err := validateCalDAVID("calendar_id", req.CalendarID, true)
	if err != nil {
		return UpdateCalendarRequest{}, "", "", nil, nil, err
	}
	observedETag, err := validateOptionalETag(req.ObservedETag)
	if err != nil {
		return UpdateCalendarRequest{}, "", "", nil, nil, err
	}
	if req.Name == nil && req.Color == nil && req.Description == nil && req.Slug == nil && req.Timezone == nil {
		return UpdateCalendarRequest{}, "", "", nil, nil, fmt.Errorf("at least one calendar property is required")
	}
	var normalizedName string
	var name *string
	var nameLang *string
	if req.Name != nil {
		value, err := ValidateCalendarName(*req.Name)
		if err != nil {
			return UpdateCalendarRequest{}, "", "", nil, nil, err
		}
		normalizedName, err = NormalizeCalendarName(value)
		if err != nil {
			return UpdateCalendarRequest{}, "", "", nil, nil, err
		}
		name = &value
		valueLang, err := validateOptionalDAVPropertyLanguagePointer("displayname xml:lang", req.NameLang)
		if err != nil {
			return UpdateCalendarRequest{}, "", "", nil, nil, err
		}
		nameLang = valueLang
	}
	var normalizedSlug *string
	var slug *string
	if req.Slug != nil {
		value, err := ValidateSlug(*req.Slug)
		if err != nil {
			return UpdateCalendarRequest{}, "", "", nil, nil, fmt.Errorf("slug: %w", err)
		}
		ns, err := NormalizeSlug(value)
		if err != nil {
			return UpdateCalendarRequest{}, "", "", nil, nil, err
		}
		slug = &value
		normalizedSlug = &ns
	}
	var normalizedTimezone *string
	var timezone *string
	if req.Timezone != nil {
		value, err := ValidateTimezone(*req.Timezone)
		if err != nil {
			return UpdateCalendarRequest{}, "", "", nil, nil, fmt.Errorf("timezone: %w", err)
		}
		nt, err := NormalizeTimezone(value)
		if err != nil {
			return UpdateCalendarRequest{}, "", "", nil, nil, err
		}
		timezone = &value
		normalizedTimezone = &nt
	}
	var color *string
	if req.Color != nil {
		value, err := ValidateCalendarColor(*req.Color)
		if err != nil {
			return UpdateCalendarRequest{}, "", "", nil, nil, err
		}
		color = &value
	}
	var description *string
	var descriptionLang *string
	if req.Description != nil {
		value, err := ValidateCalendarDescription(*req.Description)
		if err != nil {
			return UpdateCalendarRequest{}, "", "", nil, nil, err
		}
		description = &value
		valueLang, err := validateOptionalDAVPropertyLanguagePointer("calendar-description xml:lang", req.DescriptionLang)
		if err != nil {
			return UpdateCalendarRequest{}, "", "", nil, nil, err
		}
		descriptionLang = valueLang
	}
	syncToken := CalendarSyncToken(userID, calendarID, "collection-update", time.Now().UTC().Format(time.RFC3339Nano))
	return UpdateCalendarRequest{UserID: userID, ActorUserID: actorUserID, CalendarID: calendarID, Name: name, NameLang: nameLang, Slug: slug, Timezone: timezone, Color: color, Description: description, DescriptionLang: descriptionLang, ObservedETag: observedETag}, normalizedName, syncToken, normalizedSlug, normalizedTimezone, nil
}

func validateDAVPropertyLanguagePointer(field string, value *string) (*string, error) {
	if value == nil {
		empty := ""
		return &empty, nil
	}
	lang, err := ValidateDAVPropertyLanguage(*value)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", field, err)
	}
	return &lang, nil
}

func validateOptionalDAVPropertyLanguagePointer(field string, value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	lang, err := ValidateDAVPropertyLanguage(*value)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", field, err)
	}
	return &lang, nil
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

func ValidateDeliverSchedulingMessageRequest(req DeliverSchedulingMessageRequest) (DeliverSchedulingMessageRequest, string, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return DeliverSchedulingMessageRequest{}, "", err
	}
	recipient := strings.TrimSpace(req.Recipient)
	if recipient == "" {
		recipient = userID
	}
	uid, err := ValidateCalendarObjectUID(req.UID)
	if err != nil {
		return DeliverSchedulingMessageRequest{}, "", err
	}
	if len(req.ICSPayload) == 0 {
		return DeliverSchedulingMessageRequest{}, "", fmt.Errorf("ICS payload is required")
	}
	if len(req.ICSPayload) > MaxCalendarObjectBytes {
		return DeliverSchedulingMessageRequest{}, "", fmt.Errorf("ICS payload exceeds maximum size")
	}
	parsed, err := ParseICalendarObject(req.ICSPayload)
	if err != nil {
		return DeliverSchedulingMessageRequest{}, "", err
	}
	if parsed.UID != uid {
		return DeliverSchedulingMessageRequest{}, "", fmt.Errorf("ICS UID does not match request UID")
	}
	method := req.Method
	if method == "" {
		method = ScheduleMethodRequest
	}
	if !isValidScheduleMethodForDelivery(method) {
		return DeliverSchedulingMessageRequest{}, "", fmt.Errorf("invalid schedule method for delivery: %s", method)
	}
	etag, _ := StrongETag(req.ICSPayload)
	return DeliverSchedulingMessageRequest{
		UserID:     userID,
		Recipient:  recipient,
		Method:     method,
		UID:        uid,
		ICSPayload: req.ICSPayload,
	}, etag, nil
}

func ValidateSendSchedulingMessageRequest(req SendSchedulingMessageRequest) (SendSchedulingMessageRequest, string, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return SendSchedulingMessageRequest{}, "", err
	}
	uid, err := ValidateCalendarObjectUID(req.UID)
	if err != nil {
		return SendSchedulingMessageRequest{}, "", err
	}
	if len(req.ICSPayload) == 0 {
		return SendSchedulingMessageRequest{}, "", fmt.Errorf("ICS payload is required")
	}
	if len(req.ICSPayload) > MaxCalendarObjectBytes {
		return SendSchedulingMessageRequest{}, "", fmt.Errorf("ICS payload exceeds maximum size")
	}
	parsed, err := ParseICalendarObject(req.ICSPayload)
	if err != nil {
		return SendSchedulingMessageRequest{}, "", err
	}
	if parsed.UID != uid {
		return SendSchedulingMessageRequest{}, "", fmt.Errorf("ICS UID does not match request UID")
	}
	method := req.Method
	if method == "" {
		method = ScheduleMethodRequest
	}
	if !isValidScheduleMethodForSend(method) {
		return SendSchedulingMessageRequest{}, "", fmt.Errorf("invalid schedule method for sending: %s", method)
	}
	etag, _ := StrongETag(req.ICSPayload)
	return SendSchedulingMessageRequest{
		UserID:     userID,
		Method:     method,
		UID:        uid,
		ICSPayload: req.ICSPayload,
	}, etag, nil
}

func isValidScheduleMethodForDelivery(method ScheduleMethod) bool {
	switch method {
	case ScheduleMethodRequest, ScheduleMethodReply, ScheduleMethodCancel,
		ScheduleMethodAdd, ScheduleMethodCounter, ScheduleMethodDeclineCounter,
		ScheduleMethodRefresh, ScheduleMethodPublish:
		return true
	default:
		return false
	}
}

func isValidScheduleMethodForSend(method ScheduleMethod) bool {
	switch method {
	case ScheduleMethodRequest, ScheduleMethodReply, ScheduleMethodCancel,
		ScheduleMethodAdd, ScheduleMethodCounter, ScheduleMethodDeclineCounter,
		ScheduleMethodPublish:
		return true
	default:
		return false
	}
}

func parseICSScheduleMethod(payload []byte) (string, error) {
	return ExtractICSMethod(payload)
}

func isRetryableCalDAVWriteError(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "40001", "40P01", "40P02", "55P03":
		return true
	default:
		return false
	}
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func runCalDAVWriteWithRetry(ctx context.Context, fn func() error) error {
	for attempt := 0; attempt < caldavObjectWriteMaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		if !isRetryableCalDAVWriteError(err) || attempt+1 >= caldavObjectWriteMaxAttempts {
			return err
		}
		delay := caldavObjectWriteBaseDelay << attempt
		if delay > caldavObjectWriteMaxDelay {
			delay = caldavObjectWriteMaxDelay
		}
		jitter := time.Duration(time.Now().UnixNano() % int64(delay))
		if err := sleepWithContext(ctx, delay+jitter); err != nil {
			return err
		}
	}
	return nil
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
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("CalDAV object not found")
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
  AND status = 'active'`, userID, calendarID, objectName).Scan(&current)
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

func ensureCalendarCollectionETag(ctx context.Context, tx *sql.Tx, userID string, calendarID string, etag string) error {
	var calendarIDInDB string
	var syncToken string
	err := tx.QueryRowContext(ctx, `
	SELECT id::text, sync_token
FROM caldav_calendars
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'`, userID, calendarID).Scan(
		&calendarIDInDB,
		&syncToken,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("CalDAV calendar not found")
		}
		return fmt.Errorf("read CalDAV calendar collection etag: %w", err)
	}
	current, err := CalendarCollectionETag(userID, Calendar{
		ID:        calendarIDInDB,
		SyncToken: syncToken,
	})
	if err != nil {
		return fmt.Errorf("build CalDAV calendar collection etag: %w", err)
	}
	if current != etag {
		return fmt.Errorf("CalDAV calendar collection etag mismatch")
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
	var hasActiveCalendar bool
	err := tx.QueryRowContext(ctx, `
WITH active_calendar AS (
  SELECT sync_token
  FROM caldav_calendars
  WHERE user_id = $1::uuid
    AND id = $2::uuid
    AND status = 'active'
),
insert_marker AS (
  INSERT INTO caldav_calendar_sync_changes (
    user_id, calendar_id, sync_token, action
  )
  SELECT $1::uuid, $2::uuid, sync_token, 'collection-created'
  FROM active_calendar
  ON CONFLICT (calendar_id, sync_token) DO NOTHING
)
SELECT EXISTS (SELECT 1 FROM active_calendar)`, userID, calendarID).Scan(&hasActiveCalendar)
	if err != nil {
		return fmt.Errorf("read CalDAV sync marker: %w", err)
	}
	if !hasActiveCalendar {
		return fmt.Errorf("CalDAV calendar not found")
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
