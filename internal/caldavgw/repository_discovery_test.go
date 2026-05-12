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
	var _ CalendarQueryCandidateWalker = NewRepository(nil)
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
		{name: "query candidates", run: func() error {
			return repo.WalkCalendarQueryCandidates(ctx, "user-1", "calendar-1", CalendarStatusActive, ComponentVEVENT, func(CalendarObject) (bool, error) { return false, nil })
		}},
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
		ID:           "user-1",
		Kind:         directory.PrincipalKindUser,
		DisplayName:  "User One",
		PrimaryEmail: " User.One@Example.COM ",
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
	if len(principal.CalendarUserAddresses) != 1 || principal.CalendarUserAddresses[0] != "mailto:user.one@example.com" {
		t.Fatalf("CalendarUserAddresses = %+v", principal.CalendarUserAddresses)
	}
}

func TestCalDAVPrincipalFromDirectoryAddsUserAliases(t *testing.T) {
	t.Parallel()

	principal, err := calDAVPrincipalFromDirectory(directory.Principal{
		ID:           "user-1",
		Kind:         directory.PrincipalKindUser,
		DisplayName:  "User One",
		PrimaryEmail: "user.one@example.com",
	}, []directory.Alias{
		{Address: "team@example.com", TargetKind: directory.PrincipalKindUser, TargetID: "user-1"},
		{Address: "USER.ONE@example.com", TargetKind: directory.PrincipalKindUser, TargetID: "user-1"},
		{Address: "room@example.com", TargetKind: directory.PrincipalKindResource, TargetID: "user-1"},
		{Address: "other@example.com", TargetKind: directory.PrincipalKindUser, TargetID: "user-2"},
	})
	if err != nil {
		t.Fatalf("calDAVPrincipalFromDirectory returned error: %v", err)
	}
	want := []string{"mailto:user.one@example.com", "mailto:team@example.com"}
	if len(principal.CalendarUserAddresses) != len(want) {
		t.Fatalf("CalendarUserAddresses = %+v, want %+v", principal.CalendarUserAddresses, want)
	}
	for i := range want {
		if principal.CalendarUserAddresses[i] != want[i] {
			t.Fatalf("CalendarUserAddresses = %+v, want %+v", principal.CalendarUserAddresses, want)
		}
	}
}

func TestCalDAVPrincipalFromDirectoryRejectsMalformedPrimaryEmail(t *testing.T) {
	t.Parallel()

	_, err := calDAVPrincipalFromDirectory(directory.Principal{
		ID:           "user-1",
		Kind:         directory.PrincipalKindUser,
		PrimaryEmail: "user@example.com\r\nbcc@example.net",
	})
	if err == nil || !strings.Contains(err.Error(), "primary email") {
		t.Fatalf("error = %v, want primary email rejection", err)
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
