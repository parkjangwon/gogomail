package observability

import (
	"context"
	"log/slog"

	"github.com/gogomail/gogomail/internal/delivery"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

type SlogAdapter struct {
	Logger *slog.Logger
}

func NewSlogAdapter(logger *slog.Logger) SlogAdapter {
	if logger == nil {
		logger = slog.Default()
	}
	return SlogAdapter{Logger: logger}
}

func (a SlogAdapter) ObserveSMTP(ctx context.Context, event smtpd.MetricEvent) {
	logger := a.logger()
	attrs := []any{
		"component", "smtp",
		"stage", event.Stage,
		"result", event.Result,
		"remote_addr", event.RemoteAddr,
		"envelope_from", event.EnvelopeFrom,
		"recipient", event.Recipient,
		"recipient_count", len(event.Recipients),
		"size", event.Size,
	}
	if event.Error != "" {
		attrs = append(attrs, "error", event.Error)
		logger.WarnContext(ctx, "smtp metric", attrs...)
		return
	}
	logger.InfoContext(ctx, "smtp metric", attrs...)
}

func (a SlogAdapter) ObserveDelivery(ctx context.Context, event delivery.MetricEvent) {
	logger := a.logger()
	attrs := []any{
		"component", "delivery",
		"stage", event.Stage,
		"result", event.Result,
		"message_id", event.MessageID,
		"rfc_message_id", event.RFCMessageID,
		"domain_id", event.DomainID,
		"farm", event.Farm,
		"recipient_count", event.RecipientCount,
	}
	if event.Error != "" {
		attrs = append(attrs, "error", event.Error)
		logger.WarnContext(ctx, "delivery metric", attrs...)
		return
	}
	logger.InfoContext(ctx, "delivery metric", attrs...)
}

func (a SlogAdapter) logger() *slog.Logger {
	if a.Logger != nil {
		return a.Logger
	}
	return slog.Default()
}
