package jmap

import (
	"context"
	"encoding/json"
	"testing"
)

func TestThreadGetNilRepoReturnsServerFail(t *testing.T) {
	m := &threadGetMethod{deps: Deps{}}
	result, err := m.Call(context.Background(), "u1", json.RawMessage(`{"accountId":"u1","ids":["t1"]}`))
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]string
	json.Unmarshal(result, &resp)
	if resp["type"] != ErrServerFail {
		t.Errorf("want serverFail, got %q", resp["type"])
	}
}

func TestThreadChangesNilRepoReturnsServerFail(t *testing.T) {
	m := &threadChangesMethod{deps: Deps{}}
	result, err := m.Call(context.Background(), "u1", json.RawMessage(`{"accountId":"u1","sinceState":"0"}`))
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]string
	json.Unmarshal(result, &resp)
	if resp["type"] != ErrServerFail {
		t.Errorf("want serverFail, got %q", resp["type"])
	}
}

func TestThreadGetResponseStructure(t *testing.T) {
	// Verify the response JSON shape matches RFC 8621 §4.
	resp := ThreadGetResponse{
		AccountID: "u1",
		State:     "42",
		List:      []Thread{{ID: "t1", EmailIDs: []string{"m1", "m2"}}},
		NotFound:  []string{"t2"},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	json.Unmarshal(b, &out)
	if out["accountId"] != "u1" {
		t.Errorf("accountId")
	}
	list, _ := out["list"].([]any)
	if len(list) != 1 {
		t.Errorf("list length want 1, got %d", len(list))
	}
	th, _ := list[0].(map[string]any)
	emails, _ := th["emailIds"].([]any)
	if len(emails) != 2 {
		t.Errorf("emailIds length want 2, got %d", len(emails))
	}
}
