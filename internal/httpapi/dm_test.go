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
	exportResult      dm.RoomExport
	pins              []dm.PinnedMessage
	pinCall           dmPinCall
	unpinCall         dmPinCall
	pinErr            error
	unpinErr          error
	listPinsErr       error
	listPinsRoomID    string
	listPinsLimit     int
	listPinsOffset    int
}

type dmPinCall struct {
	roomID    string
	messageID string
	note      string
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

func (f *fakeDMRouteService) ExportRoom(_ context.Context, _ dm.Principal, _ string) (dm.RoomExport, error) {
	return f.exportResult, nil
}

func (f *fakeDMRouteService) RotateRoomKey(context.Context, dm.Principal, string) error {
	return nil
}

func (f *fakeDMRouteService) PinMessage(_ context.Context, _ dm.Principal, roomID string, messageID string, note string) error {
	f.pinCall = dmPinCall{roomID: roomID, messageID: messageID, note: note}
	return f.pinErr
}

func (f *fakeDMRouteService) UnpinMessage(_ context.Context, _ dm.Principal, roomID string, messageID string) error {
	f.unpinCall = dmPinCall{roomID: roomID, messageID: messageID}
	return f.unpinErr
}

func (f *fakeDMRouteService) ListPinnedMessages(_ context.Context, _ dm.Principal, roomID string, limit int, offset int) ([]dm.PinnedMessage, error) {
	f.listPinsRoomID = roomID
	f.listPinsLimit = limit
	f.listPinsOffset = offset
	return f.pins, f.listPinsErr
}

func TestDMExportRoomRespondsWithTextFile(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	svc := &fakeDMRouteService{}
	svc.exportResult = dm.RoomExport{
		Room: dm.Room{ID: "room-1", RoomType: dm.RoomTypeDirect, Name: "test-room",
			Members: []dm.User{{ID: "u1", DisplayName: "Alice", Email: "alice@example.com"}}},
		Messages: []dm.Message{
			{ID: "m1", SenderID: "u1", MessageType: dm.MessageTypeText, Body: "hello", CreatedAt: now},
		},
		ExportAt: now,
	}
	mux := http.NewServeMux()
	RegisterDMRoutes(mux, svc, nil, "")
	req := httptest.NewRequest("GET", "/api/v1/dm/rooms/room-1/export?user_id=u1&company_id=c1&domain_id=d1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("Content-Type = %q, want text/plain", ct)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") {
		t.Fatalf("Content-Disposition = %q, want attachment", cd)
	}
	if !strings.Contains(cd, "test-room") {
		t.Fatalf("Content-Disposition missing room name: %q", cd)
	}
	if !strings.Contains(w.Body.String(), "hello") {
		t.Fatalf("body missing message text, got:\n%s", w.Body.String())
	}
}

func TestDMPinMessageHandlerForwardsNote(t *testing.T) {
	t.Parallel()
	svc := &fakeDMRouteService{}
	mux := http.NewServeMux()
	RegisterDMRoutes(mux, svc, nil, "")

	body := strings.NewReader(`{"note":"important message"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dm/rooms/room-1/messages/msg-1/pin?user_id=u1&company_id=c1&domain_id=d1", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.pinCall.roomID != "room-1" || svc.pinCall.messageID != "msg-1" || svc.pinCall.note != "important message" {
		t.Fatalf("pin call = %+v", svc.pinCall)
	}
}

func TestDMPinMessageHandlerAllowsEmptyBody(t *testing.T) {
	t.Parallel()
	svc := &fakeDMRouteService{}
	mux := http.NewServeMux()
	RegisterDMRoutes(mux, svc, nil, "")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dm/rooms/room-1/messages/msg-1/pin?user_id=u1&company_id=c1&domain_id=d1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.pinCall.messageID != "msg-1" || svc.pinCall.note != "" {
		t.Fatalf("pin call = %+v", svc.pinCall)
	}
}

func TestDMPinMessageHandlerMapsForbidden(t *testing.T) {
	t.Parallel()
	svc := &fakeDMRouteService{pinErr: dm.ErrForbidden}
	mux := http.NewServeMux()
	RegisterDMRoutes(mux, svc, nil, "")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dm/rooms/room-1/messages/msg-1/pin?user_id=u1&company_id=c1&domain_id=d1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
}

func TestDMPinMessageHandlerRequiresIdentity(t *testing.T) {
	t.Parallel()
	svc := &fakeDMRouteService{}
	mux := http.NewServeMux()
	RegisterDMRoutes(mux, svc, nil, "")

	// Missing company_id/domain_id → unauthorized/bad request, handler must not call the service.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dm/rooms/room-1/messages/msg-1/pin?user_id=u1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code == http.StatusNoContent {
		t.Fatalf("status = %d, want an auth failure, not success", rec.Code)
	}
	if svc.pinCall.messageID != "" {
		t.Fatalf("service must not be called without identity, got %+v", svc.pinCall)
	}
}

func TestDMUnpinMessageHandlerForwards(t *testing.T) {
	t.Parallel()
	svc := &fakeDMRouteService{}
	mux := http.NewServeMux()
	RegisterDMRoutes(mux, svc, nil, "")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/dm/rooms/room-1/messages/msg-1/pin?user_id=u1&company_id=c1&domain_id=d1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.unpinCall.roomID != "room-1" || svc.unpinCall.messageID != "msg-1" {
		t.Fatalf("unpin call = %+v", svc.unpinCall)
	}
}

func TestDMListPinsHandlerReturnsPinsAndAttachmentURLs(t *testing.T) {
	t.Parallel()
	pinnedAt := time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC)
	svc := &fakeDMRouteService{
		pins: []dm.PinnedMessage{
			{
				Message:        dm.Message{ID: "msg-file", RoomID: "room-1", MessageType: dm.MessageTypeFile, AttachmentName: "a.png"},
				PinnedByUserID: "u2",
				PinnedAt:       pinnedAt,
				Note:           "look",
			},
			{
				Message:        dm.Message{ID: "msg-text", RoomID: "room-1", MessageType: dm.MessageTypeText, Body: "hi"},
				PinnedByUserID: "u1",
				PinnedAt:       pinnedAt,
			},
		},
	}
	mux := http.NewServeMux()
	RegisterDMRoutes(mux, svc, nil, "https://mail.example")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dm/rooms/room-1/pins?user_id=u1&company_id=c1&domain_id=d1&limit=10&offset=5", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.listPinsRoomID != "room-1" || svc.listPinsLimit != 10 || svc.listPinsOffset != 5 {
		t.Fatalf("list params: room=%q limit=%d offset=%d", svc.listPinsRoomID, svc.listPinsLimit, svc.listPinsOffset)
	}
	var body struct {
		Pins []dm.PinnedMessage `json:"pins"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(body.Pins) != 2 {
		t.Fatalf("pins = %+v", body.Pins)
	}
	wantURL := "https://mail.example/api/v1/dm/messages/msg-file/attachment?token=token-msg-file"
	if body.Pins[0].Message.AttachmentDownloadURL != wantURL {
		t.Fatalf("file pin download url = %q, want %q", body.Pins[0].Message.AttachmentDownloadURL, wantURL)
	}
	if body.Pins[1].Message.AttachmentDownloadURL != "" {
		t.Fatalf("text pin should have no download url, got %q", body.Pins[1].Message.AttachmentDownloadURL)
	}
	if body.Pins[0].Note != "look" || body.Pins[0].PinnedByUserID != "u2" {
		t.Fatalf("pin metadata = %+v", body.Pins[0])
	}
}

func TestDMListPinsHandlerRejectsInvalidOffset(t *testing.T) {
	t.Parallel()
	svc := &fakeDMRouteService{}
	mux := http.NewServeMux()
	RegisterDMRoutes(mux, svc, nil, "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dm/rooms/room-1/pins?user_id=u1&company_id=c1&domain_id=d1&offset=-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if svc.listPinsRoomID != "" {
		t.Fatalf("service must not be called on invalid offset")
	}
}
