package carddavgw

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var errFakeCardDAVNotFound = errors.New("not found")

type fakeCardDAVDiscoveryStore struct {
	principal Principal
	books     []AddressBook
	objects   []ContactObject
	changes   []AddressBookChange
}

func (s fakeCardDAVDiscoveryStore) LookupPrincipal(_ context.Context, userID string) (Principal, error) {
	if userID != s.principal.UserID {
		return Principal{}, errFakeCardDAVNotFound
	}
	return s.principal, nil
}

func (s fakeCardDAVDiscoveryStore) ListAddressBookCollections(_ context.Context, userID string) ([]AddressBook, error) {
	if userID != s.principal.UserID {
		return nil, errFakeCardDAVNotFound
	}
	return s.books, nil
}

func (s fakeCardDAVDiscoveryStore) LookupAddressBook(_ context.Context, userID string, addressBookID string) (AddressBook, error) {
	for _, book := range s.books {
		if book.UserID == userID && book.ID == addressBookID {
			return book, nil
		}
	}
	return AddressBook{}, errFakeCardDAVNotFound
}

func (s fakeCardDAVDiscoveryStore) ListAddressBookObjects(_ context.Context, userID string, addressBookID string) ([]ContactObject, error) {
	if userID != s.principal.UserID {
		return nil, errFakeCardDAVNotFound
	}
	var objects []ContactObject
	for _, object := range s.objects {
		if object.AddressBookID == addressBookID {
			objects = append(objects, object)
		}
	}
	return objects, nil
}

func (s fakeCardDAVDiscoveryStore) LookupContactObject(_ context.Context, userID string, addressBookID string, objectName string) (ContactObject, error) {
	for _, object := range s.objects {
		if object.UserID == userID && object.AddressBookID == addressBookID && object.ObjectName == objectName {
			return object, nil
		}
	}
	return ContactObject{}, errFakeCardDAVNotFound
}

func (s fakeCardDAVDiscoveryStore) ListAddressBookChangesSince(_ context.Context, req ListAddressBookChangesSinceRequest) ([]AddressBookChange, error) {
	if req.UserID != s.principal.UserID {
		return nil, errFakeCardDAVNotFound
	}
	var markerFound bool
	var changes []AddressBookChange
	for _, change := range s.changes {
		if change.AddressBookID != req.AddressBookID {
			continue
		}
		if change.SyncToken == req.SyncToken {
			markerFound = true
			continue
		}
		if markerFound {
			changes = append(changes, change)
		}
	}
	if !markerFound {
		return nil, InvalidSyncTokenError{Token: req.SyncToken}
	}
	if req.Limit > 0 && len(changes) > req.Limit {
		changes = changes[:req.Limit]
	}
	return changes, nil
}

func TestHandlerOptionsAdvertisesCardDAVDiscovery(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	handler := NewHandler(testCardDAVDiscoveryStore(t), func(*http.Request) (string, error) { return "user-1", nil })
	handler.ServeHTTP(rec, httptest.NewRequest(MethodOptions, RootPath+"/", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	for _, want := range []string{MethodOptions, MethodPropfind, MethodReport} {
		if !strings.Contains(rec.Header().Get("Allow"), want) {
			t.Fatalf("Allow = %q, missing %q", rec.Header().Get("Allow"), want)
		}
	}
	for _, want := range []string{DAVClass1, DAVClass3, DAVAddressBook, DAVSyncCollection} {
		if !strings.Contains(rec.Header().Get("DAV"), want) {
			t.Fatalf("DAV = %q, missing %q", rec.Header().Get("DAV"), want)
		}
	}
}

func TestHandlerReportAddressBookMultigetReturnsAddressData(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-multiget xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <D:href>/carddav/addressbooks/user-1/personal/contact-1.vcf</D:href>
  <D:href>/carddav/addressbooks/user-1/personal/missing.vcf</D:href>
  <D:prop><D:getetag/><C:address-data/></D:prop>
</C:addressbook-multiget>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthZero, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	for _, want := range []string{
		"<D:href>/carddav/addressbooks/user-1/personal/contact-1.vcf</D:href>",
		"<C:address-data>BEGIN:VCARD",
		"FN:Contact One",
		"<D:href>/carddav/addressbooks/user-1/personal/missing.vcf</D:href>",
		"<D:status>HTTP/1.1 404 Not Found</D:status>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("multiget REPORT missing %q:\n%s", want, text)
		}
	}
}

func TestHandlerReportAddressBookQueryFiltersTextMatch(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:prop-filter name="FN"><C:text-match>Contact One</C:text-match></C:prop-filter></C:filter>
  <D:prop><D:getetag/><C:address-data/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthZero, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	if !strings.Contains(text, "<D:href>/carddav/addressbooks/user-1/personal/contact-1.vcf</D:href>") {
		t.Fatalf("query REPORT missing matching contact:\n%s", text)
	}
	if strings.Contains(text, "contact-2.vcf") {
		t.Fatalf("query REPORT included non-matching contact:\n%s", text)
	}
}

func TestHandlerReportSyncCollectionReturnsFullSnapshotAndToken(t *testing.T) {
	t.Parallel()

	body := `<D:sync-collection xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:sync-level>1</D:sync-level>
  <D:prop><D:getetag/><C:address-data/></D:prop>
</D:sync-collection>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthZero, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	for _, want := range []string{
		"<D:href>/carddav/addressbooks/user-1/personal/contact-1.vcf</D:href>",
		"<D:sync-token>sync-123</D:sync-token>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("sync REPORT missing %q:\n%s", want, text)
		}
	}
}

func TestHandlerReportSyncCollectionReturnsChangesSinceToken(t *testing.T) {
	t.Parallel()

	body := `<D:sync-collection xmlns:D="DAV:">
  <D:sync-token>sync-old</D:sync-token>
  <D:sync-level>1</D:sync-level>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthZero, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	for _, want := range []string{
		"<D:href>/carddav/addressbooks/user-1/personal/contact-1.vcf</D:href>",
		"<D:href>/carddav/addressbooks/user-1/personal/removed.vcf</D:href>",
		"<D:status>HTTP/1.1 404 Not Found</D:status>",
		"<D:sync-token>sync-123</D:sync-token>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("sync change REPORT missing %q:\n%s", want, text)
		}
	}
}

