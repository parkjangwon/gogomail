package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAccessLogMiddlewareWritesStructuredRequestLog(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{}))
	handler := RequestIDMiddleware(AccessLogMiddleware(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := RequestIDFromContext(r.Context()); got != "req-test-1" {
			t.Fatalf("request id in context = %q, want req-test-1", got)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/123", nil)
	req.Header.Set("X-Request-ID", "req-test-1")
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "req-test-1" {
		t.Fatalf("response X-Request-ID = %q, want req-test-1", got)
	}
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("unmarshal log: %v\n%s", err, buf.String())
	}
	for key, want := range map[string]any{
		"msg":        "http request",
		"method":     "POST",
		"route":      "/api/v1/messages/{id}",
		"status":     float64(http.StatusCreated),
		"request_id": "req-test-1",
		"remote_ip":  "203.0.113.10",
		"user_agent": "test-agent",
		"bytes":      float64(2),
	} {
		if got := line[key]; got != want {
			t.Fatalf("log[%s] = %#v, want %#v\n%s", key, got, want, buf.String())
		}
	}
	if _, ok := line["duration_ms"]; !ok {
		t.Fatalf("log missing duration_ms: %s", buf.String())
	}
}
