package caldavgw

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandlerOptionsAdvertisesDAVCapabilities(t *testing.T) {
	t.Parallel()

	handler := NewHandler(&fakeDiscoveryStore{}, fixedUser("user-1"))
	req := httptest.NewRequest(MethodOptions, "/caldav/principals/user-1/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("DAV"); !strings.Contains(got, DAVCalendarAccess) || !strings.Contains(got, DAVSyncCollection) {
		t.Fatalf("DAV header = %q", got)
	}
	if got := rec.Header().Get("Allow"); !strings.Contains(got, MethodPropfind) || !strings.Contains(got, MethodReport) {
		t.Fatalf("Allow header = %q", got)
	}
	if got := rec.Header().Get("Allow"); !strings.Contains(got, MethodMkcalendar) {
		t.Fatalf("Allow header does not advertise MKCALENDAR: %q", got)
	}
}

func TestHandlerPropfindPrincipalDiscovery(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(`<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><D:current-user-principal/><C:calendar-home-set/></D:prop></D:propfind>`))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<D:current-user-principal><D:href>/caldav/principals/user-1/</D:href></D:current-user-principal>",
		"<C:calendar-home-set><D:href>/caldav/calendars/user-1/</D:href></C:calendar-home-set>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("principal discovery missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerPropfindCalendarHomeDepthOne(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/", strings.NewReader(`<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><D:displayname/><D:resourcetype/><C:supported-calendar-component-set/></D:prop></D:propfind>`))
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<D:href>/caldav/calendars/user-1/</D:href>") {
		t.Fatalf("home response missing:\n%s", body)
	}
	if !strings.Contains(body, "<D:href>/caldav/calendars/user-1/work/</D:href>") {
		t.Fatalf("child calendar response missing:\n%s", body)
	}
	if !strings.Contains(body, "<C:comp name=\"VEVENT\"></C:comp>") {
		t.Fatalf("supported component response missing:\n%s", body)
	}
}

func TestHandlerPropfindCalendarCollectionDepthOne(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propfind xmlns:D="DAV:"><D:prop><D:getetag/><D:getcontenttype/><D:resourcetype/></D:prop></D:propfind>`))
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>") {
		t.Fatalf("calendar object response missing:\n%s", body)
	}
	if !strings.Contains(body, "<D:getetag>") || !strings.Contains(body, "<D:getcontenttype>text/calendar; charset=utf-8</D:getcontenttype>") {
		t.Fatalf("calendar object properties missing:\n%s", body)
	}
}

func TestHandlerReportCalendarMultiget(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<C:calendar-multiget xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/><C:calendar-data/></D:prop>
  <D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>
</C:calendar-multiget>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>",
		"<D:getetag>",
		"<C:calendar-data>BEGIN:VCALENDAR",
		"UID:event-1@example.com",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("calendar-multiget missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerReportCalendarMultigetReturnsPropertyNotFoundForMissingHref(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<C:calendar-multiget xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/><C:calendar-data/></D:prop>
  <D:href>/caldav/calendars/user-1/work/missing.ics</D:href>
</C:calendar-multiget>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "HTTP/1.1 404 Not Found") {
		t.Fatalf("missing href did not render 404 propstat:\n%s", body)
	}
}

func TestHandlerReportCalendarQueryFiltersByTimeRange(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/><C:calendar-data/></D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT">
        <C:time-range start="20260506T000000Z" end="20260507T000000Z"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>",
		"<D:getetag>",
		"<C:calendar-data>BEGIN:VCALENDAR",
		"DTSTART:20260506T010000Z",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("calendar-query missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerReportCalendarQuerySkipsNonOverlappingTimeRange(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/><C:calendar-data/></D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT">
        <C:time-range start="20260508T000000Z" end="20260509T000000Z"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "event-1.ics") {
		t.Fatalf("non-overlapping calendar-query returned event:\n%s", rec.Body.String())
	}
}

func TestHandlerReportSyncCollectionInitialSyncReturnsObjectsAndToken(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:sync-collection xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:sync-level>1</D:sync-level>
  <D:prop><D:getetag/><C:calendar-data/></D:prop>
</D:sync-collection>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>",
		"<D:getetag>",
		"<C:calendar-data>BEGIN:VCALENDAR",
		"<D:sync-token>sync-calendar</D:sync-token>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("sync-collection missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerReportSyncCollectionCurrentTokenReturnsOnlyToken(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:sync-collection xmlns:D="DAV:">
  <D:sync-token>sync-calendar</D:sync-token>
  <D:sync-level>1</D:sync-level>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "event-1.ics") {
		t.Fatalf("current-token sync-collection returned object changes:\n%s", body)
	}
	if !strings.Contains(body, "<D:sync-token>sync-calendar</D:sync-token>") {
		t.Fatalf("sync-token missing:\n%s", body)
	}
}

func TestHandlerReportSyncCollectionRejectsStaleToken(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:sync-collection xmlns:D="DAV:">
  <D:sync-token>sync-stale</D:sync-token>
  <D:sync-level>1</D:sync-level>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<D:valid-sync-token") {
		t.Fatalf("valid-sync-token precondition missing:\n%s", body)
	}
}

func TestHandlerReportSyncCollectionRejectsTruncatingLimit(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	second := store.objects[0]
	second.ID = "object-2"
	second.ObjectName = "event-2.ics"
	second.UID = "event-2@example.com"
	second.ETag = `"1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	store.objects = append(store.objects, second)

	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:sync-collection xmlns:D="DAV:">
  <D:sync-level>1</D:sync-level>
  <D:limit><D:nresults>1</D:nresults></D:limit>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "limit would truncate") {
		t.Fatalf("truncating limit error missing:\n%s", rec.Body.String())
	}
}

