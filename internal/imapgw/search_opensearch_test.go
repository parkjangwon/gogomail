package imapgw

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func TestIMAPSearchUsesOpenSearchCandidatesForBodySearch(t *testing.T) {
	t.Parallel()

	backend := &opensearchCandidateBackend{
		searchIDs: []MessageID{"message-2"},
	}
	server := &Server{options: ServerOptions{Backend: backend}}
	state := &imapConnState{
		session:          &Session{UserID: "user-1"},
		selectedMailbox:  "inbox",
		selectedMessages: 3,
	}
	messages := []MessageSummary{
		{ID: "message-1", UID: 1, SequenceNumber: 1},
		{ID: "message-2", UID: 2, SequenceNumber: 2},
		{ID: "message-3", UID: 3, SequenceNumber: 3},
	}

	results, _, ok, err := server.imapSearchResults(context.Background(), state, []string{"BODY", "needle"}, messages, true, false)
	if err != nil {
		t.Fatalf("imapSearchResults returned error: %v", err)
	}
	if !ok {
		t.Fatal("imapSearchResults returned ok=false")
	}
	if backend.searchCalls != 1 {
		t.Fatalf("search calls = %d, want 1", backend.searchCalls)
	}
	if backend.fetchCalls != 1 {
		t.Fatalf("fetch calls = %d, want 1", backend.fetchCalls)
	}
	if len(results) != 1 || results[0].uid != 2 {
		t.Fatalf("results = %#v, want uid 2", results)
	}
	if backend.lastRequest.UserID != "user-1" || backend.lastRequest.MailboxID != "inbox" {
		t.Fatalf("search request = %#v", backend.lastRequest)
	}
	if backend.lastRequest.Query != "needle" {
		t.Fatalf("search request query = %q, want needle", backend.lastRequest.Query)
	}
}

type opensearchCandidateBackend struct {
	fakeBackend
	searchIDs   []MessageID
	searchCalls int
	fetchCalls  int
	lastRequest SearchMessagesRequest
}

func (b *opensearchCandidateBackend) SearchMessageIDs(_ context.Context, req SearchMessagesRequest) ([]MessageID, error) {
	b.searchCalls++
	b.lastRequest = req
	return append([]MessageID(nil), b.searchIDs...), nil
}

func (b *opensearchCandidateBackend) FetchMessage(_ context.Context, req FetchMessageRequest) (Message, error) {
	b.fetchCalls++
	body := "Subject: test\r\nFrom: sender@example.com\r\n\r\ndifferent content"
	if req.UID == 2 {
		body = "Subject: test\r\nFrom: sender@example.com\r\n\r\nmessage with needle inside"
	}
	return Message{Summary: MessageSummary{UID: req.UID}, Body: io.NopCloser(bytes.NewBufferString(body))}, nil
}

func (b *opensearchCandidateBackend) ListMessages(context.Context, ListMessagesRequest) ([]MessageSummary, error) {
	return []MessageSummary{
		{ID: "message-1", UID: 1, SequenceNumber: 1},
		{ID: "message-2", UID: 2, SequenceNumber: 2},
		{ID: "message-3", UID: 3, SequenceNumber: 3},
	}, nil
}

func (b *opensearchCandidateBackend) Authenticate(context.Context, string, string) (Session, error) {
	return Session{UserID: "user-1", Username: "user@example.com"}, nil
}

func (b *opensearchCandidateBackend) SelectMailbox(context.Context, SelectMailboxRequest) (MailboxState, error) {
	return MailboxState{Mailbox: Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 4}, PermanentFlags: []string{FlagSeen}}, nil
}

func (b *opensearchCandidateBackend) ListMailboxes(context.Context, ListMailboxesRequest) ([]Mailbox, error) {
	return []Mailbox{{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 4}}, nil
}

func (b *opensearchCandidateBackend) GetMailbox(context.Context, UserID, MailboxID) (Mailbox, error) {
	return Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 4}, nil
}

func (b *opensearchCandidateBackend) ListSubscribedMailboxes(context.Context, ListMailboxesRequest) ([]MailboxSubscription, error) {
	return []MailboxSubscription{{Name: "INBOX", Mailbox: Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 4}, Exists: true}}, nil
}

func (b *opensearchCandidateBackend) SubscribeMailbox(context.Context, UserID, MailboxID) (MailboxSubscription, error) {
	return MailboxSubscription{Name: "INBOX", Mailbox: Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 4}, Exists: true}, nil
}

func (b *opensearchCandidateBackend) UnsubscribeMailbox(context.Context, UserID, MailboxID) error {
	return nil
}

func (b *opensearchCandidateBackend) CopyMessages(context.Context, CopyMessagesRequest) ([]CopyMessageResult, error) {
	return nil, nil
}

func (b *opensearchCandidateBackend) MoveMessages(context.Context, MoveMessagesRequest) ([]MoveMessageResult, error) {
	return nil, nil
}

func (b *opensearchCandidateBackend) Expunge(context.Context, ExpungeRequest) ([]MessageSummary, error) {
	return nil, nil
}

func (b *opensearchCandidateBackend) StoreFlags(context.Context, StoreFlagsRequest) ([]MessageSummary, error) {
	return nil, nil
}

func (b *opensearchCandidateBackend) AppendMessage(context.Context, AppendMessageRequest) (AppendMessageResult, error) {
	return AppendMessageResult{}, nil
}

func (b *opensearchCandidateBackend) Subscribe(context.Context, UserID, MailboxID) (<-chan MailboxEvent, func(), error) {
	return nil, func() {}, nil
}
