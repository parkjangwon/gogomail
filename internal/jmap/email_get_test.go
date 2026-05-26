package jmap

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/maildb"
)

func TestEmailGetNilRepoReturnsServerFail(t *testing.T) {
	m := &emailGetMethod{deps: Deps{}}
	result, err := m.Call(context.Background(), "u1", json.RawMessage(`{"accountId":"u1","ids":["m1"]}`))
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]string
	json.Unmarshal(result, &resp)
	if resp["type"] != ErrServerFail {
		t.Errorf("want serverFail, got %q", resp["type"])
	}
}

func TestEmailGetTooManyIDsReturnsRequestTooLarge(t *testing.T) {
	m := &emailGetMethod{deps: Deps{}}
	// maxObjectsInGet is 500; send 501
	ids := make([]string, 501)
	for i := range ids {
		ids[i] = "m" + string(rune('0'+i%10))
	}
	b, _ := json.Marshal(map[string]any{"accountId": "u1", "ids": ids})
	result, err := m.Call(context.Background(), "u1", json.RawMessage(b))
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]string
	json.Unmarshal(result, &resp)
	if resp["type"] != "requestTooLarge" {
		t.Errorf("want requestTooLarge, got %q", resp["type"])
	}
}

func TestMessageDetailToJMAPPropertyFiltering(t *testing.T) {
	d := maildb.MessageDetail{
		ID:         "m1",
		Subject:    "Hello",
		FromAddr:   "alice@example.com",
		Size:       1024,
		ReceivedAt: time.Now(),
		Flags:      json.RawMessage(`{"read":true}`),
	}
	// Request only "subject" and "keywords"
	email := messageDetailToJMAP(d, []string{"subject", "keywords"})
	if email.ID != "m1" {
		t.Errorf("id always present, got %q", email.ID)
	}
	if email.Subject != "Hello" {
		t.Errorf("subject want Hello, got %q", email.Subject)
	}
	if email.Size != 0 {
		t.Errorf("size should be omitted (not in props), got %d", email.Size)
	}
	if !email.Keywords["$seen"] {
		t.Errorf("$seen keyword should be set (read flag true)")
	}
}

func TestFlagsToKeywords(t *testing.T) {
	cases := []struct {
		flags                        string
		wantSeen, wantFlagged, wantDraft bool
	}{
		{`{"read":true,"starred":true}`, true, true, false},
		{`{"read":false}`, false, false, false},
		{`{"draft":true}`, false, false, true},
		{`{}`, false, false, false},
	}
	for _, c := range cases {
		kw := flagsToKeywords(json.RawMessage(c.flags))
		if kw["$seen"] != c.wantSeen {
			t.Errorf("flags %s: $seen want %v, got %v", c.flags, c.wantSeen, kw["$seen"])
		}
		if kw["$flagged"] != c.wantFlagged {
			t.Errorf("flags %s: $flagged want %v, got %v", c.flags, c.wantFlagged, kw["$flagged"])
		}
		if kw["$draft"] != c.wantDraft {
			t.Errorf("flags %s: $draft want %v, got %v", c.flags, c.wantDraft, kw["$draft"])
		}
	}
}
