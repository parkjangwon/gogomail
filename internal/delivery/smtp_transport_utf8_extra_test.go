package delivery

import (
	"testing"

	"github.com/gogomail/gogomail/internal/outbound"
)

func TestJobNeedsUTF8FalseForASCIIAddresses(t *testing.T) {
	t.Parallel()

	job := Job{QueuedMessage: QueuedMessage{
		Event:     "mail.queued",
		MessageID: "test-id",
		From:      outbound.Address{Email: "sender@example.com"},
		To:        []outbound.Address{{Email: "recipient@example.net"}},
	}}
	if jobNeedsUTF8(job) {
		t.Error("jobNeedsUTF8 returned true for pure-ASCII addresses")
	}
}

func TestJobNeedsUTF8TrueForInternationalizedSender(t *testing.T) {
	t.Parallel()

	job := Job{QueuedMessage: QueuedMessage{
		Event:     "mail.queued",
		MessageID: "test-id",
		From:      outbound.Address{Email: "발신자@example.com"},
		To:        []outbound.Address{{Email: "recipient@example.net"}},
	}}
	if !jobNeedsUTF8(job) {
		t.Error("jobNeedsUTF8 returned false for non-ASCII sender")
	}
}

func TestJobNeedsUTF8TrueForInternationalizedRecipient(t *testing.T) {
	t.Parallel()

	job := Job{QueuedMessage: QueuedMessage{
		Event:     "mail.queued",
		MessageID: "test-id",
		From:      outbound.Address{Email: "sender@example.com"},
		To:        []outbound.Address{{Email: "수신자@example.net"}},
	}}
	if !jobNeedsUTF8(job) {
		t.Error("jobNeedsUTF8 returned false for non-ASCII recipient")
	}
}

func TestContainsNonASCIIByte(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"ascii@example.com", false},
		{"한글@example.com", true},
		{"user@xn--p1ai", false},
	}
	for _, tc := range cases {
		got := containsNonASCIIByte(tc.input)
		if got != tc.want {
			t.Errorf("containsNonASCIIByte(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}
