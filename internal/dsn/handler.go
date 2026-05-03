package dsn

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	netmail "net/mail"
	"path"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/mail"
	"github.com/gogomail/gogomail/internal/outbound"
	"github.com/gogomail/gogomail/internal/storage"
)

type Queue interface {
	Enqueue(ctx context.Context, topic string, partitionKey string, payload []byte) error
}

type HandlerOptions struct {
	Store        storage.Store
	Queue        Queue
	ReportingMTA string
	Postmaster   string
	Farm         outbound.Farm
	Now          func() time.Time
}

type BounceHandler struct {
	store        storage.Store
	queue        Queue
	reportingMTA string
	postmaster   outbound.Address
	farm         outbound.Farm
	now          func() time.Time
}

func NewBounceHandler(opts HandlerOptions) *BounceHandler {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	reportingMTA := strings.TrimSpace(opts.ReportingMTA)
	if reportingMTA == "" {
		reportingMTA = "localhost"
	}
	postmaster := strings.TrimSpace(opts.Postmaster)
	if postmaster == "" {
		postmaster = "postmaster@" + reportingMTA
	}
	postmasterAddress := parsePostmasterAddress(postmaster)
	return &BounceHandler{
		store:        opts.Store,
		queue:        opts.Queue,
		reportingMTA: reportingMTA,
		postmaster:   postmasterAddress,
		farm:         outbound.NormalizeFarm(opts.Farm),
		now:          now,
	}
}

func (h *BounceHandler) HandleEvent(ctx context.Context, msg eventstream.Message) error {
	if h.store == nil {
		return fmt.Errorf("dsn storage is required")
	}
	if h.queue == nil {
		return fmt.Errorf("dsn queue is required")
	}

	event, err := decodeBounceEvent(msg.Payload)
	if err != nil {
		return err
	}
	if !shouldGenerateFailureDSN(event) {
		return nil
	}

	now := h.now().UTC()
	composed, err := Compose(Report{
		ReportingMTA: h.reportingMTA,
		OriginalID:   event.DSN.EnvelopeID,
		From:         h.postmaster,
		To:           outbound.Address{Email: event.Sender},
		Subject:      "Delivery Status Notification (Failure)",
		Date:         now,
		Recipients: []RecipientStatus{{
			Recipient:         event.Recipient,
			OriginalRecipient: event.DSN.OriginalRecipient,
			Action:            "failed",
			Status:            event.EnhancedStatus,
			Diagnostic:        event.ErrorMessage,
			RemoteMTA:         event.RecipientDomain,
			FinalLogID:        event.MessageID,
			LastAttemptAt:     event.AttemptedAt,
		}},
	})
	if err != nil {
		return err
	}

	storagePath := dsnStoragePath(now, composed.MessageID)
	if err := h.store.Put(ctx, storagePath, bytes.NewReader(composed.Raw)); err != nil {
		return fmt.Errorf("store dsn message: %w", err)
	}

	queued := delivery.QueuedMessage{
		Event:        "mail.queued",
		MessageID:    event.MessageID,
		RFCMessageID: composed.MessageID,
		CompanyID:    event.CompanyID,
		DomainID:     event.DomainID,
		Farm:         h.farm,
		From:         outbound.Address{},
		To:           []outbound.Address{{Email: event.Sender}},
		Subject:      "Delivery Status Notification (Failure)",
		StoragePath:  storagePath,
		Size:         composed.Size,
	}
	payload, err := json.Marshal(queued)
	if err != nil {
		return fmt.Errorf("marshal dsn queue payload: %w", err)
	}
	if err := h.queue.Enqueue(ctx, "mail.outbound."+string(h.farm), event.MessageID, payload); err != nil {
		_ = h.store.Delete(ctx, storagePath)
		return err
	}
	return nil
}

func parsePostmasterAddress(value string) outbound.Address {
	parsed, err := netmail.ParseAddress(strings.TrimSpace(value))
	if err != nil {
		return outbound.Address{Name: "Mail Delivery Subsystem", Email: strings.TrimSpace(value)}
	}
	name := strings.TrimSpace(parsed.Name)
	if name == "" {
		name = "Mail Delivery Subsystem"
	}
	return outbound.Address{Name: name, Email: parsed.Address}
}

