package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/apikeys"
	"github.com/gogomail/gogomail/internal/carddavgw"
	"github.com/gogomail/gogomail/internal/directory"
)

type fakeContactRepo struct {
	addressBooks []carddavgw.AddressBook
	contacts     []carddavgw.ContactObject
	err          error
}

func (f *fakeContactRepo) ListAddressBooks(ctx context.Context, req carddavgw.ListAddressBooksRequest) ([]carddavgw.AddressBook, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.addressBooks, nil
}

func (f *fakeContactRepo) CreateAddressBook(ctx context.Context, req carddavgw.CreateAddressBookRequest) (carddavgw.AddressBook, error) {
	if f.err != nil {
		return carddavgw.AddressBook{}, f.err
	}
	if req.Name == "" {
		return carddavgw.AddressBook{}, fmt.Errorf("name is required")
	}
	return carddavgw.AddressBook{
		ID:   "addr-1",
		Name: req.Name,
	}, nil
}

func (f *fakeContactRepo) GetAddressBook(ctx context.Context, req carddavgw.GetAddressBookRequest) (carddavgw.AddressBook, error) {
	if f.err != nil {
		return carddavgw.AddressBook{}, f.err
	}
	for _, ab := range f.addressBooks {
		if ab.ID == req.AddressBookID {
			return ab, nil
		}
	}
	return carddavgw.AddressBook{}, fmt.Errorf("address book not found")
}

func (f *fakeContactRepo) UpdateAddressBookProperties(ctx context.Context, req carddavgw.UpdateAddressBookRequest) (carddavgw.AddressBook, error) {
	if f.err != nil {
		return carddavgw.AddressBook{}, f.err
	}
	return carddavgw.AddressBook{ID: req.AddressBookID, Name: "Updated"}, nil
}

func (f *fakeContactRepo) DeleteAddressBook(ctx context.Context, req carddavgw.DeleteAddressBookRequest) (carddavgw.AddressBook, error) {
	if f.err != nil {
		return carddavgw.AddressBook{}, f.err
	}
	return carddavgw.AddressBook{ID: req.AddressBookID}, nil
}

func (f *fakeContactRepo) ListContactObjects(ctx context.Context, req carddavgw.ListContactObjectsRequest) ([]carddavgw.ContactObject, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.contacts, nil
}

func (f *fakeContactRepo) GetContactObject(ctx context.Context, req carddavgw.GetContactObjectRequest) (carddavgw.ContactObject, error) {
	if f.err != nil {
		return carddavgw.ContactObject{}, f.err
	}
	for _, c := range f.contacts {
		if c.ObjectName == req.ObjectName {
			return c, nil
		}
	}
	return carddavgw.ContactObject{}, fmt.Errorf("contact not found")
}

func (f *fakeContactRepo) UpsertContactObject(ctx context.Context, req carddavgw.UpsertContactObjectRequest) (carddavgw.ContactObject, error) {
	if f.err != nil {
		return carddavgw.ContactObject{}, f.err
	}
	return carddavgw.ContactObject{ID: "contact-1", ObjectName: req.ObjectName, ETag: "etag-1"}, nil
}

func (f *fakeContactRepo) DeleteContactObject(ctx context.Context, req carddavgw.DeleteContactObjectRequest) (carddavgw.ContactObject, error) {
	if f.err != nil {
		return carddavgw.ContactObject{}, f.err
	}
	return carddavgw.ContactObject{ID: "contact-1", ObjectName: req.ObjectName}, nil
}

func (f *fakeContactRepo) SearchContacts(ctx context.Context, req carddavgw.SearchContactsRequest) ([]carddavgw.ContactObject, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.contacts, nil
}

type fakeDirectoryRepo struct {
	lastSearch directory.SearchPrincipalsRequest
	lastOrg    struct {
		companyID string
		domainID  string
	}
}

func (f *fakeDirectoryRepo) SearchPrincipals(_ context.Context, req directory.SearchPrincipalsRequest) ([]directory.Principal, error) {
	f.lastSearch = req
	return []directory.Principal{{ID: "user-1", DisplayName: "Jangwon Park", PrimaryEmail: "pjw@parkjw.org"}}, nil
}

func (f *fakeDirectoryRepo) ResolvePrincipal(_ context.Context, req directory.ResolvePrincipalRequest) (directory.Principal, error) {
	return directory.Principal{ID: req.ID, CompanyID: "company-1", DomainID: "domain-1", PrimaryEmail: "pjw@parkjw.org"}, nil
}

func (f *fakeDirectoryRepo) ListOrgTree(_ context.Context, companyID, domainID string) ([]directory.OrgTreeItem, error) {
	f.lastOrg.companyID = companyID
	f.lastOrg.domainID = domainID
	return []directory.OrgTreeItem{{ID: "org-1", DisplayName: "Team", Depth: 0}}, nil
}

