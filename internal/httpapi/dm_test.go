package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/dm"
)

func TestDMMessagesHandlerAddsAttachmentDownloadURLs(t *testing.T) {
	t.Parallel()

	service := &fakeDMRouteService{
		messages: []dm.Message{
			{
				ID:                 "msg-file",
				RoomID:             "room-1",
				MessageType:        dm.MessageTypeFile,
				AttachmentName:     "photo.png",
				AttachmentMIMEType: "image/png",
			},
			{ID: "msg-text", RoomID: "room-1", MessageType: dm.MessageTypeText, Body: "hello"},
		},
	}
	mux := http.NewServeMux()
	RegisterDMRoutes(mux, service, nil, "https://mail.example")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dm/rooms/room-1/messages?user_id=user-1&company_id=company-1&domain_id=domain-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Messages []dm.Message `json:"messages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(body.Messages) != 2 {
		t.Fatalf("messages = %+v", body.Messages)
	}
	wantURL := "https://mail.example/api/v1/dm/messages/msg-file/attachment?token=token-msg-file"
	if body.Messages[0].AttachmentDownloadURL != wantURL {
		t.Fatalf("file attachment_download_url = %q, want %q", body.Messages[0].AttachmentDownloadURL, wantURL)
	}
	if body.Messages[1].AttachmentDownloadURL != "" {
		t.Fatalf("text attachment_download_url = %q", body.Messages[1].AttachmentDownloadURL)
	}
}

func TestDMAttachmentUploadHandlerAddsAttachmentDownloadURL(t *testing.T) {
	t.Parallel()

	service := &fakeDMRouteService{
		attachmentMessage: dm.Message{
			ID:                 "upload-msg",
			RoomID:             "room-1",
			MessageType:        dm.MessageTypeFile,
			AttachmentName:     "photo.png",
			AttachmentMIMEType: "image/png",
		},
	}
	mux := http.NewServeMux()
	RegisterDMRoutes(mux, service, nil, "")

	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	part, err := writer.CreateFormFile("file", "photo.png")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := part.Write([]byte("png-body")); err != nil {
		t.Fatalf("part.Write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dm/rooms/room-1/attachments?user_id=user-1&company_id=company-1&domain_id=domain-1", &form)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Message dm.Message `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	wantURL := "/api/v1/dm/messages/upload-msg/attachment?token=token-upload-msg"
	if body.Message.AttachmentDownloadURL != wantURL {
		t.Fatalf("attachment_download_url = %q, want %q", body.Message.AttachmentDownloadURL, wantURL)
	}
	if service.upload.Filename != "photo.png" || service.upload.Size != int64(len("png-body")) {
		t.Fatalf("upload = %+v", service.upload)
	}
}

type fakeDMRouteService struct {
	messages          []dm.Message
	attachmentMessage dm.Message
	upload            dm.AttachmentUpload
}

func (f *fakeDMRouteService) CreateRoom(context.Context, dm.Principal, dm.CreateRoomRequest) (dm.Room, error) {
	return dm.Room{}, nil
}

func (f *fakeDMRouteService) ListRooms(context.Context, dm.Principal) ([]dm.Room, error) {
	return nil, nil
}

func (f *fakeDMRouteService) ListPublicRooms(context.Context, dm.Principal) ([]dm.Room, error) {
	return nil, nil
}

func (f *fakeDMRouteService) AddMembers(context.Context, dm.Principal, string, []string) ([]dm.Message, error) {
	return nil, nil
}

func (f *fakeDMRouteService) RemoveMember(context.Context, dm.Principal, string, string) (dm.RoomRemoval, error) {
	return dm.RoomRemoval{}, nil
}

func (f *fakeDMRouteService) TransferOwner(context.Context, dm.Principal, string, string) (dm.Message, error) {
	return dm.Message{}, nil
}

func (f *fakeDMRouteService) CreateInvite(context.Context, dm.Principal, string) (dm.Invite, error) {
	return dm.Invite{}, nil
}

func (f *fakeDMRouteService) JoinInvite(context.Context, dm.Principal, string) (dm.Message, error) {
	return dm.Message{}, nil
}

func (f *fakeDMRouteService) ListMessages(context.Context, dm.Principal, string, dm.MessageCursor) ([]dm.Message, error) {
	return f.messages, nil
}

func (f *fakeDMRouteService) SendMessage(context.Context, dm.Principal, string, dm.SendMessageRequest) (dm.Message, error) {
	return dm.Message{}, nil
}

func (f *fakeDMRouteService) SendAttachment(_ context.Context, _ dm.Principal, _ string, upload dm.AttachmentUpload) (dm.Message, error) {
	f.upload = upload
	return f.attachmentMessage, nil
}

func (f *fakeDMRouteService) EditMessage(context.Context, dm.Principal, string, string) (dm.Message, error) {
	return dm.Message{}, nil
}

func (f *fakeDMRouteService) DeleteMessage(context.Context, dm.Principal, string) (dm.Message, error) {
	return dm.Message{}, nil
}

func (f *fakeDMRouteService) ToggleReaction(context.Context, dm.Principal, string, string) error {
	return nil
}

func (f *fakeDMRouteService) MarkRead(context.Context, dm.Principal, string, string) error {
	return nil
}

func (f *fakeDMRouteService) Search(context.Context, dm.Principal, string, string, string, int) ([]dm.SearchResult, error) {
	return nil, nil
}

func (f *fakeDMRouteService) ListMedia(context.Context, dm.Principal, string, dm.MediaQuery) ([]dm.MediaItem, error) {
	return nil, nil
}

func (f *fakeDMRouteService) SignAttachmentDownload(messageID string, _ time.Time) (string, error) {
	return "token-" + messageID, nil
}

func (f *fakeDMRouteService) VerifyAttachmentDownload(token string) (string, error) {
	return strings.TrimPrefix(token, "token-"), nil
}

func (f *fakeDMRouteService) OpenAttachment(context.Context, string) (dm.AttachmentDownload, error) {
	return dm.AttachmentDownload{Body: io.NopCloser(strings.NewReader(""))}, nil
}
