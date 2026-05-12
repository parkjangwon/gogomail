package caldavgw

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func assertCacheControlContains(t *testing.T, rec *httptest.ResponseRecorder, values ...string) {
	t.Helper()
	got := rec.Header().Get("Cache-Control")
	for _, value := range values {
		if !strings.Contains(got, value) {
			t.Fatalf("Cache-Control = %q, missing %q", got, value)
		}
	}
}

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
	if got := rec.Header().Get("Allow"); !strings.Contains(got, MethodProppatch) {
		t.Fatalf("Allow header does not advertise PROPPATCH: %q", got)
	}
	if got := rec.Header().Get("Allow"); !strings.Contains(got, MethodMkcalendar) {
		t.Fatalf("Allow header does not advertise MKCALENDAR: %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

func TestHandlerOptionsDoesNotAdvertiseSyncCollectionWithoutSyncStore(t *testing.T) {
	t.Parallel()

	handler := NewHandler(&noSyncCalendarDiscoveryStore{store: *newFakeDiscoveryStore()}, fixedUser("user-1"))
	req := httptest.NewRequest(MethodOptions, "/caldav/principals/user-1/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("DAV"); strings.Contains(got, DAVSyncCollection) {
		t.Fatalf("DAV header = %q, should not advertise %q without SyncChangeStore", got, DAVSyncCollection)
	}
}

func TestHandlerOptionsAdvertisesOnlyImplementedMethods(t *testing.T) {
	t.Parallel()

	handler := NewHandler(&fakeDiscoveryStore{}, fixedUser("user-1"))
	req := httptest.NewRequest(MethodOptions, "/caldav/calendars/user-1/work/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	want := strings.Join(ImplementedMethods(), ", ")
	if got := rec.Header().Get("Allow"); got != want {
		t.Fatalf("Allow header = %q, want %q", got, want)
	}
	for _, futureMethod := range []string{MethodCopy, MethodMove} {
		if strings.Contains(rec.Header().Get("Allow"), futureMethod) {
			t.Fatalf("Allow header advertised unimplemented %s: %q", futureMethod, rec.Header().Get("Allow"))
		}
	}
}

func TestHandlerUnsupportedMethodReturnsImplementedAllow(t *testing.T) {
	t.Parallel()

	handler := NewHandler(&fakeDiscoveryStore{}, fixedUser("user-1"))
	req := httptest.NewRequest(MethodCopy, "/caldav/calendars/user-1/work/event-1.ics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	want := strings.Join(ImplementedMethods(), ", ")
	if got := rec.Header().Get("Allow"); got != want {
		t.Fatalf("Allow header = %q, want %q", got, want)
	}
	for _, futureMethod := range []string{MethodCopy, MethodMove} {
		if strings.Contains(rec.Header().Get("Allow"), futureMethod) {
			t.Fatalf("Allow header advertised unimplemented %s: %q", futureMethod, rec.Header().Get("Allow"))
		}
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

func TestHandlerWellKnownCalDAVRedirectsToServiceRoot(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(http.MethodGet, "/.well-known/caldav?client=probe", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/caldav/?client=probe" {
		t.Fatalf("Location = %q", got)
	}
}

func TestHandlerPropfindServiceRootDiscovery(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodPropfind, "/caldav/", strings.NewReader(`<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><D:current-user-principal/><D:principal-collection-set/><D:resourcetype/><C:calendar-home-set/></D:prop></D:propfind>`))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<D:href>/caldav/</D:href>",
		"<D:current-user-principal><D:href>/caldav/principals/user-1/</D:href></D:current-user-principal>",
		"<D:principal-collection-set><D:href>/caldav/principals/</D:href></D:principal-collection-set>",
		"<D:resourcetype><D:collection></D:collection></D:resourcetype>",
		"<D:status>HTTP/1.1 404 Not Found</D:status>",
		"<C:calendar-home-set></C:calendar-home-set>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("root discovery missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "<D:principal></D:principal>") {
		t.Fatalf("service root was advertised as a principal resource:\n%s", body)
	}
	if strings.Contains(body, "<C:calendar-home-set><D:href>") {
		t.Fatalf("service root should not expose principal-only calendar-home-set:\n%s", body)
	}
}

func TestHandlerPropfindPrincipalDiscovery(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(`<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><D:current-user-principal/><C:calendar-home-set/><C:calendar-user-address-set/></D:prop></D:propfind>`))
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
		"<C:calendar-user-address-set><D:href>mailto:user.one@example.com</D:href></C:calendar-user-address-set>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("principal discovery missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerPropfindPrincipalDiscoveryIncludesAliasAddresses(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.principal.CalendarUserAddresses = []string{"mailto:user.one@example.com", "mailto:team@example.com"}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(`<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><C:calendar-user-address-set/></D:prop></D:propfind>`))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<D:href>mailto:user.one@example.com</D:href>",
		"<D:href>mailto:team@example.com</D:href>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("principal discovery missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerPropfindAllowsDelegatedRead(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("delegate-1"))
	handler.AccessAuthorizer = &fakeCalendarAccessAuthorizer{
		allowedRoles: map[string]bool{CalendarAccessRoleRead: true},
		privileges:   []XMLName{PrivilegeRead},
	}
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propfind xmlns:D="DAV:"><D:prop><D:getetag/><D:owner/><D:current-user-privilege-set/></D:prop></D:propfind>`))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := handler.AccessAuthorizer.(*fakeCalendarAccessAuthorizer).last; got.ActorUserID != "delegate-1" || got.OwnerUserID != "user-1" || got.RequiredRole != CalendarAccessRoleRead {
		t.Fatalf("access request = %+v", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<D:href>/caldav/calendars/user-1/work/</D:href>") || !strings.Contains(body, "<D:owner><D:href>/caldav/principals/user-1/</D:href></D:owner>") {
		t.Fatalf("delegated propfind did not use owner resource:\n%s", body)
	}
	if !strings.Contains(body, "<D:current-user-privilege-set><D:privilege><D:read></D:read></D:privilege></D:current-user-privilege-set>") {
		t.Fatalf("delegated propfind missing read-only privileges:\n%s", body)
	}
	for _, denied := range []string{"<D:bind>", "<D:unbind>", "<D:write-properties>", "<D:write-content>"} {
		if strings.Contains(body, denied) {
			t.Fatalf("delegated read propfind advertised %s:\n%s", denied, body)
		}
	}
}

func TestHandlerPropfindDelegatedCalendarHomeKeepsCurrentUserPrincipalAsActor(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("delegate-1"))
	handler.AccessAuthorizer = &fakeCalendarAccessAuthorizer{
		allowedRoles: map[string]bool{CalendarAccessRoleRead: true},
		privileges:   []XMLName{PrivilegeRead},
	}
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/", strings.NewReader(`<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/><D:owner/></D:prop></D:propfind>`))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<D:href>/caldav/calendars/user-1/</D:href>",
		"<D:current-user-principal><D:href>/caldav/principals/delegate-1/</D:href></D:current-user-principal>",
		"<D:owner><D:href>/caldav/principals/user-1/</D:href></D:owner>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("delegated calendar-home PROPFIND missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerPropfindDelegatedPrincipalKeepsCurrentUserPrincipalAsActor(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("delegate-1"))
	handler.AccessAuthorizer = &fakeCalendarAccessAuthorizer{
		allowedRoles: map[string]bool{CalendarAccessRoleRead: true},
		privileges:   []XMLName{PrivilegeRead},
	}
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/user-1/", strings.NewReader(`<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/><D:owner/><D:principal-URL/></D:prop></D:propfind>`))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<D:href>/caldav/principals/user-1/</D:href>",
		"<D:current-user-principal><D:href>/caldav/principals/delegate-1/</D:href></D:current-user-principal>",
		"<D:owner><D:href>/caldav/principals/user-1/</D:href></D:owner>",
		"<D:principal-URL><D:href>/caldav/principals/user-1/</D:href></D:principal-URL>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("delegated principal PROPFIND missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerPropfindPrincipalCollectionDepthOne(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodPropfind, "/caldav/principals/", strings.NewReader(`<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/><D:principal-collection-set/><D:resourcetype/></D:prop></D:propfind>`))
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<D:href>/caldav/principals/</D:href>",
		"<D:href>/caldav/principals/user-1/</D:href>",
		"<D:current-user-principal><D:href>/caldav/principals/user-1/</D:href></D:current-user-principal>",
		"<D:principal-collection-set><D:href>/caldav/principals/</D:href></D:principal-collection-set>",
		"<D:resourcetype><D:collection></D:collection></D:resourcetype>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("principal collection discovery missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerPropfindCalendarHomeDepthOne(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/", strings.NewReader(`<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><D:current-user-principal/><D:current-user-privilege-set/><D:owner/><D:displayname/><D:resourcetype/><C:supported-calendar-component-set/></D:prop></D:propfind>`))
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
	if !strings.Contains(body, "<D:current-user-principal><D:href>/caldav/principals/user-1/</D:href></D:current-user-principal>") {
		t.Fatalf("calendar-home current-user-principal missing:\n%s", body)
	}
	if !strings.Contains(body, "<D:owner><D:href>/caldav/principals/user-1/</D:href></D:owner>") {
		t.Fatalf("calendar-home owner missing:\n%s", body)
	}
	if !strings.Contains(body, "<D:current-user-privilege-set><D:privilege><D:read></D:read></D:privilege><D:privilege><D:bind></D:bind></D:privilege><D:privilege><D:unbind></D:unbind></D:privilege></D:current-user-privilege-set>") {
		t.Fatalf("calendar-home privileges missing:\n%s", body)
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

func TestHandlerPropfindCalendarCollectionDepthOneRejectsTruncation(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	base := store.objects[0]
	store.objects = store.objects[:0]
	for i := 0; i < MaxWebDAVReportLimit+1; i++ {
		object := base
		object.ID = fmt.Sprintf("object-%d", i)
		object.ObjectName = fmt.Sprintf("event-%d.ics", i)
		object.UID = fmt.Sprintf("event-%d@example.com", i)
		store.objects = append(store.objects, object)
	}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propfind xmlns:D="DAV:"><D:prop><D:getetag/></D:prop></D:propfind>`))
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "calendar collection PROPFIND would truncate results") {
		t.Fatalf("truncating collection PROPFIND response lacks context: %s", rec.Body.String())
	}
}

func TestHandlerPropfindCalendarCollectionReportsSupportedReports(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propfind xmlns:D="DAV:"><D:prop><D:supported-report-set/></D:prop></D:propfind>`))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<D:supported-report-set>",
		"<C:calendar-query></C:calendar-query>",
		"<C:calendar-multiget></C:calendar-multiget>",
		"<C:free-busy-query></C:free-busy-query>",
		"<D:sync-collection></D:sync-collection>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("supported reports response missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerPropfindCalendarCollectionOmitsSyncReportWithoutSyncStore(t *testing.T) {
	t.Parallel()

	handler := NewHandler(&noSyncCalendarDiscoveryStore{store: *newFakeDiscoveryStore()}, fixedUser("user-1"))
	req := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propfind xmlns:D="DAV:"><D:prop><D:supported-report-set/></D:prop></D:propfind>`))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<D:supported-report-set>",
		"<C:calendar-query></C:calendar-query>",
		"<C:calendar-multiget></C:calendar-multiget>",
		"<C:free-busy-query></C:free-busy-query>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("supported reports response missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "<D:sync-collection>") {
		t.Fatalf("supported reports advertised sync-collection without SyncChangeStore:\n%s", body)
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

func TestHandlerReportCalendarMultigetAllowsDelegatedRead(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("delegate-1"))
	handler.AccessAuthorizer = &fakeCalendarAccessAuthorizer{
		allowedRoles: map[string]bool{CalendarAccessRoleRead: true},
		privileges:   []XMLName{PrivilegeRead},
	}
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<C:calendar-multiget xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/><D:current-user-privilege-set/><C:calendar-data/></D:prop>
  <D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>
</C:calendar-multiget>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := handler.AccessAuthorizer.(*fakeCalendarAccessAuthorizer).last; got.ActorUserID != "delegate-1" || got.OwnerUserID != "user-1" || got.RequiredRole != CalendarAccessRoleRead {
		t.Fatalf("access request = %+v", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>") || !strings.Contains(body, "UID:event-1@example.com") {
		t.Fatalf("delegated calendar-multiget missing owner object:\n%s", body)
	}
	if !strings.Contains(body, "<D:current-user-privilege-set><D:privilege><D:read></D:read></D:privilege></D:current-user-privilege-set>") {
		t.Fatalf("delegated calendar-multiget missing read-only privileges:\n%s", body)
	}
	for _, denied := range []string{"<D:bind>", "<D:unbind>", "<D:write-properties>", "<D:write-content>"} {
		if strings.Contains(body, denied) {
			t.Fatalf("delegated calendar-multiget advertised %s:\n%s", denied, body)
		}
	}
}

func TestHandlerReportCalendarMultigetProjectsCalendarData(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<C:calendar-multiget xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop>
    <D:getetag/>
    <C:calendar-data>
      <C:comp name="VCALENDAR">
        <C:prop name="VERSION"/>
        <C:prop name="PRODID"/>
        <C:comp name="VEVENT">
          <C:prop name="UID"/>
          <C:prop name="SUMMARY"/>
        </C:comp>
      </C:comp>
    </C:calendar-data>
  </D:prop>
  <D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>
</C:calendar-multiget>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<C:calendar-data>BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//gogomail//CalDAV Test//EN",
		"UID:event-1@example.com",
		"SUMMARY:Planning",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("projected calendar-data missing %q:\n%s", want, body)
		}
	}
	for _, forbidden := range []string{"DTSTART:", "DTEND:"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("projected calendar-data included %q:\n%s", forbidden, body)
		}
	}
}