func (f *fakeDirectoryRepo) ListOrgMembersByOrgIDs(_ context.Context, companyID, domainID string, orgIDs []string, limitPerOrg int) (map[string][]directory.Principal, error) {
	return map[string][]directory.Principal{"org-1": []directory.Principal{{ID: "user-1", DisplayName: "Jangwon Park", PrimaryEmail: "pjw@parkjw.org"}}}, nil
}

func TestContactListAddressBooks(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &ContactHandler{repo: &fakeContactRepo{}}
	RegisterContactRoutes(mux, handler, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/mail/addressbooks?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/mail/addressbooks: got status %d, want 200", rec.Code)
	}
}

func TestContactCreateRequestValidation(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &ContactHandler{repo: &fakeContactRepo{}}
	RegisterContactRoutes(mux, handler, nil)

	body := `{"name":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/mail/addressbooks?user_id=user-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/mail/addressbooks with empty name: got status %d, want 400", rec.Code)
	}
}

func TestContactGetNotFound(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &ContactHandler{repo: &fakeContactRepo{}}
	RegisterContactRoutes(mux, handler, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/mail/addressbooks/nonexistent?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/mail/addressbooks/nonexistent: got status %d, want 404", rec.Code)
	}
}

func TestContactUpdateSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &ContactHandler{repo: &fakeContactRepo{}}
	RegisterContactRoutes(mux, handler, nil)

	body := `{"name":"Updated"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/mail/addressbooks/addr-1?user_id=user-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH /api/mail/addressbooks/addr-1: got status %d, want 200", rec.Code)
	}
}

func TestAddressBookDeleteSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &ContactHandler{repo: &fakeContactRepo{}}
	RegisterContactRoutes(mux, handler, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/mail/addressbooks/addr-1?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/mail/addressbooks/addr-1: got status %d, want 204", rec.Code)
	}
}

func TestContactListSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &ContactHandler{repo: &fakeContactRepo{}}
	RegisterContactRoutes(mux, handler, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/mail/addressbooks/addr-1/contacts?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/mail/addressbooks/addr-1/contacts: got status %d, want 200", rec.Code)
	}
}

func TestContactGetSuccess(t *testing.T) {
	t.Parallel()

	repo := &fakeContactRepo{
		contacts: []carddavgw.ContactObject{
			{ID: "contact-1", ObjectName: "contact.vcf", ETag: "etag-1", VCard: []byte("BEGIN:VCARD\r\nVERSION:3.0\r\nFN:John Doe\r\nEND:VCARD")},
		},
	}
	mux := http.NewServeMux()
	handler := &ContactHandler{repo: repo}
	RegisterContactRoutes(mux, handler, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/mail/addressbooks/addr-1/contacts/contact.vcf?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/mail/addressbooks/addr-1/contacts/contact.vcf: got status %d, want 200", rec.Code)
	}
}

func TestContactPutSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &ContactHandler{repo: &fakeContactRepo{}}
	RegisterContactRoutes(mux, handler, nil)

	body := `BEGIN:VCARD\r\nVERSION:3.0\r\nFN:John Doe\r\nEND:VCARD`
	req := httptest.NewRequest(http.MethodPut, "/api/mail/addressbooks/addr-1/contacts/contact.vcf?user_id=user-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/vcard")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/mail/addressbooks/addr-1/contacts/contact.vcf: got status %d, want 200", rec.Code)
	}
}

func TestContactPutInvalidContentType(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &ContactHandler{repo: &fakeContactRepo{}}
	RegisterContactRoutes(mux, handler, nil)

	body := `not vcard`
	req := httptest.NewRequest(http.MethodPut, "/api/mail/addressbooks/addr-1/contacts/contact.vcf?user_id=user-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("PUT with invalid content type: got status %d, want 415", rec.Code)
	}
}

func TestContactDeleteSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &ContactHandler{repo: &fakeContactRepo{}}
	RegisterContactRoutes(mux, handler, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/mail/addressbooks/addr-1/contacts/contact.vcf?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/mail/addressbooks/addr-1/contacts/contact.vcf: got status %d, want 204", rec.Code)
	}
}

func TestContactListMissingUserID(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &ContactHandler{}
	RegisterContactRoutes(mux, handler, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/mail/addressbooks", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /api/mail/addressbooks without user_id: got status %d, want 400", rec.Code)
	}
}

func TestAddressBookEnvelopeJSON(t *testing.T) {
	t.Parallel()

	env := AddressBookEnvelope{
		AddressBook: carddavgw.AddressBook{ID: "addr-1", Name: "Test"},
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal AddressBookEnvelope: %v", err)
	}
	var out AddressBookEnvelope
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal AddressBookEnvelope: %v", err)
	}
	if out.AddressBook.ID != "addr-1" || out.AddressBook.Name != "Test" {
		t.Fatalf("AddressBookEnvelope round-trip: got %+v, want {ID:addr-1 Name:Test}", out.AddressBook)
	}
}

