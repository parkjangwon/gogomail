package caldavgw

import (
	"context"
	"strings"
	"testing"
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
