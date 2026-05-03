package delivery

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/outbound"
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
	metrics := &fakeMetrics{}
	handler := NewHandler(store, transport, recorder, nil).WithMetrics(metrics)

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
	if !metrics.has(MetricQueuedDecoded, MetricOK) || !metrics.has(MetricTransportDelivered, MetricOK) {
		t.Fatalf("metrics = %+v, want decoded and delivered metrics", metrics.events)
	}
	if metrics.events[len(metrics.events)-1].RecipientCount != 1 {
		t.Fatalf("last metric = %+v, want recipient count 1", metrics.events[len(metrics.events)-1])
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
	metrics := &fakeMetrics{}
	handler := NewHandler(store, &fakeTransport{err: errBoom}, recorder, retry).WithMetrics(metrics)

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
	if !metrics.has(MetricTransportFailed, MetricFailed) || !metrics.has(MetricRetryScheduled, MetricDeferred) {
		t.Fatalf("metrics = %+v, want failed transport and scheduled retry", metrics.events)
	}
}

func TestHandlerDoesNotRetryPermanentSMTPFailure(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	if err := store.Put(context.Background(), "mailstore/msg.eml", strings.NewReader("Subject: hello\r\n\r\nbody")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	recorder := &fakeRecorder{}
	retry := &fakeRetryScheduler{}
	metrics := &fakeMetrics{}
	handler := NewHandler(store, &fakeTransport{err: &SMTPStatusError{Op: "rcpt", Code: 550, Message: "no such user"}}, recorder, retry).WithMetrics(metrics)

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
	if len(recorder.attempts) != 1 || recorder.attempts[0].Status != AttemptBounced {
		t.Fatalf("attempts = %+v, want bounced attempt", recorder.attempts)
	}
	if retry.scheduled.MessageID != "" {
		t.Fatalf("scheduled retry = %+v, want none", retry.scheduled)
	}
	if !metrics.has(MetricTransportFailed, MetricBounced) {
		t.Fatalf("metrics = %+v, want bounced transport metric", metrics.events)
	}
}

func TestHandlerRecordsAndRetriesPartialDelivery(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	if err := store.Put(context.Background(), "mailstore/msg.eml", strings.NewReader("Subject: hello\r\n\r\nbody")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	recorder := &fakeRecorder{}
	retry := &fakeRetryScheduler{}
	handler := NewHandler(store, &fakeTransport{err: &PartialDeliveryError{
		Delivered: []outbound.Address{{Email: "ok@example.net"}},
		Failed: []RecipientDeliveryError{
			{Recipient: outbound.Address{Email: "gone@example.net"}, Err: &SMTPStatusError{Op: "rcpt", Code: 550, Message: "gone"}},
			{Recipient: outbound.Address{Email: "temp@example.net"}, Err: &SMTPStatusError{Op: "rcpt", Code: 451, Message: "try later"}},
		},
	}}, recorder, retry)

	err := handler.HandleEvent(context.Background(), eventstream.Message{
		ID: "1-0",
		Payload: []byte(`{
			"event":"mail.queued",
			"message_id":"msg-1",
			"from":{"email":"sender@example.com"},
			"to":[{"email":"ok@example.net"},{"email":"gone@example.net"},{"email":"temp@example.net"}],
			"storage_path":"mailstore/msg.eml",
			"farm":"general"
		}`),
	})
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if len(recorder.attempts) != 3 {
		t.Fatalf("attempts = %+v, want 3", recorder.attempts)
	}
	statusByRecipient := map[string]AttemptStatus{}
	for _, attempt := range recorder.attempts {
		statusByRecipient[attempt.Recipient] = attempt.Status
	}
	if statusByRecipient["ok@example.net"] != AttemptDelivered {
		t.Fatalf("attempts = %+v, want ok delivered", recorder.attempts)
	}
	if statusByRecipient["gone@example.net"] != AttemptBounced {
		t.Fatalf("attempts = %+v, want gone bounced", recorder.attempts)
	}
	if statusByRecipient["temp@example.net"] != AttemptFailed {
		t.Fatalf("attempts = %+v, want temp failed", recorder.attempts)
	}
	recipients := retry.scheduled.Recipients()
	if len(recipients) != 1 || recipients[0].Email != "temp@example.net" {
		t.Fatalf("scheduled retry recipients = %+v, want only temp recipient", recipients)
	}
}

func TestHandlerFiltersDSNRecipientsForPartialRetry(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	if err := store.Put(context.Background(), "mailstore/msg.eml", strings.NewReader("Subject: hello\r\n\r\nbody")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	retry := &fakeRetryScheduler{}
	handler := NewHandler(store, &fakeTransport{err: &PartialDeliveryError{
		Delivered: []outbound.Address{{Email: "ok@example.net"}},
		Failed: []RecipientDeliveryError{
			{Recipient: outbound.Address{Email: "gone@example.net"}, Err: &SMTPStatusError{Op: "rcpt", Code: 550, Message: "gone"}},
			{Recipient: outbound.Address{Email: "temp@example.net"}, Err: &SMTPStatusError{Op: "rcpt", Code: 451, Message: "try later"}},
		},
	}}, &fakeRecorder{}, retry)

	err := handler.HandleEvent(context.Background(), eventstream.Message{
		ID: "1-0",
		Payload: []byte(`{
			"event":"mail.queued",
			"message_id":"msg-1",
			"from":{"email":"sender@example.com"},
			"to":[{"email":"ok@example.net"},{"email":"gone@example.net"},{"email":"temp@example.net"}],
			"dsn":{
				"return":"FULL",
				"envelope_id":"env-1",
				"recipients":[
					{"address":"ok@example.net","notify":["SUCCESS"]},
					{"address":"gone@example.net","notify":["FAILURE"]},
					{"address":"temp@example.net","notify":["FAILURE","DELAY"],"original_recipient":"rfc822; alias@example.net"}
				]
			},
			"storage_path":"mailstore/msg.eml",
			"farm":"general"
		}`),
	})
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if retry.scheduled.DSN.Return != "FULL" || retry.scheduled.DSN.EnvelopeID != "env-1" {
		t.Fatalf("scheduled DSN envelope = %+v", retry.scheduled.DSN)
	}
	if len(retry.scheduled.DSN.Recipients) != 1 {
		t.Fatalf("scheduled DSN recipients = %+v, want retry recipient only", retry.scheduled.DSN.Recipients)
	}
	got := retry.scheduled.DSN.Recipients[0]
	if got.Address != "temp@example.net" || got.OriginalRecipient != "rfc822; alias@example.net" {
		t.Fatalf("scheduled DSN recipient = %+v, want temp metadata", got)
	}
}

func TestHandlerObservesPermanentPartialAsBounced(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	if err := store.Put(context.Background(), "mailstore/msg.eml", strings.NewReader("Subject: hello\r\n\r\nbody")); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	metrics := &fakeMetrics{}
	handler := NewHandler(store, &fakeTransport{err: &PartialDeliveryError{
		Failed: []RecipientDeliveryError{
			{Recipient: outbound.Address{Email: "gone@example.net"}, Err: &SMTPStatusError{Op: "rcpt", Code: 550, Message: "gone"}},
		},
	}}, &fakeRecorder{}, &fakeRetryScheduler{}).WithMetrics(metrics)

	err := handler.HandleEvent(context.Background(), eventstream.Message{
		ID: "1-0",
		Payload: []byte(`{
			"event":"mail.queued",
			"message_id":"msg-1",
			"from":{"email":"sender@example.com"},
			"to":[{"email":"gone@example.net"}],
			"storage_path":"mailstore/msg.eml",
			"farm":"general"
		}`),
	})
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if !metrics.has(MetricTransportFailed, MetricBounced) {
		t.Fatalf("metrics = %+v, want bounced partial failure", metrics.events)
	}
}

func TestDecodeQueuedMessageRejectsWrongEvent(t *testing.T) {
	t.Parallel()

	_, err := DecodeQueuedMessage([]byte(`{"event":"mail.stored"}`))
	if err == nil {
		t.Fatal("DecodeQueuedMessage accepted wrong event")
	}
}

func TestHandlerObservesDecodeFailure(t *testing.T) {
	t.Parallel()

	metrics := &fakeMetrics{}
	handler := NewHandler(storage.NewLocalStore(t.TempDir()), &fakeTransport{}, &fakeRecorder{}, nil).WithMetrics(metrics)
	err := handler.HandleEvent(context.Background(), eventstream.Message{
		ID:      "1-0",
		Payload: []byte(`{"event":"mail.stored"}`),
	})
	if err == nil {
		t.Fatal("HandleEvent accepted wrong event")
	}
	if !metrics.has(MetricQueuedDecoded, MetricFailed) {
		t.Fatalf("metrics = %+v, want decode failure metric", metrics.events)
	}
}

func TestDecodeQueuedMessageRejectsInvalidRecipient(t *testing.T) {
	t.Parallel()

	_, err := DecodeQueuedMessage([]byte(`{
		"event":"mail.queued",
		"message_id":"msg-1",
		"from":{"email":"sender@example.com"},
		"to":[{"email":"not-an-address"}]
	}`))
	if err == nil {
		t.Fatal("DecodeQueuedMessage accepted invalid recipient")
	}
	if !strings.Contains(err.Error(), "invalid to recipient") {
		t.Fatalf("error = %v, want invalid to recipient", err)
	}
}

func TestDecodeQueuedMessageNormalizesAndDeduplicatesRecipients(t *testing.T) {
	t.Parallel()

	queued, err := DecodeQueuedMessage([]byte(`{
		"event":"mail.queued",
		"message_id":"msg-1",
		"from":{"email":"Sender@Example.COM"},
		"to":[{"name":"User","email":"User@Example.NET"}],
		"cc":[{"name":"Duplicate","email":"user@example.net"},{"email":"Copy@Example.NET"}],
		"bcc":[{"email":"copy@example.net"}]
	}`))
	if err != nil {
		t.Fatalf("DecodeQueuedMessage returned error: %v", err)
	}
	if queued.From.Email != "sender@example.com" {
		t.Fatalf("from.email = %q, want sender@example.com", queued.From.Email)
	}
	recipients := queued.Recipients()
	if len(recipients) != 2 {
		t.Fatalf("recipients = %+v, want 2 deduplicated recipients", recipients)
	}
	if recipients[0].Email != "user@example.net" || recipients[0].Name != "User" {
		t.Fatalf("first recipient = %+v, want normalized first TO recipient", recipients[0])
	}
	if recipients[1].Email != "copy@example.net" {
		t.Fatalf("second recipient = %+v, want copy@example.net", recipients[1])
	}
}

func TestDecodeQueuedMessageAllowsNullReversePath(t *testing.T) {
	t.Parallel()

	queued, err := DecodeQueuedMessage([]byte(`{
		"event":"mail.queued",
		"message_id":"dsn-1",
		"from":{"email":"  "},
		"to":[{"email":"sender@example.net"}]
	}`))
	if err != nil {
		t.Fatalf("DecodeQueuedMessage returned error: %v", err)
	}
	if queued.From.Email != "" {
		t.Fatalf("from.email = %q, want null reverse-path", queued.From.Email)
	}
}

func TestDecodeQueuedMessagePreservesDSNOptions(t *testing.T) {
	t.Parallel()

	queued, err := DecodeQueuedMessage([]byte(`{
		"event":"mail.queued",
		"message_id":"msg-1",
		"from":{"email":"Sender@Example.COM"},
		"to":[{"email":"User@Example.NET"}],
		"dsn":{
			"return":"FULL",
			"envelope_id":"env-1",
			"recipients":[{"address":"user@example.net","notify":["FAILURE"],"original_recipient":"rfc822; alias@example.net"}]
		}
	}`))
	if err != nil {
		t.Fatalf("DecodeQueuedMessage returned error: %v", err)
	}
	if queued.DSN.Return != "FULL" || queued.DSN.EnvelopeID != "env-1" {
		t.Fatalf("DSN = %+v", queued.DSN)
	}
	if len(queued.DSN.Recipients) != 1 || queued.DSN.Recipients[0].OriginalRecipient != "rfc822; alias@example.net" {
		t.Fatalf("DSN recipients = %+v", queued.DSN.Recipients)
	}
}

func TestDecodeQueuedMessageNormalizesDSNOptions(t *testing.T) {
	t.Parallel()

	queued, err := DecodeQueuedMessage([]byte(`{
		"event":"mail.queued",
		"message_id":"msg-1",
		"from":{"email":"Sender@Example.COM"},
		"to":[{"email":"User@Example.NET"}],
		"dsn":{
			"return":"hdrs",
			"envelope_id":" env-1 ",
			"recipients":[{"address":"User@Example.NET","notify":["failure","FAILURE"," delay "],"original_recipient":" rfc822; alias@example.net "}]
		}
	}`))
	if err != nil {
		t.Fatalf("DecodeQueuedMessage returned error: %v", err)
	}
	if queued.DSN.Return != "HDRS" || queued.DSN.EnvelopeID != "env-1" {
		t.Fatalf("DSN envelope = %+v", queued.DSN)
	}
	got := queued.DSN.Recipients[0]
	if got.Address != "user@example.net" {
		t.Fatalf("DSN recipient address = %q, want normalized", got.Address)
	}
	if len(got.Notify) != 2 || got.Notify[0] != "FAILURE" || got.Notify[1] != "DELAY" {
		t.Fatalf("DSN notify = %+v, want normalized/deduplicated", got.Notify)
	}
	if got.OriginalRecipient != "rfc822; alias@example.net" {
		t.Fatalf("original recipient = %q", got.OriginalRecipient)
	}
}

func TestDecodeQueuedMessageRejectsInvalidDSNOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dsn  string
	}{
		{name: "return", dsn: `"return":"BODY"`},
		{name: "notify", dsn: `"recipients":[{"address":"user@example.net","notify":["MAYBE"]}]`},
		{name: "never combined", dsn: `"recipients":[{"address":"user@example.net","notify":["NEVER","FAILURE"]}]`},
		{name: "bad recipient", dsn: `"recipients":[{"address":"not-an-address"}]`},
		{name: "envelope newline", dsn: `"envelope_id":"env\n1"`},
		{name: "original recipient newline", dsn: `"recipients":[{"address":"user@example.net","original_recipient":"rfc822; alias\n@example.net"}]`},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := DecodeQueuedMessage([]byte(`{
				"event":"mail.queued",
				"message_id":"msg-1",
				"from":{"email":"sender@example.com"},
				"to":[{"email":"user@example.net"}],
				"dsn":{` + tt.dsn + `}
			}`))
			if err == nil {
				t.Fatalf("DecodeQueuedMessage accepted invalid DSN %s", tt.name)
			}
		})
	}
}