func TestAddressBookListEnvelopeJSON(t *testing.T) {
	t.Parallel()

	env := AddressBookListEnvelope{
		AddressBooks: []carddavgw.AddressBook{
			{ID: "addr-1", Name: "Address Book 1"},
			{ID: "addr-2", Name: "Address Book 2"},
		},
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal AddressBookListEnvelope: %v", err)
	}
	var out AddressBookListEnvelope
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal AddressBookListEnvelope: %v", err)
	}
	if len(out.AddressBooks) != 2 {
		t.Fatalf("AddressBookListEnvelope address books: got %d, want 2", len(out.AddressBooks))
	}
}

func TestContactObjectEnvelopeJSON(t *testing.T) {
	t.Parallel()

	env := ContactObjectEnvelope{
		Contact: carddavgw.ContactObject{ID: "contact-1", ObjectName: "contact.vcf", ETag: "etag-1"},
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal ContactObjectEnvelope: %v", err)
	}
	var out ContactObjectEnvelope
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal ContactObjectEnvelope: %v", err)
	}
	if out.Contact.ETag != "etag-1" {
		t.Fatalf("ContactObjectEnvelope ETag: got %s, want etag-1", out.Contact.ETag)
	}
}

func TestContactObjectListEnvelopeJSON(t *testing.T) {
	t.Parallel()

	env := ContactObjectListEnvelope{
		Contacts: []carddavgw.ContactObject{
			{ID: "contact-1", ObjectName: "contact1.vcf"},
			{ID: "contact-2", ObjectName: "contact2.vcf"},
		},
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal ContactObjectListEnvelope: %v", err)
	}
	var out ContactObjectListEnvelope
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal ContactObjectListEnvelope: %v", err)
	}
	if len(out.Contacts) != 2 {
		t.Fatalf("ContactObjectListEnvelope contacts: got %d, want 2", len(out.Contacts))
	}
}

func TestContactAutocompleteRequiresQuery(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &ContactHandler{repo: &fakeContactRepo{}, directoryRepo: nil}
	RegisterContactRoutes(mux, handler, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/mail/contacts/autocomplete", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /api/mail/contacts/autocomplete without q: got status %d, want 400", rec.Code)
	}
}

func TestContactAutocompleteWithNoResults(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &ContactHandler{repo: &fakeContactRepo{contacts: nil}, directoryRepo: nil}
	RegisterContactRoutes(mux, handler, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/mail/contacts/autocomplete?q=test&user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/mail/contacts/autocomplete: got status %d, want 200", rec.Code)
	}

	var out AutocompleteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Unmarshal AutocompleteResponse: %v", err)
	}
	if len(out.Results) != 0 {
		t.Fatalf("AutocompleteResponse results: got %d, want 0", len(out.Results))
	}
}

func TestDirectoryUsersAcceptsUserMCPAPIKey(t *testing.T) {
	t.Parallel()

	dir := &fakeDirectoryRepo{}
	mux := http.NewServeMux()
	handler := &ContactHandler{repo: &fakeContactRepo{}, directoryRepo: dir}
	RegisterContactRoutes(mux, handler, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/mail/directory/users?q=pjw&limit=5", nil)
	req = req.WithContext(apikeys.ContextWithKeyInfo(req.Context(), &apikeys.KeyInfo{
		ID:       "key-1",
		UserID:   "user-1",
		DomainID: "domain-1",
		Scopes:   []string{"contacts:read"},
	}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/mail/directory/users with MCP key: got status %d body %s, want 200", rec.Code, rec.Body.String())
	}
	if dir.lastSearch.CompanyID != "company-1" || dir.lastSearch.DomainID != "domain-1" || dir.lastSearch.Query != "pjw" || dir.lastSearch.Limit != 5 {
		t.Fatalf("lastSearch = %+v", dir.lastSearch)
	}
}

func TestDirectoryOrgTreeAcceptsUserMCPAPIKey(t *testing.T) {
	t.Parallel()

	dir := &fakeDirectoryRepo{}
	mux := http.NewServeMux()
	handler := &ContactHandler{repo: &fakeContactRepo{}, directoryRepo: dir}
	RegisterContactRoutes(mux, handler, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/mail/directory/org-tree", nil)
	req = req.WithContext(apikeys.ContextWithKeyInfo(req.Context(), &apikeys.KeyInfo{
		ID:       "key-1",
		UserID:   "user-1",
		DomainID: "domain-1",
		Scopes:   []string{"contacts:read"},
	}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/mail/directory/org-tree with MCP key: got status %d body %s, want 200", rec.Code, rec.Body.String())
	}
	if dir.lastOrg.companyID != "company-1" || dir.lastOrg.domainID != "domain-1" {
		t.Fatalf("lastOrg = %+v", dir.lastOrg)
	}
}
