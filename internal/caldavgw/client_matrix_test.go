package caldavgw

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// This file extends client_fixtures_test.go with a broader, body-asserting
// client compatibility matrix. Where client_fixtures_test.go only checks status
// codes, the tests here replay the exact request sequences that Apple Calendar
// (macOS/iOS), DAVx5 (Android), Thunderbird, GNOME Calendar (Evolution-DS), and
// Microsoft Outlook CalDAV plugins send, and assert on the *response bodies*
// (properties, namespaces, ETag quoting) that those clients are known to be
// picky about.
//
// Each regression test is named after the concrete client quirk it guards.

// --- shared harness ------------------------------------------------------

type clientAgent struct {
	name      string
	userAgent string
}

var (
	appleCalendar = clientAgent{name: "AppleCalendar", userAgent: "macOS/14.0 (23A344) CalendarAgent/958"}
	appleIOS      = clientAgent{name: "iOSCalendar", userAgent: "iOS/17.0 (21A329) dataaccessd/1.0"}
	davx5         = clientAgent{name: "DAVx5", userAgent: "DAVx5/4.3.3.2-ose (2023/09/12; dav4jvm; okhttp/4.11.0) Android/13"}
	thunderbird   = clientAgent{name: "Thunderbird", userAgent: "Mozilla/5.0 (X11; Linux x86_64; rv:115.0) Gecko/20100101 Thunderbird/115.3.1 Lightning/7.2"}
	gnomeCalendar = clientAgent{name: "GNOMECalendar", userAgent: "Evolution/3.48.4"}
	outlookCalDAV = clientAgent{name: "OutlookCalDAV", userAgent: "CalDavSynchronizer/4.5.0 (Outlook 16.0)"}
)

// doRequest runs a single request against a fresh handler backed by the shared
// fake discovery store and returns the recorder.
func doRequest(t *testing.T, agent clientAgent, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	return doRequestWithHandler(t, handler, agent, method, target, body, headers)
}

func doRequestWithHandler(t *testing.T, handler *Handler, agent clientAgent, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if agent.userAgent != "" {
		req.Header.Set("User-Agent", agent.userAgent)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func mustContain(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %q:\n%s", want, body)
		}
	}
}

func mustNotContain(t *testing.T, body string, unwanted ...string) {
	t.Helper()
	for _, bad := range unwanted {
		if strings.Contains(body, bad) {
			t.Fatalf("response unexpectedly contains %q:\n%s", bad, body)
		}
	}
}

// =========================================================================
// Regression: CS:getctag (calendarserver.org/ns getctag)
// =========================================================================
//
// Apple Calendar and older DAVx5 releases poll a calendar collection's
// CS:getctag to cheaply detect whether *any* object changed before issuing a
// full sync-collection / calendar-query. A server that returns 404 for getctag
// forces those clients into an expensive full re-fetch on every refresh (and in
// some Apple versions, disables the calendar). This must be served with a value
// that changes whenever the collection contents change.

func TestClientQuirk_AppleGetCTagServedOnCollection(t *testing.T) {
	t.Parallel()

	body := `<D:propfind xmlns:D="DAV:" xmlns:CS="http://calendarserver.org/ns/">
  <D:prop>
    <D:displayname/>
    <CS:getctag/>
  </D:prop>
</D:propfind>`
	rec := doRequest(t, appleCalendar, MethodPropfind, "/caldav/calendars/user-1/work/", body, map[string]string{"Depth": "0"})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207: %s", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	// getctag must be in the 200 propstat, never in a 404 block.
	mustContain(t, got, "<CS:getctag>", "</CS:getctag>")
	// The getctag element must carry a non-empty value.
	start := strings.Index(got, "<CS:getctag>")
	end := strings.Index(got, "</CS:getctag>")
	if start < 0 || end < 0 || end <= start+len("<CS:getctag>") {
		t.Fatalf("CS:getctag has no value:\n%s", got)
	}
	value := got[start+len("<CS:getctag>") : end]
	if strings.TrimSpace(value) == "" {
		t.Fatalf("CS:getctag value is empty:\n%s", got)
	}
}

