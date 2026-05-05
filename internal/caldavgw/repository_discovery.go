package caldavgw

import (
	"context"
	"fmt"

	"github.com/gogomail/gogomail/internal/directory"
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
