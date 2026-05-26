package jmap

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/maildb"
)

func TestIdentityGetNilRepoReturnsError(t *testing.T) {
	m := &identityGetMethod{deps: Deps{Repo: nil}}
	args, _ := json.Marshal(identityGetArgs{AccountID: "u1"})
	_, err := m.Call(context.Background(), "u1", args)
	if err == nil {
		t.Fatal("expected error with nil repo, got nil")
	}
}

func TestIdentitySetNilRepoReturnsError(t *testing.T) {
	m := &identitySetMethod{deps: Deps{Repo: nil}}
	args, _ := json.Marshal(identitySetArgs{AccountID: "u1"})
	_, err := m.Call(context.Background(), "u1", args)
	if err == nil {
		t.Fatal("expected error with nil repo, got nil")
	}
}

func TestSearchSnippetGetNilRepoReturnsError(t *testing.T) {
	m := &searchSnippetGetMethod{deps: Deps{Repo: nil}}
	args, _ := json.Marshal(searchSnippetGetArgs{AccountID: "u1", EmailIDs: []string{"e1"}})
	_, err := m.Call(context.Background(), "u1", args)
	if err == nil {
		t.Fatal("expected error with nil repo, got nil")
	}
}

func TestSearchSnippetResponseShape(t *testing.T) {
	// Verify the JSON shape of searchSnippetGetResponse directly without a DB.
	resp := searchSnippetGetResponse{
		AccountID: "account-1",
		List: []SearchSnippet{
			{EmailID: "email-1", Subject: "Hello World", Preview: "This is a preview"},
		},
		NotFound: []string{"missing-1"},
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	s := string(raw)
	for _, want := range []string{`"accountId"`, `"list"`, `"notFound"`, `"emailId"`, `"subject"`, `"preview"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing field %q in JSON: %s", want, s)
		}
	}

	// Verify deserialization round-trip.
	var decoded searchSnippetGetResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.AccountID != "account-1" {
		t.Errorf("expected accountId account-1, got %q", decoded.AccountID)
	}
	if len(decoded.List) != 1 {
		t.Fatalf("expected 1 snippet, got %d", len(decoded.List))
	}
	if decoded.List[0].EmailID != "email-1" {
		t.Errorf("expected emailId email-1, got %q", decoded.List[0].EmailID)
	}
	if decoded.List[0].Subject != "Hello World" {
		t.Errorf("expected subject Hello World, got %q", decoded.List[0].Subject)
	}
	_ = maildb.MessageDetail{} // ensure maildb import is used in test binary
}
