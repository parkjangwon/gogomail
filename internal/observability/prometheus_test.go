package observability

import (
	"context"
	"strings"
	"testing"

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
