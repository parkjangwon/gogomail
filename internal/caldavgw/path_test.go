package caldavgw

import (
	"strings"
	"testing"
)

func TestParseResourcePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want ResourcePath
	}{
		{path: "/.well-known/caldav", want: ResourcePath{Kind: ResourceWellKnown}},
		{path: "/caldav/", want: ResourcePath{Kind: ResourceRoot}},
		{path: "/caldav/principals/", want: ResourcePath{Kind: ResourcePrincipalCollection}},
		{path: "/caldav/principals/user-1/", want: ResourcePath{Kind: ResourcePrincipal, UserID: "user-1"}},
		{path: "/caldav/calendars/user-1/", want: ResourcePath{Kind: ResourceCalendarHome, UserID: "user-1"}},
		{path: "/caldav/calendars/user-1/work/", want: ResourcePath{Kind: ResourceCalendarCollection, UserID: "user-1", CalendarID: "work"}},
		{path: "/caldav/calendars/user-1/work/event-1.ics", want: ResourcePath{Kind: ResourceCalendarObject, UserID: "user-1", CalendarID: "work", ObjectName: "event-1.ics"}},
		{path: "/caldav/calendars/user%201/work/event%201.ics", want: ResourcePath{Kind: ResourceCalendarObject, UserID: "user 1", CalendarID: "work", ObjectName: "event 1.ics"}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()

			got, err := ParseResourcePath(tc.path)
			if err != nil {
				t.Fatalf("ParseResourcePath(%q) returned error: %v", tc.path, err)
			}
			if got != tc.want {
				t.Fatalf("ParseResourcePath(%q) = %+v, want %+v", tc.path, got, tc.want)
			}
		})
	}
}

func TestParseResourcePathRejectsUnsafeOrUnsupportedPaths(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"",
		"/",
		"/caldav/../admin",
		"/caldav/principals/../user-1",
		"/caldav/principals/user\n1",
		"/caldav/calendars/user-1/work/event-1.txt",
		"/caldav/calendars/user-1/work/event-1.ics/extra",
		"/api/v1/calendar",
		"/caldav/calendars/" + strings.Repeat("u", maxSegmentBytes+1),
	} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseResourcePath(value); err == nil {
				t.Fatalf("ParseResourcePath(%q) error = nil, want rejection", value)
			}
		})
	}
}

func TestParseResourceHrefAcceptsAbsoluteURIPath(t *testing.T) {
	t.Parallel()

	got, err := ParseResourceHref("https://calendar.example.test/caldav/calendars/user-1/work/event-1.ics")
	if err != nil {
		t.Fatalf("ParseResourceHref returned error: %v", err)
	}
	want := ResourcePath{Kind: ResourceCalendarObject, UserID: "user-1", CalendarID: "work", ObjectName: "event-1.ics"}
	if got != want {
		t.Fatalf("ParseResourceHref = %+v, want %+v", got, want)
	}
}

func TestParseResourceHrefRejectsQueryAndFragment(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"https://calendar.example.test/caldav/calendars/user-1/work/event-1.ics?download=1",
		"https://calendar.example.test/caldav/calendars/user-1/work/event-1.ics#part",
		"mailto:user@example.test",
		"file:/caldav/calendars/user-1/work/event-1.ics",
		"https://user:pass@calendar.example.test/caldav/calendars/user-1/work/event-1.ics",
	} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseResourceHref(value); err == nil {
				t.Fatalf("ParseResourceHref(%q) error = nil, want rejection", value)
			}
		})
	}
}

func TestBuildCalDAVPaths(t *testing.T) {
	t.Parallel()

	principal, err := PrincipalPath("user 1")
	if err != nil {
		t.Fatalf("PrincipalPath returned error: %v", err)
	}
	if principal != "/caldav/principals/user%201/" {
		t.Fatalf("principal path = %q", principal)
	}
	object, err := CalendarObjectPath("user 1", "work", "event 1.ics")
	if err != nil {
		t.Fatalf("CalendarObjectPath returned error: %v", err)
	}
	if object != "/caldav/calendars/user%201/work/event%201.ics" {
		t.Fatalf("object path = %q", object)
	}
	if _, err := CalendarObjectPath("user-1", "work", "event.txt"); err == nil {
		t.Fatal("CalendarObjectPath accepted non-ICS object")
	}
}

func TestAdvertisedDAVTokens(t *testing.T) {
	t.Parallel()

	withoutSync := AdvertisedDAVTokens(false, false)
	if strings.Join(withoutSync, ",") != "1,3,calendar-access" {
		t.Fatalf("DAV tokens without sync = %v", withoutSync)
	}
	withScheduling := AdvertisedDAVTokens(true, true)
	if got := withScheduling[len(withScheduling)-1]; got != DAVCalendarSchedule {
		t.Fatalf("last scheduling token = %q, want %q", got, DAVCalendarSchedule)
	}
	if len(Standards()) < 7 {
		t.Fatalf("Standards = %v, want RFC coverage list", Standards())
	}
}

func TestImplementedMethodsExcludeFutureMove(t *testing.T) {
	t.Parallel()

	methods := ImplementedMethods()
	want := []string{MethodOptions, MethodPropfind, MethodProppatch, MethodReport, MethodMkcalendar, MethodGet, MethodHead, MethodPut, MethodDelete}
	if strings.Join(methods, ",") != strings.Join(want, ",") {
		t.Fatalf("ImplementedMethods = %v, want %v", methods, want)
	}
	if strings.Contains(strings.Join(methods, ","), MethodMove) {
		t.Fatalf("ImplementedMethods advertised future MOVE method: %v", methods)
	}
}
