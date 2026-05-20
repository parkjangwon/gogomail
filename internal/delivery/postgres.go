package delivery

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

type PostgresRecorder struct {
	db *sql.DB
}

func NewPostgresRecorder(db *sql.DB) *PostgresRecorder {
	return &PostgresRecorder{db: db}
}

func (r *PostgresRecorder) RecordAttempt(ctx context.Context, attempt Attempt) error {
	return r.RecordAttempts(ctx, []Attempt{attempt})
}

func (r *PostgresRecorder) RecordAttempts(ctx context.Context, attempts []Attempt) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if len(attempts) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delivery attempts transaction: %w", err)
	}
	defer tx.Rollback()

	if err := insertDeliveryAttempts(ctx, tx, attempts); err != nil {
		return err
	}
	if err := suppressBouncedRecipients(ctx, tx, attempts); err != nil {
		return err
	}
	if err := insertDeliveryAttemptEvents(ctx, tx, attempts); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delivery attempts transaction: %w", err)
	}
	return nil
}

func insertDeliveryAttempts(ctx context.Context, tx *sql.Tx, attempts []Attempt) error {
	messageIDs := make([]string, 0, len(attempts))
	rfcMessageIDs := make([]string, 0, len(attempts))
	farms := make([]string, 0, len(attempts))
	recipients := make([]string, 0, len(attempts))
	recipientDomains := make([]string, 0, len(attempts))
	statuses := make([]string, 0, len(attempts))
	errorMessages := make([]string, 0, len(attempts))
	senders := make([]string, 0, len(attempts))
	enhancedStatuses := make([]string, 0, len(attempts))
	dsnReturns := make([]string, 0, len(attempts))
	dsnEnvelopeIDs := make([]string, 0, len(attempts))
	dsnNotifies := make([]string, 0, len(attempts))
	originalRecipients := make([]string, 0, len(attempts))
	attemptedAts := make([]time.Time, 0, len(attempts))

	for _, attempt := range attempts {
		diagnostics, err := deliveryAttemptDiagnostics(attempt)
		if err != nil {
			return err
		}
		messageIDs = append(messageIDs, attempt.MessageID)
		rfcMessageIDs = append(rfcMessageIDs, attempt.RFCMessageID)
		farms = append(farms, attempt.Farm)
		recipients = append(recipients, attempt.Recipient)
		recipientDomains = append(recipientDomains, attempt.RecipientDomain)
		statuses = append(statuses, string(attempt.Status))
		errorMessages = append(errorMessages, attempt.ErrorMessage)
		senders = append(senders, diagnostics.Sender)
		enhancedStatuses = append(enhancedStatuses, diagnostics.EnhancedStatus)
		dsnReturns = append(dsnReturns, diagnostics.DSNReturn)
		dsnEnvelopeIDs = append(dsnEnvelopeIDs, diagnostics.DSNEnvelopeID)
		dsnNotifies = append(dsnNotifies, string(diagnostics.DSNNotify))
		originalRecipients = append(originalRecipients, diagnostics.OriginalRecipient)
		attemptedAts = append(attemptedAts, attempt.AttemptedAt)
	}

	const query = `
INSERT INTO delivery_attempts (
  message_id, rfc_message_id, farm,
  recipient, recipient_domain,
  status, error_message,
  sender, enhanced_status, dsn_return, dsn_envelope_id, dsn_notify, original_recipient,
  attempted_at
) SELECT
  message_id::uuid, rfc_message_id, farm,
  recipient, recipient_domain,
  status, error_message,
  sender, enhanced_status, dsn_return, dsn_envelope_id, dsn_notify::jsonb, original_recipient,
  attempted_at
FROM unnest(
  $1::text[], $2::text[], $3::text[],
  $4::text[], $5::text[],
  $6::text[], $7::text[],
  $8::text[], $9::text[], $10::text[], $11::text[], $12::text[], $13::text[],
  $14::timestamptz[]
) AS attempt(
  message_id, rfc_message_id, farm,
  recipient, recipient_domain,
  status, error_message,
  sender, enhanced_status, dsn_return, dsn_envelope_id, dsn_notify, original_recipient,
  attempted_at
)`

	if _, err := tx.ExecContext(ctx, query,
		pq.Array(messageIDs), pq.Array(rfcMessageIDs), pq.Array(farms),
		pq.Array(recipients), pq.Array(recipientDomains),
		pq.Array(statuses), pq.Array(errorMessages),
		pq.Array(senders), pq.Array(enhancedStatuses), pq.Array(dsnReturns), pq.Array(dsnEnvelopeIDs), pq.Array(dsnNotifies), pq.Array(originalRecipients),
		pq.Array(attemptedAts),
	); err != nil {
		return fmt.Errorf("insert delivery attempts: %w", err)
	}
	return nil
}