func TestHandlerReportSyncCollectionRejectsInvalidSyncTokenWithDAVPrecondition(t *testing.T) {
	t.Parallel()

	body := `<D:sync-collection xmlns:D="DAV:">
  <D:sync-token>missing-sync</D:sync-token>
  <D:sync-level>1</D:sync-level>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthZero, body)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	text := rec.Body.String()
	for _, want := range []string{"<D:error", "<D:valid-sync-token/>", "<D:responsedescription>CardDAV sync-token is no longer valid</D:responsedescription>"} {
		if !strings.Contains(text, want) {
			t.Fatalf("invalid sync response missing %q:\n%s", want, text)
		}
	}
}

func TestHandlerReportRejectsDepthInfinityAndCrossUserPath(t *testing.T) {
	t.Parallel()

	body := `<D:sync-collection xmlns:D="DAV:"><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop></D:sync-collection>`
	depth := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthInfinity, body)
	if depth.Code != http.StatusForbidden {
		t.Fatalf("Depth infinity status = %d, want %d", depth.Code, http.StatusForbidden)
	}
	crossUser := runCardDAVReport(t, "/carddav/addressbooks/other-user/personal/", DepthZero, body)
	if crossUser.Code != http.StatusForbidden {
		t.Fatalf("cross-user status = %d, want %d", crossUser.Code, http.StatusForbidden)
	}
}

func TestHandlerWellKnownRedirectsToRoot(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	handler := NewHandler(testCardDAVDiscoveryStore(t), nil)
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, WellKnownCardDAVPath+"?user_id=user-1", nil))

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMovedPermanently)
	}
	if got, want := rec.Header().Get("Location"), RootPath+"/?user_id=user-1"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestHandlerPropfindRootDiscoversPrincipal(t *testing.T) {
	t.Parallel()

	body := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav"><D:prop><D:current-user-principal/><C:addressbook-home-set/></D:prop></D:propfind>`
	rec := runCardDAVPropfind(t, RootPath+"/", DepthZero, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	for _, want := range []string{
		"<D:href>/carddav/</D:href>",
		"<D:current-user-principal><D:href>/carddav/principals/user-1/</D:href></D:current-user-principal>",
		"<C:addressbook-home-set><D:href>/carddav/addressbooks/user-1/</D:href></C:addressbook-home-set>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("PROPFIND root missing %q:\n%s", want, text)
		}
	}
}

func TestHandlerPropfindAddressBookHomeDepthOneListsCollections(t *testing.T) {
	t.Parallel()

	body := `<D:propfind xmlns:D="DAV:"><D:allprop/></D:propfind>`
	rec := runCardDAVPropfind(t, "/carddav/addressbooks/user-1/", DepthOne, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	for _, want := range []string{
		"<D:href>/carddav/addressbooks/user-1/</D:href>",
		"<D:href>/carddav/addressbooks/user-1/personal/</D:href>",
		"<C:addressbook></C:addressbook>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("home PROPFIND missing %q:\n%s", want, text)
		}
	}
}

