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

type OnceQueue interface {
	EnqueueOnce(ctx context.Context, topic string, partitionKey string, dedupeKey string, payload []byte) error
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
		MessageID:    bounceDSNMessageID(event, h.reportingMTA),
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

	storagePath := dsnStoragePath(now, event.MessageID+"-"+event.Recipient)
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
	topic := "mail.outbound." + string(h.farm)
	if onceQueue, ok := h.queue.(OnceQueue); ok {
		err = onceQueue.EnqueueOnce(ctx, topic, event.MessageID, bounceDSNDedupeKey(event), payload)
	} else {
		err = h.queue.Enqueue(ctx, topic, event.MessageID, payload)
	}
	if err != nil {
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
	return q.EnqueueOnce(ctx, topic, partitionKey, "", payload)
}

func (q *PostgresOutboxQueue) EnqueueOnce(ctx context.Context, topic string, partitionKey string, dedupeKey string, payload []byte) error {
	if q.db == nil {
		return fmt.Errorf("database handle is required")
	}
	const query = `
INSERT INTO outbox (topic, partition_key, dedupe_key, payload, status)
VALUES ($1, $2, NULLIF($3, ''), $4::jsonb, 'pending')
ON CONFLICT (dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING`
	if _, err := q.db.ExecContext(ctx, query, topic, partitionKey, dedupeKey, string(payload)); err != nil {
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
	if containsLineBreak(event.RecipientDomain) || containsLineBreak(event.EnhancedStatus) || containsLineBreak(event.ErrorMessage) {
		return bounceEvent{}, fmt.Errorf("bounce event has invalid delivery status fields")
	}
	if event.EnhancedStatus != "" && (!validEnhancedStatus(event.EnhancedStatus) || !dsnStatusMatchesAction("failed", event.EnhancedStatus)) {
		return bounceEvent{}, fmt.Errorf("bounce event has invalid enhanced_status %q", event.EnhancedStatus)
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
	event.DSN.EnvelopeID = strings.TrimSpace(event.DSN.EnvelopeID)
	event.DSN.OriginalRecipient = strings.TrimSpace(event.DSN.OriginalRecipient)
	if containsLineBreak(event.DSN.EnvelopeID) || containsLineBreak(event.DSN.OriginalRecipient) {
		return bounceEvent{}, fmt.Errorf("bounce event has invalid dsn metadata")
	}
	if event.DSN.EnvelopeID != "" && !validBounceDSNEnvelopeID(event.DSN.EnvelopeID) {
		return bounceEvent{}, fmt.Errorf("bounce event has invalid dsn envelope_id")
	}
	if event.DSN.OriginalRecipient != "" && !validBounceDSNOriginalRecipient(event.DSN.OriginalRecipient) {
		return bounceEvent{}, fmt.Errorf("bounce event has invalid dsn original_recipient")
	}
	notify, err := normalizeBounceNotify(event.DSN.Notify)
	if err != nil {
		return bounceEvent{}, err
	}
	event.DSN.Notify = notify
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

func normalizeBounceNotify(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	hasNever := false
	for _, value := range values {
		value = strings.ToUpper(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		switch value {
		case "NEVER":
			hasNever = true
		case "SUCCESS", "FAILURE", "DELAY":
		default:
			return nil, fmt.Errorf("bounce event has invalid dsn notify %q", value)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	if hasNever && len(normalized) > 1 {
		return nil, fmt.Errorf("bounce event has invalid dsn notify: NEVER cannot be combined")
	}
	return normalized, nil
}

func validBounceDSNEnvelopeID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 100 {
		return false
	}
	return validBounceDSNXText(value)
}

func validBounceDSNOriginalRecipient(value string) bool {
	addrType, encodedAddress, ok := strings.Cut(strings.TrimSpace(value), ";")
	if !ok || addrType == "" || encodedAddress == "" {
		return false
	}
	for _, r := range addrType {
		if r > 127 || !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return validBounceDSNXText(encodedAddress)
}

func validBounceDSNXText(value string) bool {
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c < 33 || c > 126 || c == '=' {
			return false
		}
		if c != '+' {
			continue
		}
		if i+2 >= len(value) || !isBounceHexDigit(value[i+1]) || !isBounceHexDigit(value[i+2]) {
			return false
		}
		i += 2
	}
	return true
}

func isBounceHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')
}

func bounceDSNDedupeKey(event bounceEvent) string {
	return "dsn:bounce:" + event.MessageID + ":" + event.Recipient
}

func bounceDSNMessageID(event bounceEvent, reportingMTA string) string {
	local := sanitizeMessageIDLocal("dsn-" + event.MessageID + "-" + event.Recipient)
	reportingMTA = strings.TrimSpace(reportingMTA)
	if reportingMTA == "" {
		reportingMTA = "localhost"
	}
	return "<" + local + "@" + reportingMTA + ">"
}

func sanitizeMessageIDLocal(value string) string {
	value = sanitizeDSNValue(value)
	var b strings.Builder
	b.Grow(len(value))
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		out = "dsn"
	}
	if len(out) > 52 {
		out = strings.TrimRight(out[:52], "-.")
		if out == "" {
			out = "dsn"
		}
	}
	return out
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
