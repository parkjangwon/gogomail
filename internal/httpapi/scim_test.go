package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/httpapi"
	"github.com/gogomail/gogomail/internal/scim"
)

// fakeSCIMUserService is an in-memory SCIMUserService for tests.
type fakeSCIMUserService struct {
	users   map[string]scim.UserResource
	seq     int
	listErr error
}

func newFakeSCIMUserService() *fakeSCIMUserService {
	return &fakeSCIMUserService{users: make(map[string]scim.UserResource)}
}

func (f *fakeSCIMUserService) nextID() string {
	f.seq++
	return fmt.Sprintf("user-%d", f.seq)
}

func (f *fakeSCIMUserService) GetSCIMUser(_ context.Context, id string) (scim.UserResource, error) {
	u, ok := f.users[id]
	if !ok {
		return scim.UserResource{}, httpapi.ErrSCIMUserNotFound
	}
	return u, nil
}

func (f *fakeSCIMUserService) ListSCIMUsers(_ context.Context, filter *scim.Filter, _, _ int) ([]scim.UserResource, int, error) {
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	var result []scim.UserResource
	for _, u := range f.users {
		if filter == nil || scim.MatchesFilter(u, filter) {
			result = append(result, u)
		}
	}
	return result, len(result), nil
}

func (f *fakeSCIMUserService) CreateSCIMUser(_ context.Context, req scim.UserResource) (scim.UserResource, error) {
	req.ID = f.nextID()
	if len(req.Schemas) == 0 {
		req.Schemas = []string{scim.SchemaUser}
	}
	f.users[req.ID] = req
	return req, nil
}

func (f *fakeSCIMUserService) ReplaceSCIMUser(_ context.Context, id string, req scim.UserResource) (scim.UserResource, error) {
	if _, ok := f.users[id]; !ok {
		return scim.UserResource{}, httpapi.ErrSCIMUserNotFound
	}
	req.ID = id
	f.users[id] = req
	return req, nil
}

func (f *fakeSCIMUserService) PatchSCIMUser(_ context.Context, id string, ops []scim.PatchOperation) (scim.UserResource, error) {
	u, ok := f.users[id]
	if !ok {
		return scim.UserResource{}, httpapi.ErrSCIMUserNotFound
	}
	for _, op := range ops {
		switch strings.ToLower(op.Op) {
		case "replace":
			if op.Path == "" {
				var attrs map[string]json.RawMessage
				if err := json.Unmarshal(op.Value, &attrs); err != nil {
					continue
				}
				if raw, ok2 := attrs["active"]; ok2 {
					var active bool
					if err := json.Unmarshal(raw, &active); err == nil {
						u.Active = active
					}
				}
				if raw, ok2 := attrs["displayName"]; ok2 {
					var dn string
					if err := json.Unmarshal(raw, &dn); err == nil {
						u.Name.Formatted = dn
					}
				}
				if raw, ok2 := attrs["userName"]; ok2 {
					var un string
					if err := json.Unmarshal(raw, &un); err == nil {
						u.UserName = un
					}
				}
				continue
			}
			switch strings.ToLower(op.Path) {
			case "active":
				var active bool
				if err := json.Unmarshal(op.Value, &active); err == nil {
					u.Active = active
				}
			case "displayname":
				var dn string
				if err := json.Unmarshal(op.Value, &dn); err == nil {
					u.Name.Formatted = dn
				}
			case "username":
				var un string
				if err := json.Unmarshal(op.Value, &un); err == nil {
					u.UserName = un
				}
			}
		}
	}
	f.users[id] = u
	return u, nil
}

func (f *fakeSCIMUserService) DeleteSCIMUser(_ context.Context, id string) error {
	if _, ok := f.users[id]; !ok {
		return httpapi.ErrSCIMUserNotFound
	}
	delete(f.users, id)
	return nil
}

const scimToken = "test-scim-token"

func newSCIMServer(svc httpapi.SCIMUserService) *httptest.Server {
	mux := http.NewServeMux()
	httpapi.RegisterSCIMRoutes(mux, svc, scimToken)
	return httptest.NewServer(mux)
}

