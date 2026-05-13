package imapnotify

import (
	"context"
	"encoding/json"
	"errors"
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

func TestMailStoredHandlerPublishesExistsAfterUIDAssignment(t *testing.T) {
	t.Parallel()

	ensurer := &fakeUIDEnsurer{}
	events := &fakeMailboxEventPublisher{}
	handler := NewMailStoredHandler(ensurer).WithMailboxEvents(events)
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
	if len(events.events) != 1 {
		t.Fatalf("events = %+v, want one EXISTS event", events.events)
	}
	event := events.events[0]
	if event.Type != imapgw.MailboxEventExists || event.UserID != "user-1" || event.MailboxID != "inbox-1" || event.UID != 1 || event.Messages != 1 {
		t.Fatalf("event = %+v", event)
	}
}

func TestMailStoredHandlerIgnoresInactiveMessageUIDAssignment(t *testing.T) {
	t.Parallel()

	ensurer := &fakeUIDEnsurer{err: maildb.ErrIMAPMessageNotActive}
	events := &fakeMailboxEventPublisher{}
	handler := NewMailStoredHandler(ensurer).WithMailboxEvents(events)
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
	if len(events.events) != 0 {
		t.Fatalf("events = %+v, want no EXISTS event for inactive message", events.events)
	}
}

func TestMailStoredHandlerReturnsUIDAssignmentError(t *testing.T) {
	t.Parallel()

	ensurer := &fakeUIDEnsurer{err: errors.New("database offline")}
	handler := NewMailStoredHandler(ensurer)
	err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: json.RawMessage(`{
		"event":"mail.stored",
		"schema_version":"2026-05-04.mail-stored.v1",
		"message_id":"msg-1",
		"user_id":"user-1",
		"folder_id":"inbox-1"
	}`)})
	if err == nil || !strings.Contains(err.Error(), "ensure imap uid") {
		t.Fatalf("HandleEvent error = %v, want retryable uid assignment error", err)
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

func TestDecodeMailStoredEventRejectsOversizedIDs(t *testing.T) {
	t.Parallel()

	_, err := DecodeMailStoredEvent(json.RawMessage(`{
		"event":"mail.stored",
		"schema_version":"2026-05-04.mail-stored.v1",
		"message_id":"` + strings.Repeat("m", maxMailStoredEventIDBytes+1) + `",
		"user_id":"user-1",
		"folder_id":"inbox-1"
	}`))
	if err == nil || !strings.Contains(err.Error(), "message_id") {
		t.Fatalf("DecodeMailStoredEvent error = %v, want oversized message_id", err)
	}
}

func TestMailStoredHandlerNotifiesDeltaSync(t *testing.T) {
	t.Parallel()

	ensurer := &fakeUIDEnsurer{}
	notifier := &fakeDeltaSyncNotifier{}
	handler := NewMailStoredHandler(ensurer).WithDeltaSync(notifier)
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
	if len(notifier.calls) != 1 {
		t.Fatalf("DeltaSyncNotifier calls = %d, want 1", len(notifier.calls))
	}
	call := notifier.calls[0]
	if call.mailboxID != "inbox-1" || call.version != 1 {
		t.Fatalf("DeltaSyncNotifier call = %+v, want mailboxID=inbox-1 version=1", call)
	}
}

func TestMailStoredHandlerSkipsDeltaSyncWhenNil(t *testing.T) {
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
}

func TestMailStoredHandlerSkipsDeltaSyncOnInactiveMessage(t *testing.T) {
	t.Parallel()

	ensurer := &fakeUIDEnsurer{err: maildb.ErrIMAPMessageNotActive}
	notifier := &fakeDeltaSyncNotifier{}
	handler := NewMailStoredHandler(ensurer).WithDeltaSync(notifier)
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
	if len(notifier.calls) != 0 {
		t.Fatalf("DeltaSyncNotifier calls = %d, want 0 for inactive message", len(notifier.calls))
	}
}

type fakeUIDEnsurer struct {
	userID    string
	mailboxID string
	messageID string
	err       error
}

func (f *fakeUIDEnsurer) EnsureIMAPMessageUID(_ context.Context, userID string, mailboxID string, messageID string) (maildb.IMAPMessageUID, error) {
	f.userID = userID
	f.mailboxID = mailboxID
	f.messageID = messageID
	if f.err != nil {
		return maildb.IMAPMessageUID{}, f.err
	}
	return maildb.IMAPMessageUID{
		MessageID:      imapgw.MessageID(messageID),
		MailboxID:      imapgw.MailboxID(mailboxID),
		UID:            imapgw.UID(1),
		SequenceNumber: 1,
		ModSeq:         1,
	}, nil
}

type fakeMailboxEventPublisher struct {
	events []imapgw.MailboxEvent
}

func (f *fakeMailboxEventPublisher) Publish(_ context.Context, event imapgw.MailboxEvent) error {
	f.events = append(f.events, event)
	return nil
}

type deltaSyncCall struct {
	mailboxID string
	version   int64
}

type fakeDeltaSyncNotifier struct {
	calls []deltaSyncCall
}

func (f *fakeDeltaSyncNotifier) NotifyMailboxChange(mailboxID string, version int64) {
	f.calls = append(f.calls, deltaSyncCall{mailboxID: mailboxID, version: version})
}