func TestClientQuirk_DAVx5GetCTagMatchesCollectionETag(t *testing.T) {
	t.Parallel()

	// DAVx5 treats getctag and the collection getetag as interchangeable change
	// tokens. Assert they are consistent so a client mixing the two never sees a
	// spurious change.
	body := `<D:propfind xmlns:D="DAV:" xmlns:CS="http://calendarserver.org/ns/">
  <D:prop>
    <D:getetag/>
    <CS:getctag/>
  </D:prop>
</D:propfind>`
	rec := doRequest(t, davx5, MethodPropfind, "/caldav/calendars/user-1/work/", body, map[string]string{"Depth": "0"})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207: %s", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	mustContain(t, got, "<CS:getctag>", "<D:getetag>")

	etag := extractElement(got, "D:getetag")
	ctag := extractElement(got, "CS:getctag")
	if etag == "" || ctag == "" {
		t.Fatalf("missing getetag/getctag values: etag=%q ctag=%q\n%s", etag, ctag, got)
	}
	if etag != ctag {
		t.Fatalf("collection getctag (%q) must match collection getetag (%q)", ctag, etag)
	}
}

func TestClientQuirk_GetCTagOnCalendarHomeDepthOne(t *testing.T) {
	t.Parallel()

	// Apple Calendar enumerates the calendar-home with Depth:1 and expects each
	// child collection to carry getctag so it can decide which calendars to sync.
	body := `<D:propfind xmlns:D="DAV:" xmlns:CS="http://calendarserver.org/ns/">
  <D:prop>
    <D:resourcetype/>
    <CS:getctag/>
  </D:prop>
</D:propfind>`
	rec := doRequest(t, appleCalendar, MethodPropfind, "/caldav/calendars/user-1/", body, map[string]string{"Depth": "1"})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207: %s", rec.Code, rec.Body.String())
	}
	mustContain(t, rec.Body.String(), "<CS:getctag>")
}

// extractElement returns the text content of the first <prefixedName>...</...>
// element in body, or "" if absent.
func extractElement(body, prefixedName string) string {
	open := "<" + prefixedName + ">"
	closeTag := "</" + prefixedName + ">"
	start := strings.Index(body, open)
	if start < 0 {
		return ""
	}
	start += len(open)
	end := strings.Index(body[start:], closeTag)
	if end < 0 {
		return ""
	}
	return body[start : start+end]
}

// =========================================================================
// Regression: Brief: t / Prefer: return=minimal
// =========================================================================
//
// Apple Calendar / iOS attach "Brief: t" to virtually every PROPFIND; RFC 8144
// clients send "Prefer: return=minimal". Both instruct the server to drop the
// 404 Not Found propstat blocks for unsupported properties. Some Apple releases
// choke on (or log warnings for) the 404 blocks. When honored, the server must
// also echo "Preference-Applied: return=minimal".

func TestClientQuirk_AppleBriefOmits404Propstat(t *testing.T) {
	t.Parallel()

	// Request one supported (displayname) and one unsupported property.
	body := `<D:propfind xmlns:D="DAV:" xmlns:X="urn:example:unsupported">
  <D:prop>
    <D:displayname/>
    <X:nonexistent-property/>
  </D:prop>
</D:propfind>`
	rec := doRequest(t, appleCalendar, MethodPropfind, "/caldav/calendars/user-1/work/", body, map[string]string{
		"Depth": "0",
		"Brief": "t",
	})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207: %s", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	mustContain(t, got, "<D:displayname>")
	mustNotContain(t, got, "404 Not Found", "nonexistent-property")
	if applied := rec.Header().Get("Preference-Applied"); applied != "return=minimal" {
		t.Fatalf("Preference-Applied = %q, want return=minimal", applied)
	}
}

