package dsn

import (
	"strings"
	"testing"
)

func TestComposeRejectsInvalidOriginalRecipientAddressType(t *testing.T) {
	t.Parallel()

	for _, originalRecipient := range []string{
		"bad type; user@example.net",
		"rfc822;",
		"; user@example.net",
	} {
		originalRecipient := originalRecipient
		t.Run(originalRecipient, func(t *testing.T) {
			t.Parallel()

			_, err := Compose(Report{
				ReportingMTA: "mx.example.com",
				Recipients: []RecipientStatus{{
					Recipient:         "user@example.net",
					OriginalRecipient: originalRecipient,
					Action:            "failed",
					Status:            "5.1.1",
				}},
			})
			if err == nil || !strings.Contains(err.Error(), "invalid original recipient") {
				t.Fatalf("Compose() error = %v, want invalid original recipient", err)
			}
		})
	}
}

func TestComposeNormalizesBareFinalRecipient(t *testing.T) {
	t.Parallel()

	composed, err := Compose(Report{
		ReportingMTA: "mx.example.com",
		Recipients: []RecipientStatus{{
			Recipient: "User@Example.NET",
			Action:    "failed",
			Status:    "5.1.1",
		}},
	})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	if !strings.Contains(string(composed.Raw), "Final-Recipient: rfc822; user@example.net") {
		t.Fatalf("raw missing normalized final recipient:\n%s", string(composed.Raw))
	}
}