func TestHandlerReportCalendarMultigetAcceptsAbsoluteHref(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<C:calendar-multiget xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/><C:calendar-data/></D:prop>
  <D:href>https://calendar.example.test/caldav/calendars/user-1/work/event-1.ics</D:href>
</C:calendar-multiget>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>",
		"<C:calendar-data>BEGIN:VCALENDAR",
		"UID:event-1@example.com",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("calendar-multiget absolute href missing %q:\n%s", want, body)
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

func TestHandlerReportCalendarMultigetScopesCollectionHrefs(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	now := time.Now()
	personalICS := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:personal-1@example.com\r\nDTSTAMP:20260506T000000Z\r\nDTSTART:20260506T030000Z\r\nDTEND:20260506T040000Z\r\nSUMMARY:Personal\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	store.calendars = append(store.calendars, Calendar{
		ID:        "personal",
		UserID:    "user-1",
		Name:      "Personal",
		SyncToken: "sync-personal",
		CreatedAt: now,
		UpdatedAt: now,
	})
	store.objects = append(store.objects, CalendarObject{
		ID:         "object-personal",
		UserID:     "user-1",
		CalendarID: "personal",
		ObjectName: "personal-1.ics",
		UID:        "personal-1@example.com",
		Component:  ComponentVEVENT,
		ETag:       `"2123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`,
		Size:       int64(len(personalICS)),
		ICS:        personalICS,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<C:calendar-multiget xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/><C:calendar-data/></D:prop>
  <D:href>/caldav/calendars/user-1/personal/personal-1.ics</D:href>
</C:calendar-multiget>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<D:href>/caldav/calendars/user-1/personal/personal-1.ics</D:href>") || !strings.Contains(body, "HTTP/1.1 404 Not Found") {
		t.Fatalf("out-of-collection href should render not found:\n%s", body)
	}
	if strings.Contains(body, "UID:personal-1@example.com") {
		t.Fatalf("out-of-collection href leaked calendar-data:\n%s", body)
	}
}

func TestHandlerReportCalendarHomeMultigetAllowsUserCalendarHrefs(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/", strings.NewReader(`<C:calendar-multiget xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/></D:prop>
  <D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>
</C:calendar-multiget>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>") || !strings.Contains(body, "HTTP/1.1 200 OK") {
		t.Fatalf("calendar-home multiget missing object:\n%s", body)
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
	req.Header.Set("Depth", "1")
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

func TestHandlerReportCalendarQueryTimeRangeUsesComponentCandidates(t *testing.T) {
	t.Parallel()

	store := &queryCandidateCalendarDiscoveryStore{fakeDiscoveryStore: *newFakeDiscoveryStore()}
	todoICS := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VTODO\r\nUID:todo-1@example.com\r\nDTSTAMP:20260506T000000Z\r\nDTSTART:20260506T010000Z\r\nDUE:20260506T020000Z\r\nSUMMARY:Todo\r\nEND:VTODO\r\nEND:VCALENDAR\r\n")
	todo := store.objects[0]
	todo.ID = "object-todo"
	todo.ObjectName = "todo-1.ics"
	todo.UID = "todo-1@example.com"
	todo.Component = ComponentVTODO
	todo.ETag = `"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"`
	todo.ICS = todoICS
	todo.Size = int64(len(todoICS))
	store.objects = append([]CalendarObject{todo}, store.objects...)

	handler := NewHandler(store, fixedUser("user-1"))
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
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if store.candidateCount != 1 || store.listCount != 0 {
		t.Fatalf("candidateCount = %d, listCount = %d; want component candidate path only", store.candidateCount, store.listCount)
	}
	if len(store.components) != 1 || store.components[0] != ComponentVEVENT {
		t.Fatalf("components = %v, want [%s]", store.components, ComponentVEVENT)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event-1.ics") {
		t.Fatalf("calendar-query candidate path missing matching event:\n%s", body)
	}
	if strings.Contains(body, "todo-1.ics") {
		t.Fatalf("calendar-query candidate path returned non-requested component:\n%s", body)
	}
}

func TestHandlerReportCalendarQueryAppliesLimitAfterTimeRangeFilter(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	matching := store.objects[0]
	nonMatchingICS := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:later@example.com\r\nDTSTAMP:20260508T000000Z\r\nDTSTART:20260508T010000Z\r\nDTEND:20260508T020000Z\r\nSUMMARY:Later\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	nonMatching := matching
	nonMatching.ID = "object-later"
	nonMatching.ObjectName = "later.ics"
	nonMatching.UID = "later@example.com"
	nonMatching.ETag = `"1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	nonMatching.ICS = nonMatchingICS
	nonMatching.Size = int64(len(nonMatchingICS))
	store.objects = []CalendarObject{nonMatching, matching}

	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:limit><D:nresults>1</D:nresults></D:limit>
  <D:prop><D:getetag/></D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT">
        <C:time-range start="20260506T000000Z" end="20260507T000000Z"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`))
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>") {
		t.Fatalf("calendar-query time-range limit missed matching object:\n%s", body)
	}
	if strings.Contains(body, "later.ics") {
		t.Fatalf("calendar-query time-range limit returned non-matching object:\n%s", body)
	}
}

func TestHandlerReportCalendarQueryRejectsUnsupportedFilter(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/></D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT">
        <C:prop-filter name="SUMMARY"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`))
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<C:supported-filter/>`) || !strings.Contains(body, "prop-filter") {
		t.Fatalf("supported-filter precondition missing:\n%s", body)
	}
}

func TestHandlerReportCalendarQueryDepthZeroDoesNotScanChildren(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/><C:calendar-data/></D:prop>
  <C:filter><C:comp-filter name="VCALENDAR"/></C:filter>
</C:calendar-query>`))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "event-1.ics") {
		t.Fatalf("Depth: 0 calendar-query scanned child objects:\n%s", rec.Body.String())
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
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "event-1.ics") {
		t.Fatalf("non-overlapping calendar-query returned event:\n%s", rec.Body.String())
	}
}

func TestHandlerReportCalendarQueryFiltersByComponent(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	todoICS := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VTODO\r\nUID:todo-1@example.com\r\nSUMMARY:Review\r\nEND:VTODO\r\nEND:VCALENDAR\r\n")
	store.objects = append(store.objects, CalendarObject{
		ID:         "object-todo",
		UserID:     "user-1",
		CalendarID: "work",
		ObjectName: "todo-1.ics",
		UID:        "todo-1@example.com",
		Component:  ComponentVTODO,
		ETag:       `"1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`,
		Size:       int64(len(todoICS)),
		ICS:        todoICS,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/><C:calendar-data/></D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VTODO"/>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`))
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "todo-1.ics") {
		t.Fatalf("VTODO calendar-query missing todo object:\n%s", body)
	}
	if strings.Contains(body, "event-1.ics") {
		t.Fatalf("VTODO calendar-query returned VEVENT object:\n%s", body)
	}
}

func TestHandlerReportCalendarQueryFiltersVTODOByTimeRange(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	overlapICS := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VTODO\r\nUID:todo-overlap@example.com\r\nDTSTAMP:20260506T000000Z\r\nDTSTART:20260506T090000Z\r\nDUE:20260506T100000Z\r\nSUMMARY:Review\r\nEND:VTODO\r\nEND:VCALENDAR\r\n")
	missICS := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VTODO\r\nUID:todo-later@example.com\r\nDTSTAMP:20260506T000000Z\r\nDTSTART:20260508T090000Z\r\nDUE:20260508T100000Z\r\nSUMMARY:Later\r\nEND:VTODO\r\nEND:VCALENDAR\r\n")
	store.objects = append(store.objects,
		CalendarObject{
			ID:         "object-todo-overlap",
			UserID:     "user-1",
			CalendarID: "work",
			ObjectName: "todo-overlap.ics",
			UID:        "todo-overlap@example.com",
			Component:  ComponentVTODO,
			ETag:       `"2123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`,
			Size:       int64(len(overlapICS)),
			ICS:        overlapICS,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		CalendarObject{
			ID:         "object-todo-later",
			UserID:     "user-1",
			CalendarID: "work",
			ObjectName: "todo-later.ics",
			UID:        "todo-later@example.com",
			Component:  ComponentVTODO,
			ETag:       `"3123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`,
			Size:       int64(len(missICS)),
			ICS:        missICS,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
	)
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/><C:calendar-data/></D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VTODO">
        <C:time-range start="20260506T093000Z" end="20260506T110000Z"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`))
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "todo-overlap.ics") {
		t.Fatalf("VTODO time-range query missing overlapping todo:\n%s", body)
	}
	if strings.Contains(body, "todo-later.ics") {
		t.Fatalf("VTODO time-range query returned non-overlapping todo:\n%s", body)
	}
	if strings.Contains(body, "event-1.ics") {
		t.Fatalf("VTODO time-range query returned VEVENT object:\n%s", body)
	}
}

func TestHandlerReportCalendarQueryRejectsTruncatingLimit(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	second := store.objects[0]
	second.ID = "object-2"
	second.ObjectName = "event-2.ics"
	second.UID = "event-2@example.com"
	second.ETag = `"1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	store.objects = append(store.objects, second)

	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:limit><D:nresults>1</D:nresults></D:limit>
  <D:prop><D:getetag/></D:prop>
  <C:filter><C:comp-filter name="VCALENDAR"/></C:filter>
</C:calendar-query>`))
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "calendar-query limit would truncate results") {
		t.Fatalf("truncating calendar-query response lacks context: %s", rec.Body.String())
	}
}

func TestHandlerReportSyncCollectionInitialSyncReturnsObjectsAndToken(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:sync-collection xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:sync-token/>
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

func TestHandlerReportSyncCollectionAllowsDelegatedReadOnlyPrivileges(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("delegate-1"))
	handler.AccessAuthorizer = &fakeCalendarAccessAuthorizer{
		allowedRoles: map[string]bool{CalendarAccessRoleRead: true},
		privileges:   []XMLName{PrivilegeRead},
	}
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:sync-collection xmlns:D="DAV:">
  <D:sync-token/>
  <D:sync-level>1</D:sync-level>
  <D:prop><D:getetag/><D:current-user-privilege-set/></D:prop>
</D:sync-collection>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>") {
		t.Fatalf("delegated sync missing owner object:\n%s", body)
	}
	if !strings.Contains(body, "<D:current-user-privilege-set><D:privilege><D:read></D:read></D:privilege></D:current-user-privilege-set>") {
		t.Fatalf("delegated sync missing read-only privileges:\n%s", body)
	}
	for _, denied := range []string{"<D:bind>", "<D:unbind>", "<D:write-properties>", "<D:write-content>"} {
		if strings.Contains(body, denied) {
			t.Fatalf("delegated sync advertised %s:\n%s", denied, body)
		}
	}
}

func TestHandlerReportSyncCollectionRejectsDefaultSnapshotTruncation(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	base := store.objects[0]
	store.objects = store.objects[:0]
	for i := 0; i < MaxWebDAVReportLimit+1; i++ {
		object := base
		object.ID = fmt.Sprintf("object-%d", i)
		object.ObjectName = fmt.Sprintf("event-%d.ics", i)
		object.UID = fmt.Sprintf("event-%d@example.com", i)
		store.objects = append(store.objects, object)
	}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:sync-collection xmlns:D="DAV:">
  <D:sync-token/>
  <D:sync-level>1</D:sync-level>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "<D:number-of-matches>0</D:number-of-matches>") {
		t.Fatalf("RFC 6578 truncation response missing number-of-matches: %s", rec.Body.String())
	}
}

func TestHandlerReportSyncCollectionRejectsMissingSyncTokenElement(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:sync-collection xmlns:D="DAV:">
  <D:sync-level>1</D:sync-level>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "sync-token") {
		t.Fatalf("missing sync-token response lacks context: %s", rec.Body.String())
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

func TestHandlerReportSyncCollectionAllowsExactChangeLimit(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].SyncToken = "sync-updated"
	store.changes = append(store.changes, CalendarChange{
		ID:         int64(len(store.changes) + 1),
		UserID:     "user-1",
		CalendarID: "work",
		ObjectName: "event-1.ics",
		ETag:       store.objects[0].ETag,
		Action:     "object-upserted",
		SyncToken:  "sync-updated",
		ChangedAt:  time.Date(2026, 5, 6, 11, 12, 13, 0, time.UTC),
	})
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:sync-collection xmlns:D="DAV:">
  <D:sync-token>sync-calendar</D:sync-token>
  <D:sync-level>1</D:sync-level>
  <D:limit><D:nresults>1</D:nresults></D:limit>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>",
		"<D:sync-token>sync-updated</D:sync-token>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("exact-limit sync response missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerReportSyncCollectionCoalescesDuplicateObjectChangesBeforeLimit(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].SyncToken = "sync-updated-2"
	store.changes = append(store.changes,
		CalendarChange{
			ID:         int64(len(store.changes) + 1),
			UserID:     "user-1",
			CalendarID: "work",
			ObjectName: "event-1.ics",
			ETag:       store.objects[0].ETag,
			Action:     "object-upserted",
			SyncToken:  "sync-updated-1",
			ChangedAt:  time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
		},
		CalendarChange{
			ID:         int64(len(store.changes) + 2),
			UserID:     "user-1",
			CalendarID: "work",
			ObjectName: "event-1.ics",
			ETag:       store.objects[0].ETag,
			Action:     "object-upserted",
			SyncToken:  "sync-updated-2",
			ChangedAt:  time.Date(2026, 5, 6, 12, 1, 0, 0, time.UTC),
		},
	)

	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:sync-collection xmlns:D="DAV:">
  <D:sync-token>sync-calendar</D:sync-token>
  <D:sync-level>1</D:sync-level>
  <D:limit><D:nresults>1</D:nresults></D:limit>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if count := strings.Count(body, "<D:response>"); count != 1 {
		t.Fatalf("response count = %d, want 1:\n%s", count, body)
	}
	if !strings.Contains(body, "<D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>") {
		t.Fatalf("coalesced sync response missing latest object:\n%s", body)
	}
	if !strings.Contains(body, "<D:sync-token>sync-updated-2</D:sync-token>") {
		t.Fatalf("coalesced sync response missing latest token:\n%s", body)
	}
}

func TestHandlerReportSyncCollectionRejectsNonZeroHTTPDepth(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:sync-collection xmlns:D="DAV:">
  <D:sync-token>sync-calendar</D:sync-token>
  <D:sync-level>1</D:sync-level>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`))
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "sync-collection requires Depth: 0") {
		t.Fatalf("response did not explain Depth rejection: %s", rec.Body.String())
	}
}

