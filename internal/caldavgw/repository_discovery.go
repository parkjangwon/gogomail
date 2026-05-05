package caldavgw

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	const query = `
SELECT u.id::text, u.display_name
FROM users u
JOIN domains d ON d.id = u.domain_id
JOIN companies c ON c.id = d.company_id
WHERE u.id = $1::uuid
  AND u.status = 'active'
  AND d.status = 'active'
  AND c.status = 'active'`
	var principal Principal
	if err := r.db.QueryRowContext(ctx, query, userID).Scan(&principal.UserID, &principal.DisplayName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Principal{}, fmt.Errorf("CalDAV principal not found")
		}
		return Principal{}, fmt.Errorf("lookup CalDAV principal: %w", err)
	}
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
	return principal, nil
}

func (r *Repository) ListCalendarCollections(ctx context.Context, userID string) ([]Calendar, error) {
	return r.ListCalendars(ctx, ListCalendarsRequest{UserID: userID, Status: CalendarStatusActive})
}

func (r *Repository) LookupCalendar(ctx context.Context, userID string, calendarID string) (Calendar, error) {
	return r.GetCalendar(ctx, GetCalendarRequest{UserID: userID, CalendarID: calendarID, Status: CalendarStatusActive})
}

func (r *Repository) ListCalendarObjects(ctx context.Context, userID string, calendarID string) ([]CalendarObject, error) {
	return r.ListObjects(ctx, ListObjectsRequest{UserID: userID, CalendarID: calendarID, Status: CalendarStatusActive})
}

func (r *Repository) LookupCalendarObject(ctx context.Context, userID string, calendarID string, objectName string) (CalendarObject, error) {
	return r.GetObject(ctx, GetObjectRequest{UserID: userID, CalendarID: calendarID, ObjectName: objectName, Status: CalendarStatusActive})
}
