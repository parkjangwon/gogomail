package jmap

import (
	"context"
	"encoding/json"
	"testing"
)

func TestEmailSetNilRepoReturnsServerFail(t *testing.T) {
	m := &emailSetMethod{deps: Deps{}}
	result, err := m.Call(context.Background(), "u1", json.RawMessage(`{"accountId":"u1"}`))
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]string
	json.Unmarshal(result, &resp)
	if resp["type"] != ErrServerFail {
		t.Errorf("want serverFail, got %q", resp["type"])
	}
}

func TestEmailSetInvalidArguments(t *testing.T) {
	m := &emailSetMethod{deps: Deps{}}
	result, _ := m.Call(context.Background(), "u1", json.RawMessage(`not valid json`))
	var resp map[string]string
	json.Unmarshal(result, &resp)
	if resp["type"] != ErrInvalidArguments {
		t.Errorf("want invalidArguments, got %q", resp["type"])
	}
}

func TestAsBool(t *testing.T) {
	if !asBool(true) {
		t.Error("true should be truthy")
	}
	if asBool(false) {
		t.Error("false should be falsy")
	}
	if asBool("yes") {
		t.Error("string should be falsy")
	}
	if asBool(nil) {
		t.Error("nil should be falsy")
	}
}

func TestEmailSetArgsDecoding(t *testing.T) {
	raw := json.RawMessage(`{
		"accountId":"u1",
		"destroy":["m1","m2"],
		"update":{"m3":{"keywords/$seen":true}},
		"create":{"c1":{"mailboxIds":{},"subject":"Hello"}}
	}`)
	var args EmailSetArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		t.Fatal(err)
	}
	if len(args.Destroy) != 2 {
		t.Errorf("destroy want 2, got %d", len(args.Destroy))
	}
	if _, ok := args.Update["m3"]; !ok {
		t.Error("update m3 should be present")
	}
	if _, ok := args.Create["c1"]; !ok {
		t.Error("create c1 should be present")
	}
}

func TestEmailSetResponseShape(t *testing.T) {
	resp := SetResponse{
		AccountID:    "u1",
		OldState:     "1",
		NewState:     "2",
		Destroyed:    []string{"m1"},
		Created:      map[string]any{"c1": map[string]any{"id": "new1"}},
		Updated:      map[string]any{},
		NotCreated:   map[string]SetError{},
		NotUpdated:   map[string]SetError{},
		NotDestroyed: map[string]SetError{},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	json.Unmarshal(b, &out)
	dest, _ := out["destroyed"].([]any)
	if len(dest) != 1 || dest[0] != "m1" {
		t.Errorf("destroyed want [m1], got %v", dest)
	}
}
