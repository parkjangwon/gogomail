package caldavgw

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/directory"
	"github.com/gogomail/gogomail/internal/mail"
)

var _ DiscoveryStore = (*Repository)(nil)

func (r *Repository) LookupPrincipal(ctx context.Context, userID string) (Principal, error) {
	if r == nil || r.db == nil {
		return Principal{}, fmt.Errorf("database handle is required")
	}
	userID, err := validateCalDAVID("user_id", userID, true)
	if err != nil {
		return Principal{}, err
	}
	resolved, err := directory.NewRepository(r.db).ResolvePrincipal(ctx, directory.ResolvePrincipalRequest{
		ID:         userID,
		Kind:       directory.PrincipalKindUser,
		ActiveOnly: true,
	})
	if err != nil {
		return Principal{}, fmt.Errorf("lookup CalDAV principal: %w", err)
	}
	directoryRepo := directory.NewRepository(r.db)
	aliases, err := directoryRepo.ListAliases(ctx, directory.ListAliasesRequest{
		CompanyID:  resolved.CompanyID,
		TargetKind: directory.PrincipalKindUser,
		TargetID:   resolved.ID,
		ActiveOnly: true,
		Limit:      directory.MaxAliasListLimit,
	})
	if err != nil {
		return Principal{}, fmt.Errorf("lookup CalDAV principal aliases: %w", err)
	}
	principal, err := calDAVPrincipalFromDirectory(resolved, aliases)
	if err != nil {
		return Principal{}, err
	}
	return principal, nil
}

func calDAVPrincipalFromDirectory(resolved directory.Principal, aliases ...[]directory.Alias) (Principal, error) {
	if resolved.Kind != directory.PrincipalKindUser {
		return Principal{}, fmt.Errorf("caldav principal kind %q is not supported", resolved.Kind)
	}
	principal := Principal{UserID: resolved.ID, DisplayName: resolved.DisplayName}
	principalPath, err := PrincipalPath(principal.UserID)
	if err != nil {
		return Principal{}, err
	}
	homePath, err := CalendarHomePath(principal.UserID)
	if err != nil {
		return Principal{}, err
	}
	principal.PrincipalPath = principalPath
	principal.CalendarHomePath = homePath
	inboxPath, err := ScheduleInboxPath(principal.UserID)
	if err != nil {
		return Principal{}, err
	}
	principal.ScheduleInboxPath = inboxPath
	outboxPath, err := ScheduleOutboxPath(principal.UserID)
	if err != nil {
		return Principal{}, err
	}
	principal.ScheduleOutboxPath = outboxPath
	var aliasList []directory.Alias
	if len(aliases) > 0 {
		aliasList = aliases[0]
	}
	addresses, err := calDAVCalendarUserAddresses(resolved, aliasList)
	if err != nil {
		return Principal{}, err
	}
	principal.CalendarUserAddresses = addresses
	return principal, nil
}

func calDAVCalendarUserAddresses(resolved directory.Principal, aliases []directory.Alias) ([]string, error) {
	seen := make(map[string]struct{}, len(aliases)+1)
	addresses := make([]string, 0, len(aliases)+1)
	addAddress := func(label string, value string) error {
		if strings.TrimSpace(value) == "" {
			return nil
		}
		address, err := mail.NormalizeAddress(value)
		if err != nil {
			return fmt.Errorf("caldav principal %s: %w", label, err)
		}
		href := "mailto:" + address
		if _, ok := seen[href]; ok {
			return nil
		}
		seen[href] = struct{}{}
		addresses = append(addresses, href)
		return nil
	}
	if strings.TrimSpace(resolved.PrimaryEmail) != "" {
		if err := addAddress("primary email", resolved.PrimaryEmail); err != nil {
			return nil, err
		}
	}
	for _, alias := range aliases {
		if alias.TargetKind != directory.PrincipalKindUser || alias.TargetID != resolved.ID {
			continue
		}
		if err := addAddress("alias address", alias.Address); err != nil {
			return nil, err
		}
	}
	return addresses, nil
}

func (r *Repository) ListCalendarCollections(ctx context.Context, userID string) ([]Calendar, error) {
	return r.ListCalendars(ctx, ListCalendarsRequest{UserID: userID, Status: CalendarStatusActive})
}

func (r *Repository) LookupCalendar(ctx context.Context, userID string, calendarID string) (Calendar, error) {
	return r.GetCalendar(ctx, GetCalendarRequest{UserID: userID, CalendarID: calendarID, Status: CalendarStatusActive})
}

func (r *Repository) LookupCalendarBySlug(ctx context.Context, userID string, slug string) (Calendar, error) {
	return r.GetCalendarBySlug(ctx, GetCalendarBySlugRequest{UserID: userID, Slug: slug, Status: CalendarStatusActive})
}

func (r *Repository) ListCalendarObjects(ctx context.Context, userID string, calendarID string) ([]CalendarObject, error) {
	return r.ListObjects(ctx, ListObjectsRequest{UserID: userID, CalendarID: calendarID, Status: CalendarStatusActive})
}

func (r *Repository) ListCalendarObjectsLimit(ctx context.Context, userID string, calendarID string, limit int) ([]CalendarObject, error) {
	return r.listObjectsForSync(ctx, ListObjectsRequest{UserID: userID, CalendarID: calendarID, Status: CalendarStatusActive, Limit: limit})
}

func (r *Repository) WalkCalendarQueryCandidates(ctx context.Context, userID string, calendarID string, status string, component string, yield func(CalendarObject) (bool, error)) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if yield == nil {
		return fmt.Errorf("CalDAV calendar-query candidate yield function is required")
	}
	req, err := ValidateListObjectsForSyncRequest(ListObjectsRequest{
		UserID:     userID,
		CalendarID: calendarID,
		Status:     status,
		Limit:      MaxWebDAVReportLimit + 1,
	})
	if err != nil {
		return err
	}
	component = strings.ToUpper(strings.TrimSpace(component))
	if component == "" {
		return fmt.Errorf("CalDAV calendar-query candidate component is required")
	}
	const query = `
SELECT id::text, user_id::text, calendar_id::text, object_name, uid, component_type, etag, size, ics, created_at, updated_at
FROM caldav_calendar_objects
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND status = $3
  AND component_type = $4
ORDER BY updated_at DESC, id DESC`
	rows, err := r.db.QueryContext(ctx, query, req.UserID, req.CalendarID, req.Status, component)
	if err != nil {
		return fmt.Errorf("walk CalDAV calendar-query candidates: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var object CalendarObject
		if err := rows.Scan(&object.ID, &object.UserID, &object.CalendarID, &object.ObjectName, &object.UID, &object.Component, &object.ETag, &object.Size, &object.ICS, &object.CreatedAt, &object.UpdatedAt); err != nil {
			return fmt.Errorf("scan CalDAV calendar-query candidate: %w", err)
		}
		keepGoing, err := yield(object)
		if err != nil {
			return err
		}
		if !keepGoing {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate CalDAV calendar-query candidates: %w", err)
	}
	return nil
}

func (r *Repository) LookupCalendarObject(ctx context.Context, userID string, calendarID string, objectName string) (CalendarObject, error) {
	return r.GetObject(ctx, GetObjectRequest{UserID: userID, CalendarID: calendarID, ObjectName: objectName, Status: CalendarStatusActive})
}