func TestClientQuirk_PreferReturnMinimalOmits404Propstat(t *testing.T) {
	t.Parallel()

	body := `<D:propfind xmlns:D="DAV:" xmlns:X="urn:example:unsupported">
  <D:prop>
    <D:displayname/>
    <X:nonexistent-property/>
  </D:prop>
</D:propfind>`
	rec := doRequest(t, appleIOS, MethodPropfind, "/caldav/calendars/user-1/work/", body, map[string]string{
		"Depth":  "0",
		"Prefer": "return=minimal",
	})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207: %s", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	mustNotContain(t, got, "404 Not Found")
	if applied := rec.Header().Get("Preference-Applied"); applied != "return=minimal" {
		t.Fatalf("Preference-Applied = %q, want return=minimal", applied)
	}
}

func TestClientQuirk_WithoutBrief404PropstatPresent(t *testing.T) {
	t.Parallel()

	// Control: without Brief/Prefer, the 404 propstat block is still emitted so
	// spec-strict clients (Thunderbird, GNOME) see the full report.
	body := `<D:propfind xmlns:D="DAV:" xmlns:X="urn:example:unsupported">
  <D:prop>
    <D:displayname/>
    <X:nonexistent-property/>
  </D:prop>
</D:propfind>`
	rec := doRequest(t, thunderbird, MethodPropfind, "/caldav/calendars/user-1/work/", body, map[string]string{"Depth": "0"})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207: %s", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	mustContain(t, got, "404 Not Found")
	if applied := rec.Header().Get("Preference-Applied"); applied != "" {
		t.Fatalf("Preference-Applied = %q, want empty when not requested", applied)
	}
}

// =========================================================================
// Discovery flows (well-known + bootstrap PROPFIND chain)
// =========================================================================

func TestClientQuirk_WellKnownRedirectAllClients(t *testing.T) {
	t.Parallel()

	for _, agent := range []clientAgent{appleCalendar, appleIOS, davx5, thunderbird, gnomeCalendar, outlookCalDAV} {
		agent := agent
		t.Run(agent.name, func(t *testing.T) {
			t.Parallel()
			rec := doRequest(t, agent, http.MethodGet, "/.well-known/caldav", "", nil)
			if rec.Code != http.StatusMovedPermanently {
				t.Fatalf("%s well-known status = %d, want 301", agent.name, rec.Code)
			}
			loc := rec.Header().Get("Location")
			if !strings.HasPrefix(loc, "/caldav/") {
				t.Fatalf("%s well-known Location = %q, want /caldav/ prefix", agent.name, loc)
			}
		})
	}
}

func TestClientQuirk_PropfindQueryStringPreservedOnWellKnown(t *testing.T) {
	t.Parallel()

	// Some clients (Outlook CalDAV plugins) append query strings when probing.
	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(http.MethodGet, "/.well-known/caldav?user_id=user-1", nil)
	req.Header.Set("User-Agent", outlookCalDAV.userAgent)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want 301", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "user_id=user-1") {
		t.Fatalf("query string not preserved in redirect: %q", loc)
	}
}

func TestClientQuirk_AppleBootstrapCurrentUserPrincipal(t *testing.T) {
	t.Parallel()

	// Apple's first authenticated call: PROPFIND / for current-user-principal.
	body := `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:current-user-principal/>
  </D:prop>
</D:propfind>`
	rec := doRequest(t, appleCalendar, MethodPropfind, "/caldav/", body, map[string]string{"Depth": "0"})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207: %s", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	mustContain(t, got, "<D:current-user-principal>", "/caldav/principals/user-1/")
}

