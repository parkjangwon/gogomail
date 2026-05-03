package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/maildb"
)

func TestAdminQueueHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		queueStats: []maildb.QueueStat{{Topic: "mail.outbound.general", Status: "pending", Count: 2}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/queue", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Queues []maildb.QueueStat `json:"queues"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Queues) != 1 || body.Queues[0].Count != 2 {
		t.Fatalf("queues = %+v", body.Queues)
	}
}

func TestAdminDomainsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		domains: []maildb.DomainView{{ID: "domain-1", Name: "example.com", Status: "active"}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains?limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Domains []maildb.DomainView `json:"domains"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Domains) != 1 || body.Domains[0].Name != "example.com" {
		t.Fatalf("domains = %+v", body.Domains)
	}
	if service.lastLimit != 10 {
		t.Fatalf("lastLimit = %d", service.lastLimit)
	}
}

func TestAdminGetDomainHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		domains: []maildb.DomainView{{ID: "domain-1", Name: "example.com", Status: "active"}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/domains/domain-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Domain maildb.DomainView `json:"domain"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Domain.ID != "domain-1" || service.lastDomainID != "domain-1" {
		t.Fatalf("domain = %+v lastDomainID=%q", body.Domain, service.lastDomainID)
	}
}

func TestAdminCreateDomainHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	body := []byte(`{"company_id":"company-1","name":"Example.COM","quota_limit":1024}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/domains", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCreateDomain.CompanyID != "company-1" || service.lastCreateDomain.Name != "Example.COM" {
		t.Fatalf("lastCreateDomain = %+v", service.lastCreateDomain)
	}
}

func TestAdminUpdateDomainStatusHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/domains/domain-1/status", bytes.NewReader([]byte(`{"status":"suspended"}`)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDomainStatus.ID != "domain-1" || service.lastDomainStatus.Status != "suspended" {
		t.Fatalf("lastDomainStatus = %+v", service.lastDomainStatus)
	}
}

func TestAdminUsersHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		users: []maildb.UserView{{ID: "user-1", DomainID: "domain-1", Username: "admin", Status: "active"}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/users?domain_id=domain-1&limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Users []maildb.UserView `json:"users"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Users) != 1 || body.Users[0].Username != "admin" {
		t.Fatalf("users = %+v", body.Users)
	}
	if service.lastDomainID != "domain-1" || service.lastLimit != 10 {
		t.Fatalf("domain/limit = %q/%d", service.lastDomainID, service.lastLimit)
	}
}

func TestAdminGetUserHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		users: []maildb.UserView{{ID: "user-1", DomainID: "domain-1", Username: "admin", Status: "active"}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/users/user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		User maildb.UserView `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.User.ID != "user-1" || service.lastUserID != "user-1" {
		t.Fatalf("user = %+v lastUserID=%q", body.User, service.lastUserID)
	}
}

func TestAdminCreateUserHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	body := []byte(`{"domain_id":"domain-1","username":"admin","display_name":"Admin","address":"admin@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/users", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCreateUser.Username != "admin" || service.lastCreateUser.Address != "admin@example.com" {
		t.Fatalf("lastCreateUser = %+v", service.lastCreateUser)
	}
}

func TestAdminUpdateUserStatusHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/users/user-1/status", bytes.NewReader([]byte(`{"status":"disabled"}`)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserStatus.ID != "user-1" || service.lastUserStatus.Status != "disabled" {
		t.Fatalf("lastUserStatus = %+v", service.lastUserStatus)
	}
}

func TestAdminDeliveryAttemptsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		attempts: []maildb.DeliveryAttemptView{{
			ID:          "attempt-1",
			MessageID:   "msg-1",
			Recipient:   "user@example.net",
			Status:      "bounced",
			AttemptedAt: time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/delivery-attempts?limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastLimit != 10 {
		t.Fatalf("lastLimit = %d, want 10", service.lastLimit)
	}
}

func TestAdminSuppressionListHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		suppression: []maildb.SuppressionEntry{{
			ID:        "suppression-1",
			Email:     "user@example.net",
			Reason:    "hard_bounce",
			CreatedAt: time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/suppression-list?limit=5", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastLimit != 5 {
		t.Fatalf("lastLimit = %d, want 5", service.lastLimit)
	}
}

func TestAdminDKIMKeysHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{
		dkimKeys: []maildb.DKIMKeyView{{
			ID:       "dkim-1",
			DomainID: "domain-1",
			Selector: "s1",
			Status:   "active",
		}},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/dkim-keys?domain_id=domain-1&limit=5", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDomainID != "domain-1" {
		t.Fatalf("lastDomainID = %q, want domain-1", service.lastDomainID)
	}
	if service.lastLimit != 5 {
		t.Fatalf("lastLimit = %d, want 5", service.lastLimit)
	}
}

func TestAdminCreateDKIMKeyHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{createdDKIMKeyID: "dkim-1"}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	body := []byte(`{"domain_id":"domain-1","selector":"s1","private_key_pem":"private","public_key_dns":"v=DKIM1; p=public"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/dkim-keys", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastCreateDKIMKey.Selector != "s1" {
		t.Fatalf("lastCreateDKIMKey = %+v", service.lastCreateDKIMKey)
	}
}

func TestAdminDeactivateDKIMKeyHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/dkim-keys/dkim-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeactivateDKIMKeyID != "dkim-1" {
		t.Fatalf("lastDeactivateDKIMKeyID = %q", service.lastDeactivateDKIMKeyID)
	}
}

func TestAdminRetryOutboxHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/outbox/outbox-1/retry", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastRetryOutboxID != "outbox-1" {
		t.Fatalf("lastRetryOutboxID = %q", service.lastRetryOutboxID)
	}
}

func TestAdminDeleteSuppressionHandler(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "")

	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/suppression-list/suppression-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeleteSuppressionID != "suppression-1" {
		t.Fatalf("lastDeleteSuppressionID = %q", service.lastDeleteSuppressionID)
	}
}

func TestAdminRoutesRequireTokenWhenConfigured(t *testing.T) {
	t.Parallel()

	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "secret")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/queue", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/v1/queue", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

type fakeAdminService struct {
	domains                 []maildb.DomainView
	users                   []maildb.UserView
	queueStats              []maildb.QueueStat
	attempts                []maildb.DeliveryAttemptView
	suppression             []maildb.SuppressionEntry
	dkimKeys                []maildb.DKIMKeyView
	createdDKIMKeyID        string
	lastLimit               int
	lastDomainID            string
	lastUserID              string
	lastDomainStatus        maildb.UpdateDomainStatusRequest
	lastCreateDomain        maildb.CreateDomainRequest
	lastUserStatus          maildb.UpdateUserStatusRequest
	lastCreateUser          maildb.CreateUserRequest
	lastCreateDKIMKey       maildb.CreateDKIMKeyInput
	lastDeactivateDKIMKeyID string
	lastRetryOutboxID       string
	lastDeleteSuppressionID string
}

func (f *fakeAdminService) ListDomains(_ context.Context, limit int) ([]maildb.DomainView, error) {
	f.lastLimit = limit
	return f.domains, nil
}

func (f *fakeAdminService) CreateDomain(_ context.Context, req maildb.CreateDomainRequest) (maildb.DomainView, error) {
	f.lastCreateDomain = req
	return maildb.DomainView{ID: "domain-new", CompanyID: req.CompanyID, Name: req.Name, NameACE: req.NameACE, Status: "active"}, nil
}

func (f *fakeAdminService) GetDomain(_ context.Context, id string) (maildb.DomainView, error) {
	f.lastDomainID = id
	for _, domain := range f.domains {
		if domain.ID == id {
			return domain, nil
		}
	}
	return maildb.DomainView{}, nil
}

func (f *fakeAdminService) UpdateDomainStatus(_ context.Context, req maildb.UpdateDomainStatusRequest) error {
	f.lastDomainStatus = req
	return nil
}

func (f *fakeAdminService) ListUsers(_ context.Context, domainID string, limit int) ([]maildb.UserView, error) {
	f.lastDomainID = domainID
	f.lastLimit = limit
	return f.users, nil
}

func (f *fakeAdminService) CreateUser(_ context.Context, req maildb.CreateUserRequest) (maildb.UserView, error) {
	f.lastCreateUser = req
	return maildb.UserView{ID: "user-new", DomainID: req.DomainID, Username: req.Username, DisplayName: req.DisplayName, Status: "active"}, nil
}

func (f *fakeAdminService) GetUser(_ context.Context, id string) (maildb.UserView, error) {
	f.lastUserID = id
	for _, user := range f.users {
		if user.ID == id {
			return user, nil
		}
	}
	return maildb.UserView{}, nil
}

func (f *fakeAdminService) UpdateUserStatus(_ context.Context, req maildb.UpdateUserStatusRequest) error {
	f.lastUserStatus = req
	return nil
}

func (f *fakeAdminService) ListQueueStats(context.Context) ([]maildb.QueueStat, error) {
	return f.queueStats, nil
}

func (f *fakeAdminService) ListDeliveryAttempts(_ context.Context, limit int) ([]maildb.DeliveryAttemptView, error) {
	f.lastLimit = limit
	return f.attempts, nil
}

func (f *fakeAdminService) ListSuppressionEntries(_ context.Context, limit int) ([]maildb.SuppressionEntry, error) {
	f.lastLimit = limit
	return f.suppression, nil
}

func (f *fakeAdminService) ListDKIMKeys(_ context.Context, domainID string, limit int) ([]maildb.DKIMKeyView, error) {
	f.lastDomainID = domainID
	f.lastLimit = limit
	return f.dkimKeys, nil
}

func (f *fakeAdminService) CreateDKIMKey(_ context.Context, input maildb.CreateDKIMKeyInput) (string, error) {
	f.lastCreateDKIMKey = input
	if f.createdDKIMKeyID != "" {
		return f.createdDKIMKeyID, nil
	}
	return "dkim-1", nil
}

func (f *fakeAdminService) DeactivateDKIMKey(_ context.Context, id string) error {
	f.lastDeactivateDKIMKeyID = id
	return nil
}

func (f *fakeAdminService) RetryOutbox(_ context.Context, id string) error {
	f.lastRetryOutboxID = id
	return nil
}

func (f *fakeAdminService) DeleteSuppressionEntry(_ context.Context, id string) error {
	f.lastDeleteSuppressionID = id
	return nil
}