func TestHandlerReportSyncCollectionRejectsUnknownSyncToken(t *testing.T) {
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

func TestHandlerReportSyncCollectionReturnsDeletedObjectTombstone(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	if _, err := store.DeleteObject(context.Background(), DeleteObjectRequest{
		UserID:     "user-1",
		CalendarID: "work",
		ObjectName: "event-1.ics",
	}); err != nil {
		t.Fatalf("DeleteObject setup failed: %v", err)
	}
	handler := NewHandler(store, fixedUser("user-1"))
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
	for _, want := range []string{
		"<D:href>/caldav/calendars/user-1/work/event-1.ics</D:href>",
		"<D:status>HTTP/1.1 404 Not Found</D:status>",
		"<D:sync-token>sync-",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("sync tombstone response missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerReportSyncCollectionReturnsDeletedCollectionToken(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars = nil
	store.objects = nil
	store.changes = append(store.changes, CalendarChange{
		ID:         int64(len(store.changes) + 1),
		UserID:     "user-1",
		CalendarID: "work",
		Action:     "collection-deleted",
		SyncToken:  "sync-deleted",
		ChangedAt:  time.Date(2026, 5, 6, 10, 11, 12, 0, time.UTC),
	})
	handler := NewHandler(store, fixedUser("user-1"))
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
	if strings.Contains(body, "<D:response>") {
		t.Fatalf("deleted collection sync should not return object responses:\n%s", body)
	}
	if !strings.Contains(body, "<D:sync-token>sync-deleted</D:sync-token>") {
		t.Fatalf("deleted collection sync token missing:\n%s", body)
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
  <D:sync-token/>
  <D:sync-level>1</D:sync-level>
  <D:limit><D:nresults>1</D:nresults></D:limit>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "<D:number-of-matches>0</D:number-of-matches>") {
		t.Fatalf("RFC 6578 truncation response missing number-of-matches:\n%s", rec.Body.String())
	}
}

func TestHandlerReportSyncCollectionRejectsTruncatingChangeLimit(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].SyncToken = "sync-updated-2"
	second := store.objects[0]
	second.ID = "object-2"
	second.ObjectName = "event-2.ics"
	second.UID = "event-2@example.com"
	second.ETag = `"1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	store.objects = append(store.objects, second)
	store.changes = append(store.changes,
		CalendarChange{
			ID:         int64(len(store.changes) + 1),
			UserID:     "user-1",
			CalendarID: "work",
			ObjectName: "event-1.ics",
			ETag:       store.objects[0].ETag,
			Action:     "object-upserted",
			SyncToken:  "sync-updated-1",
			ChangedAt:  time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
		},
		CalendarChange{
			ID:         int64(len(store.changes) + 2),
			UserID:     "user-1",
			CalendarID: "work",
			ObjectName: "event-2.ics",
			ETag:       second.ETag,
			Action:     "object-upserted",
			SyncToken:  "sync-updated-2",
			ChangedAt:  time.Date(2026, 5, 6, 12, 1, 0, 0, time.UTC),
		},
	)

	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:sync-collection xmlns:D="DAV:">
  <D:sync-token>sync-calendar</D:sync-token>
  <D:sync-level>1</D:sync-level>
  <D:limit><D:nresults>1</D:nresults></D:limit>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "<D:number-of-matches>0</D:number-of-matches>") {
		t.Fatalf("RFC 6578 truncation response missing number-of-matches:\n%s", rec.Body.String())
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

func TestHandlerReportFreeBusyQueryRejectsTruncatingLimit(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	second := store.objects[0]
	second.ID = "object-2"
	second.ObjectName = "event-2.ics"
	second.UID = "event-2@example.com"
	second.ETag = `"1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	store.objects = append(store.objects, second)

	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", strings.NewReader(`<C:free-busy-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:limit><D:nresults>1</D:nresults></D:limit>
  <C:time-range start="20260506T000000Z" end="20260507T000000Z"/>
</C:free-busy-query>`))
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "free-busy-query limit would truncate results") {
		t.Fatalf("truncating free-busy response lacks context: %s", rec.Body.String())
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

func TestHandlerReportRejectsUnsupportedDepthBeforeBodyRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		depth string
		want  int
	}{
		{name: "infinity", depth: "infinity", want: http.StatusForbidden},
		{name: "invalid", depth: "children", want: http.StatusBadRequest},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			body := &readTrackingReader{data: `<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:"><D:prop><D:getetag/></D:prop><C:filter><C:comp-filter name="VCALENDAR"/></C:filter></C:calendar-query>`}
			handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
			req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", body)
			req.Header.Set("Depth", tc.depth)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tc.want, rec.Body.String())
			}
			if body.reads != 0 {
				t.Fatalf("body reads = %d, want 0", body.reads)
			}
		})
	}
}

func TestHandlerReportRejectsRepeatedDepthBeforeBodyRead(t *testing.T) {
	t.Parallel()

	body := &readTrackingReader{data: `<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:"><D:prop><D:getetag/></D:prop><C:filter><C:comp-filter name="VCALENDAR"/></C:filter></C:calendar-query>`}
	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodReport, "/caldav/calendars/user-1/work/", body)
	req.Header.Add("Depth", "0")
	req.Header.Add("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	if !strings.Contains(rec.Body.String(), "Depth header must not be repeated") {
		t.Fatalf("response did not explain repeated Depth rejection: %s", rec.Body.String())
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

func TestHandlerGetCalendarObjectAllowsDelegatedRead(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("delegate-1"))
	handler.AccessAuthorizer = &fakeCalendarAccessAuthorizer{allowedRoles: map[string]bool{CalendarAccessRoleRead: true}}
	req := httptest.NewRequest(MethodGet, "/caldav/calendars/user-1/work/event-1.ics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := handler.AccessAuthorizer.(*fakeCalendarAccessAuthorizer).last; got.ActorUserID != "delegate-1" || got.OwnerUserID != "user-1" || got.RequiredRole != CalendarAccessRoleRead {
		t.Fatalf("access request = %+v", got)
	}
	if !strings.Contains(rec.Body.String(), "UID:event-1@example.com") {
		t.Fatalf("delegated GET missing owner object:\n%s", rec.Body.String())
	}
}

func TestHandlerGetCalendarObjectHonorsIfNoneMatch(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	for _, method := range []string{MethodGet, MethodHead} {
		method := method
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(method, "/caldav/calendars/user-1/work/event-1.ics", nil)
			req.Header.Set("If-None-Match", store.objects[0].ETag)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotModified {
				t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("ETag"); got != store.objects[0].ETag {
				t.Fatalf("ETag = %q, want %q", got, store.objects[0].ETag)
			}
			if rec.Body.Len() != 0 {
				t.Fatalf("not modified body length = %d, want 0", rec.Body.Len())
			}
		})
	}
}

func TestHandlerGetCalendarObjectHonorsRepeatedIfNoneMatch(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodGet, "/caldav/calendars/user-1/work/event-1.ics", nil)
	req.Header.Add("If-None-Match", `"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`)
	req.Header.Add("If-None-Match", store.objects[0].ETag)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotModified, rec.Body.String())
	}
}

func TestHandlerGetCalendarObjectRejectsRepeatedDateConditionals(t *testing.T) {
	t.Parallel()

	for _, header := range []string{"If-Modified-Since", "If-Unmodified-Since"} {
		header := header
		t.Run(header, func(t *testing.T) {
			t.Parallel()

			handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
			req := httptest.NewRequest(MethodGet, "/caldav/calendars/user-1/work/event-1.ics", nil)
			req.Header.Add(header, "Wed, 06 May 2026 04:05:06 GMT")
			req.Header.Add(header, "Wed, 06 May 2026 04:05:07 GMT")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), header+" header must not be repeated") {
				t.Fatalf("response did not explain repeated %s rejection: %s", header, rec.Body.String())
			}
		})
	}
}

func TestHandlerGetCalendarObjectHonorsIfModifiedSince(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.objects[0].UpdatedAt = time.Date(2026, 5, 6, 4, 5, 6, 789, time.UTC)
	handler := NewHandler(store, fixedUser("user-1"))
	for _, method := range []string{MethodGet, MethodHead} {
		method := method
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(method, "/caldav/calendars/user-1/work/event-1.ics", nil)
			req.Header.Set("If-Modified-Since", "Wed, 06 May 2026 04:05:06 GMT")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotModified {
				t.Fatalf("status = %d, want 304, body = %s", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Last-Modified"); got != "Wed, 06 May 2026 04:05:06 GMT" {
				t.Fatalf("Last-Modified = %q", got)
			}
			if rec.Body.Len() != 0 {
				t.Fatalf("not modified body length = %d, want 0", rec.Body.Len())
			}
		})
	}
}

func TestHandlerGetCalendarObjectIgnoresIfModifiedSinceWhenIfNoneMatchPresent(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.objects[0].UpdatedAt = time.Date(2026, 5, 6, 4, 5, 6, 0, time.UTC)
	handler := NewHandler(store, fixedUser("user-1"))

	req := httptest.NewRequest(MethodGet, "/caldav/calendars/user-1/work/event-1.ics", nil)
	req.Header.Set("If-None-Match", `"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`)
	req.Header.Set("If-Modified-Since", "Wed, 06 May 2026 04:05:06 GMT")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("ETag"); got != store.objects[0].ETag {
		t.Fatalf("ETag = %q, want %q", got, store.objects[0].ETag)
	}
	if !strings.Contains(rec.Body.String(), "BEGIN:VCALENDAR") {
		t.Fatalf("GET body = %s", rec.Body.String())
	}
}

func TestHandlerGetCalendarObjectIgnoresStaleIfModifiedSince(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.objects[0].UpdatedAt = time.Date(2026, 5, 6, 4, 5, 6, 0, time.UTC)
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodGet, "/caldav/calendars/user-1/work/event-1.ics", nil)
	req.Header.Set("If-Modified-Since", "Wed, 06 May 2026 04:05:05 GMT")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Last-Modified"); got != "Wed, 06 May 2026 04:05:06 GMT" {
		t.Fatalf("Last-Modified = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "BEGIN:VCALENDAR") {
		t.Fatalf("GET body = %s", rec.Body.String())
	}
}

