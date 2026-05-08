package mailflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/searchindex"
)

type Handler struct {
	writer  *maildb.MailFlowLogWriter
	indexer *searchindex.MailFlowIndexer
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{writer: maildb.NewMailFlowLogWriter(db)}
}

func NewHandlerWithIndexer(db *sql.DB, indexer *searchindex.MailFlowIndexer) *Handler {
	return &Handler{writer: maildb.NewMailFlowLogWriter(db), indexer: indexer}
}

func (h *Handler) HandleEvent(ctx context.Context, msg eventstream.Message) error {
	if h.writer == nil {
		return fmt.Errorf("mail flow log writer is required")
	}
	eventName, err := eventstream.EventName(msg.Payload)
	if err != nil {
		return err
	}
	switch eventName {
	case "mail.stored":
		entry, err := h.parseInboundEvent(msg.Payload)
		if err != nil {
			return err
		}
		if err := h.writer.InsertInbound(ctx, *entry); err != nil {
			return err
		}
		return h.indexToOpenSearch(ctx, entry, "inbound")
	case "mail.delivered", "mail.bounced", "mail.delivery_failed", "mail.delivery_exhausted":
		entry, err := h.parseOutboundEvent(msg.Payload)
		if err != nil {
			return err
		}
		if err := h.writer.InsertOutbound(ctx, *entry); err != nil {
			return err
		}
		return h.indexToOpenSearch(ctx, entry, "outbound")
	}
	return nil
}

func (h *Handler) indexToOpenSearch(ctx context.Context, entry *maildb.MailFlowLogEntry, direction string) error {
	if h.indexer == nil {
		return nil
	}
	createdAt := time.Now().UTC()
	if entry.ProcessedAt != nil {
		createdAt = entry.ProcessedAt.UTC()
	} else if entry.ReceivedAt != nil {
		createdAt = entry.ReceivedAt.UTC()
	}
	toAddr := ""
	if len(entry.ToAddrs) > 0 {
		toAddr = entry.ToAddrs[0]
	}
	doc := searchindex.MailFlowDocument{
		MessageID:      entry.MessageID,
		RFCMessageID:   entry.RFCMessageID,
		Direction:      direction,
		CompanyID:      entry.CompanyID,
		DomainID:       entry.DomainID,
		UserID:         entry.UserID,
		FromAddr:       entry.FromAddr,
		ToAddr:         toAddr,
		FlowStatus:     entry.FlowStatus,
		EnhancedStatus: entry.EnhancedStatus,
		Size:           entry.Size,
		CreatedAt:      createdAt,
	}
	return h.indexer.IndexMailFlow(ctx, doc)
}

