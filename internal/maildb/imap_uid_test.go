package maildb

import (
	"reflect"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/imapgw"
)

func TestIMAPMessageFromRowMapsEnvelopeFlagsAndUID(t *testing.T) {
	internalDate := time.Date(2026, 5, 4, 12, 30, 0, 0, time.UTC)

	got := imapMessageFromRow(imapMessageRow{
		ID:           "message-1",
		MailboxID:    "mailbox-1",
		RFCMessageID: "<message-1@example.com>",
		Subject:      "Quarterly report",
		FromAddr:     "sender@example.com",
		FromName:     "Sender",
		InternalDate: internalDate,
		Size:         4096,
		Read:         true,
		Starred:      true,
		Answered:     true,
		Forwarded:    true,
	}, IMAPMessageUID{
		MessageID: "message-1",
		MailboxID: "mailbox-1",
		UID:       42,
		ModSeq:    7,
	})

	if got.ID != "message-1" || got.MailboxID != "mailbox-1" || got.UID != 42 {
		t.Fatalf("message identity = %#v, want message/mailbox/uid mapped", got)
	}
	if got.Envelope.MessageID != "<message-1@example.com>" || got.Envelope.Subject != "Quarterly report" || !got.Envelope.Date.Equal(internalDate) {
		t.Fatalf("envelope = %#v, want RFC message id, subject, and date", got.Envelope)
	}
	wantFrom := []imapgw.Address{{Name: "Sender", Mailbox: "sender", Host: "example.com"}}
	if !reflect.DeepEqual(got.Envelope.From, wantFrom) {
		t.Fatalf("from = %#v, want %#v", got.Envelope.From, wantFrom)
	}
	if !got.Flags.Read || !got.Flags.Starred || !got.Flags.Answered || !got.Flags.Forwarded {
		t.Fatalf("flags = %#v, want read/starred/answered/forwarded", got.Flags)
	}
	if got.Size != 4096 {
		t.Fatalf("size = %d, want 4096", got.Size)
	}
}

func TestIMAPEnvelopeAddressParsesDisplayAddress(t *testing.T) {
	got := imapEnvelopeAddress("", `"Ops Team" <ops@example.net>`)
	want := []imapgw.Address{{Name: "Ops Team", Mailbox: "ops", Host: "example.net"}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("imapEnvelopeAddress = %#v, want %#v", got, want)
	}
}
