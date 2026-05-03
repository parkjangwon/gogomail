package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/maildb"
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
	RegisterMailRoutes(mux, service)

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
	RegisterMailRoutes(mux, service)

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

type fakeMessageService struct {
	list          []maildb.MessageSummary
	detail        maildb.MessageDetail
	lastUserID    string
	lastMessageID string
	lastLimit     int
}

func (f *fakeMessageService) ListMessages(_ context.Context, userID string, limit int) ([]maildb.MessageSummary, error) {
	f.lastUserID = userID
	f.lastLimit = limit
	return f.list, nil
}

func (f *fakeMessageService) GetMessage(_ context.Context, userID string, messageID string) (maildb.MessageDetail, error) {
	f.lastUserID = userID
	f.lastMessageID = messageID
	return f.detail, nil
}
