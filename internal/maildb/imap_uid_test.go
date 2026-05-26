package maildb

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/imapgw"
)

func TestBuildListIMAPMessagesQueryAvoidsNullableUIDOptionalOR(t *testing.T) {
	firstPageQuery := buildListIMAPMessagesQuery(0)
	if strings.Contains(firstPageQuery, "i.uid IS NULL OR") {
		t.Fatalf("first page query still uses nullable optional OR:\n%s", firstPageQuery)
	}
	if strings.Contains(firstPageQuery, "UNION ALL") {
		t.Fatalf("first page query should stay a single ordered scan:\n%s", firstPageQuery)
	}
	if strings.Contains(firstPageQuery, "i.uid > $3") {
		t.Fatalf("first page query should not carry a cursor predicate:\n%s", firstPageQuery)
	}

	cursorQuery := buildListIMAPMessagesQuery(42)
	if strings.Contains(cursorQuery, "i.uid IS NULL OR") {
		t.Fatalf("cursor query still uses nullable optional OR:\n%s", cursorQuery)
	}
	if !strings.Contains(cursorQuery, "UNION ALL") {
		t.Fatalf("cursor query should split assigned and unassigned UID candidates:\n%s", cursorQuery)
	}
	if strings.Contains(cursorQuery, "SELECT *") {
		t.Fatalf("cursor query should explicitly project IMAP summary columns:\n%s", cursorQuery)
	}
	if !strings.Contains(cursorQuery, "rfc_message_id") || !strings.Contains(cursorQuery, "imap_keywords") || !strings.Contains(cursorQuery, "modseq") {
		t.Fatalf("cursor query should keep explicit outer projection aliases:\n%s", cursorQuery)
	}
	if !strings.Contains(cursorQuery, "JOIN imap_message_uid i ON i.message_id = m.id") || !strings.Contains(cursorQuery, "AND i.uid > $3") {
		t.Fatalf("cursor query should keep assigned UID scans indexable:\n%s", cursorQuery)
	}
	if !strings.Contains(cursorQuery, "AND i.message_id IS NULL") {
		t.Fatalf("cursor query should preserve lazy UID assignment candidates:\n%s", cursorQuery)
	}
}

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
		Keywords:     imapKeywordList{"$Project", "ClientTag"},
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
	if !got.Flags.Read || !got.Flags.Starred || !got.Flags.Answered || !got.Flags.Forwarded || !reflect.DeepEqual(got.Flags.Keywords, []string{"$Project", "ClientTag"}) {
		t.Fatalf("flags = %#v, want read/starred/answered/forwarded and keywords", got.Flags)
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
			flags: imapgw.MessageFlags{Read: true, Starred: true, Forwarded: true, Deleted: true, Keywords: []string{"$Project", "$Project", "$forwarded"}},
			mode:  imapgw.StoreFlagsAdd,
			want:  imapStoreFlagChanges{Read: boolPtr(true), Starred: boolPtr(true), Forwarded: boolPtr(true), Deleted: boolPtr(true), Keywords: imapKeywordList{"$Project", "$Forwarded"}, Mode: imapgw.StoreFlagsAdd},
		},
		"remove": {
			flags: imapgw.MessageFlags{Answered: true, Forwarded: true, Deleted: true, Keywords: []string{"ClientTag"}},
			mode:  imapgw.StoreFlagsRemove,
			want:  imapStoreFlagChanges{Answered: boolPtr(false), Forwarded: boolPtr(false), Deleted: boolPtr(false), Keywords: imapKeywordList{"ClientTag"}, Mode: imapgw.StoreFlagsRemove},
		},
		"replace": {
			flags: imapgw.MessageFlags{Read: true, Forwarded: true, Keywords: []string{"$Project"}},
			mode:  imapgw.StoreFlagsReplace,
			want:  imapStoreFlagChanges{Read: boolPtr(true), Starred: boolPtr(false), Answered: boolPtr(false), Forwarded: boolPtr(true), Deleted: boolPtr(false), Keywords: imapKeywordList{"$Project"}, Mode: imapgw.StoreFlagsReplace},
		},
		"replace empty clears all mutable flags": {
			mode: imapgw.StoreFlagsReplace,
			want: imapStoreFlagChanges{
				Read:      boolPtr(false),
				Starred:   boolPtr(false),
				Answered:  boolPtr(false),
				Forwarded: boolPtr(false),
				Deleted:   boolPtr(false),
				Mode:      imapgw.StoreFlagsReplace,
			},
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
	if _, err := newIMAPStoreFlagChanges(imapgw.MessageFlags{Keywords: []string{"bad keyword"}}, imapgw.StoreFlagsAdd); err == nil {
		t.Fatal("newIMAPStoreFlagChanges accepted invalid custom keyword")
	}
	if _, err := newIMAPStoreFlagChanges(imapgw.MessageFlags{}, imapgw.StoreFlagsMode("bad")); err == nil {
		t.Fatal("newIMAPStoreFlagChanges accepted bad mode")
	}
}

