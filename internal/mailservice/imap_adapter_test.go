package mailservice

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/storage"
)

func TestIMAPStoreAdapterDelegatesToService(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	if err := store.Put(context.Background(), "messages/msg-1.eml", strings.NewReader("Subject: hi\r\n\r\nbody")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	repo := &fakeRepository{
		imapMailboxes:      []imapgw.Mailbox{{ID: "inbox", Name: "INBOX"}},
		imapMessages:       []imapgw.MessageSummary{{ID: "msg-1", MailboxID: "inbox", UID: 12}},
		imapFlagSummaries:  []imapgw.MessageSummary{{ID: "msg-1", MailboxID: "inbox", UID: 12}},
		imapMessage:        maildb.IMAPStoredMessage{Summary: imapgw.MessageSummary{ID: "msg-1", MailboxID: "inbox", UID: 12}, StoragePath: "messages/msg-1.eml"},
		backfilledIMAPUIDs: []maildb.IMAPMessageUID{{MessageID: "msg-1", MailboxID: "inbox", UID: 12, ModSeq: 2}},
	}
	adapter := NewIMAPStoreAdapter(New(repo, store))

	if mailboxes, err := adapter.ListMailboxes(context.Background(), imapgw.ListMailboxesRequest{UserID: "user-1"}); err != nil || len(mailboxes) != 1 {
		t.Fatalf("ListMailboxes = %#v, %v", mailboxes, err)
	}
	if mailbox, err := adapter.GetMailbox(context.Background(), "user-1", "inbox"); err != nil || mailbox.ID != "inbox" {
		t.Fatalf("GetMailbox = %#v, %v", mailbox, err)
	}
	if mailbox, err := adapter.CreateMailbox(context.Background(), "user-1", "Archive"); err != nil || mailbox.ID != "inbox" {
		t.Fatalf("CreateMailbox = %#v, %v", mailbox, err)
	}
	if messages, err := adapter.ListMessages(context.Background(), imapgw.ListMessagesRequest{UserID: "user-1", MailboxID: "inbox"}); err != nil || len(messages) != 1 {
		t.Fatalf("ListMessages = %#v, %v", messages, err)
	}
	message, err := adapter.FetchMessage(context.Background(), imapgw.FetchMessageRequest{UserID: "user-1", MailboxID: "inbox", UID: 12})
	if err != nil {
		t.Fatalf("FetchMessage returned error: %v", err)
	}
	defer message.Body.Close()
	if body, err := io.ReadAll(message.Body); err != nil || !strings.Contains(string(body), "body") {
		t.Fatalf("body = %q, %v", string(body), err)
	}
	if summaries, err := adapter.StoreFlags(context.Background(), imapgw.StoreFlagsRequest{UserID: "user-1", MailboxID: "inbox", UIDs: []imapgw.UID{12}, Flags: imapgw.MessageFlags{Read: true}, Mode: imapgw.StoreFlagsAdd}); err != nil || len(summaries) != 1 {
		t.Fatalf("StoreFlags = %#v, %v", summaries, err)
	}
}
