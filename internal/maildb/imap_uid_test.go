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

func TestIMAPStoreFlagChangesForModes(t *testing.T) {
	tests := map[string]struct {
		flags imapgw.MessageFlags
		mode  imapgw.StoreFlagsMode
		want  imapStoreFlagChanges
	}{
		"add": {
			flags: imapgw.MessageFlags{Read: true, Starred: true},
			mode:  imapgw.StoreFlagsAdd,
			want:  imapStoreFlagChanges{Read: boolPtr(true), Starred: boolPtr(true)},
		},
		"remove": {
			flags: imapgw.MessageFlags{Answered: true},
			mode:  imapgw.StoreFlagsRemove,
			want:  imapStoreFlagChanges{Answered: boolPtr(false)},
		},
		"replace": {
			flags: imapgw.MessageFlags{Read: true},
			mode:  imapgw.StoreFlagsReplace,
			want:  imapStoreFlagChanges{Read: boolPtr(true), Starred: boolPtr(false), Answered: boolPtr(false)},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := newIMAPStoreFlagChanges(tt.flags, tt.mode)
			if err != nil {
				t.Fatalf("newIMAPStoreFlagChanges returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("changes = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestIMAPStoreFlagChangesRejectsDeferredDeletedDraftModel(t *testing.T) {
	if _, err := newIMAPStoreFlagChanges(imapgw.MessageFlags{Draft: true}, imapgw.StoreFlagsAdd); err == nil {
		t.Fatal("newIMAPStoreFlagChanges accepted Draft")
	}
	if _, err := newIMAPStoreFlagChanges(imapgw.MessageFlags{}, imapgw.StoreFlagsMode("bad")); err == nil {
		t.Fatal("newIMAPStoreFlagChanges accepted bad mode")
	}
}

func TestApplyIMAPStoreFlagChangesReportsActualMutation(t *testing.T) {
	row := imapMessageRow{Read: true, Starred: false, Answered: false}
	next, changed := applyIMAPStoreFlagChanges(row, imapStoreFlagChanges{
		Read:     boolPtr(true),
		Starred:  boolPtr(true),
		Answered: boolPtr(false),
	})
	if !changed {
		t.Fatal("applyIMAPStoreFlagChanges reported no change")
	}
	if !next.Read || !next.Starred || next.Answered {
		t.Fatalf("next flags = read:%v starred:%v answered:%v", next.Read, next.Starred, next.Answered)
	}

	_, changed = applyIMAPStoreFlagChanges(next, imapStoreFlagChanges{
		Read:     boolPtr(true),
		Starred:  boolPtr(true),
		Answered: boolPtr(false),
	})
	if changed {
		t.Fatal("applyIMAPStoreFlagChanges reported change for identical flags")
	}
}

func TestNormalizeIMAPUIDBackfillLimit(t *testing.T) {
	tests := map[int]int{
		0:    imapUIDBackfillDefaultLimit,
		-10:  imapUIDBackfillDefaultLimit,
		50:   50,
		5000: imapUIDBackfillMaxLimit,
	}
	for input, want := range tests {
		if got := normalizeIMAPUIDBackfillLimit(input); got != want {
			t.Fatalf("normalizeIMAPUIDBackfillLimit(%d) = %d, want %d", input, got, want)
		}
	}
}

func boolPtr(value bool) *bool {
	return &value
}
