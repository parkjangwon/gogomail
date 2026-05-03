package mailservice

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
			Flags:       []byte(`{"read":true}`),
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

func TestGetMessageMarksUnreadMessageRead(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		detail: maildb.MessageDetail{
			ID:    "msg-1",
			Flags: []byte(`{"read":false}`),
		},
	}
	service := New(repo, nil)
	if _, err := service.GetMessage(context.Background(), "user-1", "msg-1"); err != nil {
		t.Fatalf("GetMessage returned error: %v", err)
	}
	if repo.lastFlagMessageID != "msg-1" || repo.lastFlag != "read" {
		t.Fatalf("read marker = %q/%q", repo.lastFlagMessageID, repo.lastFlag)
	}
}

func TestGetMessageDoesNotRewriteReadFlag(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		detail: maildb.MessageDetail{
			ID:    "msg-1",
			Flags: []byte(`{"read":true}`),
		},
	}
	service := New(repo, nil)
	if _, err := service.GetMessage(context.Background(), "user-1", "msg-1"); err != nil {
		t.Fatalf("GetMessage returned error: %v", err)
	}
	if repo.lastFlag != "" {
		t.Fatalf("unexpected flag write = %q", repo.lastFlag)
	}
}

type fakeRepository struct {
	detail                    maildb.MessageDetail
	attachments               []maildb.Attachment
	suppressed                []string
	seenSuppressionRecipients []string
	lastDraft                 maildb.SaveDraftRequest
	lastAttachmentUpload      maildb.CreateAttachmentUploadRequest
	lastFlagMessageID         string
	lastFlag                  string
	lastPageCursor            maildb.MessageListCursor
	lastSentDraftID           string
	lastSentDraftMessageID    string
	recordErr                 error
}

func (f *fakeRepository) ListMessages(context.Context, string, int) ([]maildb.MessageSummary, error) {
	return nil, nil
}

func (f *fakeRepository) ListMessagesInFolder(context.Context, string, string, int) ([]maildb.MessageSummary, error) {
	return nil, nil
}