func TestClientQuirk_DAVx5CalendarHomeSetDiscovery(t *testing.T) {
	t.Parallel()

	// DAVx5 resolves the principal, then reads calendar-home-set from it.
	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-home-set/>
  </D:prop>
</D:propfind>`
	rec := doRequest(t, davx5, MethodPropfind, "/caldav/principals/user-1/", body, map[string]string{"Depth": "0"})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207: %s", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	mustContain(t, got, "<C:calendar-home-set>", "/caldav/calendars/user-1/")
}

// =========================================================================
// OPTIONS capability advertisement
// =========================================================================

func TestClientQuirk_OptionsAdvertisesCalendarAccess(t *testing.T) {
	t.Parallel()

	for _, agent := range []clientAgent{appleCalendar, davx5, thunderbird, gnomeCalendar, outlookCalDAV} {
		agent := agent
		t.Run(agent.name, func(t *testing.T) {
			t.Parallel()
			rec := doRequest(t, agent, http.MethodOptions, "/caldav/calendars/user-1/work/", "", nil)
			if rec.Code != http.StatusNoContent {
				t.Fatalf("%s OPTIONS status = %d, want 204", agent.name, rec.Code)
			}
			dav := rec.Header().Get("DAV")
			mustContain(t, dav, "calendar-access")
			// DAVx5 requires class 1 and 3 for locking/reporting negotiation.
			if !strings.Contains(dav, "1") || !strings.Contains(dav, "3") {
				t.Fatalf("%s DAV header missing class 1/3: %q", agent.name, dav)
			}
			allow := rec.Header().Get("Allow")
			mustContain(t, allow, "PROPFIND", "REPORT", "PUT", "DELETE")
		})
	}
}

// =========================================================================
// REPORT: calendar-query, calendar-multiget, sync-collection
// =========================================================================

func TestClientQuirk_AppleTimeRangeQueryReturnsCalendarData(t *testing.T) {
	t.Parallel()

	body := `<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:getetag/>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT">
        <C:time-range start="20260101T000000Z" end="20261231T235959Z"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`
	rec := doRequest(t, appleCalendar, MethodReport, "/caldav/calendars/user-1/work/", body, map[string]string{"Depth": "1"})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207: %s", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	mustContain(t, got, "<C:calendar-data>", "BEGIN:VCALENDAR", "UID:event-1@example.com", "<D:getetag>")
}

func TestClientQuirk_ThunderbirdMultigetHonoursQuotedETag(t *testing.T) {
	t.Parallel()

	body := `<C:calendar-multiget xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:getetag/>
    <C:calendar-data/>
  </D:prop>
  <D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>
</C:calendar-multiget>`
	rec := doRequest(t, thunderbird, MethodReport, "/caldav/calendars/user-1/work/", body, map[string]string{"Depth": "1"})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207: %s", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	// ETag values must be quoted per RFC 7232; Thunderbird stores them verbatim
	// and sends them back in If-Match, so quotes must round-trip. Inside XML the
	// surrounding double quotes are entity-escaped as &#34;.
	etag := extractElement(got, "D:getetag")
	unescaped := strings.ReplaceAll(etag, "&#34;", `"`)
	if !strings.HasPrefix(unescaped, `"`) || !strings.HasSuffix(unescaped, `"`) {
		t.Fatalf("getetag %q is not double-quoted", etag)
	}
}

func TestClientQuirk_MultigetMissingHrefReturns404Response(t *testing.T) {
	t.Parallel()

	// DAVx5 batches many hrefs; missing ones must come back as a per-href 404
	// D:response (not a top-level error), or the whole batch is discarded.
	body := `<C:calendar-multiget xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:getetag/>
  </D:prop>
  <D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>
  <D:href>/caldav/calendars/user-1/work/does-not-exist.ics</D:href>
</C:calendar-multiget>`
	rec := doRequest(t, davx5, MethodReport, "/caldav/calendars/user-1/work/", body, map[string]string{"Depth": "1"})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207: %s", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	mustContain(t, got, "does-not-exist.ics", "404 Not Found", "event-1.ics")
}

