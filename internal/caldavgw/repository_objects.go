package caldavgw

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

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
