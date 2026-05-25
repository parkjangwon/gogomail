package main

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gogomail/gogomail/internal/httpapi"
)

const redactedLogValue = "[REDACTED]"

type contextHandler struct {
	next slog.Handler
}

func newContextHandler(next slog.Handler) slog.Handler {
	return contextHandler{next: next}
}

func (h contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h contextHandler) Handle(ctx context.Context, record slog.Record) error {
	existing := make(map[string]struct{}, record.NumAttrs())
	record.Attrs(func(attr slog.Attr) bool {
		existing[attr.Key] = struct{}{}
		return true
	})
	for _, attr := range httpapi.RequestContextAttrs(ctx) {
		if _, ok := existing[attr.Key]; ok {
			continue
		}
		if attr.Value.String() != "" {
			record.AddAttrs(attr)
		}
	}
	return h.next.Handle(ctx, record)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{next: h.next.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{next: h.next.WithGroup(name)}
}

func redactLogAttr(_ []string, attr slog.Attr) slog.Attr {
	if isSensitiveLogKey(attr.Key) {
		return slog.String(attr.Key, redactedLogValue)
	}
	return attr
}

func isSensitiveLogKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return false
	}
	if key == "authorization" || key == "cookie" || key == "set-cookie" {
		return true
	}
	for _, token := range []string{"password", "passwd", "secret", "token", "private_key", "apikey", "api_key"} {
		if strings.Contains(key, token) {
			return true
		}
	}
	return key == "key" || strings.HasSuffix(key, "_key")
}