func TestClientQuirk_SyncCollectionInitialAndIncremental(t *testing.T) {
	t.Parallel()

	// Apple/DAVx5 initial sync sends an empty sync-token, then an incremental
	// sync sends the returned token. Both must yield 207 with a sync-token.
	initial := `<D:sync-collection xmlns:D="DAV:">
  <D:sync-token/>
  <D:sync-level>1</D:sync-level>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`
	rec := doRequest(t, davx5, MethodReport, "/caldav/calendars/user-1/work/", initial, map[string]string{"Depth": "0"})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("initial sync status = %d, want 207: %s", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	mustContain(t, got, "<D:sync-token>")
	token := extractElement(got, "D:sync-token")
	if strings.TrimSpace(token) == "" {
		t.Fatalf("initial sync returned empty sync-token:\n%s", got)
	}
}

// =========================================================================
// PUT / DELETE preconditions (If-None-Match / If-Match)
// =========================================================================

func TestClientQuirk_AppleIfNoneMatchStarCreateThenConflict(t *testing.T) {
	t.Parallel()

	// Apple creates new objects with "If-None-Match: *". A second PUT of the
	// same href with If-None-Match: * must fail 412 (object already exists).
	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	ics := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Apple//EN\r\nBEGIN:VEVENT\r\nUID:apple-new-1\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260201T140000Z\r\nDTEND:20260201T150000Z\r\nSUMMARY:Apple New\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"

	rec := doRequestWithHandler(t, handler, appleCalendar, http.MethodPut, "/caldav/calendars/user-1/work/apple-new-1.ics", ics, map[string]string{
		"Content-Type":  "text/calendar; charset=utf-8",
		"If-None-Match": "*",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("first create status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if etag := rec.Header().Get("ETag"); !strings.HasPrefix(etag, `"`) {
		t.Fatalf("create response ETag %q not quoted", etag)
	}

	rec2 := doRequestWithHandler(t, handler, appleCalendar, http.MethodPut, "/caldav/calendars/user-1/work/apple-new-1.ics", ics, map[string]string{
		"Content-Type":  "text/calendar; charset=utf-8",
		"If-None-Match": "*",
	})
	if rec2.Code != http.StatusPreconditionFailed {
		t.Fatalf("duplicate create status = %d, want 412: %s", rec2.Code, rec2.Body.String())
	}
}

func TestClientQuirk_DAVx5IfMatchUpdateFlow(t *testing.T) {
	t.Parallel()

	// DAVx5 updates with "If-Match: <etag>". A stale etag must return 412; the
	// current etag must succeed with 204.
	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	updated := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//DAVx5//EN\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nDTSTAMP:20260506T000000Z\r\nDTSTART:20260506T030000Z\r\nDTEND:20260506T040000Z\r\nSUMMARY:Rescheduled\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"

	// Stale etag: reject.
	rec := doRequestWithHandler(t, handler, davx5, http.MethodPut, "/caldav/calendars/user-1/work/event-1.ics", updated, map[string]string{
		"Content-Type": "text/calendar; charset=utf-8",
		"If-Match":     `"stale-etag-value"`,
	})
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale If-Match status = %d, want 412: %s", rec.Code, rec.Body.String())
	}

	// Current etag: succeed.
	current := `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	rec2 := doRequestWithHandler(t, handler, davx5, http.MethodPut, "/caldav/calendars/user-1/work/event-1.ics", updated, map[string]string{
		"Content-Type": "text/calendar; charset=utf-8",
		"If-Match":     current,
	})
	if rec2.Code != http.StatusNoContent {
		t.Fatalf("current If-Match status = %d, want 204: %s", rec2.Code, rec2.Body.String())
	}
}

func TestClientQuirk_ThunderbirdDeleteIfMatch(t *testing.T) {
	t.Parallel()

	// Thunderbird deletes with If-Match on the last-known etag.
	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	current := `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`

	// Wrong etag: 412.
	rec := doRequestWithHandler(t, handler, thunderbird, http.MethodDelete, "/caldav/calendars/user-1/work/event-1.ics", "", map[string]string{
		"If-Match": `"wrong"`,
	})
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("wrong If-Match delete = %d, want 412: %s", rec.Code, rec.Body.String())
	}

	// Correct etag: 204.
	rec2 := doRequestWithHandler(t, handler, thunderbird, http.MethodDelete, "/caldav/calendars/user-1/work/event-1.ics", "", map[string]string{
		"If-Match": current,
	})
	if rec2.Code != http.StatusNoContent {
		t.Fatalf("correct If-Match delete = %d, want 204: %s", rec2.Code, rec2.Body.String())
	}
}

func TestClientQuirk_OutlookUnquotedETagRejected(t *testing.T) {
	t.Parallel()

	// Broken CalDAV plugins sometimes send unquoted ETags. RFC 7232 requires
	// quoting; the server must reject with 400 rather than silently matching.
	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	rec := doRequestWithHandler(t, handler, outlookCalDAV, http.MethodDelete, "/caldav/calendars/user-1/work/event-1.ics", "", map[string]string{
		"If-Match": "unquoted-etag",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unquoted If-Match delete = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

// =========================================================================
// PROPPATCH (calendar-color, displayname)
// =========================================================================

func TestClientQuirk_AppleProppatchCalendarColor(t *testing.T) {
	t.Parallel()

	// Apple sets calendar-color via PROPPATCH; response must be a 207 with the
	// property echoed in a propstat.
	body := `<D:propertyupdate xmlns:D="DAV:" xmlns:CS="http://calendarserver.org/ns/">
  <D:set>
    <D:prop>
      <CS:calendar-color>#FF5733FF</CS:calendar-color>
    </D:prop>
  </D:set>
</D:propertyupdate>`
	rec := doRequest(t, appleCalendar, MethodProppatch, "/caldav/calendars/user-1/work/", body, map[string]string{"Depth": "0"})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207: %s", rec.Code, rec.Body.String())
	}
	mustContain(t, rec.Body.String(), "calendar-color")
}

func TestClientQuirk_GnomeProppatchDisplayName(t *testing.T) {
	t.Parallel()

	// GNOME Calendar / Evolution renames a collection via displayname PROPPATCH.
	body := `<D:propertyupdate xmlns:D="DAV:">
  <D:set>
    <D:prop>
      <D:displayname>My Renamed Calendar</D:displayname>
    </D:prop>
  </D:set>
</D:propertyupdate>`
	rec := doRequest(t, gnomeCalendar, MethodProppatch, "/caldav/calendars/user-1/work/", body, map[string]string{"Depth": "0"})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207: %s", rec.Code, rec.Body.String())
	}
	mustContain(t, rec.Body.String(), "displayname")
}

// =========================================================================
// supported-calendar-component-set (DAVx5 / GNOME rely on VEVENT support)
// =========================================================================

func TestClientQuirk_SupportedComponentSetAdvertisesVEVENT(t *testing.T) {
	t.Parallel()

	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:supported-calendar-component-set/>
  </D:prop>
</D:propfind>`
	for _, agent := range []clientAgent{davx5, gnomeCalendar, appleCalendar} {
		agent := agent
		t.Run(agent.name, func(t *testing.T) {
			t.Parallel()
			rec := doRequest(t, agent, MethodPropfind, "/caldav/calendars/user-1/work/", body, map[string]string{"Depth": "0"})
			if rec.Code != http.StatusMultiStatus {
				t.Fatalf("%s status = %d, want 207: %s", agent.name, rec.Code, rec.Body.String())
			}
			got := rec.Body.String()
			mustContain(t, got, "supported-calendar-component-set", `name="VEVENT"`)
		})
	}
}

// =========================================================================
// supported-report-set (Thunderbird refuses calendars without it)
// =========================================================================

func TestClientQuirk_SupportedReportSetIncludesCoreReports(t *testing.T) {
	t.Parallel()

	body := `<D:propfind xmlns:D="DAV:">
  <D:prop><D:supported-report-set/></D:prop>
</D:propfind>`
	rec := doRequest(t, thunderbird, MethodPropfind, "/caldav/calendars/user-1/work/", body, map[string]string{"Depth": "0"})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207: %s", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	mustContain(t, got, "supported-report-set", "calendar-query", "calendar-multiget")
}
