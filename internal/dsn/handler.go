package dsn

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	netmail "net/mail"
	"path"
	"sort"
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

	events, err := decodeFailureEvents(msg.Payload)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := h.handleFailureEvent(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (h *BounceHandler) handleFailureEvent(ctx context.Context, event bounceEvent) error {
	if !shouldGenerateFailureDSN(event) {
		return nil
	}

	now := h.now().UTC()
	originalHeaders, originalMessage := h.returnedContentForEvent(ctx, event)
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
		OriginalHeaders: originalHeaders,
		OriginalMessage: originalMessage,
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
	StoragePath     string    `json:"storage_path"`
	DSN             struct {
		Return            string   `json:"return"`
		EnvelopeID        string   `json:"envelope_id"`
		Notify            []string `json:"notify"`
		OriginalRecipient string   `json:"original_recipient"`
	} `json:"dsn"`
}

type exhaustedEvent struct {
	Event            string               `json:"event"`
	MessageID        string               `json:"message_id"`
	RFCMessageID     string               `json:"rfc_message_id"`
	CompanyID        string               `json:"company_id"`
	DomainID         string               `json:"domain_id"`
	Sender           string               `json:"sender"`
	ErrorMessage     string               `json:"error_message"`
	StoragePath      string               `json:"storage_path"`
	DSNReturn        string               `json:"dsn_return"`
	DSNEnvelopeID    string               `json:"dsn_envelope_id"`
	ExhaustedAt      time.Time            `json:"exhausted_at"`
	RecipientDetails []exhaustedRecipient `json:"recipient_details"`
}

type exhaustedRecipient struct {
	Recipient         string   `json:"recipient"`
	RecipientDomain   string   `json:"recipient_domain"`
	EnhancedStatus    string   `json:"enhanced_status"`
	DSNNotify         []string `json:"dsn_notify"`
	OriginalRecipient string   `json:"original_recipient"`
}

func decodeFailureEvents(payload json.RawMessage) ([]bounceEvent, error) {
	var envelope struct {
		Event string `json:"event"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("decode delivery failure event: %w", err)
	}
	switch strings.TrimSpace(envelope.Event) {
	case "mail.bounced":
		event, err := decodeBounceEvent(payload)
		if err != nil {
			return nil, err
		}
		return []bounceEvent{event}, nil
	case "mail.delivery_exhausted":
		return decodeExhaustedEvents(payload)
	default:
		return nil, fmt.Errorf("unexpected delivery failure event %q", envelope.Event)
	}
}

func decodeBounceEvent(payload json.RawMessage) (bounceEvent, error) {
	var event bounceEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return bounceEvent{}, fmt.Errorf("decode bounce event: %w", err)
	}
	if event.Event != "mail.bounced" {
		return bounceEvent{}, fmt.Errorf("unexpected bounce event %q", event.Event)
	}
	return normalizeBounceEvent(event)
}

func decodeExhaustedEvents(payload json.RawMessage) ([]bounceEvent, error) {
	var event exhaustedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("decode delivery_exhausted event: %w", err)
	}
	if event.Event != "mail.delivery_exhausted" {
		return nil, fmt.Errorf("unexpected delivery_exhausted event %q", event.Event)
	}
	events := make([]bounceEvent, 0, len(event.RecipientDetails))
	for _, recipient := range event.RecipientDetails {
		bounced := bounceEvent{
			Event:           event.Event,
			MessageID:       event.MessageID,
			RFCMessageID:    event.RFCMessageID,
			CompanyID:       event.CompanyID,
			DomainID:        event.DomainID,
			Sender:          event.Sender,
			Recipient:       recipient.Recipient,
			RecipientDomain: recipient.RecipientDomain,
			EnhancedStatus:  recipient.EnhancedStatus,
			ErrorMessage:    event.ErrorMessage,
			AttemptedAt:     event.ExhaustedAt,
			StoragePath:     event.StoragePath,
		}
		bounced.DSN.Return = event.DSNReturn
		bounced.DSN.EnvelopeID = event.DSNEnvelopeID
		bounced.DSN.Notify = recipient.DSNNotify
		bounced.DSN.OriginalRecipient = recipient.OriginalRecipient
		normalized, err := normalizeBounceEvent(bounced)
		if err != nil {
			return nil, err
		}
		events = append(events, normalized)
	}
	return events, nil
}

func normalizeBounceEvent(event bounceEvent) (bounceEvent, error) {
	event.MessageID = strings.TrimSpace(event.MessageID)
	event.Sender = strings.TrimSpace(event.Sender)
	event.Recipient = strings.TrimSpace(event.Recipient)
	event.RecipientDomain = strings.TrimSpace(event.RecipientDomain)
	event.EnhancedStatus = strings.TrimSpace(event.EnhancedStatus)
	event.StoragePath = strings.TrimSpace(event.StoragePath)
	if event.MessageID == "" {
		return bounceEvent{}, fmt.Errorf("bounce event is missing message_id")
	}
	if containsLineBreak(event.MessageID) || containsLineBreak(event.RFCMessageID) {
		return bounceEvent{}, fmt.Errorf("bounce event has invalid message identity")
	}
	if containsLineBreak(event.RecipientDomain) || containsLineBreak(event.EnhancedStatus) || containsLineBreak(event.ErrorMessage) {
		return bounceEvent{}, fmt.Errorf("bounce event has invalid delivery status fields")
	}
	if event.EnhancedStatus != "" && (!validEnhancedStatus(event.EnhancedStatus) || !failureEventStatusMatchesAction(event.Event, event.EnhancedStatus)) {
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
	event.DSN.Return = strings.ToUpper(strings.TrimSpace(event.DSN.Return))
	event.DSN.OriginalRecipient = strings.TrimSpace(event.DSN.OriginalRecipient)
	if containsLineBreak(event.StoragePath) || containsLineBreak(event.DSN.EnvelopeID) || containsLineBreak(event.DSN.OriginalRecipient) {
		return bounceEvent{}, fmt.Errorf("bounce event has invalid dsn metadata")
	}
	if event.StoragePath != "" && !validBounceStoragePath(event.StoragePath) {
		return bounceEvent{}, fmt.Errorf("bounce event has invalid storage_path")
	}
	if event.DSN.Return != "" && event.DSN.Return != "HDRS" && event.DSN.Return != "FULL" {
		return bounceEvent{}, fmt.Errorf("bounce event has invalid dsn return")
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

func failureEventStatusMatchesAction(event string, status string) bool {
	if event == "mail.delivery_exhausted" {
		return dsnStatusMatchesAction("failed", status)
	}
	return status[0] == '5'
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

const (
	maxOriginalHeaderReadBytes  = 64 * 1024
	maxOriginalMessageReadBytes = 256 * 1024
)

func (h *BounceHandler) returnedContentForEvent(ctx context.Context, event bounceEvent) ([]OriginalHeader, []byte) {
	switch event.DSN.Return {
	case "HDRS":
		return h.originalHeadersForEvent(ctx, event), nil
	case "FULL":
		if strings.TrimSpace(event.StoragePath) == "" {
			return nil, nil
		}
		message, err := readOriginalMessage(ctx, h.store, event.StoragePath)
		if err != nil {
			return nil, nil
		}
		return nil, message
	default:
		return nil, nil
	}
}

func (h *BounceHandler) originalHeadersForEvent(ctx context.Context, event bounceEvent) []OriginalHeader {
	if event.DSN.Return != "HDRS" || strings.TrimSpace(event.StoragePath) == "" {
		return nil
	}
	headers, err := readOriginalHeaders(ctx, h.store, event.StoragePath)
	if err != nil {
		return nil
	}
	return headers
}

func readOriginalMessage(ctx context.Context, store storage.Store, storagePath string) ([]byte, error) {
	if store == nil {
		return nil, fmt.Errorf("dsn storage is required")
	}
	if !validBounceStoragePath(storagePath) {
		return nil, fmt.Errorf("invalid storage_path")
	}
	body, err := store.Get(ctx, storagePath)
	if err != nil {
		return nil, fmt.Errorf("read original message: %w", err)
	}
	defer body.Close()

	raw, err := io.ReadAll(io.LimitReader(body, maxOriginalMessageReadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read original message: %w", err)
	}
	if len(raw) > maxOriginalMessageReadBytes {
		return nil, fmt.Errorf("original message exceeds %d bytes", maxOriginalMessageReadBytes)
	}
	if _, err := netmail.ReadMessage(bytes.NewReader(raw)); err != nil {
		return nil, fmt.Errorf("parse original message: %w", err)
	}
	return raw, nil
}

func readOriginalHeaders(ctx context.Context, store storage.Store, storagePath string) ([]OriginalHeader, error) {
	if store == nil {
		return nil, fmt.Errorf("dsn storage is required")
	}
	if !validBounceStoragePath(storagePath) {
		return nil, fmt.Errorf("invalid storage_path")
	}
	body, err := store.Get(ctx, storagePath)
	if err != nil {
		return nil, fmt.Errorf("read original message headers: %w", err)
	}
	defer body.Close()

	msg, err := netmail.ReadMessage(io.LimitReader(body, maxOriginalHeaderReadBytes))
	if err != nil {
		return nil, fmt.Errorf("parse original message headers: %w", err)
	}
	names := make([]string, 0, len(msg.Header))
	for name := range msg.Header {
		name = strings.TrimSpace(name)
		if validHeaderFieldName(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	headers := make([]OriginalHeader, 0, len(names))
	for _, name := range names {
		for _, value := range msg.Header[name] {
			headers = append(headers, OriginalHeader{Name: name, Value: value})
		}
	}
	return headers, nil
}

func validBounceStoragePath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 1024 || strings.ContainsAny(value, "\r\n\\") {
		return false
	}
	if path.IsAbs(value) || path.Clean(value) != value || strings.HasPrefix(value, "../") || strings.Contains(value, "/../") {
		return false
	}
	if strings.Contains(value, "//") || !strings.HasSuffix(strings.ToLower(value), ".eml") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
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
