package searchindex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/message"
)

const (
	EventMailStored        = "mail.stored"
	MailStoredSchemaV1     = "2026-05-04.mail-stored.v1"
	defaultMaxTextBodyByte = int64(1 << 20)
)

type Event struct {
	Event         string   `json:"event"`
	SchemaVersion string   `json:"schema_version"`
	MessageID     string   `json:"message_id"`
	RFCMessageID  string   `json:"rfc_message_id"`
	InReplyTo     string   `json:"in_reply_to"`
	References    []string `json:"references"`
	CompanyID     string   `json:"company_id"`
	DomainID      string   `json:"domain_id"`
	UserID        string   `json:"user_id"`
	FolderID      string   `json:"folder_id"`
	Recipient     string   `json:"recipient"`
	Subject       string   `json:"subject"`
	StoragePath   string   `json:"storage_path"`
	ReceivedAt    string   `json:"received_at"`
	Size          int64    `json:"size"`
}

type Document struct {
	MessageID     string
	RFCMessageID  string
	InReplyTo     string
	References    []string
	CompanyID     string
	DomainID      string
	UserID        string
	FolderID      string
	Recipient     string
	Subject       string
	FromAddr      string
	FromName      string
	StoragePath   string
	ReceivedAt    string
	Size          int64
	HasAttachment bool
	BodyText      string
	BodyTruncated bool
	BodyMaxBytes  int64
}

type Indexer interface {
	IndexMessage(ctx context.Context, doc Document) error
}

type StoreReader interface {
	Open(ctx context.Context, path string) (io.ReadCloser, error)
}

type StorageGetter interface {
	Get(ctx context.Context, path string) (io.ReadCloser, error)
}

type HandlerOptions struct {
	MaxTextBodyBytes int64
}

type Handler struct {
	store            StoreReader
	indexer          Indexer
	maxTextBodyBytes int64
}

func NewHandler(store StoreReader, indexer Indexer, opts HandlerOptions) *Handler {
	maxTextBodyBytes := opts.MaxTextBodyBytes
	if maxTextBodyBytes <= 0 {
		maxTextBodyBytes = defaultMaxTextBodyByte
	}
	return &Handler{
		store:            store,
		indexer:          indexer,
		maxTextBodyBytes: maxTextBodyBytes,
	}
}

func NewStorageStoreReader(store StorageGetter) StoreReader {
	return storageStoreReader{store: store}
}

func (h *Handler) HandleEvent(ctx context.Context, msg eventstream.Message) error {
	if h == nil {
		return fmt.Errorf("search index handler is required")
	}
	if h.store == nil {
		return fmt.Errorf("search index store is required")
	}
	if h.indexer == nil {
		return fmt.Errorf("search indexer is required")
	}

	event, err := DecodeEvent(msg.Payload)
	if err != nil {
		return err
	}

	raw, err := h.store.Open(ctx, event.StoragePath)
	if err != nil {
		return fmt.Errorf("open stored message %q: %w", event.StoragePath, err)
	}
	defer raw.Close()

	parsed, err := message.ParseEMLWithOptions(raw, message.ParseOptions{
		MaxTextBodyBytes: h.maxTextBodyBytes,
		MaxAttachments:   1,
	})
	if err != nil {
		return fmt.Errorf("parse stored message %q for search indexing: %w", event.StoragePath, err)
	}

	doc := Document{
		MessageID:     event.MessageID,
		RFCMessageID:  firstNonEmpty(event.RFCMessageID, parsed.MessageID),
		InReplyTo:     firstNonEmpty(event.InReplyTo, parsed.InReplyTo),
		References:    append([]string(nil), event.References...),
		CompanyID:     event.CompanyID,
		DomainID:      event.DomainID,
		UserID:        event.UserID,
		FolderID:      event.FolderID,
		Recipient:     event.Recipient,
		Subject:       firstNonEmpty(event.Subject, parsed.Subject),
		FromAddr:      parsed.From.Address,
		FromName:      parsed.From.Name,
		StoragePath:   event.StoragePath,
		ReceivedAt:    event.ReceivedAt,
		Size:          event.Size,
		HasAttachment: parsed.HasAttachment,
		BodyText:      parsed.TextBody,
		BodyTruncated: parsed.TextBodyTruncated,
		BodyMaxBytes:  h.maxTextBodyBytes,
	}
	if len(doc.References) == 0 {
		doc.References = append([]string(nil), parsed.References...)
	}

	if err := h.indexer.IndexMessage(ctx, doc); err != nil {
		return fmt.Errorf("index stored message %q: %w", event.MessageID, err)
	}
	return nil
}

func DecodeEvent(payload json.RawMessage) (Event, error) {
	var event Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return Event{}, fmt.Errorf("decode mail.stored search payload: %w", err)
	}
	if err := validateEvent(&event); err != nil {
		return Event{}, err
	}
	return event, nil
}

func validateEvent(event *Event) error {
	event.Event = strings.TrimSpace(event.Event)
	if event.Event != EventMailStored {
		return fmt.Errorf("unexpected search index event %q", event.Event)
	}
	event.SchemaVersion = strings.TrimSpace(event.SchemaVersion)
	if event.SchemaVersion != "" && event.SchemaVersion != MailStoredSchemaV1 {
		return fmt.Errorf("unsupported mail.stored search schema_version %q", event.SchemaVersion)
	}

	var err error
	if event.MessageID, err = requiredValue("message_id", event.MessageID); err != nil {
		return err
	}
	if event.UserID, err = requiredValue("user_id", event.UserID); err != nil {
		return err
	}
	if event.StoragePath, err = requiredStoragePath(event.StoragePath); err != nil {
		return err
	}

	event.RFCMessageID = strings.TrimSpace(event.RFCMessageID)
	event.InReplyTo = strings.TrimSpace(event.InReplyTo)
	event.CompanyID = strings.TrimSpace(event.CompanyID)
	event.DomainID = strings.TrimSpace(event.DomainID)
	event.FolderID = strings.TrimSpace(event.FolderID)
	event.Recipient = strings.TrimSpace(event.Recipient)
	event.Subject = strings.TrimSpace(event.Subject)
	event.ReceivedAt = strings.TrimSpace(event.ReceivedAt)
	event.References = cleanReferences(event.References)
	return nil
}

func requiredValue(name, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("mail.stored search payload is missing %s", name)
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("mail.stored search payload has invalid %s", name)
	}
	return value, nil
}

func requiredStoragePath(value string) (string, error) {
	value, err := requiredValue("storage_path", value)
	if err != nil {
		return "", err
	}
	cleaned := path.Clean(value)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." || strings.HasPrefix(cleaned, "/") || strings.Contains(cleaned, "\\") {
		return "", fmt.Errorf("mail.stored search payload has invalid storage_path")
	}
	if !strings.HasSuffix(strings.ToLower(cleaned), ".eml") {
		return "", fmt.Errorf("mail.stored search payload storage_path must reference an .eml object")
	}
	return cleaned, nil
}

func cleanReferences(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !strings.ContainsAny(value, "\r\n") {
			out = append(out, value)
		}
	}
	return out
}

func firstNonEmpty(primary, fallback string) string {
	primary = strings.TrimSpace(primary)
	if primary != "" {
		return primary
	}
	return strings.TrimSpace(fallback)
}

type storageStoreReader struct {
	store StorageGetter
}

func (r storageStoreReader) Open(ctx context.Context, path string) (io.ReadCloser, error) {
	if r.store == nil {
		return nil, fmt.Errorf("storage store is required")
	}
	return r.store.Get(ctx, path)
}
