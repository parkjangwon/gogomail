package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/httpapi"
)

func TestRedactLogAttrScrubsSensitiveKeys(t *testing.T) {
	for _, key := range []string{"authorization", "cookie", "password", "refresh_token", "api_key", "private_key"} {
		got := redactLogAttr(nil, slog.String(key, "secret-value"))
		if got.Value.String() != redactedLogValue {
			t.Fatalf("redactLogAttr(%q) = %q, want redacted value", key, got.Value.String())
		}
	}
}

func TestContextHandlerAddsRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(newContextHandler(slog.NewTextHandler(&buf, &slog.HandlerOptions{})))
	ctx := httpapi.ContextWithRequestID(context.Background(), "req-from-context")

	logger.InfoContext(ctx, "context log")

	if got := buf.String(); !strings.Contains(got, "request_id=req-from-context") {
		t.Fatalf("log = %q, want request id attr", got)
	}
}