func TestHandlerGetCalendarObjectHonorsIfMatch(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	for _, method := range []string{MethodGet, MethodHead} {
		method := method
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(method, "/caldav/calendars/user-1/work/event-1.ics", nil)
			req.Header.Set("If-Match", `"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`)
			req.Header.Set("If-None-Match", store.objects[0].ETag)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusPreconditionFailed {
				t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestHandlerGetCalendarObjectHonorsWebDAVIfHeader(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodGet, "/caldav/calendars/user-1/work/event-1.ics", nil)
	req.Header.Set("If", `(["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"])`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerGetCalendarObjectHonorsIfUnmodifiedSince(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.objects[0].UpdatedAt = time.Date(2026, 5, 6, 4, 5, 6, 0, time.UTC)
	handler := NewHandler(store, fixedUser("user-1"))
	for _, method := range []string{MethodGet, MethodHead} {
		method := method
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(method, "/caldav/calendars/user-1/work/event-1.ics", nil)
			req.Header.Set("If-Unmodified-Since", "Wed, 06 May 2026 04:05:05 GMT")
			req.Header.Set("If-None-Match", store.objects[0].ETag)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusPreconditionFailed {
				t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestHandlerGetCalendarObjectIgnoresNonMatchingIfNoneMatch(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodGet, "/caldav/calendars/user-1/work/event-1.ics", nil)
	req.Header.Set("If-None-Match", `"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "BEGIN:VCALENDAR") {
		t.Fatalf("GET body = %s", rec.Body.String())
	}
}

func TestHandlerPutCalendarObjectCreatesAndUpdates(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-2@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	req := httptest.NewRequest(MethodPut, "/caldav/calendars/user-1/work/event-2.ics", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
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
	updateReq.Header.Set("Content-Type", "text/calendar")
	updateReq.Header.Add("If-Match", `"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`)
	updateReq.Header.Add("If-Match", etag)
	updateRec := httptest.NewRecorder()
	handler.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusNoContent {
		t.Fatalf("update status = %d body = %s", updateRec.Code, updateRec.Body.String())
	}
}

func TestHandlerPutCalendarObjectRejectsUnsupportedContentType(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodPut, "/caldav/calendars/user-1/work/event-2.ics", strings.NewReader(`{"uid":"event-2"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415, body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerPutCalendarObjectRejectsUnsupportedContentTypeVersion(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-2@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	req := httptest.NewRequest(MethodPut, "/caldav/calendars/user-1/work/event-2.ics", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/calendar; version=1.0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415, body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerPutCalendarObjectRejectsRepeatedContentType(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-2@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	req := httptest.NewRequest(MethodPut, "/caldav/calendars/user-1/work/event-2.ics", strings.NewReader(body))
	req.Header.Add("Content-Type", "text/calendar")
	req.Header.Add("Content-Type", "text/calendar; version=2.0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415, body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerPutCalendarObjectRejectsDelegatedReadOnlyAccess(t *testing.T) {
	t.Parallel()

	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-2@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("delegate-1"))
	handler.AccessAuthorizer = &fakeCalendarAccessAuthorizer{allowedRoles: map[string]bool{CalendarAccessRoleRead: true}}
	req := httptest.NewRequest(MethodPut, "/caldav/calendars/user-1/work/event-2.ics", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/calendar")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := handler.AccessAuthorizer.(*fakeCalendarAccessAuthorizer).last; got.ActorUserID != "delegate-1" || got.OwnerUserID != "user-1" || got.RequiredRole != CalendarAccessRoleWrite {
		t.Fatalf("access request = %+v", got)
	}
}

func TestHandlerPutCalendarObjectPreservesDelegatedActor(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("delegate-1"))
	handler.AccessAuthorizer = &fakeCalendarAccessAuthorizer{allowedRoles: map[string]bool{CalendarAccessRoleWrite: true}}
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-2@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	req := httptest.NewRequest(MethodPut, "/caldav/calendars/user-1/work/event-2.ics", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/calendar")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if store.lastUpsert.UserID != "user-1" || store.lastUpsert.ActorUserID != "delegate-1" {
		t.Fatalf("delegated upsert request = %+v", store.lastUpsert)
	}
}

func TestCurrentUserPrivilegesForResourceScopesDelegatedManage(t *testing.T) {
	t.Parallel()

	got := currentUserPrivilegesForResource(ResourceCalendarObject, []XMLName{
		PrivilegeRead,
		PrivilegeBind,
		PrivilegeUnbind,
		PrivilegeWriteContent,
		PrivilegeWriteProps,
	})
	want := []XMLName{PrivilegeRead, PrivilegeWriteContent}
	if len(got) != len(want) {
		t.Fatalf("privileges = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("privileges = %+v, want %+v", got, want)
		}
	}
}

func TestHandlerPutRejectsIfMatchStarForMissingObject(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-2@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	req := httptest.NewRequest(MethodPut, "/caldav/calendars/user-1/work/event-2.ics", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/calendar")
	req.Header.Set("If-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerPutIfMatchStarCarriesObservedETag(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	req := httptest.NewRequest(MethodPut, "/caldav/calendars/user-1/work/event-1.ics", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/calendar")
	req.Header.Set("If-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body = %s", rec.Code, rec.Body.String())
	}
	if store.lastUpsert.ObservedETag != `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"` {
		t.Fatalf("put observed etag = %q", store.lastUpsert.ObservedETag)
	}
}

func TestHandlerPutHonorsWebDAVIfHeaderETag(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	req := httptest.NewRequest(MethodPut, "/caldav/calendars/user-1/work/event-1.ics", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/calendar")
	req.Header.Set("If", `(["0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"])`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body = %s", rec.Code, rec.Body.String())
	}
	if store.lastUpsert.ObservedETag != `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"` {
		t.Fatalf("put observed etag = %q", store.lastUpsert.ObservedETag)
	}
}

func TestHandlerPutRejectsFailedWebDAVIfHeaderETag(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := &readTrackingReader{data: "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"}
	req := httptest.NewRequest(MethodPut, "/caldav/calendars/user-1/work/event-1.ics", body)
	req.Header.Set("Content-Type", "text/calendar")
	req.Header.Set("If", `(["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"])`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
}

func TestHandlerPutRejectsFailedETagPreconditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		value  string
	}{
		{name: "if match mismatch", header: "If-Match", value: `"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`},
		{name: "if none match current", header: "If-None-Match", value: `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
			body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
			req := httptest.NewRequest(MethodPut, "/caldav/calendars/user-1/work/event-1.ics", strings.NewReader(body))
			req.Header.Set("Content-Type", "text/calendar")
			req.Header.Set(tc.header, tc.value)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusPreconditionFailed {
				t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestHandlerPutRejectsFailedIfUnmodifiedSince(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.objects[0].UpdatedAt = time.Date(2026, 5, 6, 4, 5, 6, 0, time.UTC)
	handler := NewHandler(store, fixedUser("user-1"))
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	req := httptest.NewRequest(MethodPut, "/caldav/calendars/user-1/work/event-1.ics", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/calendar")
	req.Header.Set("If-Unmodified-Since", "Wed, 06 May 2026 04:05:05 GMT")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerPutRejectsIfUnmodifiedSinceForMissingObjectBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	body := &readTrackingReader{data: "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-2@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"}
	req := httptest.NewRequest(MethodPut, "/caldav/calendars/user-1/work/event-2.ics", body)
	req.Header.Set("Content-Type", "text/calendar")
	req.Header.Set("If-Unmodified-Since", "Wed, 06 May 2026 04:05:05 GMT")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	if store.lastUpsert.ObjectName != "" {
		t.Fatalf("unexpected upsert for missing-object precondition: %+v", store.lastUpsert)
	}
	if _, err := store.LookupCalendarObject(t.Context(), "user-1", "work", "event-2.ics"); err == nil {
		t.Fatal("missing object was created despite failed If-Unmodified-Since precondition")
	}
}

func TestHandlerPutRejectsRepeatedIfUnmodifiedSince(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	req := httptest.NewRequest(MethodPut, "/caldav/calendars/user-1/work/event-1.ics", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/calendar")
	req.Header.Add("If-Unmodified-Since", "Wed, 06 May 2026 04:05:06 GMT")
	req.Header.Add("If-Unmodified-Since", "Wed, 06 May 2026 04:05:07 GMT")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "If-Unmodified-Since header must not be repeated") {
		t.Fatalf("response did not explain repeated If-Unmodified-Since rejection: %s", rec.Body.String())
	}
}

func TestHandlerMkcalendarCreatesCalendarAtRequestURI(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	calendarID := "11111111-1111-4111-8111-111111111111"
	req := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/"+calendarID+"/", strings.NewReader(`<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:" xmlns:CS="http://calendarserver.org/ns/" xmlns:I="http://apple.com/ns/icalendar/">
  <D:set>
    <D:prop>
      <D:displayname>Project Calendar</D:displayname>
      <C:calendar-description>Delivery milestones</C:calendar-description>
      <C:calendar-timezone>Asia/Seoul</C:calendar-timezone>
      <I:calendar-slug>project-calendar</I:calendar-slug>
      <CS:calendar-color>#aabbcc</CS:calendar-color>
    </D:prop>
  </D:set>
</C:mkcalendar>`))
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	assertCacheControlContains(t, rec, "no-store", "no-cache")
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
	if calendar.Timezone == nil || *calendar.Timezone != "Asia/Seoul" {
		t.Fatalf("timezone = %v", calendar.Timezone)
	}
	if calendar.Slug == nil || *calendar.Slug != "project-calendar" {
		t.Fatalf("calendar = %+v", calendar)
	}
}

func TestHandlerMkcalendarStoresAndReturnsCalendarPropertyLanguage(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	calendarID := "11111111-1111-4111-8111-111111111111"
	createReq := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/"+calendarID+"/", strings.NewReader(`<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:set>
    <D:prop xml:lang="ko-KR">
      <D:displayname>제품</D:displayname>
      <C:calendar-description>출시 일정</C:calendar-description>
    </D:prop>
  </D:set>
</C:mkcalendar>`))
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("MKCALENDAR status = %d body = %s", createRec.Code, createRec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", calendarID)
	if err != nil {
		t.Fatalf("created calendar lookup failed: %v", err)
	}
	if calendar.NameLang != "ko-KR" || calendar.DescriptionLang != "ko-KR" {
		t.Fatalf("calendar languages = name %q description %q", calendar.NameLang, calendar.DescriptionLang)
	}

	propfindReq := httptest.NewRequest(MethodPropfind, "/caldav/calendars/user-1/"+calendarID+"/", strings.NewReader(`<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:displayname/>
    <C:calendar-description/>
  </D:prop>
</D:propfind>`))
	propfindReq.Header.Set("Depth", "0")
	propfindRec := httptest.NewRecorder()
	handler.ServeHTTP(propfindRec, propfindReq)

	if propfindRec.Code != http.StatusMultiStatus {
		t.Fatalf("PROPFIND status = %d body = %s", propfindRec.Code, propfindRec.Body.String())
	}
	body := propfindRec.Body.String()
	for _, want := range []string{
		`<D:displayname xml:lang="ko-KR">제품</D:displayname>`,
		`<C:calendar-description xml:lang="ko-KR">출시 일정</C:calendar-description>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("PROPFIND body missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerMkcalendarRejectsMalformedLanguageBeforeCreate(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	calendarID := "11111111-1111-4111-8111-111111111111"
	req := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/"+calendarID+"/", strings.NewReader(`<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:set><D:prop xml:lang="ko KR"><D:displayname>Product</D:displayname></D:prop></D:set>
</C:mkcalendar>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if _, err := store.LookupCalendar(t.Context(), "user-1", calendarID); err == nil {
		t.Fatal("calendar was created despite malformed MKCALENDAR xml:lang")
	}
}

func TestHandlerMkcalendarRejectsNonXMLContentTypeBeforeCreate(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	calendarID := "11111111-1111-4111-8111-111111111111"
	body := &readTrackingReader{data: `<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:"><D:set><D:prop><D:displayname>Calendar</D:displayname></D:prop></D:set></C:mkcalendar>`}
	req := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/"+calendarID+"/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	if _, err := store.LookupCalendar(t.Context(), "user-1", calendarID); err == nil {
		t.Fatal("calendar was created despite non-XML MKCALENDAR Content-Type")
	}
}

func TestHandlerMkcalendarRejectsDuplicateContentTypeBeforeCreate(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	calendarID := "11111111-1111-4111-8111-111111111111"
	body := &readTrackingReader{data: `<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:"><D:set><D:prop><D:displayname>Calendar</D:displayname></D:prop></D:set></C:mkcalendar>`}
	req := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/"+calendarID+"/", body)
	req.Header.Add("Content-Type", "application/xml")
	req.Header.Add("Content-Type", "application/xml")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	if _, err := store.LookupCalendar(t.Context(), "user-1", calendarID); err == nil {
		t.Fatal("calendar was created despite duplicate MKCALENDAR Content-Type")
	}
}

func TestHandlerMkcalendarRejectsUnsupportedPropertyAtomically(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	calendarID := "11111111-1111-4111-8111-111111111111"
	req := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/"+calendarID+"/", strings.NewReader(`<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:" xmlns:E="urn:example:test">
  <D:set>
    <D:prop>
      <D:displayname>Project Calendar</D:displayname>
      <E:unknown>unsupported</E:unknown>
    </D:prop>
  </D:set>
</C:mkcalendar>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	assertCacheControlContains(t, rec, "no-store", "no-cache")
	if _, err := store.LookupCalendar(t.Context(), "user-1", calendarID); err == nil {
		t.Fatal("calendar was created despite unsupported MKCALENDAR property")
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<C:mkcalendar-response",
		`xmlns:X="urn:example:test"`,
		`<X:unknown xmlns:X="urn:example:test"></X:unknown>`,
		"HTTP/1.1 403 Forbidden",
		"<D:displayname></D:displayname>",
		"HTTP/1.1 424 Failed Dependency",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerMkcalendarRejectsInvalidPropertyAtomically(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	calendarID := "11111111-1111-4111-8111-111111111111"
	req := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/"+calendarID+"/", strings.NewReader(`<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:set>
    <D:prop>
      <D:displayname>Project Calendar</D:displayname>
      <C:calendar-timezone>No/Such_Zone</C:calendar-timezone>
    </D:prop>
  </D:set>
</C:mkcalendar>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if _, err := store.LookupCalendar(t.Context(), "user-1", calendarID); err == nil {
		t.Fatal("calendar was created despite invalid MKCALENDAR property")
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<C:mkcalendar-response",
		"<C:calendar-timezone></C:calendar-timezone>",
		"HTTP/1.1 409 Conflict",
		"<C:valid-calendar-data></C:valid-calendar-data>",
		"<D:displayname></D:displayname>",
		"HTTP/1.1 424 Failed Dependency",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerMkcalendarAllowsAbsentBody(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	calendarID := "11111111-1111-4111-8111-111111111111"
	req := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/"+calendarID+"/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if _, err := store.LookupCalendar(t.Context(), "user-1", calendarID); err != nil {
		t.Fatalf("created calendar lookup failed: %v", err)
	}
}

func TestHandlerMkcalendarRejectsNonEmptyBodyWithoutSet(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	calendarID := "11111111-1111-4111-8111-111111111111"
	req := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/"+calendarID+"/", strings.NewReader(`<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav"/>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if _, err := store.LookupCalendar(t.Context(), "user-1", calendarID); err == nil {
		t.Fatal("calendar was created from MKCALENDAR body without DAV:set")
	}
}

func TestHandlerMkcalendarRejectsWhitespaceOnlyBody(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	calendarID := "11111111-1111-4111-8111-111111111111"
	req := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/"+calendarID+"/", strings.NewReader(" \r\n\t "))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if _, err := store.LookupCalendar(t.Context(), "user-1", calendarID); err == nil {
		t.Fatal("calendar was created from whitespace-only MKCALENDAR body")
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

func TestHandlerMkcalendarRejectsExistingIfNoneMatchStarBeforeBodyRead(t *testing.T) {
	t.Parallel()

	body := &readTrackingReader{data: `<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav"/>`}
	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/work/", body)
	req.Header.Set("If-None-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
}

func TestHandlerMkcalendarRejectsMissingIfMatchStarBeforeBodyRead(t *testing.T) {
	t.Parallel()

	body := &readTrackingReader{data: `<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav"/>`}
	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	calendarID := "11111111-1111-4111-8111-111111111111"
	req := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/"+calendarID+"/", body)
	req.Header.Set("If-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	if _, err := handler.Store.LookupCalendar(t.Context(), "user-1", calendarID); err == nil {
		t.Fatal("calendar was created despite If-Match precondition")
	}
}

func TestHandlerMkcalendarAllowsMissingIfNoneMatchStar(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	calendarID := "11111111-1111-4111-8111-111111111111"
	req := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/"+calendarID+"/", strings.NewReader(`<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:"><D:set><D:prop><D:displayname>Calendar</D:displayname></D:prop></D:set></C:mkcalendar>`))
	req.Header.Set("If-None-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body = %s", rec.Code, rec.Body.String())
	}
	if _, err := store.LookupCalendar(t.Context(), "user-1", calendarID); err != nil {
		t.Fatalf("created calendar lookup failed: %v", err)
	}
}

func TestHandlerMkcalendarAcceptsSlugPath(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/not-a-uuid/", strings.NewReader(`<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:"><D:set><D:prop><D:displayname>Calendar</D:displayname></D:prop></D:set></C:mkcalendar>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerMkcalendarAcceptsSlugPathAndReadsBody(t *testing.T) {
	t.Parallel()

	body := &readTrackingReader{data: `<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:"><D:set><D:prop><D:displayname>Calendar</D:displayname></D:prop></D:set></C:mkcalendar>`}
	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/not-a-uuid/", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if body.reads == 0 {
		t.Fatalf("body reads = 0, want > 0 (body should be read to parse request)")
	}
}

func TestHandlerMkcalendarSlugPathWithIfMatchStar(t *testing.T) {
	t.Parallel()

	body := &readTrackingReader{data: `<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav"/>`}
	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodMkcalendar, "/caldav/calendars/user-1/not-a-uuid/", body)
	req.Header.Set("If-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
}

func TestHandlerProppatchUpdatesCalendarCollectionProperties(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:CS="http://calendarserver.org/ns/" xmlns:I="http://apple.com/ns/icalendar/">
  <D:set>
    <D:prop>
      <D:displayname>Product</D:displayname>
      <C:calendar-description>Launch dates</C:calendar-description>
      <C:calendar-timezone>Asia/Seoul</C:calendar-timezone>
      <I:calendar-slug>product</I:calendar-slug>
      <CS:calendar-color>#112233</CS:calendar-color>
    </D:prop>
  </D:set>
</D:propertyupdate>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Product" || calendar.Description != "Launch dates" || calendar.Color != "#112233" {
		t.Fatalf("calendar = %+v", calendar)
	}
	if calendar.Timezone == nil || *calendar.Timezone != "Asia/Seoul" {
		t.Fatalf("timezone = %v", calendar.Timezone)
	}
	if calendar.Slug == nil || *calendar.Slug != "product" {
		t.Fatalf("slug = %v", calendar.Slug)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<D:href>/caldav/calendars/user-1/work/</D:href>",
		"<D:displayname>Product</D:displayname>",
		"<C:calendar-description>Launch dates</C:calendar-description>",
		"BEGIN:VCALENDAR",
		"BEGIN:VTIMEZONE",
		"TZID:Asia/Seoul",
		"<I:calendar-slug>product</I:calendar-slug>",
		"<CS:calendar-color>#112233</CS:calendar-color>",
		"HTTP/1.1 200 OK",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("PROPPATCH response missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerProppatchStoresAndReturnsCalendarPropertyLanguage(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set>
    <D:prop xml:lang="ko-KR">
      <D:displayname>제품</D:displayname>
      <C:calendar-description>출시 일정</C:calendar-description>
    </D:prop>
  </D:set>
</D:propertyupdate>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.NameLang != "ko-KR" || calendar.DescriptionLang != "ko-KR" {
		t.Fatalf("calendar languages = name %q description %q", calendar.NameLang, calendar.DescriptionLang)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`<D:displayname xml:lang="ko-KR">제품</D:displayname>`,
		`<C:calendar-description xml:lang="ko-KR">출시 일정</C:calendar-description>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("PROPPATCH response missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerProppatchPreservesCalendarPropertyLanguageWhenOmitted(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].Description = "Old description"
	store.calendars[0].DescriptionLang = "fr"
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set>
    <D:prop>
      <D:displayname>Product</D:displayname>
      <C:calendar-description>Launch dates</C:calendar-description>
    </D:prop>
  </D:set>
</D:propertyupdate>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Product" || calendar.Description != "Launch dates" {
		t.Fatalf("calendar text = name %q description %q", calendar.Name, calendar.Description)
	}
	if calendar.NameLang != "ko-KR" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar languages = name %q description %q", calendar.NameLang, calendar.DescriptionLang)
	}
}

func TestHandlerProppatchClearsCalendarPropertyLanguageWhenExplicitlyEmpty(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].Description = "Old description"
	store.calendars[0].DescriptionLang = "fr"
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set>
    <D:prop xml:lang="">
      <D:displayname>Product</D:displayname>
      <C:calendar-description>Launch dates</C:calendar-description>
    </D:prop>
  </D:set>
</D:propertyupdate>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.NameLang != "" || calendar.DescriptionLang != "" {
		t.Fatalf("calendar languages = name %q description %q, want cleared", calendar.NameLang, calendar.DescriptionLang)
	}
	if !strings.Contains(rec.Body.String(), "<D:displayname>Product</D:displayname>") {
		t.Fatalf("response should omit empty xml:lang attribute:\n%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `calendar-description xml:lang`) {
		t.Fatalf("response should omit empty description xml:lang attribute:\n%s", rec.Body.String())
	}
}

func TestHandlerProppatchRejectsMalformedLanguageBeforeMutation(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:">
  <D:set><D:prop xml:lang="ko KR"><D:displayname>Product</D:displayname></D:prop></D:set>
</D:propertyupdate>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Work" || calendar.NameLang != "" {
		t.Fatalf("calendar mutated before xml:lang rejection: %+v", calendar)
	}
}

func TestHandlerProppatchRemovesOptionalCalendarProperties(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].DescriptionLang = "ko-KR"
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:CS="http://calendarserver.org/ns/">
  <D:remove>
    <D:prop>
      <C:calendar-description/>
      <CS:calendar-color/>
    </D:prop>
  </D:remove>
</D:propertyupdate>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Description != "" || calendar.Color != "" {
		t.Fatalf("calendar = %+v", calendar)
	}
	if calendar.DescriptionLang != "" {
		t.Fatalf("description lang = %q, want cleared", calendar.DescriptionLang)
	}
}

func TestHandlerProppatchDuplicateCalendarDescriptionUsesFinalInstructionOnce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "set remove set",
			body: `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set><D:prop><C:calendar-description>First</C:calendar-description></D:prop></D:set>
  <D:remove><D:prop><C:calendar-description/></D:prop></D:remove>
  <D:set><D:prop><C:calendar-description>Final launch calendar</C:calendar-description></D:prop></D:set>
</D:propertyupdate>`,
			want: "Final launch calendar",
		},
		{
			name: "set set",
			body: `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set><D:prop><C:calendar-description>First</C:calendar-description></D:prop></D:set>
  <D:set><D:prop><C:calendar-description>Final launch calendar</C:calendar-description></D:prop></D:set>
</D:propertyupdate>`,
			want: "Final launch calendar",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := newFakeDiscoveryStore()
			handler := NewHandler(store, fixedUser("user-1"))
			req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusMultiStatus {
				t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
			}
			calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
			if err != nil {
				t.Fatalf("calendar lookup failed: %v", err)
			}
			if calendar.Description != tt.want {
				t.Fatalf("calendar description = %q, want %q", calendar.Description, tt.want)
			}
			body := rec.Body.String()
			if count := strings.Count(body, "<C:calendar-description>"); count != 1 {
				t.Fatalf("calendar-description response count = %d, want 1:\n%s", count, body)
			}
			if !strings.Contains(body, "<C:calendar-description>"+tt.want+"</C:calendar-description>") {
				t.Fatalf("PROPPATCH response missing final description %q:\n%s", tt.want, body)
			}
		})
	}
}

func TestHandlerProppatchRejectsUnsupportedPropertyAtomically(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:E="urn:example:test">
  <D:set>
    <D:prop xml:lang="ja-JP">
      <D:displayname>Product</D:displayname>
      <E:unsupported>value</E:unsupported>
    </D:prop>
  </D:set>
</D:propertyupdate>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Work" {
		t.Fatalf("calendar name = %q, want unchanged Work", calendar.Name)
	}
	if calendar.NameLang != "ko-KR" {
		t.Fatalf("calendar name lang = %q, want unchanged ko-KR", calendar.NameLang)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded before rollback response: %+v", store.lastCalendarUpdate)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`xmlns:X="urn:example:test"`,
		`<X:unsupported xmlns:X="urn:example:test"></X:unsupported>`,
		"HTTP/1.1 403 Forbidden",
		"<D:displayname></D:displayname>",
		"HTTP/1.1 424 Failed Dependency",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("PROPPATCH unsupported response missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerProppatchFailureDeduplicatesRepeatedDependencyProperty(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].DescriptionLang = "fr"
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:E="urn:example:test">
  <D:set>
    <D:prop xml:lang="ja-JP">
      <E:unsupported>value</E:unsupported>
      <C:calendar-description>First</C:calendar-description>
    </D:prop>
  </D:set>
  <D:set>
    <D:prop xml:lang="">
      <C:calendar-description>Second</C:calendar-description>
    </D:prop>
  </D:set>
</D:propertyupdate>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Description != "Team calendar" {
		t.Fatalf("calendar description = %q, want unchanged Team calendar", calendar.Description)
	}
	if calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar description lang = %q, want unchanged fr", calendar.DescriptionLang)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded before rollback response: %+v", store.lastCalendarUpdate)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`<X:unsupported xmlns:X="urn:example:test"></X:unsupported>`,
		"HTTP/1.1 403 Forbidden",
		"HTTP/1.1 424 Failed Dependency",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("PROPPATCH unsupported response missing %q:\n%s", want, body)
		}
	}
	if count := strings.Count(body, "<C:calendar-description>"); count != 1 {
		t.Fatalf("failed dependency calendar-description count = %d, want 1:\n%s", count, body)
	}
}

func TestHandlerProppatchRejectsProtectedRemoveAtomically(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].DescriptionLang = "fr"
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:remove><D:prop><D:displayname/></D:prop></D:remove>
  <D:set><D:prop xml:lang=""><C:calendar-description>Launch</C:calendar-description></D:prop></D:set>
</D:propertyupdate>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Description != "Team calendar" {
		t.Fatalf("calendar description = %q, want unchanged Team calendar", calendar.Description)
	}
	if calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar description lang = %q, want unchanged fr", calendar.DescriptionLang)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded before rollback response: %+v", store.lastCalendarUpdate)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<D:displayname></D:displayname>",
		"HTTP/1.1 403 Forbidden",
		"<C:calendar-description></C:calendar-description>",
		"HTTP/1.1 424 Failed Dependency",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("PROPPATCH protected response missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerProppatchHonorsIfUnmodifiedSinceBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].UpdatedAt = time.Date(2026, 5, 6, 4, 5, 6, 0, time.UTC)
	body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:"><D:set><D:prop xml:lang="ja-JP"><D:displayname>Product</D:displayname></D:prop></D:set></D:propertyupdate>`}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", body)
	req.Header.Set("If-Unmodified-Since", "Wed, 06 May 2026 04:05:05 GMT")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Work" {
		t.Fatalf("calendar name = %q, want Work", calendar.Name)
	}
	if calendar.NameLang != "ko-KR" {
		t.Fatalf("calendar name lang = %q, want ko-KR", calendar.NameLang)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded despite failed precondition: %+v", store.lastCalendarUpdate)
	}
}

func TestHandlerProppatchRejectsRepeatedIfUnmodifiedSinceBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:"><D:set><D:prop xml:lang="ja-JP"><D:displayname>Product</D:displayname></D:prop></D:set></D:propertyupdate>`}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", body)
	req.Header.Add("If-Unmodified-Since", "Wed, 06 May 2026 04:05:06 GMT")
	req.Header.Add("If-Unmodified-Since", "Wed, 06 May 2026 04:05:07 GMT")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	if !strings.Contains(rec.Body.String(), "If-Unmodified-Since header must not be repeated") {
		t.Fatalf("response did not explain repeated If-Unmodified-Since rejection: %s", rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Work" || calendar.NameLang != "ko-KR" {
		t.Fatalf("calendar mutated despite failed precondition: %+v", calendar)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded despite failed precondition: %+v", store.lastCalendarUpdate)
	}
}

func TestHandlerProppatchRejectsMismatchedIfMatchBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:"><D:set><D:prop xml:lang="ja-JP"><D:displayname>Product</D:displayname></D:prop></D:set></D:propertyupdate>`}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", body)
	req.Header.Set("If-Match", `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Work" || calendar.NameLang != "ko-KR" {
		t.Fatalf("calendar mutated despite failed precondition: %+v", calendar)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded despite failed precondition: %+v", store.lastCalendarUpdate)
	}
}

func TestHandlerProppatchRejectsMatchingIfNoneMatchBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:"><D:set><D:prop xml:lang="ja-JP"><D:displayname>Product</D:displayname></D:prop></D:set></D:propertyupdate>`}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", body)
	req.Header.Set("If-None-Match", etag)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Work" {
		t.Fatalf("calendar name = %q, want Work", calendar.Name)
	}
	if calendar.NameLang != "ko-KR" {
		t.Fatalf("calendar name lang = %q, want ko-KR", calendar.NameLang)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded despite failed precondition: %+v", store.lastCalendarUpdate)
	}
}

func TestHandlerProppatchRejectsIfNoneMatchStarBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:"><D:set><D:prop xml:lang="ja-JP"><D:displayname>Product</D:displayname></D:prop></D:set></D:propertyupdate>`}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", body)
	req.Header.Set("If-None-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Work" {
		t.Fatalf("calendar name = %q, want Work", calendar.Name)
	}
	if calendar.NameLang != "ko-KR" {
		t.Fatalf("calendar name lang = %q, want ko-KR", calendar.NameLang)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded despite failed precondition: %+v", store.lastCalendarUpdate)
	}
}

func TestHandlerProppatchAcceptsMatchingCollectionIfMatch(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`))
	req.Header.Set("If-Match", `"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", `+etag)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Product" || calendar.Description != "Launch" {
		t.Fatalf("calendar text = name %q description %q", calendar.Name, calendar.Description)
	}
	if calendar.NameLang != "ko-KR" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar languages = name %q description %q", calendar.NameLang, calendar.DescriptionLang)
	}
	if store.lastCalendarUpdate.NameLang != nil || store.lastCalendarUpdate.DescriptionLang != nil {
		t.Fatalf("update langs = name %#v description %#v, want nil omitted language", store.lastCalendarUpdate.NameLang, store.lastCalendarUpdate.DescriptionLang)
	}
}

func TestHandlerProppatchAcceptsMatchingCollectionIfHeaderPreservesLanguage(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`))
	req.Header.Set("If", "(["+etag+"])")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Product" || calendar.Description != "Launch" {
		t.Fatalf("calendar text = name %q description %q", calendar.Name, calendar.Description)
	}
	if calendar.NameLang != "ko-KR" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar languages = name %q description %q", calendar.NameLang, calendar.DescriptionLang)
	}
	if store.lastCalendarUpdate.ObservedETag != etag {
		t.Fatalf("observed collection etag = %q, want %q", store.lastCalendarUpdate.ObservedETag, etag)
	}
	if store.lastCalendarUpdate.NameLang != nil || store.lastCalendarUpdate.DescriptionLang != nil {
		t.Fatalf("update langs = name %#v description %#v, want nil omitted language", store.lastCalendarUpdate.NameLang, store.lastCalendarUpdate.DescriptionLang)
	}
}

func TestHandlerProppatchAcceptsMatchingTaggedIfHeaderPreservesLanguage(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`))
	req.Header.Set("If", "</caldav/calendars/user-1/work/> (["+etag+"])")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Product" || calendar.Description != "Launch" {
		t.Fatalf("calendar text = name %q description %q", calendar.Name, calendar.Description)
	}
	if calendar.NameLang != "ko-KR" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar languages = name %q description %q", calendar.NameLang, calendar.DescriptionLang)
	}
	if store.lastCalendarUpdate.ObservedETag != etag {
		t.Fatalf("observed collection etag = %q, want %q", store.lastCalendarUpdate.ObservedETag, etag)
	}
	if store.lastCalendarUpdate.NameLang != nil || store.lastCalendarUpdate.DescriptionLang != nil {
		t.Fatalf("update langs = name %#v description %#v, want nil omitted language", store.lastCalendarUpdate.NameLang, store.lastCalendarUpdate.DescriptionLang)
	}
}

func TestHandlerProppatchRejectsMismatchedIfHeaderBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop xml:lang="ja-JP"><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", body)
	req.Header.Set("If", `(["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"])`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Work" || calendar.NameLang != "ko-KR" {
		t.Fatalf("calendar name mutated despite failed precondition: %+v", calendar)
	}
	if calendar.Description != "Team calendar" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar description mutated despite failed precondition: %+v", calendar)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded despite failed precondition: %+v", store.lastCalendarUpdate)
	}
}

func TestHandlerProppatchRejectsNonMatchingTaggedIfHeaderBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop xml:lang="ja-JP"><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", body)
	req.Header.Set("If", "</caldav/calendars/user-1/other/> (["+etag+"])")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Work" || calendar.NameLang != "ko-KR" {
		t.Fatalf("calendar name mutated despite failed precondition: %+v", calendar)
	}
	if calendar.Description != "Team calendar" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar description mutated despite failed precondition: %+v", calendar)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded despite failed precondition: %+v", store.lastCalendarUpdate)
	}
}

func TestHandlerProppatchRejectsMalformedNonMatchingTaggedIfHeaderBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop xml:lang="ja-JP"><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", body)
	req.Header.Set("If", `([`+etag+`]) </caldav/calendars/user-1/other/> ()`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "If header contains an empty condition list") {
		t.Fatalf("body = %s, want malformed If detail", rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Work" || calendar.NameLang != "ko-KR" {
		t.Fatalf("calendar name mutated despite malformed precondition: %+v", calendar)
	}
	if calendar.Description != "Team calendar" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar description mutated despite malformed precondition: %+v", calendar)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded despite malformed precondition: %+v", store.lastCalendarUpdate)
	}
}

func TestHandlerProppatchRejectsStaleTaggedIfHeaderBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop xml:lang="ja-JP"><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", body)
	req.Header.Set("If", `</caldav/calendars/user-1/work/> (["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"])`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Work" || calendar.NameLang != "ko-KR" {
		t.Fatalf("calendar name mutated despite failed precondition: %+v", calendar)
	}
	if calendar.Description != "Team calendar" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar description mutated despite failed precondition: %+v", calendar)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded despite failed precondition: %+v", store.lastCalendarUpdate)
	}
}

func TestHandlerProppatchAcceptsAbsoluteTaggedIfHeaderPreservesLanguage(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`))
	req.Header.Set("If", "<https://calendar.example.test/caldav/calendars/user-1/work/> (["+etag+"])")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Product" || calendar.Description != "Launch" {
		t.Fatalf("calendar text = name %q description %q", calendar.Name, calendar.Description)
	}
	if calendar.NameLang != "ko-KR" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar languages = name %q description %q", calendar.NameLang, calendar.DescriptionLang)
	}
	if store.lastCalendarUpdate.ObservedETag != etag {
		t.Fatalf("observed collection etag = %q, want %q", store.lastCalendarUpdate.ObservedETag, etag)
	}
	if store.lastCalendarUpdate.NameLang != nil || store.lastCalendarUpdate.DescriptionLang != nil {
		t.Fatalf("update langs = name %#v description %#v, want nil omitted language", store.lastCalendarUpdate.NameLang, store.lastCalendarUpdate.DescriptionLang)
	}
}

func TestHandlerProppatchRejectsAbsoluteTaggedIfHeaderPathMismatchBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop xml:lang="ja-JP"><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", body)
	req.Header.Set("If", "<https://calendar.example.test/caldav/calendars/user-1/other/> (["+etag+"])")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Work" || calendar.NameLang != "ko-KR" {
		t.Fatalf("calendar name mutated despite failed precondition: %+v", calendar)
	}
	if calendar.Description != "Team calendar" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar description mutated despite failed precondition: %+v", calendar)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded despite failed precondition: %+v", store.lastCalendarUpdate)
	}
}

