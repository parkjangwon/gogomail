package carddavgw

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var errFakeCardDAVNotFound = errors.New("not found")

type readTrackingReader struct {
	data  string
	reads int
}

func (r *readTrackingReader) Read(p []byte) (int, error) {
	r.reads++
	if r.data == "" {
		return 0, io.EOF
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

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

func (s fakeCardDAVDiscoveryStore) ListAddressBookObjectsLimit(_ context.Context, userID string, addressBookID string, limit int) ([]ContactObject, error) {
	objects, err := s.ListAddressBookObjects(context.Background(), userID, addressBookID)
	if err != nil {
		return nil, err
	}
	if limit >= 0 && len(objects) > limit {
		objects = objects[:limit]
	}
	return objects, nil
}

func (s fakeCardDAVDiscoveryStore) WalkAddressBookObjects(_ context.Context, userID string, addressBookID string, yield func(ContactObject) (bool, error)) error {
	objects, err := s.ListAddressBookObjects(context.Background(), userID, addressBookID)
	if err != nil {
		return err
	}
	for _, object := range objects {
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

func (s *fakeCardDAVDiscoveryStore) UpdateAddressBookProperties(_ context.Context, req UpdateAddressBookRequest) (AddressBook, error) {
	req, _, syncToken, err := ValidateUpdateAddressBookRequest(req)
	if err != nil {
		return AddressBook{}, err
	}
	for i, book := range s.books {
		if book.UserID == req.UserID && book.ID == req.AddressBookID {
			if req.Name != nil {
				book.Name = *req.Name
			}
			if req.Description != nil {
				book.Description = *req.Description
			}
			book.SyncToken = syncToken
			book.UpdatedAt = time.Date(2026, 5, 6, 8, 9, 10, 0, time.UTC)
			s.books[i] = book
			return book, nil
		}
	}
	return AddressBook{}, errFakeCardDAVNotFound
}

func (s *fakeCardDAVDiscoveryStore) CreateAddressBookAtPath(_ context.Context, req CreateAddressBookAtPathRequest) (AddressBook, error) {
	validated, _, syncToken, err := ValidateCreateAddressBookAtPathRequest(req)
	if err != nil {
		return AddressBook{}, err
	}
	for _, book := range s.books {
		if book.UserID == validated.UserID && book.ID == validated.AddressBookID {
			return AddressBook{}, errors.New("address book exists")
		}
	}
	now := time.Date(2026, 5, 6, 9, 10, 11, 0, time.UTC)
	book := AddressBook{
		ID:          validated.AddressBookID,
		UserID:      validated.UserID,
		Name:        validated.Name,
		Description: validated.Description,
		SyncToken:   syncToken,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.books = append(s.books, book)
	return book, nil
}

func (s *fakeCardDAVDiscoveryStore) DeleteAddressBook(_ context.Context, req DeleteAddressBookRequest) (AddressBook, error) {
	validated, err := ValidateDeleteAddressBookRequest(req)
	if err != nil {
		return AddressBook{}, err
	}
	for i, book := range s.books {
		if book.UserID == validated.UserID && book.ID == validated.AddressBookID {
			s.books = append(s.books[:i], s.books[i+1:]...)
			var objects []ContactObject
			for _, object := range s.objects {
				if object.UserID == validated.UserID && object.AddressBookID == validated.AddressBookID {
					continue
				}
				objects = append(objects, object)
			}
			s.objects = objects
			s.changes = append(s.changes, AddressBookChange{
				ID:            int64(len(s.changes) + 1),
				UserID:        validated.UserID,
				AddressBookID: validated.AddressBookID,
				Action:        "addressbook-deleted",
				SyncToken:     AddressBookSyncToken(validated.UserID, validated.AddressBookID, "delete"),
				ChangedAt:     time.Date(2026, 5, 6, 10, 11, 12, 0, time.UTC),
			})
			return book, nil
		}
	}
	return AddressBook{}, errFakeCardDAVNotFound
}

func (s fakeCardDAVDiscoveryStore) UpsertContactObject(_ context.Context, req UpsertContactObjectRequest) (ContactObject, error) {
	req, etag, _, err := ValidateUpsertContactObjectRequest(req)
	if err != nil {
		return ContactObject{}, err
	}
	for _, object := range s.objects {
		if object.UserID == req.UserID &&
			object.AddressBookID == req.AddressBookID &&
			object.ObjectName != req.ObjectName &&
			object.UID == req.UID {
			return ContactObject{}, errors.New("CardDAV contact object UID already exists")
		}
	}
	createdAt := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
	return ContactObject{
		UserID:        req.UserID,
		AddressBookID: req.AddressBookID,
		ObjectName:    req.ObjectName,
		UID:           req.UID,
		ETag:          etag,
		Size:          int64(len(req.VCard)),
		VCard:         req.VCard,
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	}, nil
}

func (s fakeCardDAVDiscoveryStore) DeleteContactObject(_ context.Context, req DeleteContactObjectRequest) (ContactObject, error) {
	req, _, err := ValidateDeleteContactObjectRequest(req)
	if err != nil {
		return ContactObject{}, err
	}
	for _, object := range s.objects {
		if object.UserID == req.UserID && object.AddressBookID == req.AddressBookID && object.ObjectName == req.ObjectName {
			if req.ObservedETag != "" && req.ObservedETag != object.ETag {
				return ContactObject{}, errors.New("CardDAV contact object etag mismatch")
			}
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
	for _, want := range []string{MethodOptions, MethodPropfind, MethodProppatch, MethodReport, MethodGet, MethodHead, MethodPut, MethodDelete, MethodMkcol} {
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

func TestHandlerOptionsAdvertisesOnlyImplementedMethods(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	handler := NewHandler(testCardDAVDiscoveryStore(t), func(*http.Request) (string, error) { return "user-1", nil })
	handler.ServeHTTP(rec, httptest.NewRequest(MethodOptions, RootPath+"/", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	want := strings.Join(ImplementedMethods(), ", ")
	if got := rec.Header().Get("Allow"); got != want {
		t.Fatalf("Allow = %q, want %q", got, want)
	}
}

func TestHandlerUnsupportedMethodReturnsImplementedAllow(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	handler := NewHandler(testCardDAVDiscoveryStore(t), func(*http.Request) (string, error) { return "user-1", nil })
	handler.ServeHTTP(rec, httptest.NewRequest("COPY", "/carddav/addressbooks/user-1/personal/contact-1.vcf", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	want := strings.Join(ImplementedMethods(), ", ")
	if got := rec.Header().Get("Allow"); got != want {
		t.Fatalf("Allow = %q, want %q", got, want)
	}
}

func TestHandlerGetAndHeadContactObject(t *testing.T) {
	t.Parallel()

	get := runCardDAVObjectRequest(t, MethodGet, "/carddav/addressbooks/user-1/personal/contact-1.vcf", "", nil)
	if get.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body = %s", get.Code, get.Body.String())
	}
	if got := get.Header().Get("Content-Type"); got != "text/vcard; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if !strings.Contains(get.Body.String(), "FN:Contact One") {
		t.Fatalf("GET body missing vCard:\n%s", get.Body.String())
	}

	head := runCardDAVObjectRequest(t, MethodHead, "/carddav/addressbooks/user-1/personal/contact-1.vcf", "", nil)
	if head.Code != http.StatusOK {
		t.Fatalf("HEAD status = %d", head.Code)
	}
	if head.Body.Len() != 0 {
		t.Fatalf("HEAD body length = %d, want 0", head.Body.Len())
	}
}

func TestHandlerGetContactObjectAllowsDelegatedRead(t *testing.T) {
	t.Parallel()

	handler := NewHandler(testCardDAVDiscoveryStore(t), func(*http.Request) (string, error) { return "delegate-1", nil })
	handler.AccessAuthorizer = &fakeCardDAVAccessAuthorizer{allowedRoles: map[string]bool{ContactsAccessRoleRead: true}}
	req := httptest.NewRequest(MethodGet, "/carddav/addressbooks/user-1/personal/contact-1.vcf", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := handler.AccessAuthorizer.(*fakeCardDAVAccessAuthorizer).last; got.ActorUserID != "delegate-1" || got.OwnerUserID != "user-1" || got.RequiredRole != ContactsAccessRoleRead {
		t.Fatalf("access request = %+v", got)
	}
	if !strings.Contains(rec.Body.String(), "FN:Contact One") {
		t.Fatalf("delegated GET missing owner vCard:\n%s", rec.Body.String())
	}
}

func TestHandlerGetContactObjectHonorsCachePreconditions(t *testing.T) {
	t.Parallel()

	headers := http.Header{"If-None-Match": []string{
		`"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`,
		`"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`,
	}}
	rec := runCardDAVObjectRequest(t, MethodGet, "/carddav/addressbooks/user-1/personal/contact-1.vcf", "", headers)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotModified)
	}

	stale := http.Header{"If-Match": []string{`"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"`}}
	rec = runCardDAVObjectRequest(t, MethodGet, "/carddav/addressbooks/user-1/personal/contact-1.vcf", "", stale)
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale status = %d, want %d", rec.Code, http.StatusPreconditionFailed)
	}
}

func TestHandlerPutContactObjectCreatesAndUpdatesWithPreconditions(t *testing.T) {
	t.Parallel()

	body := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:new-contact\r\nFN:New Contact\r\nEND:VCARD\r\n"
	headers := http.Header{"Content-Type": []string{"text/vcard; charset=utf-8"}}
	create := runCardDAVObjectRequest(t, MethodPut, "/carddav/addressbooks/user-1/personal/new-contact.vcf", body, headers)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", create.Code, create.Body.String())
	}
	if create.Header().Get("ETag") == "" {
		t.Fatal("create response missing ETag")
	}

	updateHeaders := http.Header{
		"Content-Type": []string{"text/vcard"},
		"If-Match": []string{
			`"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`,
			`"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`,
		},
	}
	update := runCardDAVObjectRequest(t, MethodPut, "/carddav/addressbooks/user-1/personal/contact-1.vcf", strings.ReplaceAll(body, "new-contact", "contact-1"), updateHeaders)
	if update.Code != http.StatusNoContent {
		t.Fatalf("update status = %d, body = %s", update.Code, update.Body.String())
	}

	ifNoneMatch := http.Header{"If-None-Match": []string{"*"}, "Content-Type": []string{"text/vcard"}}
	conflict := runCardDAVObjectRequest(t, MethodPut, "/carddav/addressbooks/user-1/personal/contact-1.vcf", body, ifNoneMatch)
	if conflict.Code != http.StatusPreconditionFailed {
		t.Fatalf("If-None-Match status = %d, want %d", conflict.Code, http.StatusPreconditionFailed)
	}
}

func TestHandlerPutContactObjectRejectsDelegatedReadOnlyAccess(t *testing.T) {
	t.Parallel()

	handler := NewHandler(testCardDAVDiscoveryStore(t), func(*http.Request) (string, error) { return "delegate-1", nil })
	handler.AccessAuthorizer = &fakeCardDAVAccessAuthorizer{allowedRoles: map[string]bool{ContactsAccessRoleRead: true}}
	body := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:new-contact\r\nFN:New Contact\r\nEND:VCARD\r\n"
	req := httptest.NewRequest(MethodPut, "/carddav/addressbooks/user-1/personal/new-contact.vcf", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/vcard")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := handler.AccessAuthorizer.(*fakeCardDAVAccessAuthorizer).last; got.ActorUserID != "delegate-1" || got.OwnerUserID != "user-1" || got.RequiredRole != ContactsAccessRoleWrite {
		t.Fatalf("access request = %+v", got)
	}
}

func TestHandlerPutContactObjectAllowsDelegatedWrite(t *testing.T) {
	t.Parallel()

	handler := NewHandler(testCardDAVDiscoveryStore(t), func(*http.Request) (string, error) { return "delegate-1", nil })
	handler.AccessAuthorizer = &fakeCardDAVAccessAuthorizer{allowedRoles: map[string]bool{ContactsAccessRoleWrite: true}}
	body := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:new-contact\r\nFN:New Contact\r\nEND:VCARD\r\n"
	req := httptest.NewRequest(MethodPut, "/carddav/addressbooks/user-1/personal/new-contact.vcf", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/vcard")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := handler.AccessAuthorizer.(*fakeCardDAVAccessAuthorizer).last; got.ActorUserID != "delegate-1" || got.OwnerUserID != "user-1" || got.RequiredRole != ContactsAccessRoleWrite {
		t.Fatalf("access request = %+v", got)
	}
}

func TestHandlerPutContactObjectValidatesContentTypeVersion(t *testing.T) {
	t.Parallel()

	v3 := "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:v3-contact\r\nFN:Version Three\r\nEND:VCARD\r\n"
	create := runCardDAVObjectRequest(t, MethodPut, "/carddav/addressbooks/user-1/personal/v3-contact.vcf", v3, http.Header{
		"Content-Type": []string{"text/vcard; version=3.0; charset=utf-8"},
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("vCard 3 create status = %d, body = %s", create.Code, create.Body.String())
	}

	badVersion := runCardDAVObjectRequest(t, MethodPut, "/carddav/addressbooks/user-1/personal/bad-version.vcf", v3, http.Header{
		"Content-Type": []string{"text/vcard; version=2.1"},
	})
	if badVersion.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("bad content-type version status = %d, want %d", badVersion.Code, http.StatusUnsupportedMediaType)
	}

	mismatch := runCardDAVObjectRequest(t, MethodPut, "/carddav/addressbooks/user-1/personal/mismatch-version.vcf", v3, http.Header{
		"Content-Type": []string{"text/vcard; version=4.0"},
	})
	if mismatch.Code != http.StatusBadRequest {
		t.Fatalf("mismatched content-type version status = %d, want %d", mismatch.Code, http.StatusBadRequest)
	}
	if !strings.Contains(mismatch.Body.String(), "VERSION") {
		t.Fatalf("mismatch response missing VERSION context: %s", mismatch.Body.String())
	}
}

func TestHandlerPutContactObjectRejectsRepeatedContentType(t *testing.T) {
	t.Parallel()

	body := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-1\r\nFN:Contact One\r\nEND:VCARD\r\n"
	rec := runCardDAVObjectRequest(t, MethodPut, "/carddav/addressbooks/user-1/personal/contact-1.vcf", body, http.Header{
		"Content-Type": []string{"text/vcard", "text/vcard; version=4.0"},
	})
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusUnsupportedMediaType, rec.Body.String())
	}
}

func TestHandlerPutContactObjectRejectsDuplicateUID(t *testing.T) {
	t.Parallel()

	body := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-1\r\nFN:Duplicate Contact\r\nEND:VCARD\r\n"
	rec := runCardDAVObjectRequest(t, MethodPut, "/carddav/addressbooks/user-1/personal/duplicate.vcf", body, http.Header{
		"Content-Type": []string{"text/vcard"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "UID") {
		t.Fatalf("duplicate UID response missing UID context: %s", rec.Body.String())
	}
}

func TestHandlerPutContactObjectRejectsInvalidContentTypeAndOversize(t *testing.T) {
	t.Parallel()

	body := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:new-contact\r\nFN:New Contact\r\nEND:VCARD\r\n"
	badType := runCardDAVObjectRequest(t, MethodPut, "/carddav/addressbooks/user-1/personal/new-contact.vcf", body, http.Header{"Content-Type": []string{"text/plain"}})
	if badType.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("content-type status = %d, want %d", badType.Code, http.StatusUnsupportedMediaType)
	}

	oversize := runCardDAVObjectRequest(t, MethodPut, "/carddav/addressbooks/user-1/personal/new-contact.vcf", strings.Repeat("x", MaxContactObjectBytes+1), nil)
	if oversize.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize status = %d, want %d", oversize.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestHandlerDeleteContactObjectHonorsIfMatch(t *testing.T) {
	t.Parallel()

	stale := runCardDAVObjectRequest(t, MethodDelete, "/carddav/addressbooks/user-1/personal/contact-1.vcf", "", http.Header{
		"If-Match": []string{`"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"`},
	})
	if stale.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale status = %d, want %d", stale.Code, http.StatusPreconditionFailed)
	}

	ok := runCardDAVObjectRequest(t, MethodDelete, "/carddav/addressbooks/user-1/personal/contact-1.vcf", "", http.Header{
		"If-Match": []string{`"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`},
	})
	if ok.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", ok.Code, ok.Body.String())
	}
}

func TestHandlerDeleteAddressBookCollectionDeletesObjects(t *testing.T) {
	t.Parallel()

	store := testCardDAVDiscoveryStore(t)
	handler := NewHandler(&store, func(*http.Request) (string, error) { return "user-1", nil })
	req := httptest.NewRequest(MethodDelete, "/carddav/addressbooks/user-1/personal/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if len(store.books) != 0 {
		t.Fatalf("address books after delete = %+v", store.books)
	}
	if len(store.objects) != 0 {
		t.Fatalf("objects after address book delete = %+v", store.objects)
	}
}

func TestHandlerDeleteAddressBookCollectionAllowsDelegatedManage(t *testing.T) {
	t.Parallel()

	store := testCardDAVDiscoveryStore(t)
	handler := NewHandler(&store, func(*http.Request) (string, error) { return "delegate-1", nil })
	handler.AccessAuthorizer = &fakeCardDAVAccessAuthorizer{allowedRoles: map[string]bool{ContactsAccessRoleManage: true}}
	req := httptest.NewRequest(MethodDelete, "/carddav/addressbooks/user-1/personal/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := handler.AccessAuthorizer.(*fakeCardDAVAccessAuthorizer).last; got.ActorUserID != "delegate-1" || got.OwnerUserID != "user-1" || got.RequiredRole != ContactsAccessRoleManage {
		t.Fatalf("access request = %+v", got)
	}
	if len(store.books) != 0 {
		t.Fatalf("address books after delete = %+v", store.books)
	}
}

func TestHandlerDeleteAddressBookCollectionRejectsMismatchedIfMatch(t *testing.T) {
	t.Parallel()

	store := testCardDAVDiscoveryStore(t)
	handler := NewHandler(&store, func(*http.Request) (string, error) { return "user-1", nil })
	req := httptest.NewRequest(MethodDelete, "/carddav/addressbooks/user-1/personal/", nil)
	req.Header.Set("If-Match", `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if len(store.books) != 1 {
		t.Fatalf("address books after rejected delete = %+v", store.books)
	}
}

func TestHandlerDeleteAddressBookCollectionAcceptsMatchingIfMatch(t *testing.T) {
	t.Parallel()

	store := testCardDAVDiscoveryStore(t)
	etag, err := AddressBookCollectionETag("user-1", store.books[0])
	if err != nil {
		t.Fatalf("AddressBookCollectionETag returned error: %v", err)
	}
	handler := NewHandler(&store, func(*http.Request) (string, error) { return "user-1", nil })
	req := httptest.NewRequest(MethodDelete, "/carddav/addressbooks/user-1/personal/", nil)
	req.Header.Set("If-Match", etag)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body = %s", rec.Code, rec.Body.String())
	}
	if len(store.books) != 0 {
		t.Fatalf("address books after delete = %+v", store.books)
	}
}

func TestHandlerDeleteAddressBookCollectionRejectsHomeTarget(t *testing.T) {
	t.Parallel()

	store := testCardDAVDiscoveryStore(t)
	handler := NewHandler(&store, func(*http.Request) (string, error) { return "user-1", nil })
	req := httptest.NewRequest(MethodDelete, "/carddav/addressbooks/user-1/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body = %s", rec.Code, rec.Body.String())
	}
	if len(store.books) != 1 {
		t.Fatalf("address books after rejected delete = %+v", store.books)
	}
}

func TestHandlerProppatchUpdatesAddressBookCollectionProperties(t *testing.T) {
	t.Parallel()

	store := testCardDAVDiscoveryStore(t)
	handler := NewHandler(&store, func(*http.Request) (string, error) { return "user-1", nil })
	req := httptest.NewRequest(MethodProppatch, "/carddav/addressbooks/user-1/personal/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:set>
    <D:prop>
      <D:displayname>Team Contacts</D:displayname>
      <C:addressbook-description>People for launch work</C:addressbook-description>
    </D:prop>
  </D:set>
</D:propertyupdate>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	book, err := store.LookupAddressBook(t.Context(), "user-1", "personal")
	if err != nil {
		t.Fatalf("address book lookup failed: %v", err)
	}
	if book.Name != "Team Contacts" || book.Description != "People for launch work" {
		t.Fatalf("address book = %+v", book)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<D:href>/carddav/addressbooks/user-1/personal/</D:href>",
		"<D:displayname>Team Contacts</D:displayname>",
		"<C:addressbook-description>People for launch work</C:addressbook-description>",
		"HTTP/1.1 200 OK",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("PROPPATCH response missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerProppatchRemovesAddressBookDescription(t *testing.T) {
	t.Parallel()

	store := testCardDAVDiscoveryStore(t)
	store.books[0].Description = "People"
	handler := NewHandler(&store, func(*http.Request) (string, error) { return "user-1", nil })
	req := httptest.NewRequest(MethodProppatch, "/carddav/addressbooks/user-1/personal/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:remove><D:prop><C:addressbook-description/></D:prop></D:remove>
</D:propertyupdate>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	book, err := store.LookupAddressBook(t.Context(), "user-1", "personal")
	if err != nil {
		t.Fatalf("address book lookup failed: %v", err)
	}
	if book.Description != "" {
		t.Fatalf("address book description = %q, want empty", book.Description)
	}
}

func TestHandlerProppatchHonorsIfUnmodifiedSinceBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := testCardDAVDiscoveryStore(t)
	store.books[0].UpdatedAt = time.Date(2026, 5, 6, 4, 5, 6, 0, time.UTC)
	body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:"><D:set><D:prop><D:displayname>Team</D:displayname></D:prop></D:set></D:propertyupdate>`}
	handler := NewHandler(&store, func(*http.Request) (string, error) { return "user-1", nil })
	req := httptest.NewRequest(MethodProppatch, "/carddav/addressbooks/user-1/personal/", body)
	req.Header.Set("If-Unmodified-Since", "Wed, 06 May 2026 04:05:05 GMT")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	book, err := store.LookupAddressBook(t.Context(), "user-1", "personal")
	if err != nil {
		t.Fatalf("address book lookup failed: %v", err)
	}
	if book.Name != "Personal" {
		t.Fatalf("address book name = %q, want Personal", book.Name)
	}
}

func TestHandlerProppatchAcceptsMatchingCollectionIfMatch(t *testing.T) {
	t.Parallel()

	store := testCardDAVDiscoveryStore(t)
	etag, err := AddressBookCollectionETag("user-1", store.books[0])
	if err != nil {
		t.Fatalf("AddressBookCollectionETag returned error: %v", err)
	}
	handler := NewHandler(&store, func(*http.Request) (string, error) { return "user-1", nil })
	req := httptest.NewRequest(MethodProppatch, "/carddav/addressbooks/user-1/personal/", strings.NewReader(`<D:propertyupdate xmlns:D="DAV:"><D:set><D:prop><D:displayname>Team</D:displayname></D:prop></D:set></D:propertyupdate>`))
	req.Header.Set("If-Match", `"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", `+etag)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	book, err := store.LookupAddressBook(t.Context(), "user-1", "personal")
	if err != nil {
		t.Fatalf("address book lookup failed: %v", err)
	}
	if book.Name != "Team" {
		t.Fatalf("address book name = %q, want Team", book.Name)
	}
}

func TestHandlerProppatchRejectsMismatchedIfMatchBeforeBodyRead(t *testing.T) {
	t.Parallel()

	store := testCardDAVDiscoveryStore(t)
	body := &readTrackingReader{data: `<D:propertyupdate xmlns:D="DAV:"><D:set><D:prop><D:displayname>Team</D:displayname></D:prop></D:set></D:propertyupdate>`}
	handler := NewHandler(&store, func(*http.Request) (string, error) { return "user-1", nil })
	req := httptest.NewRequest(MethodProppatch, "/carddav/addressbooks/user-1/personal/", body)
	req.Header.Set("If-Match", `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
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
		{name: "cross user", userID: "user-2", target: "/carddav/addressbooks/user-1/personal/", body: `<D:propertyupdate xmlns:D="DAV:"><D:set><D:prop><D:displayname>Personal</D:displayname></D:prop></D:set></D:propertyupdate>`, want: http.StatusForbidden},
		{name: "object target", userID: "user-1", target: "/carddav/addressbooks/user-1/personal/contact-1.vcf", body: `<D:propertyupdate xmlns:D="DAV:"><D:set><D:prop><D:displayname>Personal</D:displayname></D:prop></D:set></D:propertyupdate>`, want: http.StatusForbidden},
		{name: "home target", userID: "user-1", target: "/carddav/addressbooks/user-1/", body: `<D:propertyupdate xmlns:D="DAV:"><D:set><D:prop><D:displayname>Personal</D:displayname></D:prop></D:set></D:propertyupdate>`, want: http.StatusForbidden},
		{name: "invalid body", userID: "user-1", target: "/carddav/addressbooks/user-1/personal/", body: `<D:propertyupdate xmlns:D="DAV:"><D:remove><D:prop><D:displayname/></D:prop></D:remove></D:propertyupdate>`, want: http.StatusBadRequest},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := testCardDAVDiscoveryStore(t)
			handler := NewHandler(&store, func(*http.Request) (string, error) { return tc.userID, nil })
			req := httptest.NewRequest(MethodProppatch, tc.target, strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestHandlerMkcolCreatesAddressBookAtRequestURI(t *testing.T) {
	t.Parallel()

	store := testCardDAVDiscoveryStore(t)
	handler := NewHandler(&store, func(*http.Request) (string, error) { return "user-1", nil })
	bookID := "11111111-1111-4111-8111-111111111111"
	req := httptest.NewRequest(MethodMkcol, "/carddav/addressbooks/user-1/"+bookID+"/", strings.NewReader(`<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:set>
    <D:prop>
      <D:resourcetype><D:collection/><C:addressbook/></D:resourcetype>
      <D:displayname>Team Contacts</D:displayname>
      <C:addressbook-description>People for launch work</C:addressbook-description>
    </D:prop>
  </D:set>
</D:mkcol>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/carddav/addressbooks/user-1/"+bookID+"/" {
		t.Fatalf("Location = %q", got)
	}
	book, err := store.LookupAddressBook(t.Context(), "user-1", bookID)
	if err != nil {
		t.Fatalf("created address book lookup failed: %v", err)
	}
	if book.Name != "Team Contacts" || book.Description != "People for launch work" {
		t.Fatalf("address book = %+v", book)
	}
}

func TestHandlerMkcolRejectsExistingAddressBook(t *testing.T) {
	t.Parallel()

	store := testCardDAVDiscoveryStore(t)
	handler := NewHandler(&store, func(*http.Request) (string, error) { return "user-1", nil })
	req := httptest.NewRequest(MethodMkcol, "/carddav/addressbooks/user-1/personal/", strings.NewReader(`<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav"/>`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerMkcolRejectsUnsafePathIDBeforeBodyRead(t *testing.T) {
	t.Parallel()

	body := &readTrackingReader{data: `<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav"/>`}
	store := testCardDAVDiscoveryStore(t)
	handler := NewHandler(&store, func(*http.Request) (string, error) { return "user-1", nil })
	req := httptest.NewRequest(MethodMkcol, "/carddav/addressbooks/user-1/not-a-uuid/", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
}

func TestHandlerReportAddressBookMultigetReturnsAddressData(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-multiget xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <D:href>/carddav/addressbooks/user-1/personal/contact-1.vcf</D:href>
  <D:href>/carddav/addressbooks/user-1/personal/missing.vcf</D:href>
  <D:prop><D:getetag/><C:address-data/></D:prop>
</C:addressbook-multiget>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	for _, want := range []string{
		"<D:href>/carddav/addressbooks/user-1/personal/contact-1.vcf</D:href>",
		"<C:address-data content-type=\"text/vcard\" version=\"4.0\">BEGIN:VCARD",
		"FN:Contact One",
		"<D:href>/carddav/addressbooks/user-1/personal/missing.vcf</D:href>",
		"<D:status>HTTP/1.1 404 Not Found</D:status>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("multiget REPORT missing %q:\n%s", want, text)
		}
	}
}

func TestHandlerReportAddressBookMultigetAllowsDelegatedReadOnlyPrivileges(t *testing.T) {
	t.Parallel()

	handler := NewHandler(testCardDAVDiscoveryStore(t), func(*http.Request) (string, error) { return "delegate-1", nil })
	handler.AccessAuthorizer = &fakeCardDAVAccessAuthorizer{
		allowedRoles: map[string]bool{ContactsAccessRoleRead: true},
		privileges:   []XMLName{PrivilegeRead},
	}
	body := `<C:addressbook-multiget xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <D:href>/carddav/addressbooks/user-1/personal/contact-1.vcf</D:href>
  <D:prop><D:getetag/><D:current-user-privilege-set/></D:prop>
</C:addressbook-multiget>`
	req := httptest.NewRequest(MethodReport, "/carddav/addressbooks/user-1/personal/", strings.NewReader(body))
	req.Header.Set("Depth", string(DepthOne))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	if !strings.Contains(text, "<D:current-user-privilege-set><D:privilege><D:read></D:read></D:privilege></D:current-user-privilege-set>") {
		t.Fatalf("delegated multiget missing read-only privileges:\n%s", text)
	}
	for _, denied := range []string{"<D:bind>", "<D:unbind>", "<D:write-properties>", "<D:write-content>"} {
		if strings.Contains(text, denied) {
			t.Fatalf("delegated multiget advertised %s:\n%s", denied, text)
		}
	}
}

func TestHandlerReportAddressBookMultigetProjectsAddressData(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-multiget xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <D:href>/carddav/addressbooks/user-1/personal/contact-1.vcf</D:href>
  <D:prop><D:getetag/><C:address-data><C:prop name="FN"/></C:address-data></D:prop>
</C:addressbook-multiget>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	if !strings.Contains(text, "FN:Contact One") {
		t.Fatalf("projected address-data missing requested FN:\n%s", text)
	}
	if strings.Contains(text, "EMAIL;TYPE=home") {
		t.Fatalf("projected address-data included unrequested EMAIL:\n%s", text)
	}
}

func TestHandlerReportRejectsUnsupportedAddressDataWithPrecondition(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:prop-filter name="FN"/></C:filter>
  <D:prop><C:address-data content-type="application/vcard"/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	text := rec.Body.String()
	for _, want := range []string{"<D:error", "<C:supported-address-data/>", "application/vcard"} {
		if !strings.Contains(text, want) {
			t.Fatalf("unsupported address-data response missing %q:\n%s", want, text)
		}
	}
}

func TestHandlerReportRejectsUnsupportedCollationWithPrecondition(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:prop-filter name="FN"><C:text-match collation="i;octet">Contact</C:text-match></C:prop-filter></C:filter>
  <D:prop><D:getetag/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	text := rec.Body.String()
	for _, want := range []string{"<D:error", "<C:supported-collation/>", "i;octet"} {
		if !strings.Contains(text, want) {
			t.Fatalf("unsupported collation response missing %q:\n%s", want, text)
		}
	}
}

func TestHandlerReportAddressBookQuerySupportsASCIICasemapCollation(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:prop-filter name="FN"><C:text-match collation="i;ascii-casemap" match-type="equals">contact one</C:text-match></C:prop-filter></C:filter>
  <D:prop><D:getetag/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}
	text := rec.Body.String()
	if !strings.Contains(text, "<D:href>/carddav/addressbooks/user-1/personal/contact-1.vcf</D:href>") {
		t.Fatalf("ASCII casemap query did not match contact one:\n%s", text)
	}
	if strings.Contains(text, "contact-2.vcf") {
		t.Fatalf("ASCII casemap query matched the wrong contact:\n%s", text)
	}
}

func TestHandlerReportRejectsUnsupportedFilterElementWithPrecondition(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:unknown-filter name="FN"/></C:filter>
  <D:prop><D:getetag/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	text := rec.Body.String()
	for _, want := range []string{"<D:error", "<C:supported-filter/>", "unknown-filter"} {
		if !strings.Contains(text, want) {
			t.Fatalf("unsupported filter element response missing %q:\n%s", want, text)
		}
	}
}

func TestHandlerReportAddressBookMultigetRequiresDepthHeader(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-multiget xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <D:href>/carddav/addressbooks/user-1/personal/contact-1.vcf</D:href>
  <D:prop><D:getetag/></D:prop>
</C:addressbook-multiget>`
	rec := runCardDAVReportWithoutDepth(t, "/carddav/addressbooks/user-1/personal/", body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Depth header") {
		t.Fatalf("missing-depth response lacks context: %s", rec.Body.String())
	}
}

func TestHandlerReportAddressBookQueryFiltersTextMatch(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:prop-filter name="FN"><C:text-match>Contact One</C:text-match></C:prop-filter></C:filter>
  <D:prop><D:getetag/><C:address-data/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)

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

func TestHandlerReportAddressBookQueryFiltersSpecificVCardProperty(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:prop-filter name="EMAIL"><C:text-match>other@example.com</C:text-match></C:prop-filter></C:filter>
  <D:prop><D:getetag/><C:address-data/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	if !strings.Contains(text, "<D:href>/carddav/addressbooks/user-1/personal/contact-2.vcf</D:href>") {
		t.Fatalf("query REPORT missing EMAIL match:\n%s", text)
	}
	if strings.Contains(text, "contact-1.vcf") {
		t.Fatalf("query REPORT matched the wrong vCard property:\n%s", text)
	}
}

func TestHandlerReportAddressBookQueryHonorsTextMatchAttributes(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:prop-filter name="EMAIL"><C:text-match match-type="equals" negate-condition="yes">contact-one@example.com</C:text-match></C:prop-filter></C:filter>
  <D:prop><D:getetag/><C:address-data/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	if !strings.Contains(text, "<D:href>/carddav/addressbooks/user-1/personal/contact-2.vcf</D:href>") {
		t.Fatalf("query REPORT missing negated EMAIL match:\n%s", text)
	}
	if strings.Contains(text, "contact-1.vcf") {
		t.Fatalf("query REPORT ignored negate-condition or equals match-type:\n%s", text)
	}
}

func TestHandlerReportAddressBookQueryHonorsParamFilter(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:prop-filter name="EMAIL"><C:param-filter name="TYPE"><C:text-match match-type="equals">work</C:text-match></C:param-filter></C:prop-filter></C:filter>
  <D:prop><D:getetag/><C:address-data/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	if !strings.Contains(text, "<D:href>/carddav/addressbooks/user-1/personal/contact-2.vcf</D:href>") {
		t.Fatalf("query REPORT missing param-filter match:\n%s", text)
	}
	if strings.Contains(text, "contact-1.vcf") {
		t.Fatalf("query REPORT ignored param-filter:\n%s", text)
	}
}

func TestHandlerReportAddressBookQueryRejectsUnsupportedFilter(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:prop-filter name="X-GOGOMAIL-PRIVATE"><C:text-match>secret</C:text-match></C:prop-filter></C:filter>
  <D:prop><D:getetag/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	text := rec.Body.String()
	for _, want := range []string{"<D:error", "<C:supported-filter/>", "X-GOGOMAIL-PRIVATE"} {
		if !strings.Contains(text, want) {
			t.Fatalf("unsupported filter response missing %q:\n%s", want, text)
		}
	}
}

func TestHandlerReportAddressBookQueryRejectsUnsupportedFilterAtDepthZero(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:prop-filter name="X-GOGOMAIL-PRIVATE"/></C:filter>
  <D:prop><D:getetag/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthZero, body)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "<C:supported-filter/>") {
		t.Fatalf("Depth: 0 unsupported filter response missing precondition:\n%s", rec.Body.String())
	}
}

func TestHandlerReportAddressBookQueryRejectsUnsupportedParamFilter(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:prop-filter name="EMAIL"><C:param-filter name="X-SCOPE"/></C:prop-filter></C:filter>
  <D:prop><D:getetag/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "<C:supported-filter/>") {
		t.Fatalf("unsupported param response missing supported-filter precondition:\n%s", rec.Body.String())
	}
}

func TestHandlerReportAddressBookQueryHonorsFilterComposition(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter test="allof">
    <C:prop-filter name="FN"><C:text-match>Other</C:text-match></C:prop-filter>
    <C:prop-filter name="EMAIL" test="allof">
      <C:text-match match-type="ends-with">example.com</C:text-match>
      <C:param-filter name="TYPE"><C:text-match match-type="equals">work</C:text-match></C:param-filter>
    </C:prop-filter>
  </C:filter>
  <D:prop><D:getetag/><C:address-data/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	if !strings.Contains(text, "<D:href>/carddav/addressbooks/user-1/personal/contact-2.vcf</D:href>") {
		t.Fatalf("query REPORT missing composed filter match:\n%s", text)
	}
	if strings.Contains(text, "contact-1.vcf") {
		t.Fatalf("query REPORT ignored allof composition:\n%s", text)
	}
}

func TestHandlerReportAddressBookQueryHonorsLimit(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:prop-filter name="FN"/></C:filter>
  <D:limit><D:nresults>1</D:nresults></D:limit>
  <D:prop><D:getetag/><C:address-data/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if count := strings.Count(rec.Body.String(), "<D:response>"); count != 1 {
		t.Fatalf("response count = %d, body = %s", count, rec.Body.String())
	}
}

func TestHandlerReportAddressBookQueryRequiresDepthHeader(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:prop-filter name="FN"/></C:filter>
  <D:prop><D:getetag/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReportWithoutDepth(t, "/carddav/addressbooks/user-1/personal/", body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Depth header") {
		t.Fatalf("missing-depth response lacks context: %s", rec.Body.String())
	}
}

func TestHandlerReportRejectsRepeatedDepthBeforeBodyRead(t *testing.T) {
	t.Parallel()

	body := &readTrackingReader{data: `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:"><D:prop><D:getetag/></D:prop></C:addressbook-query>`}
	req := httptest.NewRequest(MethodReport, "/carddav/addressbooks/user-1/personal/", body)
	req.Header.Add("Depth", string(DepthZero))
	req.Header.Add("Depth", string(DepthOne))
	rec := httptest.NewRecorder()
	handler := NewHandler(testCardDAVDiscoveryStore(t), func(*http.Request) (string, error) { return "user-1", nil })
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

func TestHandlerReportAddressBookQueryDepthZeroReturnsCollectionScope(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:prop-filter name="FN"/></C:filter>
  <D:prop><D:getetag/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthZero, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "<D:response>") {
		t.Fatalf("Depth: 0 addressbook-query should not return child objects:\n%s", rec.Body.String())
	}
}

func TestHandlerReportAddressBookQueryAcceptsDepthInfinity(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:prop-filter name="FN"><C:text-match>Contact One</C:text-match></C:prop-filter></C:filter>
  <D:prop><D:getetag/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthInfinity, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	if !strings.Contains(text, "<D:href>/carddav/addressbooks/user-1/personal/contact-1.vcf</D:href>") {
		t.Fatalf("Depth: infinity query REPORT missing matching contact:\n%s", text)
	}
	if strings.Contains(text, "contact-2.vcf") {
		t.Fatalf("Depth: infinity query REPORT included non-matching contact:\n%s", text)
	}
}

func TestHandlerReportSyncCollectionReturnsFullSnapshotAndToken(t *testing.T) {
	t.Parallel()

	body := `<D:sync-collection xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:sync-token/>
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

func TestHandlerReportSyncCollectionAllowsDelegatedReadOnlyPrivileges(t *testing.T) {
	t.Parallel()

	handler := NewHandler(testCardDAVDiscoveryStore(t), func(*http.Request) (string, error) { return "delegate-1", nil })
	handler.AccessAuthorizer = &fakeCardDAVAccessAuthorizer{
		allowedRoles: map[string]bool{ContactsAccessRoleRead: true},
		privileges:   []XMLName{PrivilegeRead},
	}
	body := `<D:sync-collection xmlns:D="DAV:">
  <D:sync-token/>
  <D:sync-level>1</D:sync-level>
  <D:prop><D:getetag/><D:current-user-privilege-set/></D:prop>
</D:sync-collection>`
	req := httptest.NewRequest(MethodReport, "/carddav/addressbooks/user-1/personal/", strings.NewReader(body))
	req.Header.Set("Depth", string(DepthZero))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	if !strings.Contains(text, "<D:href>/carddav/addressbooks/user-1/personal/contact-1.vcf</D:href>") {
		t.Fatalf("delegated sync missing owner object:\n%s", text)
	}
	if !strings.Contains(text, "<D:current-user-privilege-set><D:privilege><D:read></D:read></D:privilege></D:current-user-privilege-set>") {
		t.Fatalf("delegated sync missing read-only privileges:\n%s", text)
	}
	for _, denied := range []string{"<D:bind>", "<D:unbind>", "<D:write-properties>", "<D:write-content>"} {
		if strings.Contains(text, denied) {
			t.Fatalf("delegated sync advertised %s:\n%s", denied, text)
		}
	}
}

func TestHandlerReportSyncCollectionRejectsDefaultSnapshotTruncation(t *testing.T) {
	t.Parallel()

	store := testCardDAVDiscoveryStore(t)
	base := store.objects[0]
	store.objects = store.objects[:0]
	for i := 0; i < MaxWebDAVReportLimit+1; i++ {
		object := base
		object.ObjectName = fmt.Sprintf("contact-%d.vcf", i)
		object.UID = fmt.Sprintf("contact-%d", i)
		store.objects = append(store.objects, object)
	}
	handler := NewHandler(&store, func(*http.Request) (string, error) { return "user-1", nil })
	body := `<D:sync-collection xmlns:D="DAV:">
  <D:sync-token/>
  <D:sync-level>1</D:sync-level>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`
	req := httptest.NewRequest(MethodReport, "/carddav/addressbooks/user-1/personal/", strings.NewReader(body))
	req.Header.Set("Depth", string(DepthZero))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "limit would truncate") {
		t.Fatalf("default snapshot truncation response lacks context: %s", rec.Body.String())
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

func TestHandlerReportSyncCollectionAllowsExactChangeLimit(t *testing.T) {
	t.Parallel()

	body := `<D:sync-collection xmlns:D="DAV:">
  <D:sync-token>sync-mid</D:sync-token>
  <D:sync-level>1</D:sync-level>
  <D:limit><D:nresults>1</D:nresults></D:limit>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthZero, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	for _, want := range []string{
		"<D:href>/carddav/addressbooks/user-1/personal/removed.vcf</D:href>",
		"<D:status>HTTP/1.1 404 Not Found</D:status>",
		"<D:sync-token>sync-123</D:sync-token>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("exact-limit sync REPORT missing %q:\n%s", want, text)
		}
	}
}

func TestHandlerReportSyncCollectionRejectsTruncatingChangeLimit(t *testing.T) {
	t.Parallel()

	body := `<D:sync-collection xmlns:D="DAV:">
  <D:sync-token>sync-old</D:sync-token>
  <D:sync-level>1</D:sync-level>
  <D:limit><D:nresults>1</D:nresults></D:limit>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthZero, body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "limit may truncate") {
		t.Fatalf("truncating change-limit response lacks context: %s", rec.Body.String())
	}
}

func TestHandlerReportSyncCollectionReturnsDeletedCollectionToken(t *testing.T) {
	t.Parallel()

	store := testCardDAVDiscoveryStore(t)
	store.books = nil
	store.objects = nil
	store.changes = append(store.changes, AddressBookChange{
		ID:            4,
		UserID:        "user-1",
		AddressBookID: "personal",
		Action:        "addressbook-deleted",
		SyncToken:     "sync-deleted",
		ChangedAt:     time.Date(2026, 5, 6, 10, 11, 12, 0, time.UTC),
	})
	handler := NewHandler(&store, func(*http.Request) (string, error) { return "user-1", nil })
	body := `<D:sync-collection xmlns:D="DAV:">
  <D:sync-token>sync-123</D:sync-token>
  <D:sync-level>1</D:sync-level>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`
	req := httptest.NewRequest(MethodReport, "/carddav/addressbooks/user-1/personal/", strings.NewReader(body))
	req.Header.Set("Depth", string(DepthZero))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	if strings.Contains(text, "<D:response>") {
		t.Fatalf("deleted collection sync should not return object responses:\n%s", text)
	}
	if !strings.Contains(text, "<D:sync-token>sync-deleted</D:sync-token>") {
		t.Fatalf("deleted collection sync token missing:\n%s", text)
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

func TestHandlerReportSyncCollectionRejectsDepthOne(t *testing.T) {
	t.Parallel()

	body := `<D:sync-collection xmlns:D="DAV:">
  <D:sync-token/>
  <D:sync-level>1</D:sync-level>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Depth: 0") {
		t.Fatalf("sync depth response lacks context: %s", rec.Body.String())
	}
}

func TestHandlerReportSyncCollectionRejectsMissingSyncTokenElement(t *testing.T) {
	t.Parallel()

	body := `<D:sync-collection xmlns:D="DAV:">
  <D:sync-level>1</D:sync-level>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthZero, body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "sync-token") {
		t.Fatalf("missing sync-token response lacks context: %s", rec.Body.String())
	}
}

func TestHandlerReportRejectsDepthInfinityAndCrossUserPath(t *testing.T) {
	t.Parallel()

	body := `<D:sync-collection xmlns:D="DAV:"><D:sync-token/><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop></D:sync-collection>`
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

func TestHandlerPropfindPrincipalCollectionDepthOne(t *testing.T) {
	t.Parallel()

	body := `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/><D:principal-collection-set/><D:resourcetype/></D:prop></D:propfind>`
	rec := runCardDAVPropfind(t, "/carddav/principals/", DepthOne, body)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	for _, want := range []string{
		"<D:href>/carddav/principals/</D:href>",
		"<D:href>/carddav/principals/user-1/</D:href>",
		"<D:current-user-principal><D:href>/carddav/principals/user-1/</D:href></D:current-user-principal>",
		"<D:principal-collection-set><D:href>/carddav/principals/</D:href></D:principal-collection-set>",
		"<D:resourcetype><D:collection></D:collection></D:resourcetype>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("principal collection PROPFIND missing %q:\n%s", want, text)
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

func TestHandlerPropfindAllowsDelegatedRead(t *testing.T) {
	t.Parallel()

	handler := NewHandler(testCardDAVDiscoveryStore(t), func(*http.Request) (string, error) { return "delegate-1", nil })
	handler.AccessAuthorizer = &fakeCardDAVAccessAuthorizer{
		allowedRoles: map[string]bool{ContactsAccessRoleRead: true},
		privileges:   []XMLName{PrivilegeRead},
	}
	req := httptest.NewRequest(MethodPropfind, "/carddav/addressbooks/user-1/personal/", strings.NewReader(`<D:propfind xmlns:D="DAV:"><D:prop><D:owner/><D:current-user-privilege-set/></D:prop></D:propfind>`))
	req.Header.Set("Depth", "0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := handler.AccessAuthorizer.(*fakeCardDAVAccessAuthorizer).last; got.ActorUserID != "delegate-1" || got.OwnerUserID != "user-1" || got.RequiredRole != ContactsAccessRoleRead {
		t.Fatalf("access request = %+v", got)
	}
	text := rec.Body.String()
	if !strings.Contains(text, "<D:href>/carddav/addressbooks/user-1/personal/</D:href>") || !strings.Contains(text, "<D:owner><D:href>/carddav/principals/user-1/</D:href></D:owner>") {
		t.Fatalf("delegated propfind did not use owner resource:\n%s", text)
	}
	if !strings.Contains(text, "<D:current-user-privilege-set><D:privilege><D:read></D:read></D:privilege></D:current-user-privilege-set>") {
		t.Fatalf("delegated propfind missing read-only privileges:\n%s", text)
	}
	for _, denied := range []string{"<D:bind>", "<D:unbind>", "<D:write-properties>", "<D:write-content>"} {
		if strings.Contains(text, denied) {
			t.Fatalf("delegated read propfind advertised %s:\n%s", denied, text)
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

func TestHandlerPropfindCollectionDepthOneRejectsTruncation(t *testing.T) {
	t.Parallel()

	store := testCardDAVDiscoveryStore(t)
	base := store.objects[0]
	store.objects = store.objects[:0]
	for i := 0; i < MaxWebDAVReportLimit+1; i++ {
		object := base
		object.ID = fmt.Sprintf("object-%d", i)
		object.ObjectName = fmt.Sprintf("contact-%d.vcf", i)
		object.UID = fmt.Sprintf("contact-%d", i)
		store.objects = append(store.objects, object)
	}
	body := `<D:propfind xmlns:D="DAV:"><D:prop><D:getetag/></D:prop></D:propfind>`
	req := httptest.NewRequest(MethodPropfind, "/carddav/addressbooks/user-1/personal/", strings.NewReader(body))
	req.Header.Set("Depth", string(DepthOne))
	rec := httptest.NewRecorder()
	NewHandler(store, func(*http.Request) (string, error) { return "user-1", nil }).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "address-book collection PROPFIND would truncate results") {
		t.Fatalf("truncating collection PROPFIND response lacks context: %s", rec.Body.String())
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

func runCardDAVReportWithoutDepth(t *testing.T, path string, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(MethodReport, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler := NewHandler(testCardDAVDiscoveryStore(t), func(*http.Request) (string, error) { return "user-1", nil })
	handler.ServeHTTP(rec, req)
	return rec
}

func runCardDAVObjectRequest(t *testing.T, method string, path string, body string, headers http.Header) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	rec := httptest.NewRecorder()
	handler := NewHandler(testCardDAVDiscoveryStore(t), func(*http.Request) (string, error) { return "user-1", nil })
	handler.ServeHTTP(rec, req)
	return rec
}

type fakeCardDAVAccessAuthorizer struct {
	allowedRoles map[string]bool
	privileges   []XMLName
	last         AccessRequest
	err          error
}

func (a *fakeCardDAVAccessAuthorizer) AuthorizeAddressBookAccess(_ context.Context, req AccessRequest) (AccessDecision, error) {
	a.last = req
	if a.err != nil {
		return AccessDecision{}, a.err
	}
	return AccessDecision{Allowed: a.allowedRoles[req.RequiredRole], Privileges: append([]XMLName(nil), a.privileges...)}, nil
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
			UID:           "contact-1",
			VCard:         []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-1\r\nFN:Contact One\r\nEMAIL;TYPE=home:contact-one@example.com\r\nEND:VCARD\r\n"),
			ETag:          `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`,
			Size:          64,
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
		}, {
			UserID:        "user-1",
			AddressBookID: "personal",
			ObjectName:    "contact-2.vcf",
			UID:           "contact-2",
			VCard:         []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-2\r\nFN:Other Person\r\nEMAIL;TYPE=work:other@example.com\r\nEND:VCARD\r\n"),
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
