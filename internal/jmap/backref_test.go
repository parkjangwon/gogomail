package jmap

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBackRefWildcard verifies that a "#ids" back-reference with a wildcard
// path (/list/*/id) collects the "id" field from every element in the array.
func TestBackRefWildcard(t *testing.T) {
	prev := map[string]json.RawMessage{
		"c0": json.RawMessage(`{"list":[{"id":"m1"},{"id":"m2"}]}`),
	}
	args := json.RawMessage(`{"accountId":"u1","#ids":{"resultOf":"c0","name":"Email/get","path":"/list/*/id"}}`)

	got, err := resolveBackRefs(args, prev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("cannot unmarshal result: %v", err)
	}

	// "#ids" should be gone; "ids" should be present.
	if _, ok := result["#ids"]; ok {
		t.Error("key '#ids' must be removed from result")
	}
	idsRaw, ok := result["ids"]
	if !ok {
		t.Fatal("key 'ids' missing from result")
	}

	var ids []string
	if err := json.Unmarshal(idsRaw, &ids); err != nil {
		t.Fatalf("cannot unmarshal 'ids': %v", err)
	}
	if len(ids) != 2 || ids[0] != "m1" || ids[1] != "m2" {
		t.Errorf("expected [m1 m2], got %v", ids)
	}

	// accountId must be preserved.
	if _, ok := result["accountId"]; !ok {
		t.Error("accountId must be preserved")
	}
}

// TestBackRefIndex verifies that a numeric path segment (/list/0/id) correctly
// selects a single element by zero-based index.
func TestBackRefIndex(t *testing.T) {
	prev := map[string]json.RawMessage{
		"c1": json.RawMessage(`{"list":[{"id":"first"},{"id":"second"}]}`),
	}
	args := json.RawMessage(`{"#targetId":{"resultOf":"c1","name":"Email/get","path":"/list/0/id"}}`)

	got, err := resolveBackRefs(args, prev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("cannot unmarshal result: %v", err)
	}

	raw, ok := result["targetId"]
	if !ok {
		t.Fatal("key 'targetId' missing from result")
	}

	var val string
	if err := json.Unmarshal(raw, &val); err != nil {
		t.Fatalf("cannot unmarshal 'targetId': %v", err)
	}
	if val != "first" {
		t.Errorf("expected 'first', got %q", val)
	}
}

// TestBackRefMissingRef verifies that referencing a callID not present in
// prevResults returns an error whose message contains "invalidResultReference".
func TestBackRefMissingRef(t *testing.T) {
	prev := map[string]json.RawMessage{} // empty — no prior results
	args := json.RawMessage(`{"#ids":{"resultOf":"nonexistent","name":"Email/get","path":"/list/*/id"}}`)

	_, err := resolveBackRefs(args, prev)
	if err == nil {
		t.Fatal("expected error for missing callID, got nil")
	}
	if !strings.Contains(err.Error(), "invalidResultReference") {
		t.Errorf("error must contain 'invalidResultReference', got: %v", err)
	}
}

// TestBackRefNoHashKeys verifies that args without any "#"-prefixed keys are
// returned completely unchanged (both structurally and byte-for-byte logically).
func TestBackRefNoHashKeys(t *testing.T) {
	prev := map[string]json.RawMessage{
		"c0": json.RawMessage(`{"list":[{"id":"m1"}]}`),
	}
	args := json.RawMessage(`{"accountId":"u1","ids":["m1","m2"]}`)

	got, err := resolveBackRefs(args, prev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Decode both to compare semantically (byte order may differ).
	var orig, resolved map[string]json.RawMessage
	if err := json.Unmarshal(args, &orig); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got, &resolved); err != nil {
		t.Fatalf("cannot unmarshal result: %v", err)
	}

	for k, v := range orig {
		rv, ok := resolved[k]
		if !ok {
			t.Errorf("key %q missing from result", k)
			continue
		}
		if string(v) != string(rv) {
			t.Errorf("key %q: expected %s, got %s", k, v, rv)
		}
	}
	if len(resolved) != len(orig) {
		t.Errorf("result has %d keys, want %d", len(resolved), len(orig))
	}
}