func TestHandlerProppatchAcceptsNotIfHeaderPreservesLanguage(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`))
	req.Header.Set("If", `(Not ["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"])`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Product" || calendar.Description != "Launch" {
		t.Fatalf("calendar text = name %q description %q", calendar.Name, calendar.Description)
	}
	if calendar.NameLang != "ko-KR" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar languages = name %q description %q", calendar.NameLang, calendar.DescriptionLang)
	}
	if store.lastCalendarUpdate.NameLang != nil || store.lastCalendarUpdate.DescriptionLang != nil {
		t.Fatalf("update langs = name %#v description %#v, want nil omitted language", store.lastCalendarUpdate.NameLang, store.lastCalendarUpdate.DescriptionLang)
	}
}

func TestHandlerProppatchRejectsCurrentNotIfHeaderBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop xml:lang="ja-JP"><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", body)
	req.Header.Set("If", "(Not ["+etag+"])")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Work" || calendar.NameLang != "ko-KR" {
		t.Fatalf("calendar name mutated despite failed precondition: %+v", calendar)
	}
	if calendar.Description != "Team calendar" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar description mutated despite failed precondition: %+v", calendar)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded despite failed precondition: %+v", store.lastCalendarUpdate)
	}
}

func TestHandlerProppatchRejectsStateTokenIfHeaderBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop xml:lang="ja-JP"><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", body)
	req.Header.Set("If", `(<opaquelocktoken:missing-lock>)`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Work" || calendar.NameLang != "ko-KR" {
		t.Fatalf("calendar name mutated despite failed precondition: %+v", calendar)
	}
	if calendar.Description != "Team calendar" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar description mutated despite failed precondition: %+v", calendar)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded despite failed precondition: %+v", store.lastCalendarUpdate)
	}
}

func TestHandlerProppatchAcceptsNotStateTokenIfHeaderPreservesLanguage(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`))
	req.Header.Set("If", `(Not <opaquelocktoken:missing-lock>)`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Product" || calendar.Description != "Launch" {
		t.Fatalf("calendar text = name %q description %q", calendar.Name, calendar.Description)
	}
	if calendar.NameLang != "ko-KR" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar languages = name %q description %q", calendar.NameLang, calendar.DescriptionLang)
	}
	if store.lastCalendarUpdate.NameLang != nil || store.lastCalendarUpdate.DescriptionLang != nil {
		t.Fatalf("update langs = name %#v description %#v, want nil omitted language", store.lastCalendarUpdate.NameLang, store.lastCalendarUpdate.DescriptionLang)
	}
}

