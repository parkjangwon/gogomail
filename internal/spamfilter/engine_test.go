package spamfilter

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/gogomail/gogomail/internal/dnsbl"
	"github.com/gogomail/gogomail/internal/message"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

func TestEngineRejectsBlockedSenderWhenQuarantineDisabled(t *testing.T) {
	policy := DefaultPolicy()
	policy.BlockedSenders = []string{"@evil.example"}
	policy.QuarantineEnabled = false

	decision := NewEngine().Evaluate(policy, smtpd.Event{
		EnvelopeFrom: "bad@evil.example",
		Parsed:       message.ParsedMessage{From: message.Address{Address: "bad@evil.example"}},
	})
	if decision.Action != ActionReject {
		t.Fatalf("action = %s, want reject", decision.Action)
	}
}

func TestEngineQuarantinesDangerousAttachment(t *testing.T) {
	policy := DefaultPolicy()
	decision := NewEngine().Evaluate(policy, smtpd.Event{
		Parsed: message.ParsedMessage{
			HasAttachment: true,
			Attachments:   []message.Attachment{{Filename: "invoice.exe"}},
		},
	})
	if decision.Action != ActionQuarantine || decision.Score < 10 {
		t.Fatalf("decision = %+v, want quarantine with high score", decision)
	}
}

func TestEngineAllowlistBypassesSpamSignals(t *testing.T) {
	policy := DefaultPolicy()
	policy.AllowedSenders = []string{"trusted@example.com"}

	decision := NewEngine().Evaluate(policy, smtpd.Event{
		Parsed: message.ParsedMessage{
			From:          message.Address{Address: "trusted@example.com"},
			Subject:       "urgent password expired",
			HasAttachment: true,
			Attachments:   []message.Attachment{{Filename: "invoice.exe"}},
		},
	})
	if decision.Action != ActionAccept || decision.Score != 0 {
		t.Fatalf("decision = %+v, want allowlisted accept", decision)
	}
}

func TestEngineQuarantinesStrictAuthenticationFailure(t *testing.T) {
	policy := DefaultPolicy()
	decision := NewEngine().Evaluate(policy, smtpd.Event{
		Authentication: smtpd.AuthenticationResults{
			SPF:   smtpd.AuthCheckResult{Result: smtpd.AuthResultFail},
			DKIM:  smtpd.AuthCheckResult{Result: smtpd.AuthResultNone},
			DMARC: smtpd.AuthCheckResult{Result: smtpd.AuthResultFail},
		},
	})
	if decision.Action != ActionQuarantine || decision.Score < float64(policy.SpamThreshold) {
		t.Fatalf("decision = %+v, want strict auth quarantine", decision)
	}
}

func TestEngineQuarantinesBulkUnauthenticatedInbound(t *testing.T) {
	policy := DefaultPolicy()
	policy.BulkRecipientLimit = 2
	decision := NewEngine().Evaluate(policy, smtpd.Event{
		RemoteAddr:   "203.0.113.10:25",
		Recipients:   []string{"a@example.net", "b@example.net", "c@example.net"},
		EnvelopeFrom: "sender@example.org",
	})
	if decision.Action != ActionQuarantine {
		t.Fatalf("decision = %+v, want quarantine", decision)
	}
}

func TestEngineScoresSuspiciousBody(t *testing.T) {
	policy := DefaultPolicy()
	policy.SpamThreshold = 2
	decision := NewEngine().Evaluate(policy, smtpd.Event{
		Parsed: message.ParsedMessage{TextBody: "Please verify your account and reset your password now."},
	})
	if decision.Action != ActionQuarantine {
		t.Fatalf("decision = %+v, want quarantine", decision)
	}
}

func TestPolicyNormalizesRBLZonesAndBulkLimit(t *testing.T) {
	policy := NormalizePolicy(Policy{
		RBLZones:           []string{" Zen.SpamHaus.Org. ", "bad zone", "zen.spamhaus.org", "x@y"},
		BulkRecipientLimit: -1,
	})
	if len(policy.RBLZones) != 1 || policy.RBLZones[0] != "zen.spamhaus.org" {
		t.Fatalf("rbl zones = %#v, want normalized single zone", policy.RBLZones)
	}
	if policy.BulkRecipientLimit != 50 {
		t.Fatalf("bulk recipient limit = %d, want default 50", policy.BulkRecipientLimit)
	}
}

func TestHookRejectsRBLListedRemote(t *testing.T) {
	policy := DefaultPolicy()
	policy.RBLCheckEnabled = true
	policy.RBLRejectEnabled = true
	policy.RBLZones = []string{"rbl.test"}
	raw, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}

	hook := Hook(Options{
		Resolver:    staticPolicyResolver(raw),
		RBLResolver: listedRBLResolver{},
	})
	err = hook(context.Background(), smtpd.Event{
		Stage:      smtpd.StageDedupChecked,
		RemoteAddr: "203.0.113.10:25",
		Mailbox:    smtpd.Mailbox{UserID: "u1", DomainID: "d1", CompanyID: "c1", Address: "user@example.net"},
	})
	if err == nil {
		t.Fatal("hook error = nil, want RBL rejection")
	}
}

func TestHookRBLFailOpen(t *testing.T) {
	policy := DefaultPolicy()
	policy.RBLCheckEnabled = true
	policy.RBLRejectEnabled = true
	policy.RBLZones = []string{"rbl.test"}
	raw, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}

	hook := Hook(Options{
		Resolver:    staticPolicyResolver(raw),
		RBLResolver: failingRBLResolver{},
	})
	if err := hook(context.Background(), smtpd.Event{
		Stage:      smtpd.StageDedupChecked,
		RemoteAddr: "203.0.113.10:25",
		Mailbox:    smtpd.Mailbox{UserID: "u1", DomainID: "d1", CompanyID: "c1", Address: "user@example.net"},
	}); err != nil {
		t.Fatalf("hook error = %v, want fail-open nil", err)
	}
}

type staticPolicyResolver json.RawMessage

func (r staticPolicyResolver) Resolve(context.Context, string, string, string, string) (json.RawMessage, error) {
	return json.RawMessage(r), nil
}

type listedRBLResolver struct{}

func (listedRBLResolver) LookupHost(string) ([]string, error) {
	return []string{"127.0.0.2"}, nil
}

type failingRBLResolver struct{}

func (failingRBLResolver) LookupHost(string) ([]string, error) {
	return nil, &net.DNSError{Err: "timeout"}
}

var _ dnsbl.Resolver = listedRBLResolver{}
