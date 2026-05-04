package pushnotify

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gogomail/gogomail/internal/eventstream"
)

func TestHandlerEnqueuesMailStoredNotification(t *testing.T) {
	t.Parallel()

	sink := &fakeSink{}
	handler := NewHandler(sink)
	payload := json.RawMessage(`{
		"event":"mail.stored",
		"message_id":"msg-1",
		"rfc_message_id":"<msg-1@example.com>",
		"company_id":"company-1",
		"domain_id":"domain-1",
		"user_id":"user-1",
		"recipient":"user@example.com",
		"subject":"Hello",
		"received_at":"2026-05-04T00:00:00Z"
	}`)

	if err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: payload}); err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if sink.last.UserID != "user-1" || sink.last.MessageID != "msg-1" || sink.last.Subject != "Hello" {
		t.Fatalf("notification = %+v", sink.last)
	}
}

func TestHandlerResolvesNotificationTargets(t *testing.T) {
	t.Parallel()

	sink := &fakeSink{}
	recorder := &fakeCandidateRecorder{}
	resolver := &fakeTargetResolver{
		targets: []Target{{DeviceID: "device-1", Platform: "fcm", Token: "token-1", TokenSuffix: "token-1"}},
	}
	handler := NewHandler(sink, WithTargetResolver(resolver), WithCandidateRecorder(recorder))

	if err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: validMailStoredPayload()}); err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if resolver.last.UserID != "user-1" {
		t.Fatalf("resolver event = %+v", resolver.last)
	}
	if sink.calls != 1 || len(sink.last.Targets) != 1 || sink.last.Targets[0].DeviceID != "device-1" {
		t.Fatalf("sink calls=%d notification=%+v", sink.calls, sink.last)
	}
	if len(recorder.records) != 1 || recorder.records[0].MessageID != "msg-1" || recorder.records[0].DeviceID != "device-1" {
		t.Fatalf("records = %+v", recorder.records)
	}
	if sink.last.Targets[0].AttemptID != "attempt-1" {
		t.Fatalf("attempt id = %q, want attempt-1", sink.last.Targets[0].AttemptID)
	}
}

func TestHandlerRecordsQueuedOutcomesAfterSinkSuccess(t *testing.T) {
	t.Parallel()

	sink := &fakeSink{}
	recorder := &fakeCandidateRecorder{}
	outcomes := &fakeOutcomeRecorder{}
	resolver := &fakeTargetResolver{
		targets: []Target{{DeviceID: "device-1", Platform: "fcm", Token: "token-1", TokenSuffix: "token-1"}},
	}
	handler := NewHandler(
		sink,
		WithTargetResolver(resolver),
		WithCandidateRecorder(recorder),
		WithOutcomeRecorder(outcomes),
	)

	if err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: validMailStoredPayload()}); err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if len(outcomes.outcomes) != 1 {
		t.Fatalf("outcomes = %+v", outcomes.outcomes)
	}
	if outcomes.outcomes[0].AttemptID != "attempt-1" || outcomes.outcomes[0].Status != "queued" {
		t.Fatalf("outcome = %+v", outcomes.outcomes[0])
	}
}

func TestHandlerDoesNotRecordQueuedOutcomeWhenSinkFails(t *testing.T) {
	t.Parallel()

	sink := &fakeSink{err: errFakeSink}
	recorder := &fakeCandidateRecorder{}
	outcomes := &fakeOutcomeRecorder{}
	resolver := &fakeTargetResolver{
		targets: []Target{{DeviceID: "device-1", Platform: "fcm", Token: "token-1", TokenSuffix: "token-1"}},
	}
	handler := NewHandler(
		sink,
		WithTargetResolver(resolver),
		WithCandidateRecorder(recorder),
		WithOutcomeRecorder(outcomes),
	)

	if err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: validMailStoredPayload()}); err == nil {
		t.Fatal("HandleEvent returned nil error")
	}
	if len(outcomes.outcomes) != 0 {
		t.Fatalf("outcomes = %+v", outcomes.outcomes)
	}
}

func TestHandlerSkipsSinkWhenResolverHasNoTargets(t *testing.T) {
	t.Parallel()

	sink := &fakeSink{}
	handler := NewHandler(sink, WithTargetResolver(&fakeTargetResolver{}))

	if err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: validMailStoredPayload()}); err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if sink.calls != 0 {
		t.Fatalf("sink calls = %d, want 0", sink.calls)
	}
}

func TestDecodeEventRequiresStoredMessageIdentity(t *testing.T) {
	t.Parallel()

	_, err := DecodeEvent(json.RawMessage(`{"event":"mail.stored","user_id":"user-1"}`))
	if err == nil {
		t.Fatal("DecodeEvent accepted payload without message_id")
	}
}

func TestDecodeEventRejectsUnsupportedSchemaVersion(t *testing.T) {
	t.Parallel()

	_, err := DecodeEvent(json.RawMessage(`{
		"event":"mail.stored",
		"schema_version":"2099-01-01.mail-stored.v9",
		"message_id":"msg-1",
		"user_id":"user-1"
	}`))
	if err == nil {
		t.Fatal("DecodeEvent accepted unsupported schema version")
	}
}

type fakeSink struct {
	calls int
	last  Notification
	err   error
}

func (s *fakeSink) EnqueuePush(_ context.Context, notification Notification) error {
	s.calls++
	s.last = notification
	return s.err
}

type fakeTargetResolver struct {
	targets []Target
	last    Event
}

func (r *fakeTargetResolver) ResolvePushTargets(_ context.Context, event Event) ([]Target, error) {
	r.last = event
	return r.targets, nil
}

type fakeCandidateRecorder struct {
	records []CandidateRecord
}

func (r *fakeCandidateRecorder) RecordCandidate(_ context.Context, record CandidateRecord) (CandidateRecordResult, error) {
	r.records = append(r.records, record)
	return CandidateRecordResult{ID: "attempt-1"}, nil
}

type fakeOutcomeRecorder struct {
	outcomes []AttemptOutcome
}

func (r *fakeOutcomeRecorder) RecordOutcome(_ context.Context, outcome AttemptOutcome) error {
	r.outcomes = append(r.outcomes, outcome)
	return nil
}

var errFakeSink = &fakeSinkError{}

type fakeSinkError struct{}

func (*fakeSinkError) Error() string {
	return "fake sink error"
}

func validMailStoredPayload() json.RawMessage {
	return json.RawMessage(`{
		"event":"mail.stored",
		"message_id":"msg-1",
		"rfc_message_id":"<msg-1@example.com>",
		"company_id":"company-1",
		"domain_id":"domain-1",
		"user_id":"user-1",
		"recipient":"user@example.com",
		"subject":"Hello",
		"received_at":"2026-05-04T00:00:00Z"
	}`)
}