func TestApplyIMAPStoreFlagChangesReportsActualMutation(t *testing.T) {
	row := imapMessageRow{Read: true, Starred: false, Answered: false, Forwarded: false, Deleted: false, Keywords: imapKeywordList{"$Project"}}
	next, changed := applyIMAPStoreFlagChanges(row, imapStoreFlagChanges{
		Read:      boolPtr(true),
		Starred:   boolPtr(true),
		Answered:  boolPtr(false),
		Forwarded: boolPtr(true),
		Deleted:   boolPtr(true),
		Keywords:  imapKeywordList{"ClientTag"},
		Mode:      imapgw.StoreFlagsAdd,
	})
	if !changed {
		t.Fatal("applyIMAPStoreFlagChanges reported no change")
	}
	if !next.Read || !next.Starred || next.Answered || !next.Forwarded || !next.Deleted || !reflect.DeepEqual(next.Keywords, imapKeywordList{"$Project", "ClientTag"}) {
		t.Fatalf("next flags = read:%v starred:%v answered:%v forwarded:%v deleted:%v keywords:%#v", next.Read, next.Starred, next.Answered, next.Forwarded, next.Deleted, next.Keywords)
	}

	_, changed = applyIMAPStoreFlagChanges(next, imapStoreFlagChanges{
		Read:      boolPtr(true),
		Starred:   boolPtr(true),
		Answered:  boolPtr(false),
		Forwarded: boolPtr(true),
		Deleted:   boolPtr(true),
		Keywords:  imapKeywordList{"ClientTag"},
		Mode:      imapgw.StoreFlagsAdd,
	})
	if changed {
		t.Fatal("applyIMAPStoreFlagChanges reported change for identical flags")
	}

	cleared, changed := applyIMAPStoreFlagChanges(next, imapStoreFlagChanges{
		Read:      boolPtr(false),
		Starred:   boolPtr(false),
		Answered:  boolPtr(false),
		Forwarded: boolPtr(false),
		Deleted:   boolPtr(false),
		Mode:      imapgw.StoreFlagsReplace,
	})
	if !changed {
		t.Fatal("applyIMAPStoreFlagChanges reported no change for empty replace")
	}
	if cleared.Read || cleared.Starred || cleared.Answered || cleared.Forwarded || cleared.Deleted || len(cleared.Keywords) != 0 {
		t.Fatalf("cleared flags = %#v", cleared)
	}
}