func shouldSuppressBouncedRecipient(attempt Attempt) bool {
	return attempt.Status == AttemptBounced && attempt.Sender != ""
}

func suppressBouncedRecipients(ctx context.Context, tx *sql.Tx, attempts []Attempt) error {
	domainIDs, emails, messageIDs := bouncedSuppressionRows(attempts)
	if len(emails) == 0 {
		return nil
	}

	const query = `
INSERT INTO suppression_list (domain_id, email, reason, source_message_id)
SELECT domain_id, email, 'hard_bounce', source_message_id
FROM unnest($1::uuid[], $2::text[], $3::uuid[]) AS bounced(domain_id, email, source_message_id)
ON CONFLICT DO NOTHING`

	if _, err := tx.ExecContext(ctx, query, pq.Array(domainIDs), pq.Array(emails), pq.Array(messageIDs)); err != nil {
		return fmt.Errorf("insert suppression list entries: %w", err)
	}
	return nil
}

func bouncedSuppressionRows(attempts []Attempt) ([]sql.NullString, []string, []sql.NullString) {
	domainIDs := make([]sql.NullString, 0, len(attempts))
	emails := make([]string, 0, len(attempts))
	messageIDs := make([]sql.NullString, 0, len(attempts))
	seen := make(map[string]struct{}, len(attempts))
	for _, attempt := range attempts {
		if !shouldSuppressBouncedRecipient(attempt) {
			continue
		}
		email := strings.TrimSpace(attempt.Recipient)
		key := strings.TrimSpace(attempt.DomainID) + "\x00" + strings.ToLower(email)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		domainIDs = append(domainIDs, uuidNullString(attempt.DomainID))
		emails = append(emails, email)
		messageIDs = append(messageIDs, uuidNullString(attempt.MessageID))
	}
	return domainIDs, emails, messageIDs
}

func insertDeliveryAttemptEvents(ctx context.Context, tx *sql.Tx, attempts []Attempt) error {
	partitionKeys := make([]string, 0, len(attempts))
	payloads := make([]string, 0, len(attempts))
	for _, attempt := range attempts {
		payload, err := deliveryAttemptEventPayload(attempt)
		if err != nil {
			return err
		}
		partitionKeys = append(partitionKeys, attempt.MessageID)
		payloads = append(payloads, string(payload))
	}

	const query = `
INSERT INTO outbox (topic, partition_key, payload, status)
SELECT 'mail.event', partition_key, payload::jsonb, 'pending'
FROM unnest($1::text[], $2::text[]) AS event(partition_key, payload)`

	if _, err := tx.ExecContext(ctx, query, pq.Array(partitionKeys), pq.Array(payloads)); err != nil {
		return fmt.Errorf("insert delivery attempt events: %w", err)
	}
	return nil
}

func uuidNullString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	return sql.NullString{String: value, Valid: value != ""}
}

func (r *PostgresRecorder) RecordExhausted(ctx context.Context, queued QueuedMessage, cause error) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin exhaustion transaction: %w", err)
	}
	defer tx.Rollback()

	causeMsg := ""
	if cause != nil {
		causeMsg = truncateUTF8Bytes(cause.Error(), 2000)
	}

	if err := insertDeliveryAttempts(ctx, tx, attemptsFor(Job{QueuedMessage: queued}, AttemptExhausted, cause, timeNow())); err != nil {
		return err
	}

	payload, err := exhaustedEventPayload(queued, causeMsg)
	if err != nil {
		return err
	}
	const outboxQuery = `
INSERT INTO outbox (topic, partition_key, payload, status)
VALUES ('mail.event', $1, $2::jsonb, 'pending')`
	if _, err := tx.ExecContext(ctx, outboxQuery, queued.MessageID, string(payload)); err != nil {
		return fmt.Errorf("insert delivery_exhausted event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit exhaustion transaction: %w", err)
	}
	return nil
}

