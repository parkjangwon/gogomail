package jmap_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gogomail/gogomail/internal/jmap"
)

func newTestHandler() *jmap.Handler {
	return jmap.NewHandler(func(_ context.Context, userID, accountID string) (*jmap.Session, error) {
		return jmap.BuildSession(userID, accountID, "https://mail.example.com"), nil
	})
}

// TestServeSessionReturnsJSON verifies that GET /.well-known/jmap returns
// HTTP 200 with a valid JSON Session object.
func TestServeSessionReturnsJSON(t *testing.T) {
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/jmap?u=alice@example.com", nil)
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

// TestServeAPIEmailGet verifies that Email/get returns a valid EmailGetResponse.
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
	if mr.Name != "Email/get" {
		t.Errorf("expected method name 'Email/get', got %q", mr.Name)
	}

	var getResp jmap.EmailGetResponse
	if err := json.Unmarshal(mr.Result, &getResp); err != nil {
		t.Fatalf("cannot decode EmailGetResponse: %v", err)
	}
	if getResp.List == nil {
		t.Error("List must not be nil")
	}
	if getResp.NotFound == nil {
		t.Error("NotFound must not be nil")
	}
	if getResp.State == "" {
		t.Error("State must not be empty")
	}
}

// TestServeAPIEmailQuery verifies that Email/query returns a valid EmailQueryResponse.
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

	var queryResp jmap.EmailQueryResponse
	if err := json.Unmarshal(mr.Result, &queryResp); err != nil {
		t.Fatalf("cannot decode EmailQueryResponse: %v", err)
	}
	if queryResp.IDs == nil {
		t.Error("IDs must not be nil")
	}
	if queryResp.QueryState == "" {
		t.Error("QueryState must not be empty")
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
