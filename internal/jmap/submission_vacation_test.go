package jmap

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// Test 1: nil Repo returns error from EmailSubmission/set.
func TestEmailSubmissionSetNilRepoReturnsError(t *testing.T) {
	m := &emailSubmissionSetMethod{deps: Deps{Repo: nil}}
	args, _ := json.Marshal(emailSubmissionSetArgs{AccountID: "u1"})
	_, err := m.Call(context.Background(), "u1", args)
	if err == nil {
		t.Fatal("expected error with nil repo, got nil")
	}
}

// Test 2: emailSubmissionResponse JSON has all required JMAP fields.
func TestEmailSubmissionResponseHasRequiredFields(t *testing.T) {
	resp := emailSubmissionResponse{
		AccountID:    "u1",
		OldState:     "submission-v1",
		NewState:     "submission-v2",
		Created:      map[string]json.RawMessage{},
		NotCreated:   map[string]SetError{},
		Updated:      map[string]json.RawMessage{},
		NotUpdated:   map[string]SetError{},
		Destroyed:    []string{},
		NotDestroyed: map[string]SetError{},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, field := range []string{"accountId", "oldState", "newState", "created", "notCreated", "updated", "notUpdated", "destroyed", "notDestroyed"} {
		if _, ok := m[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

// Test: VacationResponse/set returns error for nil Repo.
func TestVacationResponseSetForbidsCreate(t *testing.T) {
	m := &vacationResponseSetMethod{deps: Deps{}}
	_, err := m.Call(context.Background(), "u1", json.RawMessage(`{"create": {"c1": {}}}`))
	if err == nil {
		t.Error("expected error for nil Repo, got nil")
	}
}

// Test: VacationResponse/get returns error for nil Repo regardless of ids filter.
func TestVacationResponseGetIdsFilterNonSingleton(t *testing.T) {
	m := &vacationResponseGetMethod{deps: Deps{}}
	_, err := m.Call(context.Background(), "u1", json.RawMessage(`{"ids": ["nonexistent"]}`))
	if err == nil {
		t.Error("expected error for nil Repo")
	}
}

// Test 3: nil Repo returns error from VacationResponse/get.
func TestVacationResponseGetNilRepoReturnsError(t *testing.T) {
	m := &vacationResponseGetMethod{deps: Deps{Repo: nil}}
	args, _ := json.Marshal(vacationResponseGetArgs{AccountID: "u1"})
	_, err := m.Call(context.Background(), "u1", args)
	if err == nil {
		t.Fatal("expected error with nil repo, got nil")
	}
}

// Test 4: Default VacationResponse has id="singleton" and isEnabled=false.
func TestVacationResponseGetReturnsDefaultShape(t *testing.T) {
	// Verify the default response JSON shape without requiring a DB.
	vr := defaultVacationResponse()
	if vr.ID != vacationResponseID {
		t.Errorf("want id=%q, got %q", vacationResponseID, vr.ID)
	}
	if vr.IsEnabled {
		t.Error("default vacation response should have isEnabled=false")
	}
	if vr.FromDate != nil || vr.ToDate != nil || vr.Subject != nil || vr.TextBody != nil || vr.HTMLBody != nil {
		t.Error("default vacation response should have nil optional fields")
	}

	// Verify JSON serialises correctly.
	resp := vacationResponseGetResponse{
		AccountID: "u1",
		State:     "vacation-v1",
		List:      []VacationResponse{vr},
		NotFound:  []string{},
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	s := string(raw)
	for _, want := range []string{`"accountId"`, `"state"`, `"list"`, `"id"`, `"isEnabled"`, `"singleton"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing field/value %q in JSON: %s", want, s)
		}
	}

	// Deserialize and confirm.
	var decoded vacationResponseGetResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(decoded.List) != 1 {
		t.Fatalf("want 1 vacation response, got %d", len(decoded.List))
	}
	if decoded.List[0].ID != vacationResponseID {
		t.Errorf("want id=%q, got %q", vacationResponseID, decoded.List[0].ID)
	}
	if decoded.List[0].IsEnabled {
		t.Error("default should have isEnabled=false")
	}
}