func TestHandlerProppatchAcceptsCompoundIfHeaderPreservesLanguage(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`))
	req.Header.Set("If", `([`+etag+`] Not ["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"])`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Product" || calendar.Description != "Launch" {
		t.Fatalf("calendar text = name %q description %q", calendar.Name, calendar.Description)
	}
	if calendar.NameLang != "ko-KR" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar languages = name %q description %q", calendar.NameLang, calendar.DescriptionLang)
	}
	if store.lastCalendarUpdate.ObservedETag != etag {
		t.Fatalf("observed collection etag = %q, want %q", store.lastCalendarUpdate.ObservedETag, etag)
	}
	if store.lastCalendarUpdate.NameLang != nil || store.lastCalendarUpdate.DescriptionLang != nil {
		t.Fatalf("update langs = name %#v description %#v, want nil omitted language", store.lastCalendarUpdate.NameLang, store.lastCalendarUpdate.DescriptionLang)
	}
}

func TestHandlerProppatchRejectsCompoundIfHeaderBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop xml:lang="ja-JP"><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", body)
	req.Header.Set("If", `([`+etag+`] <opaquelocktoken:missing-lock>)`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Work" || calendar.NameLang != "ko-KR" {
		t.Fatalf("calendar name mutated despite failed precondition: %+v", calendar)
	}
	if calendar.Description != "Team calendar" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar description mutated despite failed precondition: %+v", calendar)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded despite failed precondition: %+v", store.lastCalendarUpdate)
	}
}

func TestHandlerProppatchAcceptsMultiListIfHeaderPreservesLanguage(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`))
	req.Header.Set("If", `(["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]) ([`+etag+`])`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Product" || calendar.Description != "Launch" {
		t.Fatalf("calendar text = name %q description %q", calendar.Name, calendar.Description)
	}
	if calendar.NameLang != "ko-KR" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar languages = name %q description %q", calendar.NameLang, calendar.DescriptionLang)
	}
	if store.lastCalendarUpdate.ObservedETag != etag {
		t.Fatalf("observed collection etag = %q, want %q", store.lastCalendarUpdate.ObservedETag, etag)
	}
	if store.lastCalendarUpdate.NameLang != nil || store.lastCalendarUpdate.DescriptionLang != nil {
		t.Fatalf("update langs = name %#v description %#v, want nil omitted language", store.lastCalendarUpdate.NameLang, store.lastCalendarUpdate.DescriptionLang)
	}
}

func TestHandlerProppatchRejectsMultiListIfHeaderBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop xml:lang="ja-JP"><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", body)
	req.Header.Set("If", `(["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]) (Not [`+etag+`])`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Work" || calendar.NameLang != "ko-KR" {
		t.Fatalf("calendar name mutated despite failed precondition: %+v", calendar)
	}
	if calendar.Description != "Team calendar" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar description mutated despite failed precondition: %+v", calendar)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded despite failed precondition: %+v", store.lastCalendarUpdate)
	}
}

