package delivery

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/storage"
)

func TestHandlerDeliversQueuedMessage(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	if err := store.Put(context.Background(), "mailstore/msg.eml", strings.NewReader("Subject: hello\r\n\r\nbody")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	transport := &fakeTransport{}
	handler := NewHandler(store, transport)

	err := handler.HandleEvent(context.Background(), eventstream.Message{
		ID: "1-0",
		Payload: []byte(`{
			"event":"mail.queued",
			"message_id":"msg-1",
			"from":{"email":"sender@example.com"},
			"to":[{"email":"recipient@example.net"}],
			"storage_path":"mailstore/msg.eml",
			"farm":"general"
		}`),
	})
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if transport.delivered.MessageID != "msg-1" {
		t.Fatalf("MessageID = %q, want msg-1", transport.delivered.MessageID)
	}
	if !strings.Contains(transport.raw, "Subject: hello") {
		t.Fatalf("raw = %q", transport.raw)
	}
}

func TestDecodeQueuedMessageRejectsWrongEvent(t *testing.T) {
	t.Parallel()

	_, err := DecodeQueuedMessage([]byte(`{"event":"mail.stored"}`))
	if err == nil {
		t.Fatal("DecodeQueuedMessage accepted wrong event")
	}
}

type fakeTransport struct {
	delivered QueuedMessage
	raw       string
}

func (t *fakeTransport) Deliver(ctx context.Context, job Job) error {
	t.delivered = job.QueuedMessage
	body, err := job.OpenMessage(ctx)
	if err != nil {
		return err
	}
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	t.raw = string(raw)
	return nil
}