func TestHandlerReportFreeBusyQueryReturnsCalendarBody(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<C:free-busy-query xmlns:C="urn:ietf:params:xml:ns:caldav">
  <C:time-range start="20260506T000000Z" end="20260507T000000Z"/>
</C:free-busy-query>`))
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "text/calendar; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"BEGIN:VCALENDAR",
		"BEGIN:VFREEBUSY",
		"DTSTART:20260506T000000Z",
		"DTEND:20260507T000000Z",
		"FREEBUSY;FBTYPE=BUSY:20260506T010000Z/20260506T020000Z",
		"END:VFREEBUSY",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("free-busy response missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerReportFreeBusyQueryDepthZeroReturnsEmptyVFreeBusy(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<C:free-busy-query xmlns:C="urn:ietf:params:xml:ns:caldav">
  <C:time-range start="20260506T000000Z" end="20260507T000000Z"/>
</C:free-busy-query>`))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "BEGIN:VFREEBUSY") {
		t.Fatalf("VFREEBUSY missing:\n%s", body)
	}
	if strings.Contains(body, "FREEBUSY") && strings.Contains(body, "FBTYPE") {
		t.Fatalf("Depth: 0 free-busy returned child busy periods:\n%s", body)
	}
}

func TestHandlerReportFreeBusyQueryRejectsObjectTarget(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/event-1.ics", strings.NewReader(`<C:free-busy-query xmlns:C="urn:ietf:params:xml:ns:caldav">
  <C:time-range start="20260506T000000Z" end="20260507T000000Z"/>
</C:free-busy-query>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerReportRejectsUnsupportedReports(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:sync-collection xmlns:D="DAV:"><D:sync-level>1</D:sync-level></D:sync-collection>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerGetAndHeadCalendarObject(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	for _, method := range []string{MethodGet, MethodHead} {
		method := method
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(method, "/caldav/calendars/user-1/work/event-1.ics", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("ETag"); got == "" {
				t.Fatal("ETag header is empty")
			}
			if got := rec.Header().Get("Content-Type"); got != "text/calendar; charset=utf-8" {
				t.Fatalf("Content-Type = %q", got)
			}
			if method == MethodHead && rec.Body.Len() != 0 {
				t.Fatalf("HEAD body length = %d, want 0", rec.Body.Len())
			}
			if method == MethodGet && !strings.Contains(rec.Body.String(), "BEGIN:VCALENDAR") {
				t.Fatalf("GET body = %s", rec.Body.String())
			}
		})
	}
}

func TestHandlerPutCalendarObjectCreatesAndUpdates(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:event-2@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	req := httptest.NewRequest(MethodPut, "/caldav/calendars/user-1/work/event-2.ics", strings.NewReader(body))
	req.Header.Set("If-None-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body = %s", rec.Code, rec.Body.String())
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("created ETag is empty")
	}

	updateReq := httptest.NewRequest(MethodPut, "/caldav/calendars/user-1/work/event-2.ics", strings.NewReader(body))
	updateReq.Header.Set("If-Match", etag)
	updateRec := httptest.NewRecorder()
	handler.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusNoContent {
		t.Fatalf("update status = %d body = %s", updateRec.Code, updateRec.Body.String())
	}
}

