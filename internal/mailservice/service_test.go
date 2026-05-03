package mailservice

import (
	"context"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/outbound"
	"github.com/gogomail/gogomail/internal/storage"
)

func TestGetMessageParsesTextBodyFromStorage(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	path := "mailstore/c/d/u/maildir/2026/05/msg.eml"
	raw := strings.Join([]string{
		"Message-ID: <body@example.com>",
		"From: sender@example.net",
		"To: admin@example.com",
		"Subject: body",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"hello body",
	}, "\r\n")
	if err := store.Put(context.Background(), path, strings.NewReader(raw)); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}

	service := New(&fakeRepository{
		detail: maildb.MessageDetail{
			ID:          "msg-1",
			StoragePath: path,
		},
	}, store)

	msg, err := service.GetMessage(context.Background(), "user-1", "msg-1")
	if err != nil {
		t.Fatalf("GetMessage returned error: %v", err)
	}
	if msg.TextBody != "hello body" {
		t.Fatalf("TextBody = %q", msg.TextBody)
	}
}

type fakeRepository struct {
	detail     maildb.MessageDetail
	suppressed []string
}

func (f *fakeRepository) ListMessages(context.Context, string, int) ([]maildb.MessageSummary, error) {
	return nil, nil
}

func (f *fakeRepository) ListMessagesInFolder(context.Context, string, string, int) ([]maildb.MessageSummary, error) {
	return nil, nil
}

func (f *fakeRepository) ListFolders(context.Context, string) ([]maildb.Folder, error) {
	return nil, nil
}

func (f *fakeRepository) CreateFolder(context.Context, maildb.CreateFolderRequest) (maildb.Folder, error) {
	return maildb.Folder{}, nil
}

func (f *fakeRepository) RenameFolder(context.Context, string, string, string) (maildb.Folder, error) {
	return maildb.Folder{}, nil
}

func (f *fakeRepository) GetMessage(context.Context, string, string) (maildb.MessageDetail, error) {
	return f.detail, nil
}

func (f *fakeRepository) SetMessageFlag(context.Context, string, string, string, bool) error {
	return nil
}

func (f *fakeRepository) MoveMessage(context.Context, string, string, string) error {
	return nil
}

func (f *fakeRepository) DeleteMessage(context.Context, string, string) error {
	return nil
}

func (f *fakeRepository) SenderForUser(context.Context, string, string) (maildb.Sender, error) {
	return maildb.Sender{
		CompanyID:   "company-1",
		DomainID:    "domain-1",
		UserID:      "user-1",
		Address:     "sender@example.com",
		DisplayName: "Sender",
	}, nil
}

func (f *fakeRepository) RecordOutgoing(context.Context, maildb.OutgoingMessage) (string, error) {
	return "msg-1", nil
}

func (f *fakeRepository) SuppressedRecipients(context.Context, string, []string) ([]string, error) {
	return f.suppressed, nil
}

func TestSendTextStoresOutgoingMessage(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	service := New(&fakeRepository{}, store)

	result, err := service.SendText(context.Background(), SendTextRequest{
		UserID:   "user-1",
		To:       []outbound.Address{{Email: "user@example.net"}},
		Subject:  "hello",
		TextBody: "body",
	})
	if err != nil {
		t.Fatalf("SendText returned error: %v", err)
	}
	if result.ID != "msg-1" {
		t.Fatalf("ID = %q, want msg-1", result.ID)
	}
	if result.Farm != outbound.FarmGeneral {
		t.Fatalf("Farm = %q, want general", result.Farm)
	}
}

func TestSendTextRejectsSuppressedRecipients(t *testing.T) {
	t.Parallel()

	service := New(&fakeRepository{suppressed: []string{"blocked@example.net"}}, storage.NewLocalStore(t.TempDir()))

	_, err := service.SendText(context.Background(), SendTextRequest{
		UserID:   "user-1",
		To:       []outbound.Address{{Email: "blocked@example.net"}},
		Subject:  "hello",
		TextBody: "body",
	})
	if err == nil {
		t.Fatal("SendText accepted suppressed recipient")
	}
}
