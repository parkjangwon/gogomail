package caldavgw

import (
	"context"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/directory"
)

func TestRepositorySatisfiesDiscoveryStore(t *testing.T) {
	t.Parallel()

	var _ DiscoveryStore = NewRepository(nil)
}

func TestRepositoryDiscoveryMethodsRequireDatabase(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	ctx := context.Background()
	tests := []struct {
		name string
		run  func() error
	}{
		{name: "principal", run: func() error { _, err := repo.LookupPrincipal(ctx, "user-1"); return err }},
		{name: "calendars", run: func() error { _, err := repo.ListCalendarCollections(ctx, "user-1"); return err }},
		{name: "calendar", run: func() error { _, err := repo.LookupCalendar(ctx, "user-1", "calendar-1"); return err }},
		{name: "objects", run: func() error { _, err := repo.ListCalendarObjects(ctx, "user-1", "calendar-1"); return err }},
		{name: "object", run: func() error {
			_, err := repo.LookupCalendarObject(ctx, "user-1", "calendar-1", "event.ics")
			return err
		}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.run(); err == nil || !strings.Contains(err.Error(), "database handle is required") {
				t.Fatalf("error = %v, want database handle requirement", err)
			}
		})
	}
}

func TestCalDAVPrincipalFromDirectoryAcceptsUserPrincipal(t *testing.T) {
	t.Parallel()

	principal, err := calDAVPrincipalFromDirectory(directory.Principal{
		ID:          "user-1",
		Kind:        directory.PrincipalKindUser,
		DisplayName: "User One",
	})
	if err != nil {
		t.Fatalf("calDAVPrincipalFromDirectory returned error: %v", err)
	}
	if principal.UserID != "user-1" || principal.DisplayName != "User One" {
		t.Fatalf("principal = %+v", principal)
	}
	if principal.PrincipalPath != "/caldav/principals/user-1/" {
		t.Fatalf("PrincipalPath = %q", principal.PrincipalPath)
	}
	if principal.CalendarHomePath != "/caldav/calendars/user-1/" {
		t.Fatalf("CalendarHomePath = %q", principal.CalendarHomePath)
	}
}

func TestCalDAVPrincipalFromDirectoryRejectsNonUserPrincipals(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{
		directory.PrincipalKindOrganization,
		directory.PrincipalKindGroup,
		directory.PrincipalKindResource,
	} {
		kind := kind
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			_, err := calDAVPrincipalFromDirectory(directory.Principal{ID: "principal-1", Kind: kind})
			if err == nil || !strings.Contains(err.Error(), "caldav principal kind") || !strings.Contains(err.Error(), "is not supported") {
				t.Fatalf("error = %v, want unsupported CalDAV principal kind", err)
			}
		})
	}
}