func TestDecodeQueuedMessageNormalizesFarm(t *testing.T) {
	t.Parallel()

	queued, err := DecodeQueuedMessage([]byte(`{
		"event":"mail.queued",
		"message_id":"msg-1",
		"from":{"email":"sender@example.com"},
		"to":[{"email":"user@example.net"}],
		"farm":" BULK "
	}`))
	if err != nil {
		t.Fatalf("DecodeQueuedMessage returned error: %v", err)
	}
	if queued.Farm != outbound.FarmBulk {
		t.Fatalf("Farm = %q, want bulk", queued.Farm)
	}
}

func TestAttemptsForUsesDeduplicatedRecipients(t *testing.T) {
	t.Parallel()

	attempts := attemptsFor(Job{QueuedMessage: QueuedMessage{
		MessageID: "msg-1",
		Farm:      "general",
		To:        []outbound.Address{{Email: "User@Example.NET"}},
		Cc:        []outbound.Address{{Email: "user@example.net"}},
	}}, AttemptDelivered, nil, timeNow())
	if len(attempts) != 1 {
		t.Fatalf("attempts = %+v, want 1 deduplicated attempt", attempts)
	}
	if attempts[0].Recipient != "user@example.net" || attempts[0].RecipientDomain != "example.net" {
		t.Fatalf("attempt = %+v, want normalized recipient/domain", attempts[0])
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

type fakeMetrics struct {
	events []MetricEvent
}

func (m *fakeMetrics) ObserveDelivery(_ context.Context, event MetricEvent) {
	m.events = append(m.events, event)
}

func (m *fakeMetrics) has(stage MetricStage, result MetricResult) bool {
	for _, event := range m.events {
		if event.Stage == stage && event.Result == result {
			return true
		}
	}
	return false
}
