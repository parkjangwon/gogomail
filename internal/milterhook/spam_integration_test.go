package milterhook

import (
	"context"
	"testing"

	"github.com/gogomail/gogomail/internal/message"
	"github.com/gogomail/gogomail/internal/milter"
	"github.com/gogomail/gogomail/internal/spam"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

type mockSpamFilter struct {
	score spam.SpamScore
	err   error
}

func (m *mockSpamFilter) Check(ctx context.Context, from string, to []string, body []byte) (spam.SpamScore, error) {
	if m.err != nil {
		return spam.SpamScore{}, m.err
	}
	return m.score, nil
}

func TestSpamHookAccept(t *testing.T) {
	filter := &mockSpamFilter{
		score: spam.SpamScore{Score: 2.0, Rules: []string{"BAYES_HAM"}},
	}
	engine := spam.NewDecisionEngine(spam.DefaultThresholds())
	cfg := SpamConfig{
		Enabled: true,
		Filter:  filter,
		Engine:  engine,
	}

	hook := SpamHook(cfg)
	parsed := message.ParsedMessage{
		Subject:   "test subject",
		TextBody:  "test message body",
	}
	event := smtpd.Event{
		Stage:      smtpd.StageAuthenticationChecked,
		Recipients: []string{"user@example.com"},
		Parsed:     parsed,
	}

	err := hook(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error for low spam score, got: %v", err)
	}
}

func TestSpamHookReject(t *testing.T) {
	filter := &mockSpamFilter{
		score: spam.SpamScore{Score: 20.0, Rules: []string{"BAYES_SPAM"}},
	}
	engine := spam.NewDecisionEngine(spam.DefaultThresholds())
	cfg := SpamConfig{
		Enabled: true,
		Filter:  filter,
		Engine:  engine,
	}

	hook := SpamHook(cfg)
	parsed := message.ParsedMessage{
		Subject:   "test subject",
		TextBody:  "test message body",
	}
	event := smtpd.Event{
		Stage:      smtpd.StageAuthenticationChecked,
		Recipients: []string{"user@example.com"},
		Parsed:     parsed,
	}

	err := hook(context.Background(), event)
	if err == nil {
		t.Fatal("expected error for high spam score")
	}
}

func TestSpamHookShadowMode(t *testing.T) {
	filter := &mockSpamFilter{
		score: spam.SpamScore{Score: 20.0, Rules: []string{"BAYES_SPAM"}},
	}
	engine := spam.NewDecisionEngine(spam.DefaultThresholds())
	cfg := SpamConfig{
		Enabled:    true,
		Filter:     filter,
		Engine:     engine,
		ShadowMode: true, // Shadow mode allows rejectable messages
	}

	hook := SpamHook(cfg)
	parsed := message.ParsedMessage{
		Subject:   "test subject",
		TextBody:  "test message body",
	}
	event := smtpd.Event{
		Stage:      smtpd.StageAuthenticationChecked,
		Recipients: []string{"user@example.com"},
		Parsed:     parsed,
	}

	err := hook(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error in shadow mode, got: %v", err)
	}
}

func TestSpamHookDisabled(t *testing.T) {
	cfg := SpamConfig{
		Enabled: false, // Disabled
	}

	hook := SpamHook(cfg)
	event := smtpd.Event{
		Stage: smtpd.StageAuthenticationChecked,
	}

	err := hook(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error when disabled, got: %v", err)
	}
}

func TestSpamVerdictHeaders(t *testing.T) {
	verdict := spam.Verdict{
		Action: spam.ActionReject,
		Score:  8.5,
		Reason: "spam detected",
		Tags:   []string{"BAYES_SPAM", "PHISHING"},
	}

	score := spam.SpamScore{
		Score: 8.5,
		Rules: []string{"BAYES_SPAM", "PHISHING"},
	}

	headers := SpamVerdictHeaders(verdict, score)

	if len(headers) == 0 {
		t.Fatal("headers should not be empty")
	}

	hasScore := false
	hasStatus := false
	for _, h := range headers {
		if _, ok := h["X-Spam-Score"]; ok && h["X-Spam-Score"] == "8.50" {
			hasScore = true
		}
		if _, ok := h["X-Spam-Status"]; ok && h["X-Spam-Status"] == "YES" {
			hasStatus = true
		}
	}

	if !hasScore {
		t.Fatal("X-Spam-Score header not found or incorrect")
	}
	if !hasStatus {
		t.Fatal("X-Spam-Status header not found or incorrect")
	}
}

func TestMilterSpamVerdict(t *testing.T) {
	tests := []struct {
		action   spam.Action
		expected milter.Action
	}{
		{spam.ActionAccept, milter.ActionAccept},
		{spam.ActionQuarantine, milter.ActionAccept}, // Quarantine handled by routing
		{spam.ActionTempfail, milter.ActionTempfail},
		{spam.ActionReject, milter.ActionReject},
	}

	for _, test := range tests {
		verdict := spam.Verdict{Action: test.action}
		result := MilterSpamVerdict(verdict)
		if result != test.expected {
			t.Fatalf("action %s: expected %v, got %v", test.action, test.expected, result)
		}
	}
}
