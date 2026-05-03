package httpapi

import (
	"context"
	"encoding/json"
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
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Messages) != 1 || body.Messages[0].Subject != "hello" {
		t.Fatalf("messages = %+v", body.Messages)
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
			{ID: "folder-1", Name: "Inbox", FullPath: "Inbox", Type: "system", SystemType: "inbox"},
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
	if service.lastUserID != "user-1" {
		t.Fatalf("lastUserID = %q", service.lastUserID)
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
	folders          []maildb.Folder
	list             []maildb.MessageSummary
	detail           maildb.MessageDetail
	sendResult       mailservice.SendTextResult
	lastSend         mailservice.SendTextRequest
	lastUserID       string
	lastMessageID    string
	lastFolderID     string
	lastMoveFolderID string
	lastDeletedID    string
	lastFlag         string
	lastFlagValue    bool
	lastLimit        int
}

func (f *fakeMessageService) ListFolders(_ context.Context, userID string) ([]maildb.Folder, error) {
	f.lastUserID = userID
	return f.folders, nil
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

func (f *fakeMessageService) SendText(_ context.Context, req mailservice.SendTextRequest) (mailservice.SendTextResult, error) {
	f.lastSend = req
	return f.sendResult, nil
}
