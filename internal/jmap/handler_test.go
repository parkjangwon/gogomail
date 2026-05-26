package jmap_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/jmap"
)

// newTestHandler returns a Handler with no Auth (test mode: any request accepted).
func newTestHandler() *jmap.Handler {
	return jmap.NewHandler(jmap.Deps{}, func(_ context.Context, userID, accountID string) (*jmap.Session, error) {
		return jmap.BuildSession(userID, accountID, "https://mail.example.com"), nil
	})
}

// TestServeSessionReturnsJSON verifies that GET /.well-known/jmap returns
// HTTP 200 with a valid JSON Session object when Auth is nil (test mode).
func TestServeSessionReturnsJSON(t *testing.T) {
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/jmap", nil)
	req.Header.Set("X-Test-UserID", "alice@example.com")
	rec := httptest.NewRecorder()

	h.ServeSession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var sess jmap.Session
	if err := json.NewDecoder(rec.Body).Decode(&sess); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if sess.Username != "alice@example.com" {
		t.Errorf("username: got %q, want %q", sess.Username, "alice@example.com")
	}
	if sess.APIUrl == "" {
		t.Error("apiUrl must not be empty")
	}
	if len(sess.Capabilities) == 0 {
		t.Error("capabilities must not be empty")
	}
}

// TestServeSessionRequiresAuth verifies that ServeSession returns 401 when
// Auth is configured and no valid Bearer token is provided.
func TestServeSessionRequiresAuth(t *testing.T) {
	tm, err := auth.NewTokenManager("test-secret-32-bytes-minimum!!xx")
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	deps := jmap.Deps{Auth: tm}
	h := jmap.NewHandler(deps, func(_ context.Context, userID, accountID string) (*jmap.Session, error) {
		return jmap.BuildSession(userID, accountID, "https://mail.example.com"), nil
	})
	req := httptest.NewRequest(http.MethodGet, "/.well-known/jmap", nil)
	// No Authorization header — must get 401.
	rec := httptest.NewRecorder()
	h.ServeSession(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// TestServeAPIUnknownMethod verifies that calling an unregistered method
// returns an error MethodResponse with type "unknownMethod".
func TestServeAPIUnknownMethod(t *testing.T) {
	h := newTestHandler()

	reqBody, _ := json.Marshal(jmap.Request{
		Using: []string{jmap.CapabilityCore},
		MethodCalls: []jmap.MethodCall{
			{Name: "NoSuchMethod/get", Args: json.RawMessage(`{}`), CallID: "c1"},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/jmap/api", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeAPI(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp jmap.Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if len(resp.MethodResponses) != 1 {
		t.Fatalf("expected 1 method response, got %d", len(resp.MethodResponses))
	}
	mr := resp.MethodResponses[0]
	if mr.Name != "error" {
		t.Errorf("expected method name 'error', got %q", mr.Name)
	}
	if mr.CallID != "c1" {
		t.Errorf("expected call-id 'c1', got %q", mr.CallID)
	}

	var errObj struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(mr.Result, &errObj); err != nil {
		t.Fatalf("cannot decode error result: %v", err)
	}
	if errObj.Type != jmap.ErrUnknownMethod {
		t.Errorf("expected error type %q, got %q", jmap.ErrUnknownMethod, errObj.Type)
	}
}

// TestServeAPIEmailGet verifies that Email/get dispatches correctly.
// With no repository configured (test handler), it returns a serverFail error
// at the method level (HTTP 200 with error in method response).
func TestServeAPIEmailGet(t *testing.T) {
	h := newTestHandler()

	args, _ := json.Marshal(jmap.EmailGetArgs{
		AccountID: "u1",
		IDs:       []string{"id1", "id2"},
	})
	reqBody, _ := json.Marshal(jmap.Request{
		Using: []string{jmap.CapabilityCore, jmap.CapabilityMail},
		MethodCalls: []jmap.MethodCall{
			{Name: "Email/get", Args: args, CallID: "r1"},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/jmap/api", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeAPI(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp jmap.Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("cannot decode response: %v", err)
	}
	if len(resp.MethodResponses) != 1 {
		t.Fatalf("expected 1 method response, got %d", len(resp.MethodResponses))
	}
	mr := resp.MethodResponses[0]
	// The method response name is "Email/get" (not "error") because the method
	// itself returns the error JSON — the dispatcher wraps only panics/Go errors.
	if mr.Name != "Email/get" {
		t.Errorf("expected method name 'Email/get', got %q", mr.Name)
	}
	// Without a real repo, the method returns {"type":"serverFail"}.
	var errResp map[string]string
	if err := json.Unmarshal(mr.Result, &errResp); err != nil {
		t.Fatalf("cannot decode method result: %v", err)
	}
	if errResp["type"] != "serverFail" {
		t.Errorf("expected serverFail, got %q", errResp["type"])
	}
}

// TestServeAPIEmailQuery verifies that Email/query returns serverFail when repo is nil.
// A nil repo is what newTestHandler provides; real DB integration is tested separately.
func TestServeAPIEmailQuery(t *testing.T) {
	h := newTestHandler()

	args, _ := json.Marshal(jmap.EmailQueryArgs{
		AccountID: "u1",
		Limit:     10,
	})
	reqBody, _ := json.Marshal(jmap.Request{
		Using: []string{jmap.CapabilityCore, jmap.CapabilityMail},
		MethodCalls: []jmap.MethodCall{
			{Name: "Email/query", Args: args, CallID: "q1"},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/jmap/api", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeAPI(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp jmap.Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("cannot decode response: %v", err)
	}
	if len(resp.MethodResponses) != 1 {
		t.Fatalf("expected 1 method response, got %d", len(resp.MethodResponses))
	}
	mr := resp.MethodResponses[0]
	if mr.Name != "Email/query" {
		t.Errorf("expected method name 'Email/query', got %q", mr.Name)
	}

	// Nil repo → serverFail error result.
	var errResp map[string]string
	if err := json.Unmarshal(mr.Result, &errResp); err != nil {
		t.Fatalf("cannot decode error result: %v", err)
	}
	if errResp["type"] != jmap.ErrServerFail {
		t.Errorf("expected serverFail, got %q", errResp["type"])
	}
}

// TestMethodCallJSONRoundtrip verifies that MethodCall marshals/unmarshals
// correctly as the three-element JSON array [name, args, call-id].
func TestMethodCallJSONRoundtrip(t *testing.T) {
	original := jmap.MethodCall{
		Name:   "Email/get",
		Args:   json.RawMessage(`{"accountId":"u1","ids":["m1"]}`),
		CallID: "call-42",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Confirm it is a JSON array.
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("expected JSON array, got error: %v", err)
	}
	if len(raw) != 3 {
		t.Fatalf("expected 3-element array, got %d", len(raw))
	}

	var decoded jmap.MethodCall
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Name != original.Name {
		t.Errorf("Name: got %q, want %q", decoded.Name, original.Name)
	}
	if decoded.CallID != original.CallID {
		t.Errorf("CallID: got %q, want %q", decoded.CallID, original.CallID)
	}
	if string(decoded.Args) != string(original.Args) {
		t.Errorf("Args: got %s, want %s", decoded.Args, original.Args)
	}
}

// TestServeAPIUnknownCapability verifies that an unknown capability in the
// using array causes a 400 with type "unknownCapability".
func TestServeAPIUnknownCapability(t *testing.T) {
	h := newTestHandler()

	reqBody, _ := json.Marshal(jmap.Request{
		Using: []string{jmap.CapabilityCore, "urn:example:unknown"},
		MethodCalls: []jmap.MethodCall{
			{Name: "Email/get", Args: json.RawMessage(`{"accountId":"u1","ids":[]}`), CallID: "c1"},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/jmap/api", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeAPI(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var errObj struct {
		Type string `json:"type"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&errObj); err != nil {
		t.Fatalf("cannot decode error body: %v", err)
	}
	if errObj.Type != jmap.ErrUnknownCapability {
		t.Errorf("expected type %q, got %q", jmap.ErrUnknownCapability, errObj.Type)
	}
}

// TestServeAPIRequestTooLarge verifies that sending more than 16 method calls
// causes a 400 with type "requestTooLarge".
func TestServeAPIRequestTooLarge(t *testing.T) {
	h := newTestHandler()

	calls := make([]jmap.MethodCall, 17)
	for i := range calls {
		calls[i] = jmap.MethodCall{Name: "Email/get", Args: json.RawMessage(`{"accountId":"u1","ids":[]}`), CallID: "c"}
	}

	reqBody, _ := json.Marshal(jmap.Request{
		Using:       []string{jmap.CapabilityCore, jmap.CapabilityMail},
		MethodCalls: calls,
	})

	req := httptest.NewRequest(http.MethodPost, "/jmap/api", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeAPI(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var errObj struct {
		Type string `json:"type"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&errObj); err != nil {
		t.Fatalf("cannot decode error body: %v", err)
	}
	if errObj.Type != jmap.ErrRequestTooLarge {
		t.Errorf("expected type %q, got %q", jmap.ErrRequestTooLarge, errObj.Type)
	}
}
