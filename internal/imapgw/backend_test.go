package imapgw

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestBackendInterfaceComposesIMAPGatewayBoundaries(t *testing.T) {
	t.Parallel()

	var _ Backend = fakeBackend{}
	backend := fakeBackend{}
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

type fakeBackend struct{}

func (fakeBackend) Authenticate(context.Context, string, string) (Session, error) {
	return Session{UserID: "user-1", Username: "user@example.com"}, nil
}

func (fakeBackend) ListMailboxes(context.Context, ListMailboxesRequest) ([]Mailbox, error) {
	return []Mailbox{{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 2}}, nil
}

func (fakeBackend) GetMailbox(context.Context, UserID, MailboxID) (Mailbox, error) {
	return Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 2}, nil
}

func (fakeBackend) ListMessages(context.Context, ListMessagesRequest) ([]MessageSummary, error) {
	return []MessageSummary{{ID: "message-1", UID: 1}}, nil
}

func (fakeBackend) FetchMessage(context.Context, FetchMessageRequest) (Message, error) {
	return Message{Summary: MessageSummary{ID: "message-1", UID: 1}, Body: io.NopCloser(strings.NewReader(""))}, nil
}

func (fakeBackend) StoreFlags(context.Context, StoreFlagsRequest) ([]MessageSummary, error) {
	return []MessageSummary{{ID: "message-1", UID: 1, Flags: MessageFlags{Read: true}}}, nil
}

func (fakeBackend) SelectMailbox(context.Context, SelectMailboxRequest) (MailboxState, error) {
	return MailboxState{
		Mailbox:        Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 2},
		PermanentFlags: []string{FlagSeen, FlagFlagged, FlagAnswered},
	}, nil
}

func (fakeBackend) MoveMessages(context.Context, MoveMessagesRequest) ([]MessageSummary, error) {
	return []MessageSummary{{ID: "message-1", UID: 1}}, nil
}

func (fakeBackend) Expunge(context.Context, ExpungeRequest) ([]UID, error) {
	return nil, nil
}

func (fakeBackend) Subscribe(context.Context, UserID, MailboxID) (<-chan MailboxEvent, func(), error) {
	events := make(chan MailboxEvent)
	cancel := func() { close(events) }
	return events, cancel, nil
}