func TestHandlerPropfindCollectionDepthOneListsObjects(t *testing.T) {
	t.Parallel()

	body := `<D:propfind xmlns:D="DAV:"><D:prop><D:getetag/><D:getcontenttype/></D:prop></D:propfind>`
	rec := runCardDAVPropfind(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	for _, want := range []string{
		"<D:href>/carddav/addressbooks/user-1/personal/</D:href>",
		"<D:href>/carddav/addressbooks/user-1/personal/contact-1.vcf</D:href>",
		"<D:getetag>&#34;0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef&#34;</D:getetag>",
		"<D:getcontenttype>text/vcard; charset=utf-8</D:getcontenttype>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("collection PROPFIND missing %q:\n%s", want, text)
		}
	}
}

func TestHandlerPropfindContactObjectRequiresDepthZero(t *testing.T) {
	t.Parallel()

	body := `<D:propfind xmlns:D="DAV:"><D:allprop/></D:propfind>`
	rec := runCardDAVPropfind(t, "/carddav/addressbooks/user-1/personal/contact-1.vcf", DepthOne, body)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandlerPropfindRejectsCrossUserPath(t *testing.T) {
	t.Parallel()

	body := `<D:propfind xmlns:D="DAV:"><D:allprop/></D:propfind>`
	rec := runCardDAVPropfind(t, "/carddav/addressbooks/other-user/", DepthZero, body)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestHandlerPropfindRejectsDepthInfinity(t *testing.T) {
	t.Parallel()

	body := `<D:propfind xmlns:D="DAV:"><D:allprop/></D:propfind>`
	rec := runCardDAVPropfind(t, RootPath+"/", DepthInfinity, body)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func runCardDAVPropfind(t *testing.T, path string, depth Depth, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(MethodPropfind, path, strings.NewReader(body))
	req.Header.Set("Depth", string(depth))
	rec := httptest.NewRecorder()
	handler := NewHandler(testCardDAVDiscoveryStore(t), func(*http.Request) (string, error) { return "user-1", nil })
	handler.ServeHTTP(rec, req)
	return rec
}

func runCardDAVReport(t *testing.T, path string, depth Depth, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(MethodReport, path, strings.NewReader(body))
	req.Header.Set("Depth", string(depth))
	rec := httptest.NewRecorder()
	handler := NewHandler(testCardDAVDiscoveryStore(t), func(*http.Request) (string, error) { return "user-1", nil })
	handler.ServeHTTP(rec, req)
	return rec
}

func testCardDAVDiscoveryStore(t *testing.T) fakeCardDAVDiscoveryStore {
	t.Helper()

	createdAt := time.Date(2026, 5, 6, 1, 2, 3, 0, time.UTC)
	updatedAt := time.Date(2026, 5, 6, 4, 5, 6, 0, time.UTC)
	return fakeCardDAVDiscoveryStore{
		principal: Principal{
			UserID:              "user-1",
			DisplayName:         "User One",
			PrincipalPath:       "/carddav/principals/user-1/",
			AddressBookHomePath: "/carddav/addressbooks/user-1/",
		},
		books: []AddressBook{{
			ID:        "personal",
			UserID:    "user-1",
			Name:      "Personal",
			SyncToken: "sync-123",
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		}},
		objects: []ContactObject{{
			UserID:        "user-1",
			AddressBookID: "personal",
			ObjectName:    "contact-1.vcf",
			VCard:         []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-1\r\nFN:Contact One\r\nEND:VCARD\r\n"),
			ETag:          `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`,
			Size:          64,
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
		}, {
			UserID:        "user-1",
			AddressBookID: "personal",
			ObjectName:    "contact-2.vcf",
			VCard:         []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-2\r\nFN:Other Person\r\nEND:VCARD\r\n"),
			ETag:          `"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"`,
			Size:          65,
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
		}},
		changes: []AddressBookChange{{
			ID:            1,
			UserID:        "user-1",
			AddressBookID: "personal",
			Action:        "addressbook-created",
			SyncToken:     "sync-old",
			ChangedAt:     createdAt,
		}, {
			ID:            2,
			UserID:        "user-1",
			AddressBookID: "personal",
			ObjectName:    "contact-1.vcf",
			Action:        "contact-upserted",
			SyncToken:     "sync-mid",
			ChangedAt:     updatedAt,
		}, {
			ID:            3,
			UserID:        "user-1",
			AddressBookID: "personal",
			ObjectName:    "removed.vcf",
			Action:        "contact-deleted",
			SyncToken:     "sync-123",
			ChangedAt:     updatedAt,
		}},
	}
}