func scimRequest(t *testing.T, srv *httptest.Server, method, path string, body any) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req, err := http.NewRequest(method, srv.URL+path, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+scimToken)
	req.Header.Set("Content-Type", "application/scim+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestSCIMServiceProviderConfig(t *testing.T) {
	srv := newSCIMServer(newFakeSCIMUserService())
	defer srv.Close()

	resp := scimRequest(t, srv, http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	schemas, _ := body["schemas"].([]any)
	if len(schemas) == 0 || schemas[0] != "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig" {
		t.Errorf("unexpected schemas: %v", schemas)
	}
}

func TestSCIMResourceTypes(t *testing.T) {
	srv := newSCIMServer(newFakeSCIMUserService())
	defer srv.Close()

	resp := scimRequest(t, srv, http.MethodGet, "/scim/v2/ResourceTypes", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if total, _ := body["totalResults"].(float64); total != 1 {
		t.Errorf("totalResults = %v, want 1", total)
	}
}

func TestSCIMCreateAndGetUser(t *testing.T) {
	svc := newFakeSCIMUserService()
	srv := newSCIMServer(svc)
	defer srv.Close()

	// POST create
	createResp := scimRequest(t, srv, http.MethodPost, "/scim/v2/Users", map[string]any{
		"schemas":  []string{scim.SchemaUser},
		"userName": "alice@example.com",
		"name":     map[string]any{"givenName": "Alice", "familyName": "Smith"},
		"active":   true,
	})
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201", createResp.StatusCode)
	}
	var created scim.UserResource
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.ID == "" {
		t.Fatal("created user has no ID")
	}
	if created.UserName != "alice@example.com" {
		t.Errorf("userName = %q, want alice@example.com", created.UserName)
	}

	// GET single
	getResp := scimRequest(t, srv, http.MethodGet, "/scim/v2/Users/"+created.ID, nil)
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d, want 200", getResp.StatusCode)
	}
	var fetched scim.UserResource
	if err := json.NewDecoder(getResp.Body).Decode(&fetched); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("id = %q, want %q", fetched.ID, created.ID)
	}
}

func TestSCIMListUsers(t *testing.T) {
	svc := newFakeSCIMUserService()
	srv := newSCIMServer(svc)
	defer srv.Close()

	// Create two users
	for _, name := range []string{"alice@example.com", "bob@example.com"} {
		r := scimRequest(t, srv, http.MethodPost, "/scim/v2/Users", map[string]any{
			"schemas":  []string{scim.SchemaUser},
			"userName": name,
			"active":   true,
		})
		r.Body.Close()
	}

	// List all
	listResp := scimRequest(t, srv, http.MethodGet, "/scim/v2/Users", nil)
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, want 200", listResp.StatusCode)
	}
	var body scim.ListResponse
	if err := json.NewDecoder(listResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if body.TotalResults != 2 {
		t.Errorf("totalResults = %d, want 2", body.TotalResults)
	}
}

func TestSCIMListUsersWithFilter(t *testing.T) {
	svc := newFakeSCIMUserService()
	srv := newSCIMServer(svc)
	defer srv.Close()

	for _, name := range []string{"alice@example.com", "bob@example.com"} {
		r := scimRequest(t, srv, http.MethodPost, "/scim/v2/Users", map[string]any{
			"schemas":  []string{scim.SchemaUser},
			"userName": name,
			"active":   true,
		})
		r.Body.Close()
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/scim/v2/Users?filter=userName+eq+%22alice%40example.com%22", nil)
	req.Header.Set("Authorization", "Bearer "+scimToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("filter status = %d, want 200", resp.StatusCode)
	}
	var body scim.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.TotalResults != 1 {
		t.Errorf("totalResults = %d, want 1", body.TotalResults)
	}
	if len(body.Resources) > 0 && body.Resources[0].UserName != "alice@example.com" {
		t.Errorf("userName = %q, want alice@example.com", body.Resources[0].UserName)
	}
}

func TestSCIMInternalErrorsDoNotLeakBackendDetails(t *testing.T) {
	svc := newFakeSCIMUserService()
	svc.listErr = errors.New(`pq: relation "users" does not exist`)
	srv := newSCIMServer(svc)
	defer srv.Close()

	resp := scimRequest(t, srv, http.MethodGet, "/scim/v2/Users", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	detail, _ := body["detail"].(string)
	if detail != "internal server error" {
		t.Fatalf("detail = %q, want generic internal server error", detail)
	}
	if strings.Contains(detail, "pq:") || strings.Contains(detail, "users") {
		t.Fatalf("detail leaked backend error: %q", detail)
	}
}

func TestSCIMDeleteUser(t *testing.T) {
	svc := newFakeSCIMUserService()
	srv := newSCIMServer(svc)
	defer srv.Close()

	createResp := scimRequest(t, srv, http.MethodPost, "/scim/v2/Users", map[string]any{
		"schemas":  []string{scim.SchemaUser},
		"userName": "alice@example.com",
		"active":   true,
	})
	var created scim.UserResource
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()

	deleteResp := scimRequest(t, srv, http.MethodDelete, "/scim/v2/Users/"+created.ID, nil)
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", deleteResp.StatusCode)
	}

	// Confirm gone
	getResp := scimRequest(t, srv, http.MethodGet, "/scim/v2/Users/"+created.ID, nil)
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("after delete, get status = %d, want 404", getResp.StatusCode)
	}
}

