package caldavgw

import (
	"context"
	"database/sql"
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