func TestHandlerProppatchAcceptsRepeatedIfHeadersPreservesLanguage(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`))
	req.Header.Add("If", `(["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"])`)
	req.Header.Add("If", `([`+etag+`])`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Product" || calendar.Description != "Launch" {
		t.Fatalf("calendar text = name %q description %q", calendar.Name, calendar.Description)
	}
	if calendar.NameLang != "ko-KR" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar languages = name %q description %q", calendar.NameLang, calendar.DescriptionLang)
	}
	if store.lastCalendarUpdate.ObservedETag != etag {
		t.Fatalf("observed collection etag = %q, want %q", store.lastCalendarUpdate.ObservedETag, etag)
	}
	if store.lastCalendarUpdate.NameLang != nil || store.lastCalendarUpdate.DescriptionLang != nil {
		t.Fatalf("update langs = name %#v description %#v, want nil omitted language", store.lastCalendarUpdate.NameLang, store.lastCalendarUpdate.DescriptionLang)
	}
}

func TestHandlerProppatchRejectsRepeatedIfHeadersBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop xml:lang="ja-JP"><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", body)
	req.Header.Add("If", `(["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"])`)
	req.Header.Add("If", `(Not [`+etag+`])`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Work" || calendar.NameLang != "ko-KR" {
		t.Fatalf("calendar name mutated despite failed precondition: %+v", calendar)
	}
	if calendar.Description != "Team calendar" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar description mutated despite failed precondition: %+v", calendar)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded despite failed precondition: %+v", store.lastCalendarUpdate)
	}
}

func TestHandlerProppatchRejectsRepeatedMalformedIfHeadersBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].NameLang = "ko-KR"
	store.calendars[0].DescriptionLang = "fr"
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop xml:lang="ja-JP"><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", body)
	req.Header.Add("If", `([`+etag+`])`)
	req.Header.Add("If", `()`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "If header contains an empty condition list") {
		t.Fatalf("body = %s, want malformed If detail", rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
	if err != nil {
		t.Fatalf("calendar lookup failed: %v", err)
	}
	if calendar.Name != "Work" || calendar.NameLang != "ko-KR" {
		t.Fatalf("calendar name mutated despite malformed precondition: %+v", calendar)
	}
	if calendar.Description != "Team calendar" || calendar.DescriptionLang != "fr" {
		t.Fatalf("calendar description mutated despite malformed precondition: %+v", calendar)
	}
	if store.lastCalendarUpdate.CalendarID != "" {
		t.Fatalf("update request recorded despite malformed precondition: %+v", store.lastCalendarUpdate)
	}
}

func TestHandlerProppatchRejectsMalformedIfHeaderBeforeBodyRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ifHeader   string
		wantDetail string
	}{
		{
			name:       "line break",
			ifHeader:   "(\n)",
			wantDetail: "If header must not contain line breaks",
		},
		{
			name:       "unterminated condition list",
			ifHeader:   `(["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]`,
			wantDetail: "If header contains an unterminated condition list",
		},
		{
			name:       "empty condition list",
			ifHeader:   `()`,
			wantDetail: "If header contains an empty condition list",
		},
		{
			name:       "unsupported condition",
			ifHeader:   `(bogus)`,
			wantDetail: "If header contains an unsupported condition",
		},
		{
			name:       "unterminated entity tag",
			ifHeader:   `(["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa)`,
			wantDetail: "If header contains an unterminated entity-tag",
		},
		{
			name:       "unterminated state token",
			ifHeader:   `(<opaquelocktoken:test)`,
			wantDetail: "If header contains an unterminated state-token",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := newFakeDiscoveryStore()
			store.calendars[0].NameLang = "ko-KR"
			store.calendars[0].DescriptionLang = "fr"
			body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop xml:lang="ja-JP"><D:displayname>Product</D:displayname><C:calendar-description>Launch</C:calendar-description></D:prop></D:set></D:propertyupdate>`}
			handler := NewHandler(store, fixedUser("user-1"))
			req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", body)
			req.Header.Set("If", tc.ifHeader)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
			}
			if body.reads != 0 {
				t.Fatalf("body reads = %d, want 0", body.reads)
			}
			if !strings.Contains(rec.Body.String(), tc.wantDetail) {
				t.Fatalf("response missing %q: %s", tc.wantDetail, rec.Body.String())
			}
			calendar, err := store.LookupCalendar(t.Context(), "user-1", "work")
			if err != nil {
				t.Fatalf("calendar lookup failed: %v", err)
			}
			if calendar.Name != "Work" || calendar.NameLang != "ko-KR" {
				t.Fatalf("calendar name mutated despite malformed If header: %+v", calendar)
			}
			if calendar.Description != "Team calendar" || calendar.DescriptionLang != "fr" {
				t.Fatalf("calendar description mutated despite malformed If header: %+v", calendar)
			}
			if store.lastCalendarUpdate.CalendarID != "" {
				t.Fatalf("update request recorded despite malformed If header: %+v", store.lastCalendarUpdate)
			}
		})
	}
}

func TestHandlerProppatchIfMatchStarCarriesObservedCollectionETag(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodProppatch, "/caldav/calendars/user-1/work/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:"><D:set><D:prop xml:lang="ja-JP"><D:displayname>Product</D:displayname></D:prop></D:set></D:propertyupdate>`))
	req.Header.Set("If-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if store.lastCalendarUpdate.ObservedETag != etag {
		t.Fatalf("observed collection etag = %q, want %q", store.lastCalendarUpdate.ObservedETag, etag)
	}
	if store.lastCalendarUpdate.NameLang == nil || *store.lastCalendarUpdate.NameLang != "ja-JP" {
		t.Fatalf("update name lang = %#v, want ja-JP", store.lastCalendarUpdate.NameLang)
	}
}

func TestHandlerProppatchRejectsUnsafeTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		userID string
		target string
		body   string
		want   int
	}{
		{name: "cross user", userID: "user-2", target: "/caldav/calendars/user-1/work/", body: `<D:propertyupdate xmlns:D="DAV:"><D:set><D:prop><D:displayname>Work</D:displayname></D:prop></D:set></D:propertyupdate>`, want: http.StatusForbidden},
		{name: "object target", userID: "user-1", target: "/caldav/calendars/user-1/work/event-1.ics", body: `<D:propertyupdate xmlns:D="DAV:"><D:set><D:prop><D:displayname>Work</D:displayname></D:prop></D:set></D:propertyupdate>`, want: http.StatusForbidden},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := NewHandler(newFakeDiscoveryStore(), fixedUser(tc.userID))
			req := httptest.NewRequest(MethodProppatch, tc.target, strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestHandlerPutRejectsFailedPreconditions(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
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

func TestHandlerDeleteCalendarCollectionDeletesObjects(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if len(store.calendars) != 0 {
		t.Fatalf("calendars after delete = %+v", store.calendars)
	}
	if len(store.objects) != 0 {
		t.Fatalf("objects after delete = %+v", store.objects)
	}
}

func TestHandlerDeleteCalendarCollectionAllowsDelegatedManage(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("delegate-1"))
	handler.AccessAuthorizer = &fakeCalendarAccessAuthorizer{allowedRoles: map[string]bool{CalendarAccessRoleManage: true}}
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := handler.AccessAuthorizer.(*fakeCalendarAccessAuthorizer).last; got.ActorUserID != "delegate-1" || got.OwnerUserID != "user-1" || got.RequiredRole != CalendarAccessRoleManage {
		t.Fatalf("access request = %+v", got)
	}
	if len(store.calendars) != 0 {
		t.Fatalf("calendars after delete = %+v", store.calendars)
	}
}

func TestHandlerDeleteCalendarCollectionHonorsIfUnmodifiedSince(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.calendars[0].UpdatedAt = time.Date(2026, 5, 6, 4, 5, 6, 0, time.UTC)
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/", nil)
	req.Header.Set("If-Unmodified-Since", "Wed, 06 May 2026 04:05:05 GMT")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if len(store.calendars) != 1 {
		t.Fatalf("calendars after rejected delete = %d, want 1", len(store.calendars))
	}
	if len(store.objects) != 1 {
		t.Fatalf("objects after rejected delete = %d, want 1", len(store.objects))
	}
}

func TestHandlerDeleteCalendarCollectionRejectsRepeatedIfUnmodifiedSince(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/", nil)
	req.Header.Add("If-Unmodified-Since", "Wed, 06 May 2026 04:05:06 GMT")
	req.Header.Add("If-Unmodified-Since", "Wed, 06 May 2026 04:05:07 GMT")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "If-Unmodified-Since header must not be repeated") {
		t.Fatalf("response did not explain repeated If-Unmodified-Since rejection: %s", rec.Body.String())
	}
}

func TestHandlerDeleteCalendarCollectionRejectsMismatchedIfMatch(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/", nil)
	req.Header.Set("If-Match", `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if len(store.calendars) != 1 {
		t.Fatalf("calendars after rejected delete = %d, want 1", len(store.calendars))
	}
}

func TestHandlerDeleteCalendarCollectionRejectsMatchingIfNoneMatch(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/", nil)
	req.Header.Set("If-None-Match", etag)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if len(store.calendars) != 1 {
		t.Fatalf("calendars after rejected delete = %d, want 1", len(store.calendars))
	}
	if len(store.objects) != 1 {
		t.Fatalf("objects after rejected delete = %d, want 1", len(store.objects))
	}
}

func TestHandlerDeleteCalendarCollectionRejectsIfNoneMatchStar(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/", nil)
	req.Header.Set("If-None-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if len(store.calendars) != 1 {
		t.Fatalf("calendars after rejected delete = %d, want 1", len(store.calendars))
	}
	if len(store.objects) != 1 {
		t.Fatalf("objects after rejected delete = %d, want 1", len(store.objects))
	}
}

func TestHandlerDeleteCalendarCollectionAcceptsMatchingIfMatch(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/", nil)
	req.Header.Set("If-Match", etag)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body = %s", rec.Code, rec.Body.String())
	}
	if len(store.calendars) != 0 {
		t.Fatalf("calendars after delete = %+v", store.calendars)
	}
}

func TestHandlerDeleteUsesDefaultUserResolver(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := &Handler{Store: store}
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/event-1.ics?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body = %s", rec.Code, rec.Body.String())
	}
	if len(store.objects) != 0 {
		t.Fatalf("objects after delete = %+v", store.objects)
	}
}

func TestHandlerDeleteCalendarCollectionAcceptsIfMatchStar(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	etag, err := CalendarCollectionETag("user-1", store.calendars[0])
	if err != nil {
		t.Fatalf("CalendarCollectionETag returned error: %v", err)
	}
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/", nil)
	req.Header.Set("If-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body = %s", rec.Code, rec.Body.String())
	}
	if len(store.calendars) != 0 {
		t.Fatalf("calendars after delete = %+v", store.calendars)
	}
	if store.lastCalendarDelete.ObservedETag != etag {
		t.Fatalf("observed collection etag = %q, want %q", store.lastCalendarDelete.ObservedETag, etag)
	}
}

func TestHandlerDeleteCalendarCollectionRejectsCrossUser(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-2"))
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerDeleteRejectsCalendarHome(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
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

func TestHandlerDeleteRejectsIfMatchStarForMissingObject(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/missing.ics", nil)
	req.Header.Set("If-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerDeleteRejectsMatchingIfNoneMatch(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/event-1.ics", nil)
	req.Header.Set("If-None-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if _, err := store.LookupCalendarObject(context.Background(), "user-1", "work", "event-1.ics"); err != nil {
		t.Fatalf("object was deleted despite If-None-Match precondition: %v", err)
	}
}

func TestHandlerDeleteIfMatchStarCarriesObservedETag(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/event-1.ics", nil)
	req.Header.Set("If-Match", "*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body = %s", rec.Code, rec.Body.String())
	}
	if store.lastDelete.ObservedETag != `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"` {
		t.Fatalf("delete observed etag = %q", store.lastDelete.ObservedETag)
	}
}

func TestHandlerDeleteRejectsFailedIfUnmodifiedSince(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	store.objects[0].UpdatedAt = time.Date(2026, 5, 6, 4, 5, 6, 0, time.UTC)
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/event-1.ics", nil)
	req.Header.Set("If-Unmodified-Since", "Wed, 06 May 2026 04:05:05 GMT")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if len(store.objects) != 1 {
		t.Fatalf("objects after rejected delete = %d, want 1", len(store.objects))
	}
}

func TestHandlerDeleteCalendarObjectPreservesDelegatedActor(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("delegate-1"))
	handler.AccessAuthorizer = &fakeCalendarAccessAuthorizer{allowedRoles: map[string]bool{CalendarAccessRoleWrite: true}}
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/event-1.ics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if store.lastDelete.UserID != "user-1" || store.lastDelete.ActorUserID != "delegate-1" {
		t.Fatalf("delegated delete request = %+v", store.lastDelete)
	}
}

func TestHandlerDeleteCalendarObjectRejectsRepeatedIfUnmodifiedSince(t *testing.T) {
	t.Parallel()

	handler := NewHandler(newFakeDiscoveryStore(), fixedUser("user-1"))
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/event-1.ics", nil)
	req.Header.Add("If-Unmodified-Since", "Wed, 06 May 2026 04:05:06 GMT")
	req.Header.Add("If-Unmodified-Since", "Wed, 06 May 2026 04:05:07 GMT")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "If-Unmodified-Since header must not be repeated") {
		t.Fatalf("response did not explain repeated If-Unmodified-Since rejection: %s", rec.Body.String())
	}
}

func TestHandlerDeleteAcceptsListedETag(t *testing.T) {
	t.Parallel()

	store := newFakeDiscoveryStore()
	handler := NewHandler(store, fixedUser("user-1"))
	req := httptest.NewRequest(MethodDelete, "/caldav/calendars/user-1/work/event-1.ics", nil)
	req.Header.Set("If-Match", `"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", `+store.objects[0].ETag)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body = %s", rec.Code, rec.Body.String())
	}
	if store.lastDelete.ObservedETag != `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"` {
		t.Fatalf("delete observed etag = %q", store.lastDelete.ObservedETag)
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

func TestBuildVTIMEZONEKeepsCalendarPropertiesInsideCalendar(t *testing.T) {
	t.Parallel()

	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		t.Fatalf("LoadLocation returned error: %v", err)
	}
	body, err := buildVTIMEZONE("Asia/Seoul", loc)
	if err != nil {
		t.Fatalf("buildVTIMEZONE returned error: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"BEGIN:VCALENDAR\r\n",
		"BEGIN:VTIMEZONE\r\n",
		"TZID:Asia/Seoul\r\n",
		"END:VTIMEZONE\r\n",
		"X-WR-CALDESC:Generated by gogomail CalDAV Timezone Service\r\n",
		"END:VCALENDAR\r\n",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("VTIMEZONE body missing %q:\n%s", want, text)
		}
	}
	if strings.Index(text, "X-WR-CALDESC:") > strings.Index(text, "END:VCALENDAR") {
		t.Fatalf("calendar property emitted after END:VCALENDAR:\n%s", text)
	}
}

type fakeDiscoveryStore struct {
	principal          Principal
	calendars          []Calendar
	objects            []CalendarObject
	changes            []CalendarChange
	lastUpsert         UpsertObjectRequest
	lastDelete         DeleteObjectRequest
	lastCalendarUpdate UpdateCalendarRequest
	lastCalendarDelete DeleteCalendarRequest
}

type noSyncCalendarDiscoveryStore struct {
	store fakeDiscoveryStore
}

type queryCandidateCalendarDiscoveryStore struct {
	fakeDiscoveryStore
	candidateCount int
	listCount      int
	components     []string
}

func (s *noSyncCalendarDiscoveryStore) LookupPrincipal(ctx context.Context, userID string) (Principal, error) {
	return s.store.LookupPrincipal(ctx, userID)
}

func (s *noSyncCalendarDiscoveryStore) ListCalendarCollections(ctx context.Context, userID string) ([]Calendar, error) {
	return s.store.ListCalendarCollections(ctx, userID)
}

func (s *noSyncCalendarDiscoveryStore) LookupCalendar(ctx context.Context, userID string, calendarID string) (Calendar, error) {
	return s.store.LookupCalendar(ctx, userID, calendarID)
}

func (s *noSyncCalendarDiscoveryStore) LookupCalendarBySlug(ctx context.Context, userID string, slug string) (Calendar, error) {
	return s.store.LookupCalendarBySlug(ctx, userID, slug)
}

func (s *noSyncCalendarDiscoveryStore) ListCalendarObjects(ctx context.Context, userID string, calendarID string) ([]CalendarObject, error) {
	return s.store.ListCalendarObjects(ctx, userID, calendarID)
}