func (f *fakeRepository) ListMessagesPage(_ context.Context, _ string, _ string, _ int, cursor maildb.MessageListCursor) ([]maildb.MessageSummary, error) {
	f.lastPageCursor = cursor
	return []maildb.MessageSummary{{ID: "msg-page"}}, nil
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

func (f *fakeRepository) SetMessageFlag(_ context.Context, _ string, messageID string, flag string, _ bool) error {
	f.lastFlagMessageID = messageID
	f.lastFlag = flag
	return nil
}

func (f *fakeRepository) BulkSetMessageFlag(_ context.Context, req maildb.BulkMessageFlagRequest) (int64, error) {
	return int64(len(req.MessageIDs)), nil
}

func (f *fakeRepository) MoveMessage(context.Context, string, string, string) error {
	return nil
}

func (f *fakeRepository) BulkMoveMessages(_ context.Context, req maildb.BulkMessageMoveRequest) (int64, error) {
	return int64(len(req.MessageIDs)), nil
}

func (f *fakeRepository) DeleteMessage(context.Context, string, string) error {
	return nil
}

func (f *fakeRepository) BulkDeleteMessages(_ context.Context, req maildb.BulkMessageDeleteRequest) (int64, error) {
	return int64(len(req.MessageIDs)), nil
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
	if f.recordErr != nil {
		return "", f.recordErr
	}
	return "msg-1", nil
}

func (f *fakeRepository) SuppressedRecipients(_ context.Context, _ string, recipients []string) ([]string, error) {
	f.seenSuppressionRecipients = append([]string(nil), recipients...)
	return f.suppressed, nil
}

func (f *fakeRepository) SaveDraft(_ context.Context, req maildb.SaveDraftRequest) (maildb.MessageDetail, error) {
	f.lastDraft = req
	return maildb.MessageDetail{ID: "draft-1", Subject: req.Subject}, nil
}

func (f *fakeRepository) DeleteDraft(context.Context, string, string) error {
	return nil
}

func (f *fakeRepository) GetDraftForSend(context.Context, string, string) (maildb.DraftForSend, error) {
	return maildb.DraftForSend{
		ID:       "draft-1",
		UserID:   "user-1",
		Intent:   string(ComposeIntentNew),
		To:       []outbound.Address{{Email: "recipient@example.net"}},
		Subject:  "draft subject",
		TextBody: "draft body",
	}, nil
}

func (f *fakeRepository) MarkDraftSent(_ context.Context, _ string, draftID string, sentMessageID string) error {
	f.lastSentDraftID = draftID
	f.lastSentDraftMessageID = sentMessageID
	return nil
}

func (f *fakeRepository) CreateAttachmentUpload(_ context.Context, req maildb.CreateAttachmentUploadRequest) (maildb.Attachment, error) {
	f.lastAttachmentUpload = req
	return maildb.Attachment{ID: "att-1", Filename: req.Filename, MIMEType: req.MIMEType, Size: req.Size}, nil
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

func TestSendTextReturnsRecordErrorAfterStorageWrite(t *testing.T) {
	t.Parallel()

	service := New(&fakeRepository{recordErr: fmt.Errorf("record failed")}, storage.NewLocalStore(t.TempDir()))
	_, err := service.SendText(context.Background(), SendTextRequest{
		UserID:   "user-1",
		To:       []outbound.Address{{Email: "user@example.net"}},
		Subject:  "hello",
		TextBody: "body",
	})
	if err == nil {
		t.Fatal("SendText succeeded despite record failure")
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

func TestSendTextMarksReplySourceAnswered(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, storage.NewLocalStore(t.TempDir()))
	_, err := service.SendText(context.Background(), SendTextRequest{
		UserID:          "user-1",
		Intent:          ComposeIntentReply,
		SourceMessageID: "msg-original",
		To:              []outbound.Address{{Email: "sender@example.net"}},
		Subject:         "Re: hello",
		TextBody:        "body",
	})
	if err != nil {
		t.Fatalf("SendText returned error: %v", err)
	}
	if repo.lastFlagMessageID != "msg-original" || repo.lastFlag != "answered" {
		t.Fatalf("flag = %q/%q", repo.lastFlagMessageID, repo.lastFlag)
	}
}

func TestSendDraftSendsAndMarksDraftSent(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, storage.NewLocalStore(t.TempDir()))
	result, err := service.SendDraft(context.Background(), "user-1", "draft-1")
	if err != nil {
		t.Fatalf("SendDraft returned error: %v", err)
	}
	if result.ID != "msg-1" {
		t.Fatalf("result = %+v", result)
	}
	if repo.lastSentDraftID != "draft-1" || repo.lastSentDraftMessageID != "msg-1" {
		t.Fatalf("sent draft marker = %q/%q", repo.lastSentDraftID, repo.lastSentDraftMessageID)
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

func TestListMessagesPageDelegatesCursor(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)
	cursor := maildb.MessageListCursor{ID: "11111111-1111-1111-1111-111111111111"}
	messages, err := service.ListMessagesPage(context.Background(), "user-1", "", 10, cursor)
	if err != nil {
		t.Fatalf("ListMessagesPage returned error: %v", err)
	}
	if len(messages) != 1 || messages[0].ID != "msg-page" {
		t.Fatalf("messages = %+v", messages)
	}
	if repo.lastPageCursor.ID != cursor.ID {
		t.Fatalf("cursor = %+v, want %+v", repo.lastPageCursor, cursor)
	}
}

func TestCreateAttachmentUploadDelegatesToRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	service := New(repo, nil)
	attachment, err := service.CreateAttachmentUpload(context.Background(), CreateAttachmentUploadRequest{
		UserID:   "user-1",
		Filename: "report.pdf",
		Size:     42,
		MIMEType: "application/pdf",
	})
	if err != nil {
		t.Fatalf("CreateAttachmentUpload returned error: %v", err)
	}
	if attachment.ID != "att-1" || repo.lastAttachmentUpload.Filename != "report.pdf" {
		t.Fatalf("attachment = %+v last = %+v", attachment, repo.lastAttachmentUpload)
	}
}

func TestUploadAttachmentWritesStorageAndRecordsMetadata(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	store := storage.NewLocalStore(t.TempDir())
	service := New(repo, store)

	attachment, err := service.UploadAttachment(context.Background(), UploadAttachmentRequest{
		UserID:   "user-1",
		DraftID:  "draft-1",
		Filename: "report.pdf",
		Size:     7,
		MIMEType: "application/pdf",
		Body:     strings.NewReader("content"),
	})
	if err != nil {
		t.Fatalf("UploadAttachment returned error: %v", err)
	}
	if attachment.ID != "att-1" {
		t.Fatalf("attachment = %+v", attachment)
	}
	if repo.lastAttachmentUpload.StoragePath == "" {
		t.Fatal("StoragePath was not recorded")
	}
	body, err := store.Get(context.Background(), repo.lastAttachmentUpload.StoragePath)
	if err != nil {
		t.Fatalf("stored attachment missing: %v", err)
	}
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(raw) != "content" {
		t.Fatalf("stored body = %q", raw)
	}
}

func TestUploadAttachmentRejectsBodyLargerThanLimit(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	store := storage.NewLocalStore(t.TempDir())
	service := New(repo, store)

	_, err := service.UploadAttachment(context.Background(), UploadAttachmentRequest{
		UserID:   "user-1",
		Filename: "large.bin",
		Size:     1,
		MIMEType: "application/octet-stream",
		Body:     bytes.NewReader(bytes.Repeat([]byte("x"), int(MaxAttachmentUploadBytes)+1)),
	})
	if err == nil {
		t.Fatal("UploadAttachment accepted oversized body")
	}
	if repo.lastAttachmentUpload.Filename != "" {
		t.Fatalf("metadata should not be recorded: %+v", repo.lastAttachmentUpload)
	}
}

func TestUploadAttachmentRejectsDeclaredSizeMismatch(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	store := storage.NewLocalStore(t.TempDir())
	service := New(repo, store)

	_, err := service.UploadAttachment(context.Background(), UploadAttachmentRequest{
		UserID:   "user-1",
		Filename: "report.pdf",
		Size:     99,
		MIMEType: "application/pdf",
		Body:     strings.NewReader("content"),
	})
	if err == nil {
		t.Fatal("UploadAttachment accepted mismatched declared size")
	}
	if repo.lastAttachmentUpload.Filename != "" {
		t.Fatalf("metadata should not be recorded: %+v", repo.lastAttachmentUpload)
	}
}
