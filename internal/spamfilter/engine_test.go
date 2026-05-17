package spamfilter

import (
	"testing"

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
