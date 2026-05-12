package caldavgw

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAppleICalCalendarQueryReport(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:getetag/>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT">
        <C:time-range start="20240101T000000Z" end="20241231T235959Z"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Apple iCal calendar-query status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestAppleICalMultiGetReport(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<C:calendar-multiget xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:getetag/>
    <D:getcontenttype/>
    <C:calendar-data/>
  </D:prop>
  <D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>
</C:calendar-multiget>`
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Apple iCal multiget status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestThunderbirdLightningCalendarQueryReport(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:getetag/>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT">
        <C:time-range start="20240101T000000Z" end="20240630T235959Z"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Thunderbird calendar-query status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestThunderbirdLightningPropfindDepthOne(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:displayname/>
    <D:resourcetype/>
    <C:supported-calendar-component-set/>
    <C:calendar-description/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Thunderbird PROPFIND Depth:1 status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestDavx5CalendarMultiGetReport(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<C:calendar-multiget xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:getetag/>
    <C:calendar-data/>
  </D:prop>
  <D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>
</C:calendar-multiget>`
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("DAVx⁵ calendar-multiget status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestDavx5PrincipalDiscovery(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:CS="http://calendarserver.org/ns/">
  <D:prop>
    <D:current-user-principal/>
    <C:calendar-home-set/>
    <C:schedule-inbox-URL/>
    <C:schedule-outbox-URL/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("DAVx⁵ principal discovery status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
	respBody := rec.Body.String()
	if !strings.Contains(respBody, "<C:schedule-inbox-URL>") {
		t.Fatalf("DAVx⁵ expects schedule-inbox-URL: %s", respBody)
	}
}

func TestAppleICalWellKnownRedirect(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(http.MethodGet, "/.well-known/caldav", nil)
	req.Header.Set("User-Agent", "Mac OS X/10.15.7 CalendarAgent/425.6")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("Apple iCal well-known redirect status = %d, want %d: %s", rec.Code, http.StatusMovedPermanently, rec.Body.String())
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "/caldav/") {
		t.Fatalf("Apple iCal expects redirect to /caldav/, got: %s", location)
	}
}

func TestDavx5UserAgent(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:current-user-principal/>
    <C:calendar-home-set/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "DAVx5/4.x (2023/08/07) davdroid/2.x")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("DAVx⁵ User-Agent propfind status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestThunderbirdUserAgent(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:current-user-principal/>
    <C:calendar-home-set/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:115.0) Gecko/20100101 Thunderbird/115.0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Thunderbird User-Agent propfind status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestAppleICalCalendarColorPropfind(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:CS="http://calendarserver.org/ns/">
  <D:prop>
    <D:displayname/>
    <CS:calendar-color/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Apple iCal calendar-color propfind status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestThunderbirdProppatchCalendarColor(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propertyupdate xmlns:D="DAV:" xmlns:CS="http://calendarserver.org/ns/">
  <D:set>
    <D:prop>
      <CS:calendar-color>#FF0000</CS:calendar-color>
    </D:prop>
  </D:set>
</D:propertyupdate>`
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus && rec.Code != http.StatusForbidden {
		t.Fatalf("Thunderbird calendar-color proppatch status = %d, want 207 or 403: %s", rec.Code, rec.Body.String())
	}
}

func TestAppleICalCalendarTimezonePropfind(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	tz := "Asia/Seoul"
	store.calendars[0].Timezone = &tz
	handler := NewHandler(store, fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:displayname/>
    <C:calendar-timezone/>
    <C:supported-calendar-data/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Apple iCal timezone propfind status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
	bodyText := rec.Body.String()
	for _, want := range []string{"<C:calendar-timezone>", "BEGIN:VCALENDAR", "BEGIN:VTIMEZONE", "TZID:Asia/Seoul", "END:VCALENDAR"} {
		if !strings.Contains(bodyText, want) {
			t.Fatalf("Apple iCal timezone propfind missing %q:\n%s", want, bodyText)
		}
	}
}

func TestDavx5MaxResourceSize(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:max-resource-size/>
    <C:supported-calendar-data/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("DAVx⁵ max-resource-size propfind status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestThunderbirdCalendarDescriptionPropfind(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:displayname/>
    <C:calendar-description/>
    <C:supported-calendar-component-set/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Thunderbird description propfind status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestAppleICalSupportedCalendarData(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:supported-calendar-data/>
    <C:supported-calendar-component-sets/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Apple iCal supported-calendar-data status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestAppleICalOptionsRequests(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(http.MethodOptions, "/caldav/calendars/user-1/work/", nil)
	req.Header.Set("User-Agent", "CalendarAgent/425.6")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("Apple iCal OPTIONS status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	davHeader := rec.Header().Get("DAV")
	if !strings.Contains(davHeader, "calendar-access") {
		t.Fatalf("Apple iCal expects calendar-access in DAV header, got: %s", davHeader)
	}
}

func TestThunderbirdOptionsRequests(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(http.MethodOptions, "/caldav/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:115.0) Gecko/20100101 Thunderbird/115.0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("Thunderbird OPTIONS status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestDavx5OptionsRequests(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(http.MethodOptions, "/caldav/principals/user-1/", nil)
	req.Header.Set("User-Agent", "DAVx5/4.x (2023/08/07) davdroid/2.x")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("DAVx⁵ OPTIONS status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestAppleICalCreateEventWithUID(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	icsBody := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Apple Inc.//iCal 5.0.1//EN
BEGIN:VEVENT
UID:new-event-123
DTSTART:20240120T140000Z
DTEND:20240120T150000Z
SUMMARY:New Event
CREATED:20240115T120000Z
END:VEVENT
END:VCALENDAR`
	req := httptest.NewRequest(http.MethodPut, "/caldav/calendars/user-1/work/new-event-123.ics", strings.NewReader(icsBody))
	req.Header.Set("Content-Type", "text/calendar; charset=UTF-8")
	req.Header.Set("User-Agent", "CalendarAgent/425.6")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated && rec.Code != http.StatusNoContent {
		t.Fatalf("Apple iCal create event status = %d, want 201/204: %s", rec.Code, rec.Body.String())
	}
}

func TestThunderbirdCreateEventWithUID(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	icsBody := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Mozilla Thunderbird//EN
BEGIN:VEVENT
UID:thunderbird-event-456
DTSTART:20240125T090000Z
DTEND:20240125T100000Z
SUMMARY:Thunderbird Event
CREATED:20240120T080000Z
END:VEVENT
END:VCALENDAR`
	req := httptest.NewRequest(http.MethodPut, "/caldav/calendars/user-1/work/thunderbird-event-456.ics", strings.NewReader(icsBody))
	req.Header.Set("Content-Type", "text/calendar; method=PUBLISH")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:115.0) Gecko/20100101 Thunderbird/115.0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated && rec.Code != http.StatusNoContent {
		t.Fatalf("Thunderbird create event status = %d, want 201/204: %s", rec.Code, rec.Body.String())
	}
}

func TestAppleICalSyncCollectionReport(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<sync-collection xmlns="DAV:">
  <sync-token>sync-123</sync-token>
  <sync-level>1</sync-level>
  <D:prop>
    <D:getetag/>
  </D:prop>
</sync-collection>`
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus && rec.Code != http.StatusBadRequest {
		t.Fatalf("Apple iCal sync-collection status = %d, want 207 or 400: %s", rec.Code, rec.Body.String())
	}
}

func TestDavx5SyncCollectionWithToken(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<sync-collection xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <sync-token>sync-123</sync-token>
  <sync-level>1</sync-level>
  <D:prop>
    <D:getetag/>
  </D:prop>
</sync-collection>`
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus && rec.Code != http.StatusBadRequest {
		t.Fatalf("DAVx⁵ sync-collection status = %d, want 207 or 400: %s", rec.Code, rec.Body.String())
	}
}

func TestAppleICalCalendarRootDiscovery(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:current-user-principal/>
    <D:principal-collection-set/>
    <D:resourcetype/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "CalendarAgent/425.6")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Apple iCal calendar root discovery status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestDavx5SyncTokenRequest(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:sync-token/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "DAVx5/4.x (2023/08/07) davdroid/2.x")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("DAVx⁵ sync-token request status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestAppleICalDepthInfinity(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:displayname/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "infinity")
	req.Header.Set("User-Agent", "CalendarAgent/425.6")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus && rec.Code != http.StatusForbidden {
		t.Fatalf("Apple iCal Depth:infinity status = %d, want 207 or 403: %s", rec.Code, rec.Body.String())
	}
}

func TestThunderbirdDepthZero(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-home-set/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:115.0) Gecko/20100101 Thunderbird/115.0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Thunderbird Depth:0 status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestDavx5ContentTypeApplicationXML(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:current-user-principal/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("User-Agent", "DAVx5/4.x (2023/08/07) davdroid/2.x")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("DAVx⁵ application/xml content-type status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestAppleICalTextCalendarContentType(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	icsBody := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Apple Inc.//iCal 5.0.1//EN
BEGIN:VEVENT
UID:test-content-type
DTSTART:20240130T100000Z
DTEND:20240130T110000Z
SUMMARY:Content Type Test
END:VEVENT
END:VCALENDAR`
	req := httptest.NewRequest(http.MethodPut, "/caldav/calendars/user-1/work/test-content-type.ics", strings.NewReader(icsBody))
	req.Header.Set("Content-Type", "text/calendar; charset=UTF-8")
	req.Header.Set("User-Agent", "CalendarAgent/425.6")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated && rec.Code != http.StatusNoContent {
		t.Fatalf("Apple iCal text/calendar content-type status = %d, want 201/204: %s", rec.Code, rec.Body.String())
	}
}

func TestAppleICalResourceTypeCollection(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:resourcetype/>
    <D:displayname/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "CalendarAgent/425.6")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Apple iCal resourcetype status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
	respBody := rec.Body.String()
	if !strings.Contains(respBody, "<D:collection>") {
		t.Fatalf("Apple iCal expects collection resourcetype: %s", respBody)
	}
}

func TestThunderbirdGetLastModified(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:getlastmodified/>
    <D:getetag/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/event-1.ics", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:115.0) Gecko/20100101 Thunderbird/115.0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Thunderbird getlastmodified status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestAppleICalOwnerProperty(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:owner/>
    <D:current-user-privilege-set/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "CalendarAgent/425.6")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Apple iCal owner property status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestThunderbirdSupportedReportSet(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:supported-report-set/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:115.0) Gecko/20100101 Thunderbird/115.0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Thunderbird supported-report-set status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestDavx5CreationDate(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:creationdate/>
    <D:getlastmodified/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/carddav/addressbooks/user-1/contacts/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "DAVx5/4.x (2023/08/07) davdroid/2.x")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus && rec.Code != http.StatusNotFound {
		t.Fatalf("DAVx⁵ creationdate status = %d, want 207 or 404: %s", rec.Code, rec.Body.String())
	}
}

func TestAppleICalProxyForSupport(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:CS="http://calendarserver.org/ns/">
  <D:prop>
    <CS:calendar-proxy-read-for/>
    <CS:calendar-proxy-write-for/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "CalendarAgent/425.6")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Apple iCal proxy support status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestThunderbirdResourceId(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:resource-id/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:115.0) Gecko/20100101 Thunderbird/115.0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Thunderbird resource-id status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestDavx5ExplicitComponentSet(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:supported-calendar-component-set/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "DAVx5/4.x (2023/08/07) davdroid/2.x")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("DAVx⁵ supported-calendar-component-set status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestAppleICalMinMaxResourceSize(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:max-resource-size/>
    <C:min-resource-size/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "CalendarAgent/425.6")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Apple iCal resource size status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestThunderbirdCalendarFreeBusySet(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-free-busy-set/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:115.0) Gecko/20100101 Thunderbird/115.0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Thunderbird calendar-free-busy-set status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestDavx5ScheduleInboxOutbox(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:schedule-inbox-URL/>
    <C:schedule-outbox-URL/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "DAVx5/4.x (2023/08/07) davdroid/2.x")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("DAVx⁵ schedule inbox/outbox status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestAppleICalCalendarUserAddressSet(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-user-address-set/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "CalendarAgent/425.6")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Apple iCal calendar-user-address-set status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestThunderbirdBulkFetchReports(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<C:calendar-multiget xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:getetag/>
    <D:getcontenttype/>
    <C:calendar-data/>
  </D:prop>
  <D:href>/caldav/calendars/user-1/work/event-a.ics</D:href>
  <D:href>/caldav/calendars/user-1/work/event-b.ics</D:href>
  <D:href>/caldav/calendars/user-1/work/event-c.ics</D:href>
</C:calendar-multiget>`
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:115.0) Gecko/20100101 Thunderbird/115.0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Thunderbird bulk fetch status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestAppleICalCalendarHomeDepthOne(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:CS="http://calendarserver.org/ns/">
  <D:prop>
    <D:displayname/>
    <D:resourcetype/>
    <C:supported-calendar-component-set/>
    <CS:calendar-color/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "1")
	req.Header.Set("User-Agent", "CalendarAgent/425.6")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Apple iCal calendar-home Depth:1 status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestAppleICalAllPropRequest(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<propfind xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <allprop/>
</propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "CalendarAgent/425.6")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Apple iCal allprop status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestThunderbirdPropNameRequest(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<propname xmlns="DAV:"/>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:115.0) Gecko/20100101 Thunderbird/115.0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus && rec.Code != http.StatusBadRequest {
		t.Fatalf("Thunderbird propname status = %d, want 207 or 400: %s", rec.Code, rec.Body.String())
	}
}

func TestThunderbirdPrincipalURL(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:principal-URL/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:115.0) Gecko/20100101 Thunderbird/115.0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Thunderbird principal-URL status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
	respBody := rec.Body.String()
	if !strings.Contains(respBody, "<D:principal-URL>") {
		t.Fatalf("Thunderbird expects principal-URL: %s", respBody)
	}
}

func TestDavx5PrincipalCollectionSet(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:principal-collection-set/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("User-Agent", "DAVx5/4.x (2023/08/07) davdroid/2.x")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("DAVx⁵ principal-collection-set status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestDavx5PreferHeader(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:current-user-principal/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("Prefer", "return=representation")
	req.Header.Set("Accept", "application/xml")
	req.Header.Set("User-Agent", "DAVx5/4.x (2023/08/07) davdroid/2.x")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("DAVx⁵ Prefer header status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestAppleICalBriefHeader(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:displayname/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("Prefer", "return=minimal")
	req.Header.Set("User-Agent", "CalendarAgent/425.6")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Apple iCal Prefer:minimal status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestThunderbirdBriefHeader(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:displayname/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("Brief", "t")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:115.0) Gecko/20100101 Thunderbird/115.0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Thunderbird Brief:t status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestDavx5TimeoutHeader(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:current-user-principal/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("Timeout", "Second-3600")
	req.Header.Set("User-Agent", "DAVx5/4.x (2023/08/07) davdroid/2.x")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("DAVx⁵ Timeout header status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestAppleICalCORSHeaders(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(http.MethodOptions, "/caldav/calendars/user-1/work/", nil)
	req.Header.Set("Access-Control-Request-Method", "PROPFIND")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Depth, Authorization")
	req.Header.Set("Origin", "https://www.icloud.com")
	req.Header.Set("User-Agent", "CalendarAgent/425.6")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Fatalf("Apple iCal CORS preflight status = %d, want 204/200: %s", rec.Code, rec.Body.String())
	}
}

func TestDavx5AcceptHeader(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:current-user-principal/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("Accept", "application/xml, text/xml")
	req.Header.Set("User-Agent", "DAVx5/4.x (2023/08/07) davdroid/2.x")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("DAVx⁵ Accept:application/xml status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestThunderbirdAcceptHeader(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:current-user-principal/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:115.0) Gecko/20100101 Thunderbird/115.0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Thunderbird Accept:*/* status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestDavx5CacheControl(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:current-user-principal/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", "DAVx5/4.x (2023/08/07) davdroid/2.x")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("DAVx⁵ Cache-Control:no-cache status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}

func TestAppleICalDateHeader(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:current-user-principal/>
  </D:prop>
</D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(body))
	req.Header.Set("Depth", "0")
	req.Header.Set("Date", "Mon, 08 Jan 2024 12:00:00 GMT")
	req.Header.Set("User-Agent", "CalendarAgent/425.6")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("Apple iCal Date header status = %d, want %d: %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
}
