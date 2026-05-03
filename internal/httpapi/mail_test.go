package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
	"github.com/gogomail/gogomail/internal/outbound"
)

func TestListMessagesHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		list: []maildb.MessageSummary{
			{
				ID:            "msg-1",
				Subject:       "hello",
				FromAddr:      "sender@example.net",
				FromName:      "Sender",
				ReceivedAt:    time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
				Size:          123,
				HasAttachment: true,
				Read:          false,
				Starred:       true,
			},
		},
	}

	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=user-1&limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Messages []maildb.MessageSummary `json:"messages"`
		Limit    int                     `json:"limit"`
		HasMore  bool                    `json:"has_more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Messages) != 1 || body.Messages[0].Subject != "hello" {
		t.Fatalf("messages = %+v", body.Messages)
	}
	if body.Limit != 10 || body.HasMore {
		t.Fatalf("page metadata = limit:%d has_more:%v", body.Limit, body.HasMore)
	}
	if service.lastUserID != "user-1" {
		t.Fatalf("lastUserID = %q", service.lastUserID)
	}
	if service.lastLimit != 10 {
		t.Fatalf("lastLimit = %d", service.lastLimit)
	}
}

func TestListFoldersHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		folders: []maildb.Folder{
			{ID: "folder-1", Name: "Inbox", FullPath: "Inbox", Type: "system", SystemType: "inbox", Starred: 2},
		},
	}

	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/folders?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Folders []maildb.Folder `json:"folders"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Folders) != 1 || body.Folders[0].SystemType != "inbox" {
		t.Fatalf("folders = %+v", body.Folders)
	}
	if body.Folders[0].Starred != 2 {
		t.Fatalf("starred = %d", body.Folders[0].Starred)
	}
	if service.lastUserID != "user-1" {
		t.Fatalf("lastUserID = %q", service.lastUserID)
	}
}

func TestCreateFolderHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/folders?user_id=user-1", strings.NewReader(`{"name":"Projects"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Folder maildb.Folder `json:"folder"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Folder.Name != "Projects" || body.Folder.Type != "user" {
		t.Fatalf("folder = %+v", body.Folder)
	}
}

func TestRenameFolderHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/folders/folder-1?user_id=user-1", strings.NewReader(`{"name":"Renamed"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastFolderID != "folder-1" || service.lastFolderName != "Renamed" {
		t.Fatalf("rename = folder:%q name:%q", service.lastFolderID, service.lastFolderName)
	}
}

func TestDeleteFolderHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/folders/folder-1?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeletedFolderID != "folder-1" {
		t.Fatalf("lastDeletedFolderID = %q", service.lastDeletedFolderID)
	}
}

func TestListMessagesHandlerFiltersByFolder(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=user-1&folder_id=folder-1&limit=25", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastFolderID != "folder-1" {
		t.Fatalf("lastFolderID = %q", service.lastFolderID)
	}
	if service.lastLimit != 25 {
		t.Fatalf("lastLimit = %d", service.lastLimit)
	}
}

func TestListMessagesHandlerRejectsInvalidCursor(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=user-1&cursor=bad", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestGetMessageHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		detail: maildb.MessageDetail{
			ID:          "msg-1",
			Subject:     "hello",
			FromAddr:    "sender@example.net",
			FromName:    "Sender",
			ToAddrs:     json.RawMessage(`[{"name":"Admin","address":"admin@example.com"}]`),
			StoragePath: "mailstore/example.eml",
			TextBody:    "body",
		},
	}

	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/msg-1?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Message maildb.MessageDetail `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Message.ID != "msg-1" || body.Message.TextBody != "body" {
		t.Fatalf("message = %+v", body.Message)
	}
	if service.lastMessageID != "msg-1" {
		t.Fatalf("lastMessageID = %q", service.lastMessageID)
	}
}

func TestSetMessageFlagHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/messages/msg-1/flags?user_id=user-1", strings.NewReader(`{"flag":"read","value":true}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastMessageID != "msg-1" || service.lastFlag != "read" || !service.lastFlagValue {
		t.Fatalf("flag update = id:%q flag:%q value:%v", service.lastMessageID, service.lastFlag, service.lastFlagValue)
	}
}

func TestMoveMessageHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/messages/msg-1/folder?user_id=user-1", strings.NewReader(`{"folder_id":"folder-2"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastMessageID != "msg-1" || service.lastMoveFolderID != "folder-2" {
		t.Fatalf("move = id:%q folder:%q", service.lastMessageID, service.lastMoveFolderID)
	}
}

func TestDeleteMessageHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/messages/msg-1?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeletedID != "msg-1" {
		t.Fatalf("lastDeletedID = %q", service.lastDeletedID)
	}
}

func TestSaveDraftHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drafts", strings.NewReader(`{
		"user_id":"user-1",
		"subject":"draft",
		"text_body":"body"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDraft.UserID != "user-1" || service.lastDraft.Subject != "draft" {
		t.Fatalf("lastDraft = %+v", service.lastDraft)
	}
}

func TestDeleteDraftHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/drafts/draft-1?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDeletedDraftID != "draft-1" {
		t.Fatalf("lastDeletedDraftID = %q", service.lastDeletedDraftID)
	}
}

func TestUpdateDraftHandlerUsesPathID(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/drafts/draft-1", strings.NewReader(`{
		"user_id":"user-1",
		"draft_id":"ignored",
		"subject":"updated"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastDraft.DraftID != "draft-1" || service.lastDraft.Subject != "updated" {
		t.Fatalf("lastDraft = %+v", service.lastDraft)
	}
}

func TestListAttachmentsHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		attachments: []maildb.Attachment{
			{ID: "att-1", MessageID: "msg-1", Filename: "report.pdf", MIMEType: "application/pdf", Size: 42, Status: "stored"},
		},
	}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/msg-1/attachments?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Attachments []maildb.Attachment `json:"attachments"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Attachments) != 1 || body.Attachments[0].Filename != "report.pdf" {
		t.Fatalf("attachments = %+v", body.Attachments)
	}
	if service.lastMessageID != "msg-1" {
		t.Fatalf("lastMessageID = %q", service.lastMessageID)
	}
}

func TestCreateAttachmentUploadHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/attachments", strings.NewReader(`{
		"user_id":"user-1",
		"draft_id":"draft-1",
		"filename":"report.pdf",
		"size":42,
		"mime_type":"application/pdf"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastAttachmentUpload.DraftID != "draft-1" || service.lastAttachmentUpload.Filename != "report.pdf" {
		t.Fatalf("lastAttachmentUpload = %+v", service.lastAttachmentUpload)
	}
}

func TestDownloadAttachmentHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/msg-1/attachments/att-1/download?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "content" {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="report.pdf"`) {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func TestSendMessageHandler(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{
		sendResult: mailservice.SendTextResult{
			ID:           "msg-1",
			RFCMessageID: "<msg-1@example.com>",
			Farm:         outbound.FarmGeneral,
		},
	}

	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/send", strings.NewReader(`{
		"user_id":"user-1",
		"to":[{"email":"recipient@example.net"}],
		"subject":"hello",
		"text_body":"body"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastSend.UserID != "user-1" {
		t.Fatalf("lastSend.UserID = %q", service.lastSend.UserID)
	}
	if len(service.lastSend.To) != 1 || service.lastSend.To[0].Email != "recipient@example.net" {
		t.Fatalf("lastSend.To = %+v", service.lastSend.To)
	}
}

func TestSendReplyHandlerPassesSourceMessageID(t *testing.T) {
	t.Parallel()

	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/send", strings.NewReader(`{
		"user_id":"user-1",
		"intent":"reply",
		"source_message_id":"msg-original",
		"to":[{"email":"sender@example.net"}],
		"subject":"Re: hello",
		"text_body":"body"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastSend.Intent != mailservice.ComposeIntentReply || service.lastSend.SourceMessageID != "msg-original" {
		t.Fatalf("lastSend = %+v", service.lastSend)
	}
}

func TestListMessagesHandlerUsesJWTUser(t *testing.T) {
	t.Parallel()

	manager, err := auth.NewTokenManager("secret")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	token, err := manager.Sign(auth.Claims{UserID: "jwt-user"}, time.Minute)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserID != "jwt-user" {
		t.Fatalf("lastUserID = %q, want jwt-user", service.lastUserID)
	}
}

func TestMailRoutesRequireJWTWhenConfigured(t *testing.T) {
	t.Parallel()

	manager, err := auth.NewTokenManager("secret")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	service := &fakeMessageService{}
	mux := http.NewServeMux()
	RegisterMailRoutes(mux, service, manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

type fakeMessageService struct {
	folders              []maildb.Folder
	createdFolder        maildb.Folder
	list                 []maildb.MessageSummary
	attachments          []maildb.Attachment
	download             mailservice.AttachmentDownload
	detail               maildb.MessageDetail
	sendResult           mailservice.SendTextResult
	lastSend             mailservice.SendTextRequest
	lastDraft            mailservice.SaveDraftRequest
	lastAttachmentUpload mailservice.CreateAttachmentUploadRequest
	lastUserID           string
	lastFolderName       string
	lastDeletedFolderID  string
	lastMessageID        string
	lastFolderID         string
	lastMoveFolderID     string
	lastDeletedID        string
	lastDeletedDraftID   string
	lastFlag             string
	lastFlagValue        bool
	lastLimit            int
}

func (f *fakeMessageService) ListFolders(_ context.Context, userID string) ([]maildb.Folder, error) {
	f.lastUserID = userID
	return f.folders, nil
}

func (f *fakeMessageService) CreateFolder(_ context.Context, req maildb.CreateFolderRequest) (maildb.Folder, error) {
	f.lastUserID = req.UserID
	f.lastFolderName = req.Name
	if f.createdFolder.ID != "" {
		return f.createdFolder, nil
	}
	return maildb.Folder{ID: "folder-new", Name: req.Name, FullPath: req.Name, Type: "user"}, nil
}

func (f *fakeMessageService) RenameFolder(_ context.Context, userID string, folderID string, name string) (maildb.Folder, error) {
	f.lastUserID = userID
	f.lastFolderID = folderID
	f.lastFolderName = name
	return maildb.Folder{ID: folderID, Name: name, FullPath: name, Type: "user"}, nil
}

func (f *fakeMessageService) DeleteFolder(_ context.Context, userID string, folderID string) error {
	f.lastUserID = userID
	f.lastDeletedFolderID = folderID
	return nil
}

func (f *fakeMessageService) ListMessages(_ context.Context, userID string, limit int) ([]maildb.MessageSummary, error) {
	f.lastUserID = userID
	f.lastLimit = limit
	return f.list, nil
}

func (f *fakeMessageService) ListMessagesInFolder(_ context.Context, userID string, folderID string, limit int) ([]maildb.MessageSummary, error) {
	f.lastUserID = userID
	f.lastFolderID = folderID
	f.lastLimit = limit
	return f.list, nil
}

func (f *fakeMessageService) ListMessagesPage(_ context.Context, userID string, folderID string, limit int, _ maildb.MessageListCursor) ([]maildb.MessageSummary, error) {
	f.lastUserID = userID
	f.lastFolderID = folderID
	f.lastLimit = limit
	return f.list, nil
}

func (f *fakeMessageService) GetMessage(_ context.Context, userID string, messageID string) (maildb.MessageDetail, error) {
	f.lastUserID = userID
	f.lastMessageID = messageID
	return f.detail, nil
}

func (f *fakeMessageService) SetMessageFlag(_ context.Context, userID string, messageID string, flag string, value bool) error {
	f.lastUserID = userID
	f.lastMessageID = messageID
	f.lastFlag = flag
	f.lastFlagValue = value
	return nil
}

func (f *fakeMessageService) MoveMessage(_ context.Context, userID string, messageID string, folderID string) error {
	f.lastUserID = userID
	f.lastMessageID = messageID
	f.lastMoveFolderID = folderID
	return nil
}

func (f *fakeMessageService) DeleteMessage(_ context.Context, userID string, messageID string) error {
	f.lastUserID = userID
	f.lastDeletedID = messageID
	return nil
}

func (f *fakeMessageService) SaveDraft(_ context.Context, req mailservice.SaveDraftRequest) (maildb.MessageDetail, error) {
	f.lastDraft = req
	return maildb.MessageDetail{ID: "draft-1", Subject: req.Subject}, nil
}

func (f *fakeMessageService) DeleteDraft(_ context.Context, userID string, draftID string) error {
	f.lastUserID = userID
	f.lastDeletedDraftID = draftID
	return nil
}

func (f *fakeMessageService) CreateAttachmentUpload(_ context.Context, req mailservice.CreateAttachmentUploadRequest) (maildb.Attachment, error) {
	f.lastAttachmentUpload = req
	return maildb.Attachment{ID: "att-1", Filename: req.Filename, MIMEType: req.MIMEType, Size: req.Size}, nil
}

func (f *fakeMessageService) ListAttachments(_ context.Context, userID string, messageID string) ([]maildb.Attachment, error) {
	f.lastUserID = userID
	f.lastMessageID = messageID
	return f.attachments, nil
}

func (f *fakeMessageService) OpenAttachment(_ context.Context, userID string, messageID string, attachmentID string) (mailservice.AttachmentDownload, error) {
	f.lastUserID = userID
	f.lastMessageID = messageID
	if f.download.Body != nil {
		return f.download, nil
	}
	return mailservice.AttachmentDownload{
		Attachment: maildb.Attachment{ID: attachmentID, Filename: "report.pdf", MIMEType: "application/pdf", Size: 7},
		Body:       io.NopCloser(strings.NewReader("content")),
	}, nil
}

func (f *fakeMessageService) SendText(_ context.Context, req mailservice.SendTextRequest) (mailservice.SendTextResult, error) {
	f.lastSend = req
	return f.sendResult, nil
}
