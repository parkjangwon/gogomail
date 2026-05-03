package maildb

import (
	"testing"

	"github.com/gogomail/gogomail/internal/message"
)

func TestSubmittedBccRecipientsPreservesEnvelopeOnlyRecipients(t *testing.T) {
	t.Parallel()

	bcc := submittedBccRecipients(message.ParsedMessage{
		To: []message.Address{{Name: "Visible", Address: "visible@example.net"}},
	}, []string{"visible@example.net", "Hidden@Example.NET"})

	if len(bcc) != 1 {
		t.Fatalf("bcc recipients = %d, want 1", len(bcc))
	}
	if bcc[0].Email != "hidden@example.net" {
		t.Fatalf("hidden recipient = %+v", bcc[0])
	}
}