type PostgresOutboxQueue struct {
	db *sql.DB
}

func NewPostgresOutboxQueue(db *sql.DB) *PostgresOutboxQueue {
	return &PostgresOutboxQueue{db: db}
}

func (q *PostgresOutboxQueue) Enqueue(ctx context.Context, topic string, partitionKey string, payload []byte) error {
	if q.db == nil {
		return fmt.Errorf("database handle is required")
	}
	const query = `
INSERT INTO outbox (topic, partition_key, payload, status)
VALUES ($1, $2, $3::jsonb, 'pending')`
	if _, err := q.db.ExecContext(ctx, query, topic, partitionKey, string(payload)); err != nil {
		return fmt.Errorf("insert dsn outbox event: %w", err)
	}
	return nil
}

type bounceEvent struct {
	Event           string    `json:"event"`
	MessageID       string    `json:"message_id"`
	RFCMessageID    string    `json:"rfc_message_id"`
	CompanyID       string    `json:"company_id"`
	DomainID        string    `json:"domain_id"`
	Sender          string    `json:"sender"`
	Recipient       string    `json:"recipient"`
	RecipientDomain string    `json:"recipient_domain"`
	EnhancedStatus  string    `json:"enhanced_status"`
	ErrorMessage    string    `json:"error_message"`
	AttemptedAt     time.Time `json:"attempted_at"`
	DSN             struct {
		EnvelopeID        string   `json:"envelope_id"`
		Notify            []string `json:"notify"`
		OriginalRecipient string   `json:"original_recipient"`
	} `json:"dsn"`
}

func decodeBounceEvent(payload json.RawMessage) (bounceEvent, error) {
	var event bounceEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return bounceEvent{}, fmt.Errorf("decode bounce event: %w", err)
	}
	if event.Event != "mail.bounced" {
		return bounceEvent{}, fmt.Errorf("unexpected bounce event %q", event.Event)
	}
	event.MessageID = strings.TrimSpace(event.MessageID)
	event.Sender = strings.TrimSpace(event.Sender)
	event.Recipient = strings.TrimSpace(event.Recipient)
	event.RecipientDomain = strings.TrimSpace(event.RecipientDomain)
	event.EnhancedStatus = strings.TrimSpace(event.EnhancedStatus)
	if event.MessageID == "" {
		return bounceEvent{}, fmt.Errorf("bounce event is missing message_id")
	}
	if containsLineBreak(event.MessageID) || containsLineBreak(event.RFCMessageID) {
		return bounceEvent{}, fmt.Errorf("bounce event has invalid message identity")
	}
	if event.Recipient == "" {
		return bounceEvent{}, fmt.Errorf("bounce event is missing recipient")
	}
	if event.Sender != "" {
		sender, err := mail.NormalizeAddress(event.Sender)
		if err != nil {
			return bounceEvent{}, fmt.Errorf("bounce event has invalid sender: %w", err)
		}
		event.Sender = sender
	}
	recipient, err := mail.NormalizeAddress(event.Recipient)
	if err != nil {
		return bounceEvent{}, fmt.Errorf("bounce event has invalid recipient: %w", err)
	}
	event.Recipient = recipient
	return event, nil
}

func containsLineBreak(value string) bool {
	return strings.ContainsAny(value, "\r\n")
}

func shouldGenerateFailureDSN(event bounceEvent) bool {
	if strings.TrimSpace(event.Sender) == "" {
		return false
	}
	if len(event.DSN.Notify) == 0 {
		return true
	}
	wantFailure := false
	for _, value := range event.DSN.Notify {
		switch strings.ToUpper(strings.TrimSpace(value)) {
		case "NEVER":
			return false
		case "FAILURE":
			wantFailure = true
		}
	}
	return wantFailure
}

func dsnStoragePath(now time.Time, messageID string) string {
	messageID = strings.Trim(messageID, "<>")
	var builder strings.Builder
	for _, r := range messageID {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '@' {
			builder.WriteRune(r)
		}
	}
	name := builder.String()
	if name == "" {
		name = "dsn"
	}
	return path.Join("dsn", now.Format("2006"), now.Format("01"), name+".eml")
}
