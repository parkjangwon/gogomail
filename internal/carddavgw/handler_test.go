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

func TestHandlerOptionsAdvertisesCardDAVDiscovery(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	handler := NewHandler(testCardDAVDiscoveryStore(t), func(*http.Request) (string, error) { return "user-1", nil })
	handler.ServeHTTP(rec, httptest.NewRequest(MethodOptions, RootPath+"/", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	for _, want := range []string{MethodOptions, MethodPropfind} {
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
			ETag:          `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`,
			Size:          64,
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
		}},
	}
}
