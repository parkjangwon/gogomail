package imapnotify

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/gogomail/gogomail/internal/maildb"
)

func TestMailStoredHandlerEnsuresIMAPMessageUID(t *testing.T) {
	t.Parallel()

	ensurer := &fakeUIDEnsurer{}
	handler := NewMailStoredHandler(ensurer)
	err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: json.RawMessage(`{
		"event":"mail.stored",
		"schema_version":"2026-05-04.mail-stored.v1",
		"message_id":"msg-1",
		"user_id":"user-1",
		"folder_id":"inbox-1"
	}`)})
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if ensurer.userID != "user-1" || ensurer.mailboxID != "inbox-1" || ensurer.messageID != "msg-1" {
		t.Fatalf("ensured uid for %q/%q/%q", ensurer.userID, ensurer.mailboxID, ensurer.messageID)
	}
}

func TestDecodeMailStoredEventRejectsUnsupportedSchema(t *testing.T) {
	t.Parallel()

	_, err := DecodeMailStoredEvent(json.RawMessage(`{
		"event":"mail.stored",
		"schema_version":"2099-01-01.mail-stored.v9",
		"message_id":"msg-1",
		"user_id":"user-1",
		"folder_id":"inbox-1"
	}`))
	if err == nil || !strings.Contains(err.Error(), "unsupported mail.stored imap schema_version") {
		t.Fatalf("DecodeMailStoredEvent error = %v, want unsupported schema", err)
	}
}

func TestDecodeMailStoredEventRequiresFolderID(t *testing.T) {
	t.Parallel()

	_, err := DecodeMailStoredEvent(json.RawMessage(`{
		"event":"mail.stored",
		"schema_version":"2026-05-04.mail-stored.v1",
		"message_id":"msg-1",
		"user_id":"user-1"
	}`))
	if err == nil || !strings.Contains(err.Error(), "folder_id") {
		t.Fatalf("DecodeMailStoredEvent error = %v, want folder_id requirement", err)
	}
}

type fakeUIDEnsurer struct {
	userID    string
	mailboxID string
	messageID string
}

func (f *fakeUIDEnsurer) EnsureIMAPMessageUID(_ context.Context, userID string, mailboxID string, messageID string) (maildb.IMAPMessageUID, error) {
	f.userID = userID
	f.mailboxID = mailboxID
	f.messageID = messageID
	return maildb.IMAPMessageUID{
		MessageID: imapgw.MessageID(messageID),
		MailboxID: imapgw.MailboxID(mailboxID),
		UID:       imapgw.UID(1),
		ModSeq:    1,
	}, nil
}