func TestIMAPKeywordListScanCanonicalizesAndRejectsInvalid(t *testing.T) {
	var got imapKeywordList
	if err := got.Scan([]byte(`["$project","ClientTag","$Project"]`)); err != nil {
		t.Fatalf("scan keywords returned error: %v", err)
	}
	if want := (imapKeywordList{"$project", "ClientTag", "$Project"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("keywords = %#v, want %#v", got, want)
	}

	if err := got.Scan([]byte(`["bad keyword"]`)); err == nil {
		t.Fatal("scan keywords accepted invalid keyword")
	}
}

func TestIMAPFlagsJSONPersistsCustomKeywords(t *testing.T) {
	raw, err := imapFlagsJSON(imapgw.MessageFlags{
		Read:     true,
		Keywords: []string{"$Project", "$Project", "$forwarded"},
	})
	if err != nil {
		t.Fatalf("imapFlagsJSON returned error: %v", err)
	}
	var got struct {
		Read     bool     `json:"read"`
		Keywords []string `json:"imap_keywords"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal imap flags json: %v", err)
	}
	if !got.Read || !reflect.DeepEqual(got.Keywords, []string{"$Project", "$Forwarded"}) {
		t.Fatalf("flags json = read:%v keywords:%#v", got.Read, got.Keywords)
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

	tests := map[string]struct {
		want      string
		wantAllow bool
	}{
		`"INBOX"`:         {want: "inbox", wantAllow: true},
		"/Archive/2026/":  {want: "archive/2026", wantAllow: true},
		"Archive\t2026":   {want: "archive 2026", wantAllow: true},
		" Archive\t2026 ": {want: "archive 2026", wantAllow: false},
	}
	for input, tc := range tests {
		got, allow := normalizeIMAPMailboxLookupName(input)
		if got != tc.want || allow != tc.wantAllow {
			t.Fatalf("normalizeIMAPMailboxLookupName(%q) = %q/%v, want %q/%v", input, got, allow, tc.want, tc.wantAllow)
		}
	}
}

func TestIMAPMailboxLookupUsesTypedMailboxIDFastPath(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"imap_mailboxes.go", "imap_append.go"} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			text := string(source)
			if !strings.Contains(text, "f.id = $2::uuid") {
				t.Fatalf("%s query does not use typed mailbox id predicate", path)
			}
			if strings.Contains(text, "f.id::text = $2") {
				t.Fatalf("%s query casts indexed mailbox id column", path)
			}
		})
	}
}

func BenchmarkIMAPUIDArray1K(b *testing.B) {
	benchIMAPUIDArray(b, 1_000)
}

func BenchmarkIMAPUIDArray10K(b *testing.B) {
	benchIMAPUIDArray(b, 10_000)
}

func benchIMAPUIDArray(b *testing.B, count int) {
	b.Helper()
	uids := benchmarkIMAPUIDs(count)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		values := imapUIDArray(uids)
		if len(values) != count {
			b.Fatalf("imapUIDArray len = %d, want %d", len(values), count)
		}
	}
}

func benchmarkIMAPUIDs(count int) []imapgw.UID {
	uids := make([]imapgw.UID, 0, count)
	for len(uids) < count {
		uids = append(uids, imapgw.UID(len(uids)+1))
	}
	return uids
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
	if got.UserID != "user-1" || got.MailboxID != " inbox " || got.Limit != imapUIDBackfillDefaultLimit || got.AssignedCount != maxIMAPUIDBackfillAuditSample+2 {
		t.Fatalf("audit detail = %+v", got)
	}
	if len(got.Assigned) != maxIMAPUIDBackfillAuditSample || got.Assigned[0].MessageID != "msg-0" || got.Assigned[0].UID != 100 {
		t.Fatalf("assigned sample = %+v", got.Assigned)
	}
}

func TestIMAPMailboxFromFolderPredictsUnassignedUIDState(t *testing.T) {
	t.Parallel()

	folder := Folder{
		ID:             "inbox",
		Name:           "Inbox",
		FullPath:       "/Inbox",
		Total:          3,
		Unread:         1,
		TotalSize:      512,
		IMAPUnassigned: 2,
	}
	state := IMAPUIDState{UIDValidity: 7, UIDNext: 5, HighestModSeq: 11}

	got := imapMailboxFromFolder(folder, state)
	if got.UIDNext != 7 {
		t.Fatalf("UIDNext = %d, want 7", got.UIDNext)
	}
	if got.HighestModSeq != 13 {
		t.Fatalf("HighestModSeq = %d, want 13", got.HighestModSeq)
	}
	if got.Messages != 3 || got.Unseen != 1 || got.Size != 512 {
		t.Fatalf("mailbox counts = messages %d unseen %d size %d, want 3/1/512", got.Messages, got.Unseen, got.Size)
	}
}

func TestAssignIMAPListSequenceNumbersUsesMailboxBase(t *testing.T) {
	t.Parallel()

	messages := []imapgw.MessageSummary{
		{ID: "msg-3", UID: 9},
		{ID: "msg-4", UID: 10},
	}
	if err := assignIMAPListSequenceNumbers(messages, 2); err != nil {
		t.Fatalf("assignIMAPListSequenceNumbers returned error: %v", err)
	}
	if messages[0].SequenceNumber != 3 || messages[1].SequenceNumber != 4 {
		t.Fatalf("sequence numbers = %d/%d, want 3/4", messages[0].SequenceNumber, messages[1].SequenceNumber)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
