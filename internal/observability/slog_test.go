package observability

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/delivery"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

func TestSlogAdapterObservesSMTPMetrics(t *testing.T) {
	var buf bytes.Buffer
	adapter := NewSlogAdapter(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{})))

	adapter.ObserveSMTP(context.Background(), smtpd.MetricEvent{
		Stage:        smtpd.StageRcpt,
		Result:       smtpd.MetricRejected,
		EnvelopeFrom: "sender@example.net",
		Error:        "rate limit",
	})
	got := buf.String()
	for _, want := range []string{"component=smtp", "protocol=smtp", "request_id=smtp-", "stage=rcpt", "result=rejected", "rate limit"} {
		if !strings.Contains(got, want) {
			t.Fatalf("log = %q, want %q", got, want)
		}
	}
}

func TestSlogAdapterObservesDeliveryMetrics(t *testing.T) {
	var buf bytes.Buffer
	adapter := NewSlogAdapter(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{})))

	adapter.ObserveDelivery(context.Background(), delivery.MetricEvent{
		Stage:          delivery.MetricThrottled,
		Result:         delivery.MetricDeferred,
		MessageID:      "msg-1",
		Farm:           "bulk",
		RoutePool:      "bulk-relay",
		RecipientCount: 2,
	})
	got := buf.String()
	for _, want := range []string{"component=delivery", "protocol=smtp-delivery", "request_id=delivery-", "stage=throttled", "result=deferred", "msg-1", "route_pool=bulk-relay"} {
		if !strings.Contains(got, want) {
			t.Fatalf("log = %q, want %q", got, want)
		}
	}
}
