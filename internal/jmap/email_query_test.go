package jmap

import (
	"context"
	"encoding/json"
	"testing"
)

func TestEmailQueryNilRepoReturnsServerFail(t *testing.T) {
	m := &emailQueryMethod{deps: Deps{}}
	result, _ := m.Call(context.Background(), "u1", json.RawMessage(`{"accountId":"u1"}`))
	var resp map[string]string
	json.Unmarshal(result, &resp)
	if resp["type"] != ErrServerFail {
		t.Errorf("want serverFail, got %q", resp["type"])
	}
}

func TestEmailQueryChangesReturnsCannotCalculate(t *testing.T) {
	m := &emailQueryChangesMethod{deps: Deps{}}
	result, _ := m.Call(context.Background(), "u1", json.RawMessage(`{"accountId":"u1","sinceQueryState":"0"}`))
	var resp map[string]string
	json.Unmarshal(result, &resp)
	if resp["type"] != "cannotCalculateChanges" {
		t.Errorf("want cannotCalculateChanges, got %q", resp["type"])
	}
}

func TestEmailQueryArgsDecoding(t *testing.T) {
	raw := json.RawMessage(`{
		"accountId":"u1",
		"filter":{"inMailbox":"f1","text":"hello"},
		"sort":[{"property":"receivedAt","isAscending":false}],
		"position":10,
		"limit":50
	}`)
	var args EmailQueryArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		t.Fatal(err)
	}
	if args.Filter.InMailbox != "f1" {
		t.Errorf("inMailbox want f1, got %q", args.Filter.InMailbox)
	}
	if args.Filter.Text != "hello" {
		t.Errorf("text want hello, got %q", args.Filter.Text)
	}
	if len(args.Sort) == 0 || args.Sort[0].Property != "receivedAt" {
		t.Errorf("sort not decoded")
	}
	if args.Position != 10 || args.Limit != 50 {
		t.Errorf("position/limit not decoded")
	}
}

func TestEmailQueryResponseShape(t *testing.T) {
	resp := EmailQueryResponse{
		AccountID:           "u1",
		QueryState:          "42",
		CanCalculateChanges: false,
		Position:            0,
		IDs:                 []string{"m1", "m2"},
		Total:               2,
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	json.Unmarshal(b, &out)
	if out["canCalculateChanges"] != false {
		t.Errorf("canCalculateChanges should be false")
	}
	ids, _ := out["ids"].([]any)
	if len(ids) != 2 {
		t.Errorf("ids length want 2, got %d", len(ids))
	}
}