func (h *Handler) parseInboundEvent(payload json.RawMessage) (*maildb.MailFlowLogEntry, error) {
	var event struct {
		Event         string `json:"event"`
		SchemaVersion string `json:"schema_version"`
		MessageID     string `json:"message_id"`
		RFCMessageID  string `json:"rfc_message_id"`
		CompanyID   string `json:"company_id"`
		DomainID    string `json:"domain_id"`
		UserID      string `json:"user_id"`
		Recipient   string `json:"recipient"`
		Subject     string `json:"subject"`
		StoragePath string `json:"storage_path"`
		ReceivedAt  string `json:"received_at"`
		Size        int64  `json:"size"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("decode mail.stored mail flow payload: %w", err)
	}
	event.Event = strings.TrimSpace(event.Event)
	if event.Event != "mail.stored" {
		return nil, fmt.Errorf("unexpected mail flow event %q", event.Event)
	}
	event.SchemaVersion = strings.TrimSpace(event.SchemaVersion)
	if event.SchemaVersion != "" && event.SchemaVersion != "2026-05-04.mail-stored.v1" {
		return nil, fmt.Errorf("unsupported mail.stored mail flow schema_version %q", event.SchemaVersion)
	}

	messageID := strings.TrimSpace(event.MessageID)
	if messageID == "" {
		return nil, fmt.Errorf("mail.stored mail flow payload is missing message_id")
	}
	if strings.ContainsAny(messageID, "\r\n") {
		return nil, fmt.Errorf("mail.stored mail flow payload has invalid message_id")
	}

	companyID := strings.TrimSpace(event.CompanyID)
	domainID := strings.TrimSpace(event.DomainID)
	userID := strings.TrimSpace(event.UserID)
	receivedAt := strings.TrimSpace(event.ReceivedAt)

	var receivedAtTime *time.Time
	if receivedAt != "" {
		if t, err := time.Parse(time.RFC3339, receivedAt); err == nil {
			receivedAtTime = &t
		}
	}

	var toAddrs []string
	if recipient := strings.TrimSpace(event.Recipient); recipient != "" {
		toAddrs = []string{recipient}
	}

	return &maildb.MailFlowLogEntry{
		CompanyID:    companyID,
		DomainID:     domainID,
		UserID:       userID,
		MessageID:    messageID,
		RFCMessageID: strings.TrimSpace(event.RFCMessageID),
		ToAddrs:      toAddrs,
		Subject:      strings.TrimSpace(event.Subject),
		FlowStatus:   string(maildb.MailFlowStatusReceived),
		Size:         event.Size,
		ReceivedAt:   receivedAtTime,
		ProcessedAt:  receivedAtTime,
		Transport:    "smtp",
		RcptTo:       strings.TrimSpace(event.Recipient),
	}, nil
}

func (h *Handler) parseOutboundEvent(payload json.RawMessage) (*maildb.MailFlowLogEntry, error) {
	var event struct {
		Event           string `json:"event"`
		MessageID       string `json:"message_id"`
		RFCMessageID  string `json:"rfc_message_id"`
		CompanyID     string `json:"company_id"`
		DomainID      string `json:"domain_id"`
		Farm          string `json:"farm"`
		Sender        string `json:"sender"`
		Recipient     string `json:"recipient"`
		RecipientDomain string `json:"recipient_domain"`
		Status        string `json:"status"`
		ErrorMessage  string `json:"error_message"`
		AttemptedAt   string `json:"attempted_at"`
		StoragePath   string `json:"storage_path"`
		EnhancedStatus string `json:"enhanced_status"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("decode delivery mail flow payload: %w", err)
	}
	event.Event = strings.TrimSpace(event.Event)
	event.MessageID = strings.TrimSpace(event.MessageID)
	if event.MessageID == "" {
		return nil, fmt.Errorf("delivery mail flow payload is missing message_id")
	}
	if strings.ContainsAny(event.MessageID, "\r\n") {
		return nil, fmt.Errorf("delivery mail flow payload has invalid message_id")
	}

	flowStatus := h.mapDeliveryStatus(event.Event, event.Status)
	attemptedAt := strings.TrimSpace(event.AttemptedAt)
	var processedAt *time.Time
	if attemptedAt != "" {
		if t, err := time.Parse(time.RFC3339, attemptedAt); err == nil {
			processedAt = &t
		}
	}

	var spamScore *float64

	return &maildb.MailFlowLogEntry{
		CompanyID:     strings.TrimSpace(event.CompanyID),
		DomainID:      strings.TrimSpace(event.DomainID),
		MessageID:     event.MessageID,
		RFCMessageID:  strings.TrimSpace(event.RFCMessageID),
		FromAddr:     strings.TrimSpace(event.Sender),
		ToAddrs:      []string{strings.TrimSpace(event.Recipient)},
		FlowStatus:   string(flowStatus),
		EnhancedStatus: strings.TrimSpace(event.EnhancedStatus),
		ErrorMessage: strings.TrimSpace(event.ErrorMessage),
		SpamScore:    spamScore,
		Farm:         strings.TrimSpace(event.Farm),
		ProcessedAt:  processedAt,
		Transport:    "smtp",
		RcptTo:       strings.TrimSpace(event.Recipient),
	}, nil
}

func (h *Handler) mapDeliveryStatus(event, status string) maildb.MailFlowStatus {
	switch event {
	case "mail.delivered":
		return maildb.MailFlowStatusDelivered
	case "mail.bounced":
		return maildb.MailFlowStatusBounced
	case "mail.delivery_failed":
		if status == "temporary_failure" {
			return maildb.MailFlowStatusPending
		}
		return maildb.MailFlowStatusFailed
	case "mail.delivery_exhausted":
		return maildb.MailFlowStatusFailed
	default:
		return maildb.MailFlowStatusFailed
	}
}
