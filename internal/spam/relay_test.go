package spam

import (
	"context"
	"errors"
	"strings"
	"testing"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

type fakeRelay struct {
	verdict Verdict
	err     error
	seen    Request
}

func (r *fakeRelay) Check(_ context.Context, req Request) (Verdict, error) {
	r.seen = req
	return r.verdict, r.err
}

func TestHookPassesAuthenticationStageToRelay(t *testing.T) {
	relay := &fakeRelay{verdict: Verdict{Action: ActionAccept}}
	hook := Hook(HookOptions{Relay: relay})

	err := hook(context.Background(), smtpd.Event{
		Stage:        smtpd.StageAuthenticationChecked,
		EnvelopeFrom: "sender@example.net",
		Recipients:   []string{"user@example.com"},
		Authentication: smtpd.AuthenticationResults{
			SPF: smtpd.AuthCheckResult{Result: smtpd.AuthResultPass},
		},
		Size: 123,
	})
	if err != nil {
		t.Fatalf("hook returned error: %v", err)
	}
	if relay.seen.EnvelopeFrom != "sender@example.net" || relay.seen.Size != 123 {
		t.Fatalf("relay request = %+v", relay.seen)
	}
	if relay.seen.Authentication.SPF.Result != smtpd.AuthResultPass {
		t.Fatalf("relay auth = %+v, want SPF pass", relay.seen.Authentication)
	}
}

func TestHookRejectsSpamVerdict(t *testing.T) {
	relay := &fakeRelay{verdict: Verdict{Action: ActionReject, Reason: "rspamd reject"}}
	hook := Hook(HookOptions{Relay: relay})

	err := hook(context.Background(), smtpd.Event{Stage: smtpd.StageAuthenticationChecked})
	if err == nil || !strings.Contains(err.Error(), "rspamd reject") {
		t.Fatalf("hook error = %v, want reject reason", err)
	}
}

func TestHookShadowModeSuppressesRelayFailure(t *testing.T) {
	relay := &fakeRelay{err: errors.New("relay down")}
	hook := Hook(HookOptions{Relay: relay, Shadow: true})

	if err := hook(context.Background(), smtpd.Event{Stage: smtpd.StageAuthenticationChecked}); err != nil {
		t.Fatalf("shadow hook returned error: %v", err)
	}
}

func TestHookIgnoresOtherStages(t *testing.T) {
	relay := &fakeRelay{verdict: Verdict{Action: ActionReject}}
	hook := Hook(HookOptions{Relay: relay})

	if err := hook(context.Background(), smtpd.Event{Stage: smtpd.StageParsed}); err != nil {
		t.Fatalf("hook returned error for ignored stage: %v", err)
	}
	if relay.seen.Stage != "" {
		t.Fatalf("relay was called for ignored stage: %+v", relay.seen)
	}
}
