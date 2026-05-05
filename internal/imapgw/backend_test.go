package imapgw

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestBackendInterfaceComposesIMAPGatewayBoundaries(t *testing.T) {
	t.Parallel()

	var _ Backend = fakeComposedBackend{}
	backend := fakeComposedBackend{}
	session, err := backend.Authenticate(context.Background(), "user@example.com", "secret")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if session.UserID != "user-1" {
		t.Fatalf("session = %+v", session)
	}
	state, err := backend.SelectMailbox(context.Background(), SelectMailboxRequest{UserID: session.UserID, MailboxID: "inbox"})
	if err != nil {
		t.Fatalf("SelectMailbox returned error: %v", err)
	}
	if state.UIDValidity == 0 || state.UIDNext == 0 {
		t.Fatalf("state must expose durable UID state: %+v", state)
	}
}

type fakeComposedBackend struct{}

func (fakeComposedBackend) Authenticate(context.Context, string, string) (Session, error) {
	return Session{UserID: "user-1", Username: "user@example.com"}, nil
}

func (fakeComposedBackend) ListMailboxes(context.Context, ListMailboxesRequest) ([]Mailbox, error) {
	return []Mailbox{{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 2}}, nil
}

func (fakeComposedBackend) GetMailbox(context.Context, UserID, MailboxID) (Mailbox, error) {
	return Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 2}, nil
}

func (fakeComposedBackend) CreateMailbox(context.Context, UserID, MailboxID) (Mailbox, error) {
	return Mailbox{ID: "archive", Name: "Archive", UIDValidity: 2, UIDNext: 1}, nil
}

func (fakeComposedBackend) DeleteMailbox(context.Context, UserID, MailboxID) error {
	return nil
}

func (fakeComposedBackend) RenameMailbox(context.Context, UserID, MailboxID, MailboxID) (Mailbox, error) {
	return Mailbox{ID: "renamed", Name: "Renamed", UIDValidity: 2, UIDNext: 1}, nil
}

func (fakeComposedBackend) ListMessages(context.Context, ListMessagesRequest) ([]MessageSummary, error) {
	return []MessageSummary{{ID: "message-1", UID: 1}}, nil
}

func (fakeComposedBackend) FetchMessage(context.Context, FetchMessageRequest) (Message, error) {
	return Message{Summary: MessageSummary{ID: "message-1", UID: 1, SequenceNumber: 1}, Body: io.NopCloser(strings.NewReader(""))}, nil
}

func (fakeComposedBackend) StoreFlags(context.Context, StoreFlagsRequest) ([]MessageSummary, error) {
	return []MessageSummary{{ID: "message-1", UID: 1, SequenceNumber: 1, Flags: MessageFlags{Read: true}}}, nil
}

func (fakeComposedBackend) AppendMessage(context.Context, AppendMessageRequest) (AppendMessageResult, error) {
	return AppendMessageResult{Summary: MessageSummary{ID: "message-append-1", MailboxID: "inbox", UID: 2}, UIDValidity: 1}, nil
}

func (fakeComposedBackend) SelectMailbox(context.Context, SelectMailboxRequest) (MailboxState, error) {
	return MailboxState{
		Mailbox:        Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 2},
		PermanentFlags: []string{FlagSeen, FlagFlagged, FlagAnswered},
	}, nil
}

func (fakeComposedBackend) CopyMessages(context.Context, CopyMessagesRequest) ([]MessageSummary, error) {
	return []MessageSummary{{ID: "message-copy-1", MailboxID: "archive", UID: 2}}, nil
}

func (fakeComposedBackend) MoveMessages(context.Context, MoveMessagesRequest) ([]MessageSummary, error) {
	return []MessageSummary{{ID: "message-1", UID: 1}}, nil
}

func (fakeComposedBackend) Expunge(context.Context, ExpungeRequest) ([]MessageSummary, error) {
	return nil, nil
}

func (fakeComposedBackend) Subscribe(context.Context, UserID, MailboxID) (<-chan MailboxEvent, func(), error) {
	events := make(chan MailboxEvent)
	cancel := func() { close(events) }
	return events, cancel, nil
}
