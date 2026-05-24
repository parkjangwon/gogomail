package spamfilter

import (
	"context"
	"encoding/json"
	"net"
	"strings"
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

func TestEngineCatchesConfusableCredentialLure(t *testing.T) {
	policy := DefaultPolicy()
	policy.SpamThreshold = 2
	decision := NewEngine().Evaluate(policy, smtpd.Event{
		Parsed: message.ParsedMessage{
			Subject: "pаsswоrd еxpired", // Cyrillic a/o/e in an otherwise English lure.
		},
	})
	if decision.Action != ActionQuarantine || !contains(decision.Rules, "SUSPICIOUS_SUBJECT") {
		t.Fatalf("decision = %+v, want obfuscated suspicious subject quarantine", decision)
	}
}

func TestEngineScoresDisguisedCredentialFormLink(t *testing.T) {
	policy := DefaultPolicy()
	decision := NewEngine().Evaluate(policy, smtpd.Event{
		Parsed: message.ParsedMessage{
			HTMLBody: `<p>Please sign in.</p><a href="https://evil.example/login">https://mail.example.com/login</a><form><input type="password"></form>`,
		},
	})
	if decision.Action != ActionQuarantine {
		t.Fatalf("decision = %+v, want quarantine for URL mismatch and credential form", decision)
	}
	if !contains(decision.Rules, "PACK:gogomail-core-url:link-text-mismatch") || !contains(decision.Rules, "PACK:gogomail-core-url:credential-form") {
		t.Fatalf("rules = %#v, want URL mismatch and credential form rules", decision.Rules)
	}
}

func TestEngineScoresPunycodeAndRawIPURLs(t *testing.T) {
	policy := DefaultPolicy()
	decision := NewEngine().Evaluate(policy, smtpd.Event{
		Parsed: message.ParsedMessage{
			TextBody: "Open https://xn--pple-43d.example/login then http://203.0.113.4/reset",
		},
		Authentication: smtpd.AuthenticationResults{
			SPF:   smtpd.AuthCheckResult{Result: smtpd.AuthResultPass},
			DKIM:  smtpd.AuthCheckResult{Result: smtpd.AuthResultPass},
			DMARC: smtpd.AuthCheckResult{Result: smtpd.AuthResultPass},
		},
	})
	if decision.Action != ActionAccept || decision.Score < 4 {
		t.Fatalf("decision = %+v, want scored but below-threshold URL risk", decision)
	}
	if !contains(decision.Rules, "PACK:gogomail-core-url:punycode-url") || !contains(decision.Rules, "PACK:gogomail-core-url:raw-ip-url") {
		t.Fatalf("rules = %#v, want punycode and raw IP URL rules", decision.Rules)
	}
}

func TestEngineScoresEnvelopeFromMismatch(t *testing.T) {
	policy := DefaultPolicy()
	policy.SpamThreshold = 2
	decision := NewEngine().Evaluate(policy, smtpd.Event{
		EnvelopeFrom: "bounce@evil.example",
		Parsed: message.ParsedMessage{
			From: message.Address{Address: "security@example.com"},
		},
	})
	if decision.Action != ActionQuarantine || !contains(decision.Rules, "PACK:gogomail-core-sender:from-envelope-mismatch") {
		t.Fatalf("decision = %+v, want sender mismatch quarantine", decision)
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

func TestPolicyKeepsFilterPacksTenantScopedAndNormalized(t *testing.T) {
	policy := NormalizePolicy(Policy{
		FilterPacks: FilterPackBundle{
			EnabledPackIDs: []string{"CUSTOM-ALERTS", "bad id", "custom-alerts"},
			CustomPacks: []FilterPack{
				{
					ID:       "custom-alerts",
					Name:     "Tenant alerts",
					Category: "phishing",
					Source:   "system",
					Rules: []FilterRuleDefinition{
						{ID: "Wire-Transfer", Type: RuleTypePhrase, Target: RuleTargetSubjectBody, Patterns: []string{"wire transfer", "bad\r\nheader"}, Score: 4, Enabled: true},
					},
				},
				{ID: "gogomail-core-shadow", Rules: []FilterRuleDefinition{{ID: "x", Type: RuleTypeBulkRecipient, Score: 1}}},
			},
		},
	})
	if got := policy.FilterPacks.EnabledPackIDs; len(got) != 1 || got[0] != "custom-alerts" {
		t.Fatalf("enabled pack ids = %#v, want normalized custom-alerts only", got)
	}
	if len(policy.FilterPacks.CustomPacks) != 1 {
		t.Fatalf("custom packs = %#v, want one tenant custom pack", policy.FilterPacks.CustomPacks)
	}
	pack := policy.FilterPacks.CustomPacks[0]
	if pack.Source != "custom" || pack.Rules[0].Patterns[0] != "wire transfer" {
		t.Fatalf("pack = %#v, want sanitized custom pack", pack)
	}
}

func TestEngineScoresCustomFilterPackPhrase(t *testing.T) {
	policy := DefaultPolicy()
	policy.SpamThreshold = 4
	policy.FilterPacks = FilterPackBundle{
		EnabledPackIDs: []string{"tenant-wire-alert"},
		CustomPacks: []FilterPack{{
			ID:       "tenant-wire-alert",
			Name:     "Tenant wire alert",
			Category: "phishing",
			Rules: []FilterRuleDefinition{{
				ID:       "wire-transfer",
				Type:     RuleTypePhrase,
				Target:   RuleTargetSubjectBody,
				Patterns: []string{"wire transfer approval"},
				Score:    4,
				Enabled:  true,
			}},
		}},
	}

	decision := NewEngine().Evaluate(policy, smtpd.Event{
		Parsed: message.ParsedMessage{Subject: "Wire transfer approval needed"},
	})
	if decision.Action != ActionQuarantine || !contains(decision.Rules, "PACK:tenant-wire-alert:wire-transfer") {
		t.Fatalf("decision = %+v, want custom pack quarantine", decision)
	}
}

func TestBuiltinFilterPackCatalogMarksEnabledSystemPacks(t *testing.T) {
	catalog := FilterPackCatalog(DefaultPolicy().FilterPacks)
	if len(catalog) < 6 {
		t.Fatalf("catalog length = %d, want built-in packs", len(catalog))
	}
	for _, pack := range catalog {
		if strings.HasPrefix(pack.ID, "gogomail-core-") && !pack.Enabled {
			t.Fatalf("builtin pack %s disabled in default catalog", pack.ID)
		}
	}
}

func TestEngineScoresCustomURLHostRule(t *testing.T) {
	policy := DefaultPolicy()
	policy.SpamThreshold = 3
	policy.FilterPacks = FilterPackBundle{
		EnabledPackIDs: []string{"tenant-bad-hosts"},
		CustomPacks: []FilterPack{{
			ID:       "tenant-bad-hosts",
			Name:     "Bad hosts",
			Category: "phishing",
			Rules: []FilterRuleDefinition{{
				ID:       "bad-link-host",
				Type:     RuleTypeURLHost,
				Patterns: []string{"evil.example"},
				Score:    3,
				Enabled:  true,
			}},
		}},
	}

	decision := NewEngine().Evaluate(policy, smtpd.Event{
		Parsed: message.ParsedMessage{TextBody: "See https://login.evil.example/reset"},
	})
	if decision.Action != ActionQuarantine || !contains(decision.Rules, "PACK:tenant-bad-hosts:bad-link-host") {
		t.Fatalf("decision = %+v, want custom URL host quarantine", decision)
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

func BenchmarkEngineEvaluateURLAndAnomalyPacks(b *testing.B) {
	policy := DefaultPolicy()
	event := smtpd.Event{
		EnvelopeFrom: "bounce@evil.example",
		RemoteAddr:   "203.0.113.10:25",
		Recipients:   []string{"user@example.net"},
		Parsed: message.ParsedMessage{
			From:     message.Address{Address: "security@example.com"},
			Subject:  "pаsswоrd еxpired",
			TextBody: "Open https://xn--pple-43d.example/login then http://203.0.113.4/reset",
			HTMLBody: `<p>Please sign in.</p><a href="https://evil.example/login">https://mail.example.com/login</a><form><input type="password"></form>`,
		},
	}
	engine := NewEngine()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = engine.Evaluate(policy, event)
	}
}
