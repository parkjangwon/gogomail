package observability

import (
	"context"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/delivery"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

func TestPrometheusAdapterObservesSMTPEvents(t *testing.T) {
	t.Parallel()

	adapter := NewPrometheusAdapter()
	adapter.ObserveSMTP(context.Background(), smtpd.MetricEvent{Stage: smtpd.StageRcpt, Result: smtpd.MetricRejected})
	adapter.ObserveSMTP(context.Background(), smtpd.MetricEvent{Stage: smtpd.StageRcpt, Result: smtpd.MetricRejected})

	got := adapter.Text()
	want := `gogomail_smtp_events_total{result="rejected",stage="rcpt"} 2`
	if !strings.Contains(got, want) {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
}

func TestPrometheusAdapterObservesDeliveryEvents(t *testing.T) {
	t.Parallel()

	adapter := NewPrometheusAdapter()
	adapter.ObserveDelivery(context.Background(), delivery.MetricEvent{
		Stage:  delivery.MetricThrottled,
		Result: delivery.MetricDeferred,
		Farm:   "bulk",
	})

	got := adapter.Text()
	want := `gogomail_delivery_events_total{farm="bulk",result="deferred",stage="throttled"} 1`
	if !strings.Contains(got, want) {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
}
