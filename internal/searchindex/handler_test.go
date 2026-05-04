package searchindex

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/eventstream"
)

func TestHandlerIndexesStoredMessageBody(t *testing.T) {
	store := fakeStore{
		"domains/example/users/user-1/messages/msg-1.eml": "Message-ID: <rfc-1@example.com>\r\nSubject: Hello\r\nFrom: Alice <alice@example.com>\r\nTo: Bob <bob@example.com>\r\n\r\nBody for search.\r\n",
	}
	indexer := &fakeIndexer{}
	handler := NewHandler(store, indexer, HandlerOptions{MaxTextBodyBytes: 64})

	err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: mustJSON(t, Event{
		Event:        "mail.stored",
		MessageID:    "msg-1",
		RFCMessageID: "<rfc-1@example.com>",
		CompanyID:    "company-1",
		DomainID:     "domain-1",
		UserID:       "user-1",
		FolderID:     "folder-1",
		Recipient:    "bob@example.com",
		Subject:      "Hello",
		StoragePath:  "domains/example/users/user-1/messages/msg-1.eml",
		Size:         127,
	})})
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}

	if len(indexer.documents) != 1 {
		t.Fatalf("indexed documents = %d, want 1", len(indexer.documents))
	}
	doc := indexer.documents[0]
	if doc.MessageID != "msg-1" {
		t.Fatalf("MessageID = %q, want msg-1", doc.MessageID)
	}
	if doc.UserID != "user-1" {
		t.Fatalf("UserID = %q, want user-1", doc.UserID)
	}
	if doc.FolderID != "folder-1" {
		t.Fatalf("FolderID = %q, want folder-1", doc.FolderID)
	}
	if doc.StoragePath != "domains/example/users/user-1/messages/msg-1.eml" {
		t.Fatalf("StoragePath = %q", doc.StoragePath)
	}
	if doc.BodyText != "Body for search." {
		t.Fatalf("BodyText = %q, want parsed plain body", doc.BodyText)
	}
	if doc.FromAddr != "alice@example.com" || doc.FromName != "Alice" {
		t.Fatalf("from = %q/%q, want parsed sender", doc.FromAddr, doc.FromName)
	}
	if doc.BodyTruncated {
		t.Fatal("BodyTruncated = true, want false")
	}
}

func TestHandlerRejectsMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		payload Event
		want    string
	}{
		{
			name: "event",
			payload: Event{
				MessageID:   "msg-1",
				UserID:      "user-1",
				StoragePath: "messages/msg-1.eml",
			},
			want: "event",
		},
		{
			name: "message id",
			payload: Event{
				Event:       "mail.stored",
				UserID:      "user-1",
				StoragePath: "messages/msg-1.eml",
			},
			want: "message_id",
		},
		{
			name: "user id",
			payload: Event{
				Event:       "mail.stored",
				MessageID:   "msg-1",
				StoragePath: "messages/msg-1.eml",
			},
			want: "user_id",
		},
		{
			name: "storage path",
			payload: Event{
				Event:     "mail.stored",
				MessageID: "msg-1",
				UserID:    "user-1",
			},
			want: "storage_path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indexer := &fakeIndexer{}
			handler := NewHandler(fakeStore{}, indexer, HandlerOptions{})

			err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: mustJSON(t, tt.payload)})
			if err == nil {
				t.Fatal("HandleEvent returned nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want it to mention %q", err.Error(), tt.want)
			}
			if len(indexer.documents) != 0 {
				t.Fatalf("indexed documents = %d, want 0", len(indexer.documents))
			}
		})
	}
}

func TestHandlerRejectsUnsupportedSchemaVersion(t *testing.T) {
	t.Parallel()

	indexer := &fakeIndexer{}
	handler := NewHandler(fakeStore{}, indexer, HandlerOptions{})

	err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: mustJSON(t, Event{
		Event:         "mail.stored",
		SchemaVersion: "2099-01-01.mail-stored.v9",
		MessageID:     "msg-1",
		UserID:        "user-1",
		StoragePath:   "messages/msg-1.eml",
	})})
	if err == nil {
		t.Fatal("HandleEvent returned nil, want schema version validation error")
	}
	if !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("error = %q, want schema_version", err.Error())
	}
}

func TestHandlerRejectsOversizedRequiredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload Event
		want    string
	}{
		{
			name: "message id",
			payload: Event{
				Event:       "mail.stored",
				MessageID:   strings.Repeat("m", maxEventIdentityBytes+1),
				UserID:      "user-1",
				StoragePath: "messages/msg-1.eml",
			},
			want: "message_id",
		},
		{
			name: "user id",
			payload: Event{
				Event:       "mail.stored",
				MessageID:   "msg-1",
				UserID:      strings.Repeat("u", maxEventIdentityBytes+1),
				StoragePath: "messages/msg-1.eml",
			},
			want: "user_id",
		},
		{
			name: "storage path",
			payload: Event{
				Event:       "mail.stored",
				MessageID:   "msg-1",
				UserID:      "user-1",
				StoragePath: strings.Repeat("a", maxEventStoragePathBytes) + ".eml",
			},
			want: "storage_path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := NewHandler(fakeStore{}, &fakeIndexer{}, HandlerOptions{})
			err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: mustJSON(t, tt.payload)})
			if err == nil {
				t.Fatal("HandleEvent returned nil, want oversized field validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestHandlerRejectsAmbiguousStoragePath(t *testing.T) {
	t.Parallel()

	tests := []string{
		"messages/../secret.eml",
		"messages//msg-1.eml",
		"./messages/msg-1.eml",
		"messages\\msg-1.eml",
		"/messages/msg-1.eml",
	}

	for _, storagePath := range tests {
		t.Run(storagePath, func(t *testing.T) {
			t.Parallel()

			handler := NewHandler(fakeStore{}, &fakeIndexer{}, HandlerOptions{})
			err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: mustJSON(t, Event{
				Event:       "mail.stored",
				MessageID:   "msg-1",
				UserID:      "user-1",
				StoragePath: storagePath,
			})})
			if err == nil {
				t.Fatal("HandleEvent returned nil, want storage path validation error")
			}
			if !strings.Contains(err.Error(), "storage_path") {
				t.Fatalf("error = %q, want storage_path", err.Error())
			}
		})
	}
}

func TestHandlerIndexesBoundedTruncatedBody(t *testing.T) {
	store := fakeStore{
		"messages/msg-1.eml": "Subject: Long\r\n\r\nabcdefghijklmnopqrstuvwxyz",
	}
	indexer := &fakeIndexer{}
	handler := NewHandler(store, indexer, HandlerOptions{MaxTextBodyBytes: 10})

	err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: mustJSON(t, Event{
		Event:       "mail.stored",
		MessageID:   "msg-1",
		UserID:      "user-1",
		StoragePath: "messages/msg-1.eml",
	})})
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}

	if len(indexer.documents) != 1 {
		t.Fatalf("indexed documents = %d, want 1", len(indexer.documents))
	}
	doc := indexer.documents[0]
	if doc.BodyText != "abcdefghij" {
		t.Fatalf("BodyText = %q, want first 10 bytes", doc.BodyText)
	}
	if !doc.BodyTruncated {
		t.Fatal("BodyTruncated = false, want true")
	}
	if doc.BodyMaxBytes != 10 {
		t.Fatalf("BodyMaxBytes = %d, want 10", doc.BodyMaxBytes)
	}
}

func TestDecodeEventCapsReferences(t *testing.T) {
	t.Parallel()

	refs := make([]string, 0, maxEventReferences+10)
	for i := 0; i < maxEventReferences+10; i++ {
		refs = append(refs, "<ref@example.com>")
	}
	event, err := DecodeEvent(mustJSON(t, Event{
		Event:       "mail.stored",
		MessageID:   "msg-1",
		UserID:      "user-1",
		StoragePath: "messages/msg-1.eml",
		References:  refs,
	}))
	if err != nil {
		t.Fatalf("DecodeEvent returned error: %v", err)
	}
	if len(event.References) != maxEventReferences {
		t.Fatalf("References = %d, want %d", len(event.References), maxEventReferences)
	}
}

type fakeStore map[string]string

func (s fakeStore) Open(_ context.Context, path string) (io.ReadCloser, error) {
	if body, ok := s[path]; ok {
		return io.NopCloser(strings.NewReader(body)), nil
	}
	return nil, errFakeNotFound(path)
}

type errFakeNotFound string

func (e errFakeNotFound) Error() string {
	return "not found: " + string(e)
}

type fakeIndexer struct {
	documents []Document
}

func (i *fakeIndexer) IndexMessage(_ context.Context, doc Document) error {
	i.documents = append(i.documents, doc)
	return nil
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	return raw
}
