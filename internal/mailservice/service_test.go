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
		attachments: []maildb.Attachment{
			{ID: "att-1", Filename: "report.pdf"},
		},
	}, store)

	msg, err := service.GetMessage(context.Background(), "user-1", "msg-1")
	if err != nil {
		t.Fatalf("GetMessage returned error: %v", err)
	}
	if msg.TextBody != "hello body" {
		t.Fatalf("TextBody = %q", msg.TextBody)
	}
	if len(msg.Attachments) != 1 || msg.Attachments[0].Filename != "report.pdf" {
		t.Fatalf("Attachments = %+v", msg.Attachments)
	}
}

type fakeRepository struct {
	detail                    maildb.MessageDetail
	attachments               []maildb.Attachment
	suppressed                []string
	seenSuppressionRecipients []string
	lastDraft                 SaveDraftRequest
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

func (f *fakeRepository) DeleteFolder(context.Context, string, string) error {
	return nil
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

func (f *fakeRepository) ListAttachments(context.Context, string, string) ([]maildb.Attachment, error) {
	return f.attachments, nil
}

func (f *fakeRepository) GetAttachment(context.Context, string, string, string) (maildb.Attachment, error) {
	return maildb.Attachment{}, nil
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

func (f *fakeRepository) SuppressedRecipients(_ context.Context, _ string, recipients []string) ([]string, error) {
	f.seenSuppressionRecipients = append([]string(nil), recipients...)
	return f.suppressed, nil
}

func (f *fakeRepository) SaveDraft(_ context.Context, req SaveDraftRequest) (maildb.MessageDetail, error) {
	f.lastDraft = req
	return maildb.MessageDetail{ID: "draft-1", Subject: req.Subject}, nil
}

func (f *fakeRepository) DeleteDraft(context.Context, string, string) error {
	return nil
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

func TestSendTextRejectsMissingRecipients(t *testing.T) {
	t.Parallel()

	service := New(&fakeRepository{}, storage.NewLocalStore(t.TempDir()))
	_, err := service.SendText(context.Background(), SendTextRequest{
		UserID:   "user-1",
		Subject:  "hello",
		TextBody: "body",
	})
	if err == nil {
		t.Fatal("SendText accepted missing recipients")
	}
}

func TestSendTextDeduplicatesSuppressionRecipients(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, storage.NewLocalStore(t.TempDir()))

	_, err := service.SendText(context.Background(), SendTextRequest{
		UserID:   "user-1",
		To:       []outbound.Address{{Email: "User@Example.net"}},
		Cc:       []outbound.Address{{Email: "user@example.net"}},
		Bcc:      []outbound.Address{{Email: "other@example.net"}},
		Subject:  "hello",
		TextBody: "body",
	})
	if err != nil {
		t.Fatalf("SendText returned error: %v", err)
	}
	want := []string{"user@example.net", "other@example.net"}
	if strings.Join(repo.seenSuppressionRecipients, ",") != strings.Join(want, ",") {
		t.Fatalf("suppression recipients = %v, want %v", repo.seenSuppressionRecipients, want)
	}
}

func TestSaveDraftDelegatesToDraftRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)
	draft, err := service.SaveDraft(context.Background(), SaveDraftRequest{
		UserID:  "user-1",
		Subject: "draft",
	})
	if err != nil {
		t.Fatalf("SaveDraft returned error: %v", err)
	}
	if draft.ID != "draft-1" || repo.lastDraft.Subject != "draft" {
		t.Fatalf("draft = %+v last = %+v", draft, repo.lastDraft)
	}
}