func (s *noSyncCalendarDiscoveryStore) LookupCalendarObject(ctx context.Context, userID string, calendarID string, objectName string) (CalendarObject, error) {
	return s.store.LookupCalendarObject(ctx, userID, calendarID, objectName)
}

func (s *queryCandidateCalendarDiscoveryStore) ListCalendarObjects(ctx context.Context, userID string, calendarID string) ([]CalendarObject, error) {
	s.listCount++
	return s.fakeDiscoveryStore.ListCalendarObjects(ctx, userID, calendarID)
}

func (s *queryCandidateCalendarDiscoveryStore) WalkCalendarQueryCandidates(_ context.Context, userID string, calendarID string, status string, component string, yield func(CalendarObject) (bool, error)) error {
	s.candidateCount++
	s.components = append(s.components, component)
	for _, object := range s.objects {
		if object.UserID != userID || object.CalendarID != calendarID || !strings.EqualFold(object.Component, component) {
			continue
		}
		keepGoing, err := yield(object)
		if err != nil {
			return err
		}
		if !keepGoing {
			return nil
		}
	}
	return nil
}

type fakeCalendarAccessAuthorizer struct {
	allowedRoles map[string]bool
	privileges   []XMLName
	last         AccessRequest
	err          error
}

func (a *fakeCalendarAccessAuthorizer) AuthorizeCalendarAccess(_ context.Context, req AccessRequest) (AccessDecision, error) {
	a.last = req
	if a.err != nil {
		return AccessDecision{}, a.err
	}
	return AccessDecision{Allowed: a.allowedRoles[req.RequiredRole], Privileges: append([]XMLName(nil), a.privileges...)}, nil
}

type readTrackingReader struct {
	data  string
	reads int
	pos   int
}

func (r *readTrackingReader) Read(p []byte) (int, error) {
	r.reads++
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func newFakeDiscoveryStore() *fakeDiscoveryStore {
	now := time.Now()
	eventICS := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//CalDAV Test//EN\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nDTSTAMP:20260506T000000Z\r\nDTSTART:20260506T010000Z\r\nDTEND:20260506T020000Z\r\nSUMMARY:Planning\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	return &fakeDiscoveryStore{
		principal: Principal{
			UserID:                "user-1",
			DisplayName:           "User One",
			CalendarHomePath:      "/caldav/calendars/user-1/",
			PrincipalPath:         "/caldav/principals/user-1/",
			CalendarUserAddresses: []string{"mailto:user.one@example.com"},
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
			Component:  ComponentVEVENT,
			ETag:       `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`,
			Size:       int64(len(eventICS)),
			ICS:        eventICS,
			CreatedAt:  now,
			UpdatedAt:  now,
		}},
		changes: []CalendarChange{{
			ID:         1,
			UserID:     "user-1",
			CalendarID: "work",
			SyncToken:  "sync-calendar",
			Action:     "collection-created",
			ChangedAt:  now,
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

func (s *fakeDiscoveryStore) LookupCalendarBySlug(_ context.Context, userID string, slug string) (Calendar, error) {
	for _, calendar := range s.calendars {
		if calendar.UserID == userID && calendar.Slug != nil && *calendar.Slug == slug {
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

func (s *fakeDiscoveryStore) ListCalendarObjectsLimit(_ context.Context, userID string, calendarID string, limit int) ([]CalendarObject, error) {
	objects, err := s.ListCalendarObjects(context.Background(), userID, calendarID)
	if err != nil {
		return nil, err
	}
	if limit >= 0 && len(objects) > limit {
		objects = objects[:limit]
	}
	return objects, nil
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
	s.lastUpsert = validated
	now := time.Now()
	object := CalendarObject{
		ID:         "object-" + validated.ObjectName,
		UserID:     validated.UserID,
		CalendarID: validated.CalendarID,
		ObjectName: validated.ObjectName,
		UID:        validated.UID,
		Component:  validated.Component,
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
			s.recordChange(validated.UserID, validated.CalendarID, "object-upserted", validated.ObjectName, etag)
			return object, nil
		}
	}
	s.objects = append(s.objects, object)
	s.recordChange(validated.UserID, validated.CalendarID, "object-upserted", validated.ObjectName, etag)
	return object, nil
}

func (s *fakeDiscoveryStore) CreateCalendarAtPath(_ context.Context, req CreateCalendarAtPathRequest) (Calendar, error) {
	validated, _, syncToken, _, _, err := ValidateCreateCalendarAtPathRequest(req)
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
		Slug:        validated.Slug,
		Timezone:    validated.Timezone,
		Color:       validated.Color,
		Description: validated.Description,
		SyncToken:   syncToken,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if validated.NameLang != nil {
		calendar.NameLang = *validated.NameLang
	}
	if validated.DescriptionLang != nil {
		calendar.DescriptionLang = *validated.DescriptionLang
	}
	s.calendars = append(s.calendars, calendar)
	return calendar, nil
}

func (s *fakeDiscoveryStore) UpdateCalendarProperties(_ context.Context, req UpdateCalendarRequest) (Calendar, error) {
	validated, _, _, _, _, err := ValidateUpdateCalendarRequest(req)
	if err != nil {
		return Calendar{}, err
	}
	s.lastCalendarUpdate = validated
	for i, calendar := range s.calendars {
		if calendar.UserID == validated.UserID && calendar.ID == validated.CalendarID {
			if validated.ObservedETag != "" {
				etag, err := CalendarCollectionETag(validated.UserID, calendar)
				if err != nil {
					return Calendar{}, err
				}
				if etag != validated.ObservedETag {
					return Calendar{}, errFakeNotFound
				}
			}
			if validated.Name != nil {
				calendar.Name = *validated.Name
			}
			if validated.NameLang != nil {
				calendar.NameLang = *validated.NameLang
			}
			if validated.Slug != nil {
				calendar.Slug = validated.Slug
			}
			if validated.Timezone != nil {
				calendar.Timezone = validated.Timezone
			}
			if validated.Color != nil {
				calendar.Color = *validated.Color
			}
			if validated.Description != nil {
				calendar.Description = *validated.Description
			}
			if validated.DescriptionLang != nil {
				calendar.DescriptionLang = *validated.DescriptionLang
			}
			calendar.UpdatedAt = time.Now()
			s.calendars[i] = calendar
			s.recordChange(validated.UserID, validated.CalendarID, "collection-updated", "", "")
			return s.calendars[i], nil
		}
	}
	return Calendar{}, errFakeNotFound
}

func (s *fakeDiscoveryStore) DeleteObject(_ context.Context, req DeleteObjectRequest) (CalendarObject, error) {
	validated, _, err := ValidateDeleteObjectRequest(req)
	if err != nil {
		return CalendarObject{}, err
	}
	s.lastDelete = validated
	for i, object := range s.objects {
		if object.UserID == validated.UserID && object.CalendarID == validated.CalendarID && object.ObjectName == validated.ObjectName {
			if validated.ObservedETag != "" && object.ETag != validated.ObservedETag {
				return CalendarObject{}, errFakeNotFound
			}
			s.objects = append(s.objects[:i], s.objects[i+1:]...)
			s.recordChange(validated.UserID, validated.CalendarID, "object-deleted", validated.ObjectName, object.ETag)
			return object, nil
		}
	}
	return CalendarObject{}, errFakeNotFound
}

func (s *fakeDiscoveryStore) DeleteCalendar(_ context.Context, req DeleteCalendarRequest) (Calendar, error) {
	validated, err := ValidateDeleteCalendarRequest(req)
	if err != nil {
		return Calendar{}, err
	}
	s.lastCalendarDelete = validated
	for i, calendar := range s.calendars {
		if calendar.UserID == validated.UserID && calendar.ID == validated.CalendarID {
			if validated.ObservedETag != "" {
				etag, err := CalendarCollectionETag(validated.UserID, calendar)
				if err != nil {
					return Calendar{}, err
				}
				if etag != validated.ObservedETag {
					return Calendar{}, errFakeNotFound
				}
			}
			s.calendars = append(s.calendars[:i], s.calendars[i+1:]...)
			objects := s.objects[:0]
			for _, object := range s.objects {
				if object.UserID == validated.UserID && object.CalendarID == validated.CalendarID {
					continue
				}
				objects = append(objects, object)
			}
			s.objects = objects
			s.recordChange(validated.UserID, validated.CalendarID, "collection-deleted", "", "")
			return calendar, nil
		}
	}
	return Calendar{}, errFakeNotFound
}

func (s *fakeDiscoveryStore) ListCalendarChangesSince(_ context.Context, req ListChangesSinceRequest) ([]CalendarChange, error) {
	validated, err := ValidateListChangesSinceRequest(req)
	if err != nil {
		return nil, err
	}
	marker := int64(0)
	for _, change := range s.changes {
		if change.UserID == validated.UserID && change.CalendarID == validated.CalendarID && change.SyncToken == validated.SyncToken {
			marker = change.ID
			break
		}
	}
	if marker == 0 {
		return nil, InvalidSyncTokenError{Token: validated.SyncToken}
	}
	var changes []CalendarChange
	for _, change := range s.changes {
		if change.UserID == validated.UserID && change.CalendarID == validated.CalendarID && change.ID > marker {
			changes = append(changes, change)
		}
	}
	if validated.Limit < len(changes) {
		changes = changes[:validated.Limit]
	}
	return changes, nil
}

func (s *fakeDiscoveryStore) recordChange(userID string, calendarID string, action string, objectName string, etag string) {
	token := CalendarSyncToken(userID, calendarID, action, objectName, etag, time.Now().UTC().Format(time.RFC3339Nano))
	for i := range s.calendars {
		if s.calendars[i].UserID == userID && s.calendars[i].ID == calendarID {
			s.calendars[i].SyncToken = token
			s.calendars[i].UpdatedAt = time.Now()
			break
		}
	}
	s.changes = append(s.changes, CalendarChange{
		ID:         int64(len(s.changes) + 1),
		UserID:     userID,
		CalendarID: calendarID,
		ObjectName: objectName,
		ETag:       etag,
		Action:     action,
		SyncToken:  token,
		ChangedAt:  time.Now(),
	})
}

type fakeNotFoundError struct{}

func (fakeNotFoundError) Error() string { return "not found" }

var errFakeNotFound fakeNotFoundError

type fakeExistsError struct{}

func (fakeExistsError) Error() string { return "already exists" }

var errFakeExists fakeExistsError

// fakeSchedulingStore embeds fakeDiscoveryStore and also implements SchedulingStore.
type fakeSchedulingStore struct {
	fakeDiscoveryStore
	delivered []DeliverSchedulingMessageRequest
	sent      []SendSchedulingMessageRequest
}

func (s *fakeSchedulingStore) DeliverSchedulingMessage(_ context.Context, req DeliverSchedulingMessageRequest) (SchedulingMessage, error) {
	s.delivered = append(s.delivered, req)
	return SchedulingMessage{
		UserID:      req.UserID,
		Recipient:   req.Recipient,
		Method:      req.Method,
		UID:         req.UID,
		ICSPayload:  req.ICSPayload,
		ProcessedAt: time.Now(),
	}, nil
}

func (s *fakeSchedulingStore) SendSchedulingMessage(_ context.Context, req SendSchedulingMessageRequest) (SchedulingMessage, error) {
	s.sent = append(s.sent, req)
	return SchedulingMessage{
		UserID:      req.UserID,
		Method:      req.Method,
		UID:         req.UID,
		ICSPayload:  req.ICSPayload,
		ProcessedAt: time.Now(),
	}, nil
}

// minimalSchedulingICS is a valid VCALENDAR body for scheduling tests.
// The handler defaults to ScheduleMethodRequest when no METHOD property is present
// (ParseICalendarObject rejects METHOD on stored objects per RFC 4791).
const minimalSchedulingICS = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//gogomail//test//EN\r\nBEGIN:VEVENT\r\n" +
	"UID:sched-test-uid-1234\r\n" +
	"DTSTART:20260601T100000Z\r\nDTEND:20260601T110000Z\r\n" +
	"SUMMARY:Test Meeting\r\n" +
	"ORGANIZER:mailto:org@example.com\r\n" +
	"ATTENDEE:mailto:user-1@example.com\r\n" +
	"END:VEVENT\r\nEND:VCALENDAR\r\n"

func TestHandlerSchedulingPostInboxDelivers(t *testing.T) {
	t.Parallel()

	store := &fakeSchedulingStore{}
	handler := NewHandler(store, fixedUser("user-1"))
	handler.IncludeScheduling = true

	req := httptest.NewRequest(http.MethodPost, "/caldav/calendars/user-1/inbox/",
		strings.NewReader(minimalSchedulingICS))
	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(store.delivered) != 1 {
		t.Fatalf("delivered count = %d, want 1", len(store.delivered))
	}
	if store.delivered[0].UID != "sched-test-uid-1234" {
		t.Errorf("UID = %q, want sched-test-uid-1234", store.delivered[0].UID)
	}
}

func TestHandlerSchedulingPostInboxForbiddenWithoutFlag(t *testing.T) {
	t.Parallel()

	store := &fakeSchedulingStore{}
	handler := NewHandler(store, fixedUser("user-1"))
	// IncludeScheduling defaults to false

	req := httptest.NewRequest(http.MethodPost, "/caldav/calendars/user-1/inbox/",
		strings.NewReader(minimalSchedulingICS))
	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (scheduling not enabled)", rec.Code)
	}
}

func TestHandlerSchedulingPostOutboxSends(t *testing.T) {
	t.Parallel()

	store := &fakeSchedulingStore{}
	handler := NewHandler(store, fixedUser("user-1"))
	handler.IncludeScheduling = true

	req := httptest.NewRequest(http.MethodPost, "/caldav/calendars/user-1/outbox/",
		strings.NewReader(minimalSchedulingICS))
	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(store.sent) != 1 {
		t.Fatalf("sent count = %d, want 1", len(store.sent))
	}
}