func TestSCIMAuthRequired(t *testing.T) {
	srv := newSCIMServer(newFakeSCIMUserService())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/scim/v2/Users", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestSCIMGetUserNotFound(t *testing.T) {
	srv := newSCIMServer(newFakeSCIMUserService())
	defer srv.Close()

	resp := scimRequest(t, srv, http.MethodGet, "/scim/v2/Users/does-not-exist", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestSCIMPatchUserActive(t *testing.T) {
	svc := newFakeSCIMUserService()
	srv := newSCIMServer(svc)
	defer srv.Close()

	// Create a user that starts active.
	createResp := scimRequest(t, srv, http.MethodPost, "/scim/v2/Users", map[string]any{
		"schemas":  []string{scim.SchemaUser},
		"userName": "alice@example.com",
		"active":   true,
	})
	var created scim.UserResource
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()

	// PATCH to deactivate using path-less replace.
	patchBody := map[string]any{
		"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		"Operations": []map[string]any{
			{"op": "replace", "value": map[string]any{"active": false}},
		},
	}
	patchResp := scimRequest(t, srv, http.MethodPatch, "/scim/v2/Users/"+created.ID, patchBody)
	defer patchResp.Body.Close()
	if patchResp.StatusCode != http.StatusOK {
		t.Fatalf("patch status = %d, want 200", patchResp.StatusCode)
	}
	var patched scim.UserResource
	if err := json.NewDecoder(patchResp.Body).Decode(&patched); err != nil {
		t.Fatalf("decode patch response: %v", err)
	}
	if patched.Active {
		t.Errorf("after PATCH active=false, user is still active")
	}

	// PATCH to reactivate using path-targeted replace.
	patchReactivate := map[string]any{
		"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		"Operations": []map[string]any{
			{"op": "replace", "path": "active", "value": true},
		},
	}
	reactResp := scimRequest(t, srv, http.MethodPatch, "/scim/v2/Users/"+created.ID, patchReactivate)
	defer reactResp.Body.Close()
	if reactResp.StatusCode != http.StatusOK {
		t.Fatalf("reactivate patch status = %d, want 200", reactResp.StatusCode)
	}
	var reactivated scim.UserResource
	if err := json.NewDecoder(reactResp.Body).Decode(&reactivated); err != nil {
		t.Fatalf("decode reactivate response: %v", err)
	}
	if !reactivated.Active {
		t.Errorf("after PATCH active=true, user is still inactive")
	}
}

func TestSCIMPatchUserNotFound(t *testing.T) {
	srv := newSCIMServer(newFakeSCIMUserService())
	defer srv.Close()

	patchBody := map[string]any{
		"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		"Operations": []map[string]any{
			{"op": "replace", "value": map[string]any{"active": false}},
		},
	}
	resp := scimRequest(t, srv, http.MethodPatch, "/scim/v2/Users/does-not-exist", patchBody)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestSCIMServiceProviderConfigPatchSupported(t *testing.T) {
	srv := newSCIMServer(newFakeSCIMUserService())
	defer srv.Close()

	resp := scimRequest(t, srv, http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	defer resp.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	patch, _ := body["patch"].(map[string]any)
	if supported, _ := patch["supported"].(bool); !supported {
		t.Errorf("patch.supported = false, want true")
	}
}

// Ensure strings import is used (compile guard).
var _ = strings.ToLower