func exhaustedEventPayload(queued QueuedMessage, causeMsg string) ([]byte, error) {
	recipients := make([]string, 0, len(queued.Recipients()))
	recipientDetails := make([]map[string]any, 0, len(queued.Recipients()))
	cause := errorFromMessage(causeMsg)
	dsnByAddress := dsnRecipientOptionsByAddress(queued.DSN.Recipients)
	for _, r := range queued.Recipients() {
		if email := strings.TrimSpace(r.Email); email != "" {
			recipients = append(recipients, email)
			_, domain, _ := strings.Cut(email, "@")
			domain = strings.ToLower(strings.TrimSuffix(domain, "."))
			dsnRecipient := dsnByAddress[strings.ToLower(email)]
			notify := append([]string(nil), dsnRecipient.Notify...)
			if notify == nil {
				notify = []string{}
			}
			recipientDetails = append(recipientDetails, map[string]any{
				"recipient":          email,
				"recipient_domain":   domain,
				"enhanced_status":    enhancedStatusForAttempt(AttemptExhausted, cause),
				"dsn_notify":         notify,
				"original_recipient": strings.TrimSpace(dsnRecipient.OriginalRecipient),
			})
		}
	}
	raw, err := json.Marshal(map[string]any{
		"event":             "mail.delivery_exhausted",
		"message_id":        strings.TrimSpace(queued.MessageID),
		"rfc_message_id":    strings.TrimSpace(queued.RFCMessageID),
		"company_id":        strings.TrimSpace(queued.CompanyID),
		"domain_id":         strings.TrimSpace(queued.DomainID),
		"farm":              strings.TrimSpace(string(queued.Farm)),
		"sender":            strings.TrimSpace(queued.From.Email),
		"recipients":        recipients,
		"recipient_details": recipientDetails,
		"error_message":     strings.TrimSpace(causeMsg),
		"storage_path":      strings.TrimSpace(queued.StoragePath),
		"dsn_return":        strings.TrimSpace(queued.DSN.Return),
		"dsn_envelope_id":   strings.TrimSpace(queued.DSN.EnvelopeID),
		"exhausted_at":      timeNow(),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal delivery_exhausted event: %w", err)
	}
	return raw, nil
}

func errorFromMessage(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}
	return fmt.Errorf("%s", message)
}

type attemptDiagnostics struct {
	Sender            string
	EnhancedStatus    string
	DSNReturn         string
	DSNEnvelopeID     string
	DSNNotify         []byte
	OriginalRecipient string
}

func deliveryAttemptDiagnostics(attempt Attempt) (attemptDiagnostics, error) {
	notify := append([]string(nil), attempt.DSNNotify...)
	if notify == nil {
		notify = []string{}
	}
	dsnNotify, err := json.Marshal(notify)
	if err != nil {
		return attemptDiagnostics{}, fmt.Errorf("marshal delivery attempt dsn notify: %w", err)
	}
	return attemptDiagnostics{
		Sender:            truncateUTF8Bytes(strings.TrimSpace(attempt.Sender), 320),
		EnhancedStatus:    truncateUTF8Bytes(strings.TrimSpace(attempt.EnhancedStatus), 64),
		DSNReturn:         truncateUTF8Bytes(strings.TrimSpace(attempt.DSNReturn), 16),
		DSNEnvelopeID:     truncateUTF8Bytes(strings.TrimSpace(attempt.DSNEnvelopeID), 100),
		DSNNotify:         dsnNotify,
		OriginalRecipient: truncateUTF8Bytes(strings.TrimSpace(attempt.OriginalRecipient), 500),
	}, nil
}

func deliveryAttemptEventPayload(attempt Attempt) ([]byte, error) {
	event := "mail.delivery_failed"
	switch attempt.Status {
	case AttemptDelivered:
		event = "mail.delivered"
	case AttemptBounced:
		event = "mail.bounced"
	}

	payload := map[string]any{
		"event":            event,
		"message_id":       strings.TrimSpace(attempt.MessageID),
		"rfc_message_id":   strings.TrimSpace(attempt.RFCMessageID),
		"company_id":       strings.TrimSpace(attempt.CompanyID),
		"domain_id":        strings.TrimSpace(attempt.DomainID),
		"farm":             strings.TrimSpace(attempt.Farm),
		"sender":           strings.TrimSpace(attempt.Sender),
		"recipient":        strings.TrimSpace(attempt.Recipient),
		"recipient_domain": strings.TrimSpace(attempt.RecipientDomain),
		"status":           attempt.Status,
		"enhanced_status":  strings.TrimSpace(attempt.EnhancedStatus),
		"error_message":    strings.TrimSpace(attempt.ErrorMessage),
		"attempted_at":     attempt.AttemptedAt,
	}
	if strings.TrimSpace(attempt.StoragePath) != "" {
		payload["storage_path"] = strings.TrimSpace(attempt.StoragePath)
	}
	if attempt.DSNReturn != "" || attempt.DSNEnvelopeID != "" || len(attempt.DSNNotify) > 0 || attempt.OriginalRecipient != "" {
		payload["dsn"] = map[string]any{
			"return":             strings.TrimSpace(attempt.DSNReturn),
			"envelope_id":        strings.TrimSpace(attempt.DSNEnvelopeID),
			"notify":             trimStringList(attempt.DSNNotify),
			"original_recipient": strings.TrimSpace(attempt.OriginalRecipient),
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal delivery attempt event: %w", err)
	}
	return raw, nil
}

func trimStringList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	if out == nil {
		return []string{}
	}
	return out
}

func uuidOrNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}
