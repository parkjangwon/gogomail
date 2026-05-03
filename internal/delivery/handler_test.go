package delivery

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/storage"
)

var errBoom = errors.New("boom")

func TestHandlerDeliversQueuedMessage(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	if err := store.Put(context.Background(), "mailstore/msg.eml", strings.NewReader("Subject: hello\r\n\r\nbody")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	transport := &fakeTransport{}
	recorder := &fakeRecorder{}
	handler := NewHandler(store, transport, recorder, nil)

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
	if len(recorder.attempts) != 1 || recorder.attempts[0].Status != AttemptDelivered {
		t.Fatalf("attempts = %+v, want delivered attempt", recorder.attempts)
	}
}

func TestHandlerSchedulesRetryAfterFailure(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	if err := store.Put(context.Background(), "mailstore/msg.eml", strings.NewReader("Subject: hello\r\n\r\nbody")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	recorder := &fakeRecorder{}
	retry := &fakeRetryScheduler{}
	handler := NewHandler(store, &fakeTransport{err: errBoom}, recorder, retry)

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
	if len(recorder.attempts) != 1 || recorder.attempts[0].Status != AttemptFailed {
		t.Fatalf("attempts = %+v, want failed attempt", recorder.attempts)
	}
	if retry.scheduled.MessageID != "msg-1" {
		t.Fatalf("scheduled message = %+v", retry.scheduled)
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
	err       error
}

func (t *fakeTransport) Deliver(ctx context.Context, job Job) error {
	if t.err != nil {
		return t.err
	}
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

type fakeRecorder struct {
	attempts []Attempt
}

func (r *fakeRecorder) RecordAttempt(_ context.Context, attempt Attempt) error {
	r.attempts = append(r.attempts, attempt)
	return nil
}

type fakeRetryScheduler struct {
	scheduled QueuedMessage
}

func (s *fakeRetryScheduler) ScheduleRetry(_ context.Context, job Job, _ error) error {
	s.scheduled = job.QueuedMessage
	return nil
}
