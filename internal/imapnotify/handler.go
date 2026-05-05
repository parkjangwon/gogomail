package imapnotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/gogomail/gogomail/internal/maildb"
)

const (
	EventMailStored           = "mail.stored"
	MailStoredSchemaV1        = "2026-05-04.mail-stored.v1"
	maxMailStoredEventIDBytes = 200
)

type UIDEnsurer interface {
	EnsureIMAPMessageUID(ctx context.Context, userID string, mailboxID string, messageID string) (maildb.IMAPMessageUID, error)
}

type MailboxEventPublisher interface {
	Publish(ctx context.Context, event imapgw.MailboxEvent) error
}

type MailStoredHandler struct {
	uidEnsurer UIDEnsurer
	events     MailboxEventPublisher
}

func NewMailStoredHandler(uidEnsurer UIDEnsurer) *MailStoredHandler {
	return &MailStoredHandler{uidEnsurer: uidEnsurer}
}

func (h *MailStoredHandler) WithMailboxEvents(events MailboxEventPublisher) *MailStoredHandler {
	h.events = events
	return h
}

func (h *MailStoredHandler) HandleEvent(ctx context.Context, msg eventstream.Message) error {
	if h == nil || h.uidEnsurer == nil {
		return fmt.Errorf("imap uid ensurer is required")
	}
	event, err := DecodeMailStoredEvent(msg.Payload)
	if err != nil {
		return err
	}
	uid, err := h.uidEnsurer.EnsureIMAPMessageUID(ctx, event.UserID, event.FolderID, event.MessageID)
	if err != nil {
		if errors.Is(err, maildb.ErrIMAPMessageNotActive) {
			return nil
		}
		return fmt.Errorf("ensure imap uid for stored message %q: %w", event.MessageID, err)
	}
	if h.events != nil {
		if err := h.events.Publish(ctx, imapgw.MailboxEvent{
			Type:      imapgw.MailboxEventExists,
			UserID:    imapgw.UserID(event.UserID),
			MailboxID: uid.MailboxID,
			UID:       uid.UID,
		}); err != nil {
			return fmt.Errorf("publish imap exists for stored message %q: %w", event.MessageID, err)
		}
	}
	return nil
}

type MailStoredEvent struct {
	Event         string `json:"event"`
	SchemaVersion string `json:"schema_version"`
	MessageID     string `json:"message_id"`
	UserID        string `json:"user_id"`
	FolderID      string `json:"folder_id"`
}

func DecodeMailStoredEvent(payload json.RawMessage) (MailStoredEvent, error) {
	var event MailStoredEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return MailStoredEvent{}, fmt.Errorf("decode mail.stored imap payload: %w", err)
	}
	event.Event = strings.TrimSpace(event.Event)
	if event.Event != EventMailStored {
		return MailStoredEvent{}, fmt.Errorf("unexpected imap notification event %q", event.Event)
	}
	event.SchemaVersion = strings.TrimSpace(event.SchemaVersion)
	if event.SchemaVersion != "" && event.SchemaVersion != MailStoredSchemaV1 {
		return MailStoredEvent{}, fmt.Errorf("unsupported mail.stored imap schema_version %q", event.SchemaVersion)
	}
	var err error
	if event.MessageID, err = requiredEventValue("message_id", event.MessageID); err != nil {
		return MailStoredEvent{}, err
	}
	if event.UserID, err = requiredEventValue("user_id", event.UserID); err != nil {
		return MailStoredEvent{}, err
	}
	if event.FolderID, err = requiredEventValue("folder_id", event.FolderID); err != nil {
		return MailStoredEvent{}, err
	}
	return event, nil
}

func requiredEventValue(name string, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("mail.stored imap payload is missing %s", name)
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("mail.stored imap payload has invalid %s", name)
	}
	if len(value) > maxMailStoredEventIDBytes {
		return "", fmt.Errorf("mail.stored imap payload has oversized %s", name)
	}
	return value, nil
}