func TestHandlerMkcalendarCreatesCalendarAtRequestURI(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	calendarID := "11111111-1111-4111-8111-111111111111"
	req := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/"+calendarID+"/", strings.NewReader(`<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:" xmlns:CS="http://calendarserver.org/ns/">
  <D:set>
    <D:prop>
      <D:displayname>Project Calendar</D:displayname>
      <C:calendar-description>Delivery milestones</C:calendar-description>
      <CS:calendar-color>#aabbcc</CS:calendar-color>
    </D:prop>
  </D:set>
</C:mkcalendar>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/caldav/calendars/user-1/"+calendarID+"/" {
		t.Fatalf("Location = %q", got)
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", calendarID)
	if err != nil {
		t.Fatalf("created calendar lookup failed: %v", err)
	}
	if calendar.Name != "Project Calendar" || calendar.Description != "Delivery milestones" || calendar.Color != "#AABBCC" {
		t.Fatalf("calendar = %+v", calendar)
	}
}

func TestHandlerMkcalendarRejectsExistingCalendar(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/work/", strings.NewReader(`<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav"/>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerMkcalendarRejectsUnsafePathID(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/not-a-uuid/", strings.NewReader(`<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav"/>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerPutRejectsFailedPreconditions(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	req := httptest.NewRequest(MethodPut, "/caldav/calendars/user-1/work/event-1.ics", strings.NewReader(body))
	req.Header.Set("If-None-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerDeleteCalendarObject(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/event-1.ics", nil)
	req.Header.Set("If-Match", store.objects[0].ETag)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if len(store.objects) != 0 {
		t.Fatalf("objects after delete = %+v", store.objects)
	}
}

func TestHandlerDeleteRejectsETagMismatch(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/event-1.ics", nil)
	req.Header.Set("If-Match", `"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerPropfindRejectsUnsafeDiscovery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		userID string
		target string
		depth  string
		want   int
	}{
		{name: "cross user", userID: "user-2", target: "/caldav/principals/user-1/", depth: "0", want: http.StatusForbidden},
		{name: "infinity", userID: "user-1", target: "/caldav/calendars/user-1/", depth: "infinity", want: http.StatusForbidden},
		{name: "bad depth", userID: "user-1", target: "/caldav/calendars/user-1/", depth: "2", want: http.StatusBadRequest},
		{name: "object depth one", userID: "user-1", target: "/caldav/calendars/user-1/work/event-1.ics", depth: "1", want: http.StatusNotFound},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := NewHandler(newFakeDiscoveryStore(), fixedUser(tc.userID))
			req := httptest.NewRequest(MethodPropfind, tc.target, strings.NewReader(`<D:propfind xmlns:D="DAV:"><D:allprop/></D:propfind>`))
			req.Header.Set("Depth", tc.depth)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func fixedUser(userID string) UserResolver {
	return func(*http.Request) (string, error) { return userID, nil }
}

type fakeDiscoveryStore struct {
	principal Principal
	calendars []Calendar
	objects   []CalendarObject
}

func newFakeDiscoveryStore() *fakeDiscoveryStore {
	now := time.Now()
	eventICS := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nDTSTAMP:20260506T000000Z\r\nDTSTART:20260506T010000Z\r\nDTEND:20260506T020000Z\r\nSUMMARY:Planning\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	return &fakeDiscoveryStore{
		principal: Principal{
			UserID:           "user-1",
			DisplayName:      "User One",
			CalendarHomePath: "/caldav/calendars/user-1/",
			PrincipalPath:    "/caldav/principals/user-1/",
		},
		calendars: []Calendar{{
			ID:          "work",
			UserID:      "user-1",
			Name:        "Work",
			Color:       "#AABBCC",
			Description: "Team calendar",
			SyncToken:   "sync-calendar",
			CreatedAt:   now,
			UpdatedAt:   now,
		}},
		objects: []CalendarObject{{
			ID:         "object-1",
			UserID:     "user-1",
			CalendarID: "work",
			ObjectName: "event-1.ics",
			UID:        "event-1@example.com",
			ETag:       `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`,
			Size:       int64(len(eventICS)),
			ICS:        eventICS,
			CreatedAt:  now,
			UpdatedAt:  now,
		}},
	}
}

func (s *fakeDiscoveryStore) LookupPrincipal(_ context.Context, userID string) (Principal, error) {
	if s.principal.UserID != userID {
		return Principal{}, errFakeNotFound
	}
	return s.principal, nil
}

func (s *fakeDiscoveryStore) ListCalendarCollections(_ context.Context, userID string) ([]Calendar, error) {
	if s.principal.UserID != userID {
		return nil, errFakeNotFound
	}
	return append([]Calendar(nil), s.calendars...), nil
}

func (s *fakeDiscoveryStore) LookupCalendar(_ context.Context, userID string, calendarID string) (Calendar, error) {
	for _, calendar := range s.calendars {
		if calendar.UserID == userID && calendar.ID == calendarID {
			return calendar, nil
		}
	}
	return Calendar{}, errFakeNotFound
}

func (s *fakeDiscoveryStore) ListCalendarObjects(_ context.Context, userID string, calendarID string) ([]CalendarObject, error) {
	if _, err := s.LookupCalendar(context.Background(), userID, calendarID); err != nil {
		return nil, err
	}
	return append([]CalendarObject(nil), s.objects...), nil
}

func (s *fakeDiscoveryStore) LookupCalendarObject(_ context.Context, userID string, calendarID string, objectName string) (CalendarObject, error) {
	for _, object := range s.objects {
		if object.UserID == userID && object.CalendarID == calendarID && object.ObjectName == objectName {
			return object, nil
		}
	}
	return CalendarObject{}, errFakeNotFound
}

func (s *fakeDiscoveryStore) UpsertObject(_ context.Context, req UpsertObjectRequest) (CalendarObject, error) {
	validated, etag, _, err := ValidateUpsertObjectRequest(req)
	if err != nil {
		return CalendarObject{}, err
	}
	now := time.Now()
	object := CalendarObject{
		ID:         "object-" + validated.ObjectName,
		UserID:     validated.UserID,
		CalendarID: validated.CalendarID,
		ObjectName: validated.ObjectName,
		UID:        validated.UID,
		ETag:       etag,
		Size:       int64(len(validated.ICS)),
		ICS:        append([]byte(nil), validated.ICS...),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	for i, existing := range s.objects {
		if existing.UserID == object.UserID && existing.CalendarID == object.CalendarID && existing.ObjectName == object.ObjectName {
			if validated.ObservedETag != "" && existing.ETag != validated.ObservedETag {
				return CalendarObject{}, errFakeNotFound
			}
			object.ID = existing.ID
			object.CreatedAt = existing.CreatedAt
			s.objects[i] = object
			return object, nil
		}
	}
	s.objects = append(s.objects, object)
	return object, nil
}

func (s *fakeDiscoveryStore) CreateCalendarAtPath(_ context.Context, req CreateCalendarAtPathRequest) (Calendar, error) {
	validated, _, syncToken, err := ValidateCreateCalendarAtPathRequest(req)
	if err != nil {
		return Calendar{}, err
	}
	for _, calendar := range s.calendars {
		if calendar.UserID == validated.UserID && calendar.ID == validated.CalendarID {
			return Calendar{}, errFakeExists
		}
	}
	now := time.Now()
	calendar := Calendar{
		ID:          validated.CalendarID,
		UserID:      validated.UserID,
		Name:        validated.Name,
		Color:       validated.Color,
		Description: validated.Description,
		SyncToken:   syncToken,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.calendars = append(s.calendars, calendar)
	return calendar, nil
}

func (s *fakeDiscoveryStore) DeleteObject(_ context.Context, req DeleteObjectRequest) (CalendarObject, error) {
	validated, _, err := ValidateDeleteObjectRequest(req)
	if err != nil {
		return CalendarObject{}, err
	}
	for i, object := range s.objects {
		if object.UserID == validated.UserID && object.CalendarID == validated.CalendarID && object.ObjectName == validated.ObjectName {
			s.objects = append(s.objects[:i], s.objects[i+1:]...)
			return object, nil
		}
	}
	return CalendarObject{}, errFakeNotFound
}

type fakeNotFoundError struct{}

func (fakeNotFoundError) Error() string { return "not found" }

var errFakeNotFound fakeNotFoundError

type fakeExistsError struct{}

func (fakeExistsError) Error() string { return "already exists" }

var errFakeExists fakeExistsError
