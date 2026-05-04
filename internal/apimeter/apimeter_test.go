package apimeter

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandlerRecordsRequestAndResponseEvent(t *testing.T) {
	sink := &captureSink{events: make(chan Event, 1)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/messages/{id}", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if string(body) != "request-body" {
			t.Fatalf("body = %q, want request-body", body)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("response-body"))
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/msg-1?user_id=user-1", strings.NewReader("request-body"))
	rec := httptest.NewRecorder()

	Handler(mux, sink, WithTimeout(time.Second)).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if rec.Body.String() != "response-body" {
		t.Fatalf("response body = %q, want response-body", rec.Body.String())
	}

	event := receiveEvent(t, sink.events)
	if event.Method != http.MethodPost {
		t.Fatalf("method = %q, want POST", event.Method)
	}
	if event.RoutePattern != "POST /api/v1/messages/{id}" {
		t.Fatalf("route pattern = %q, want ServeMux pattern", event.RoutePattern)
	}
	if event.Status != http.StatusCreated {
		t.Fatalf("event status = %d, want %d", event.Status, http.StatusCreated)
	}
	if event.RequestBytes != int64(len("request-body")) {
		t.Fatalf("request bytes = %d, want %d", event.RequestBytes, len("request-body"))
	}
	if event.ResponseBytes != int64(len("response-body")) {
		t.Fatalf("response bytes = %d, want %d", event.ResponseBytes, len("response-body"))
	}
	if event.UserID != "user-1" {
		t.Fatalf("user id = %q, want user-1", event.UserID)
	}
	if event.AuthSource != "query_user_id" {
		t.Fatalf("auth source = %q, want query_user_id", event.AuthSource)
	}
	if event.Identity.UserID != "user-1" || event.Identity.AuthSource != "query_user_id" {
		t.Fatalf("identity = %+v", event.Identity)
	}
	if event.Timestamp.IsZero() {
		t.Fatal("timestamp was not recorded")
	}
	if event.Latency < 0 {
		t.Fatalf("latency = %s, want nonnegative", event.Latency)
	}
}

func TestHandlerDefaultsStatusAndUsesContentLengthWhenBodyUnread(t *testing.T) {
	sink := &captureSink{events: make(chan Event, 1)}
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	req := httptest.NewRequest(http.MethodPut, "/unread-body", strings.NewReader("not-read"))
	rec := httptest.NewRecorder()

	Handler(next, sink, WithTimeout(time.Second)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	event := receiveEvent(t, sink.events)
	if event.Status != http.StatusOK {
		t.Fatalf("event status = %d, want %d", event.Status, http.StatusOK)
	}
	if event.RequestBytes != int64(len("not-read")) {
		t.Fatalf("request bytes = %d, want %d", event.RequestBytes, len("not-read"))
	}
	if event.ResponseBytes != 0 {
		t.Fatalf("response bytes = %d, want 0", event.ResponseBytes)
	}
}

func TestHandlerFailsOpenWhenSinkReturnsError(t *testing.T) {
	sink := &errorSink{called: make(chan struct{}, 1)}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rec := httptest.NewRecorder()

	Handler(next, sink, WithTimeout(time.Second)).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	select {
	case <-sink.called:
	case <-time.After(time.Second):
		t.Fatal("sink was not called")
	}
}

func TestHandlerFailsOpenWhenSinkTimesOut(t *testing.T) {
	sink := &timeoutSink{done: make(chan struct{}, 1)}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/timeout", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	Handler(next, sink, WithTimeout(10*time.Millisecond)).ServeHTTP(rec, req)
	elapsed := time.Since(start)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("handler blocked for %s", elapsed)
	}
	select {
	case <-sink.done:
	case <-time.After(time.Second):
		t.Fatal("sink did not observe timeout")
	}
}

func TestHandlerUsesIdentityResolver(t *testing.T) {
	sink := &captureSink{events: make(chan Event, 1)}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rec := httptest.NewRecorder()

	Handler(next, sink, WithTimeout(time.Second), WithIdentityResolver(func(*http.Request) Identity {
		return Identity{
			TenantID:   " tenant-1 ",
			CompanyID:  " company-1 ",
			DomainID:   " domain-1 ",
			UserID:     " user-1 ",
			APIKeyID:   " api-key-1 ",
			AuthSource: AuthSourceBearer,
		}
	})).ServeHTTP(rec, req)

	event := receiveEvent(t, sink.events)
	if event.UserID != "user-1" || event.AuthSource != AuthSourceBearer {
		t.Fatalf("event identity fields = user:%q auth:%q", event.UserID, event.AuthSource)
	}
	if event.Identity.TenantID != "tenant-1" || event.Identity.CompanyID != "company-1" || event.Identity.DomainID != "domain-1" {
		t.Fatalf("identity dimensions = %+v", event.Identity)
	}
	if event.Identity.PrincipalID != "user-1" {
		t.Fatalf("principal id = %q, want user-1", event.Identity.PrincipalID)
	}
}

func TestDefaultIdentityFromRequestExtractsHeaders(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/?user_id=user-1", nil)
	req.Header.Set("X-Gogomail-Tenant-ID", "tenant-1")
	req.Header.Set("X-Gogomail-Company-ID", "company-1")
	req.Header.Set("X-Gogomail-Domain-ID", "domain-1")
	req.Header.Set("X-Gogomail-API-Key-ID", "api-key-1")

	id := defaultIdentityFromRequest(req).Normalize()
	if id.TenantID != "tenant-1" || id.CompanyID != "company-1" || id.DomainID != "domain-1" {
		t.Fatalf("identity dimensions = %+v", id)
	}
	if id.UserID != "user-1" || id.APIKeyID != "api-key-1" || id.AuthSource != AuthSourceQueryUserID {
		t.Fatalf("identity principals = %+v", id)
	}
}

func TestAuthSourceFromRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  *http.Request
		want string
	}{
		{name: "nil", want: "anonymous"},
		{name: "bearer", req: requestWithHeader("Authorization", "Bearer token"), want: "bearer"},
		{name: "admin token", req: requestWithHeader("X-Admin-Token", "secret"), want: "admin_token"},
		{name: "query user", req: httptest.NewRequest(http.MethodGet, "/?user_id=user-1", nil), want: "query_user_id"},
		{name: "anonymous", req: httptest.NewRequest(http.MethodGet, "/", nil), want: "anonymous"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := authSourceFromRequest(tc.req); got != tc.want {
				t.Fatalf("authSourceFromRequest() = %q, want %q", got, tc.want)
			}
		})
	}
}

func requestWithHeader(key, value string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(key, value)
	return req
}

type captureSink struct {
	events chan Event
}

func (s *captureSink) Record(_ context.Context, event Event) error {
	s.events <- event
	return nil
}

type errorSink struct {
	called chan struct{}
}

func (s *errorSink) Record(context.Context, Event) error {
	s.called <- struct{}{}
	return errors.New("metering unavailable")
}

type timeoutSink struct {
	done chan struct{}
}

func (s *timeoutSink) Record(ctx context.Context, _ Event) error {
	<-ctx.Done()
	s.done <- struct{}{}
	return ctx.Err()
}

func receiveEvent(t *testing.T, events <-chan Event) Event {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
		return Event{}
	}
}
