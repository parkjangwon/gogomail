package maildb

import (
	"encoding/json"
	"reflect"
	"strconv"
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
		ToAddrs:      json.RawMessage(`[{"name":"Recipient","address":"recipient@example.com"}]`),
		CcAddrs:      json.RawMessage(`[{"name":"Copy","address":"copy@example.com"}]`),
		BccAddrs:     json.RawMessage(`[{"name":"Blind","address":"blind@example.com"}]`),
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
	if got.ModSeq != 7 {
		t.Fatalf("modseq = %d, want 7", got.ModSeq)
	}
	if got.Envelope.MessageID != "<message-1@example.com>" || got.Envelope.Subject != "Quarterly report" || !got.Envelope.Date.Equal(internalDate) {
		t.Fatalf("envelope = %#v, want RFC message id, subject, and date", got.Envelope)
	}
	wantFrom := []imapgw.Address{{Name: "Sender", Mailbox: "sender", Host: "example.com"}}
	if !reflect.DeepEqual(got.Envelope.From, wantFrom) {
		t.Fatalf("from = %#v, want %#v", got.Envelope.From, wantFrom)
	}
	if want := []imapgw.Address{{Name: "Recipient", Mailbox: "recipient", Host: "example.com"}}; !reflect.DeepEqual(got.Envelope.To, want) {
		t.Fatalf("to = %#v, want %#v", got.Envelope.To, want)
	}
	if want := []imapgw.Address{{Name: "Copy", Mailbox: "copy", Host: "example.com"}}; !reflect.DeepEqual(got.Envelope.Cc, want) {
		t.Fatalf("cc = %#v, want %#v", got.Envelope.Cc, want)
	}
	if want := []imapgw.Address{{Name: "Blind", Mailbox: "blind", Host: "example.com"}}; !reflect.DeepEqual(got.Envelope.Bcc, want) {
		t.Fatalf("bcc = %#v, want %#v", got.Envelope.Bcc, want)
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

func TestIMAPEnvelopeAddressesDecodesStoredJSON(t *testing.T) {
	got := imapEnvelopeAddresses(json.RawMessage(`[{"name":"Ops","address":"ops@example.net"},{"name":"","address":"local"}]`))
	want := []imapgw.Address{
		{Name: "Ops", Mailbox: "ops", Host: "example.net"},
		{Mailbox: "local"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("imapEnvelopeAddresses = %#v, want %#v", got, want)
	}
}

func TestIMAPStoreFlagChangesForModes(t *testing.T) {
	tests := map[string]struct {
		flags imapgw.MessageFlags
		mode  imapgw.StoreFlagsMode
		want  imapStoreFlagChanges
	}{
		"add": {
			flags: imapgw.MessageFlags{Read: true, Starred: true, Forwarded: true, Deleted: true},
			mode:  imapgw.StoreFlagsAdd,
			want:  imapStoreFlagChanges{Read: boolPtr(true), Starred: boolPtr(true), Forwarded: boolPtr(true), Deleted: boolPtr(true)},
		},
		"remove": {
			flags: imapgw.MessageFlags{Answered: true, Forwarded: true, Deleted: true},
			mode:  imapgw.StoreFlagsRemove,
			want:  imapStoreFlagChanges{Answered: boolPtr(false), Forwarded: boolPtr(false), Deleted: boolPtr(false)},
		},
		"replace": {
			flags: imapgw.MessageFlags{Read: true, Forwarded: true},
			mode:  imapgw.StoreFlagsReplace,
			want:  imapStoreFlagChanges{Read: boolPtr(true), Starred: boolPtr(false), Answered: boolPtr(false), Forwarded: boolPtr(true), Deleted: boolPtr(false)},
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

func TestIMAPStoreFlagChangesRejectsDraftModel(t *testing.T) {
	if _, err := newIMAPStoreFlagChanges(imapgw.MessageFlags{Draft: true}, imapgw.StoreFlagsAdd); err == nil {
		t.Fatal("newIMAPStoreFlagChanges accepted Draft")
	}
	if _, err := newIMAPStoreFlagChanges(imapgw.MessageFlags{}, imapgw.StoreFlagsMode("bad")); err == nil {
		t.Fatal("newIMAPStoreFlagChanges accepted bad mode")
	}
}

func TestApplyIMAPStoreFlagChangesReportsActualMutation(t *testing.T) {
	row := imapMessageRow{Read: true, Starred: false, Answered: false, Forwarded: false, Deleted: false}
	next, changed := applyIMAPStoreFlagChanges(row, imapStoreFlagChanges{
		Read:      boolPtr(true),
		Starred:   boolPtr(true),
		Answered:  boolPtr(false),
		Forwarded: boolPtr(true),
		Deleted:   boolPtr(true),
	})
	if !changed {
		t.Fatal("applyIMAPStoreFlagChanges reported no change")
	}
	if !next.Read || !next.Starred || next.Answered || !next.Forwarded || !next.Deleted {
		t.Fatalf("next flags = read:%v starred:%v answered:%v forwarded:%v deleted:%v", next.Read, next.Starred, next.Answered, next.Forwarded, next.Deleted)
	}

	_, changed = applyIMAPStoreFlagChanges(next, imapStoreFlagChanges{
		Read:      boolPtr(true),
		Starred:   boolPtr(true),
		Answered:  boolPtr(false),
		Forwarded: boolPtr(true),
		Deleted:   boolPtr(true),
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

func TestNormalizeIMAPMailboxLookupName(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		`"INBOX"`:         "inbox",
		"/Archive/2026/":  "archive/2026",
		" Archive\t2026 ": "archive 2026",
	}
	for input, want := range tests {
		if got := normalizeIMAPMailboxLookupName(input); got != want {
			t.Fatalf("normalizeIMAPMailboxLookupName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestIMAPUIDBackfillAuditDetailSamplesAssignments(t *testing.T) {
	t.Parallel()

	assigned := make([]IMAPMessageUID, 0, maxIMAPUIDBackfillAuditSample+2)
	for i := 0; i < maxIMAPUIDBackfillAuditSample+2; i++ {
		assigned = append(assigned, IMAPMessageUID{
			MessageID: imapgw.MessageID("msg-" + strconv.Itoa(i)),
			MailboxID: "inbox",
			UID:       imapgw.UID(100 + uint32(i)),
			ModSeq:    uint64(200 + i),
		})
	}

	detail, err := imapUIDBackfillAuditDetail(" user-1 ", " inbox ", 0, assigned)
	if err != nil {
		t.Fatalf("imapUIDBackfillAuditDetail returned error: %v", err)
	}
	var got struct {
		UserID        string `json:"user_id"`
		MailboxID     string `json:"mailbox_id"`
		Limit         int    `json:"limit"`
		AssignedCount int    `json:"assigned_count"`
		Assigned      []struct {
			MessageID string `json:"message_id"`
			UID       uint32 `json:"uid"`
			ModSeq    uint64 `json:"modseq"`
		} `json:"assigned_sample"`
	}
	if err := json.Unmarshal(detail, &got); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if got.UserID != "user-1" || got.MailboxID != "inbox" || got.Limit != imapUIDBackfillDefaultLimit || got.AssignedCount != maxIMAPUIDBackfillAuditSample+2 {
		t.Fatalf("audit detail = %+v", got)
	}
	if len(got.Assigned) != maxIMAPUIDBackfillAuditSample || got.Assigned[0].MessageID != "msg-0" || got.Assigned[0].UID != 100 {
		t.Fatalf("assigned sample = %+v", got.Assigned)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
