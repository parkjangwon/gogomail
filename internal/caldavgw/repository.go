package caldavgw

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

type CreateCalendarRequest struct {
	UserID      string
	Name        string
	Color       string
	Description string
}

type CreateCalendarAtPathRequest struct {
	UserID      string
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
	UserID     string
	CalendarID string
	ObjectName string
}

type DeleteCalendarRequest struct {
	UserID     string
	CalendarID string
}

func (r *Repository) CreateCalendar(ctx context.Context, req CreateCalendarRequest) (Calendar, error) {
	if r == nil || r.db == nil {
		return Calendar{}, fmt.Errorf("database handle is required")
	}
	req, normalizedName, syncToken, err := ValidateCreateCalendarRequest(req)
	if err != nil {
		return Calendar{}, err
	}
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
	err = r.db.QueryRowContext(ctx, query,
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
	err = r.db.QueryRowContext(ctx, query,
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
	if req.ObservedETag != "" {
		if err := ensureObjectETag(ctx, tx, req.UserID, req.CalendarID, req.ObjectName, req.ObservedETag); err != nil {
			return CalendarObject{}, err
		}
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
RETURNING id::text, user_id::text, calendar_id::text, object_name, uid, etag, size, ics, created_at, updated_at`
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
		&object.ETag,
		&object.Size,
		&object.ICS,
		&object.CreatedAt,
		&object.UpdatedAt,
	)
	if err != nil {
		return CalendarObject{}, fmt.Errorf("upsert CalDAV object: %w", err)
	}
	if err := updateCalendarSyncToken(ctx, tx, req.UserID, req.CalendarID, syncToken); err != nil {
		return CalendarObject{}, err
	}
	if err := tx.Commit(); err != nil {
		return CalendarObject{}, fmt.Errorf("commit CalDAV object upsert: %w", err)
	}
	return object, nil
}

func (r *Repository) ListObjects(ctx context.Context, req ListObjectsRequest) ([]CalendarObject, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateListObjectsRequest(req)
	if err != nil {
		return nil, err
	}
	const query = `
SELECT id::text, user_id::text, calendar_id::text, object_name, uid, etag, size, ics, created_at, updated_at
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
		if err := rows.Scan(&object.ID, &object.UserID, &object.CalendarID, &object.ObjectName, &object.UID, &object.ETag, &object.Size, &object.ICS, &object.CreatedAt, &object.UpdatedAt); err != nil {
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
SELECT id::text, user_id::text, calendar_id::text, object_name, uid, etag, size, ics, created_at, updated_at
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
	const query = `
UPDATE caldav_calendar_objects
SET status = 'deleted', deleted_at = now(), updated_at = now()
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND object_name = $3
  AND status = 'active'
RETURNING id::text, user_id::text, calendar_id::text, object_name, uid, etag, size, ics, created_at, updated_at`
	var object CalendarObject
	err = tx.QueryRowContext(ctx, query, req.UserID, req.CalendarID, req.ObjectName).Scan(
		&object.ID,
		&object.UserID,
		&object.CalendarID,
		&object.ObjectName,
		&object.UID,
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
	if err := tx.Commit(); err != nil {
		return Calendar{}, fmt.Errorf("commit CalDAV calendar delete: %w", err)
	}
	return calendar, nil
}

func ValidateCreateCalendarRequest(req CreateCalendarRequest) (CreateCalendarRequest, string, string, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
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
	return CreateCalendarRequest{UserID: userID, Name: name, Color: color, Description: description}, normalizedName, syncToken, nil
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
		Name:        name,
		Color:       req.Color,
		Description: req.Description,
	})
	if err != nil {
		return CreateCalendarAtPathRequest{}, "", "", err
	}
	return CreateCalendarAtPathRequest{
		UserID:      create.UserID,
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
	return UpsertObjectRequest{UserID: userID, CalendarID: calendarID, ObjectName: objectName, UID: uid, Component: component, ICS: req.ICS, ObservedETag: observedETag}, etag, syncToken, nil
}

func ValidateListObjectsRequest(req ListObjectsRequest) (ListObjectsRequest, error) {
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
	return ListObjectsRequest{UserID: userID, CalendarID: calendarID, Status: status, Limit: normalizeCalDAVLimit(req.Limit)}, nil
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
	calendarID, err := validateCalDAVID("calendar_id", req.CalendarID, true)
	if err != nil {
		return DeleteObjectRequest{}, "", err
	}
	objectName, err := ValidateCalendarObjectName(req.ObjectName)
	if err != nil {
		return DeleteObjectRequest{}, "", err
	}
	syncToken := CalendarSyncToken(userID, calendarID, objectName, "delete", time.Now().UTC().Format(time.RFC3339Nano))
	return DeleteObjectRequest{UserID: userID, CalendarID: calendarID, ObjectName: objectName}, syncToken, nil
}

func ValidateDeleteCalendarRequest(req DeleteCalendarRequest) (DeleteCalendarRequest, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return DeleteCalendarRequest{}, err
	}
	calendarID, err := validateCalDAVID("calendar_id", req.CalendarID, true)
	if err != nil {
		return DeleteCalendarRequest{}, err
	}
	return DeleteCalendarRequest{UserID: userID, CalendarID: calendarID}, nil
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

func validateOptionalETag(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	return ValidateStrongETag(value)
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
