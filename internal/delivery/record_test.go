package delivery

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gogomail/gogomail/internal/outbound"
)

func TestAttemptsForNormalizesRecipientDomain(t *testing.T) {
	t.Parallel()

	attempts := attemptsFor(Job{QueuedMessage: QueuedMessage{
		MessageID: "msg-1",
		To:        []outbound.Address{{Email: "user@Example.NET"}},
	}}, AttemptFailed, nil, time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC))
	if len(attempts) != 1 {
		t.Fatalf("attempts = %+v, want one attempt", attempts)
	}
	if attempts[0].RecipientDomain != "example.net" {
		t.Fatalf("RecipientDomain = %q, want normalized domain", attempts[0].RecipientDomain)
	}
}

func TestAttemptsForTruncatesErrorMessageAtUTF8Boundary(t *testing.T) {
	t.Parallel()

	err := errors.New(strings.Repeat("a", 1999) + "한")
	attempts := attemptsFor(Job{QueuedMessage: QueuedMessage{
		MessageID: "msg-1",
		To:        []outbound.Address{{Email: "user@example.net"}},
	}}, AttemptFailed, err, time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC))
	if len(attempts) != 1 {
		t.Fatalf("attempts = %+v, want one attempt", attempts)
	}
	if len(attempts[0].ErrorMessage) > 2000 {
		t.Fatalf("ErrorMessage length = %d, want <= 2000 bytes", len(attempts[0].ErrorMessage))
	}
	if !utf8.ValidString(attempts[0].ErrorMessage) {
		t.Fatalf("ErrorMessage is invalid UTF-8: %q", attempts[0].ErrorMessage)
	}
}

func TestAttemptsForCarriesDSNMetadata(t *testing.T) {
	t.Parallel()

	attempts := attemptsFor(Job{QueuedMessage: QueuedMessage{
		MessageID: "msg-1",
		From:      outbound.Address{Email: "sender@example.com"},
		To:        []outbound.Address{{Email: "User@Example.NET"}},
		DSN: DSNOptions{
			Return:     "HDRS",
			EnvelopeID: "env+2D1",
			Recipients: []DSNRecipientOptions{{
				Address:           "user@example.net",
				Notify:            []string{"FAILURE", "DELAY"},
				OriginalRecipient: "rfc822;alias+40example.net",
			}},
		},
	}}, AttemptBounced, nil, time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC))

	if len(attempts) != 1 {
		t.Fatalf("attempts = %+v, want one attempt", attempts)
	}
	attempt := attempts[0]
	if attempt.DSNReturn != "HDRS" || attempt.DSNEnvelopeID != "env+2D1" {
		t.Fatalf("attempt DSN envelope = return %q envid %q", attempt.DSNReturn, attempt.DSNEnvelopeID)
	}
	if len(attempt.DSNNotify) != 2 || attempt.DSNNotify[0] != "FAILURE" || attempt.DSNNotify[1] != "DELAY" {
		t.Fatalf("attempt DSN notify = %+v", attempt.DSNNotify)
	}
	if attempt.OriginalRecipient != "rfc822;alias+40example.net" {
		t.Fatalf("OriginalRecipient = %q", attempt.OriginalRecipient)
	}
}
