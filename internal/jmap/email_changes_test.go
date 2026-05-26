package jmap

import (
	"context"
	"encoding/json"
	"testing"
)

func TestEmailChangesNilRepoReturnsServerFail(t *testing.T) {
	m := &emailChangesMethod{deps: Deps{}}
	result, _ := m.Call(context.Background(), "u1", json.RawMessage(`{"accountId":"u1","sinceState":"0"}`))
	var resp map[string]string
	json.Unmarshal(result, &resp)
	if resp["type"] != ErrServerFail {
		t.Errorf("want serverFail, got %q", resp["type"])
	}
}

func TestEmailImportNilRepoReturnsServerFail(t *testing.T) {
	m := &emailImportMethod{deps: Deps{}}
	result, _ := m.Call(context.Background(), "u1", json.RawMessage(`{"accountId":"u1","emails":{}}`))
	var resp map[string]string
	json.Unmarshal(result, &resp)
	if resp["type"] != ErrServerFail {
		t.Errorf("want serverFail, got %q", resp["type"])
	}
}

func TestParseMIMEToJMAPSimplePlainText(t *testing.T) {
	raw := []byte("Subject: Hello\r\nFrom: alice@example.com\r\nTo: bob@example.com\r\n\r\nThis is the body.\r\n")
	email, err := parseMIMEToJMAP(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if email.Subject != "Hello" {
		t.Errorf("subject want Hello, got %q", email.Subject)
	}
	if len(email.From) == 0 || email.From[0].Email != "alice@example.com" {
		t.Errorf("from not parsed correctly: %+v", email.From)
	}
}

func TestEmailCopyReturnsNotCreated(t *testing.T) {
	m := &emailCopyMethod{deps: Deps{}}
	result, _ := m.Call(context.Background(), "u1", json.RawMessage(`{
		"accountId":"u1",
		"fromAccountId":"u1",
		"create":{"c1":{"mailboxIds":{"f1":true}}}
	}`))
	var resp SetResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := resp.NotCreated["c1"]; !ok {
		t.Errorf("expected c1 in notCreated, got: %+v", resp)
	}
}
