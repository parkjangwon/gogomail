package inboundfilter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
	"github.com/gogomail/gogomail/internal/outbound"
)

const EventMailStored = "mail.stored"

// vacationCooldown is how long to wait before sending another vacation reply to the same sender.
const vacationCooldown = 7 * 24 * time.Hour

// Service is the subset of mailservice.Service used by the handler.
type Service interface {
	GetWebmailPreferences(ctx context.Context, userID string) (json.RawMessage, error)
	ListFolders(ctx context.Context, userID string) ([]maildb.Folder, error)
	MoveMessage(ctx context.Context, userID string, messageID string, folderID string) error
	SendText(ctx context.Context, req mailservice.SendTextRequest) (mailservice.SendTextResult, error)
}

// Handler processes mail.stored events and applies user-level inbound filters.
type Handler struct {
	service Service
	// vacationSent tracks when we last sent a vacation reply: key = "userID:senderNorm"
	vacationSent sync.Map
}

func NewHandler(svc Service) *Handler {
	return &Handler{service: svc}
}

type storedEvent struct {
	Event        string `json:"event"`
	MessageID    string `json:"message_id"`
	UserID       string `json:"user_id"`
	FolderID     string `json:"folder_id"`
	EnvelopeFrom string `json:"envelope_from"`
	Recipient    string `json:"recipient"`
	Subject      string `json:"subject"`
	InReplyTo    string `json:"in_reply_to"`
}

func (h *Handler) HandleEvent(ctx context.Context, msg eventstream.Message) error {
	var ev storedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		return fmt.Errorf("inboundfilter: decode event: %w", err)
	}
	if ev.Event != EventMailStored {
		return nil
	}
	if strings.TrimSpace(ev.UserID) == "" || strings.TrimSpace(ev.MessageID) == "" {
		return nil
	}

	prefs, err := h.service.GetWebmailPreferences(ctx, ev.UserID)
	if err != nil {
		return fmt.Errorf("inboundfilter: get preferences: %w", err)
	}

	var p preferences
	if err := json.Unmarshal(prefs, &p); err != nil {
		return nil // malformed prefs — skip silently
	}

	// 1. Blocked senders
	if len(p.BlockedSenders) > 0 && ev.EnvelopeFrom != "" {
		fromNorm := strings.ToLower(strings.TrimSpace(ev.EnvelopeFrom))
		for _, blocked := range p.BlockedSenders {
			if strings.ToLower(strings.TrimSpace(blocked)) == fromNorm {
				if err := h.moveToTrash(ctx, ev.UserID, ev.MessageID); err != nil {
					return fmt.Errorf("inboundfilter: move blocked message: %w", err)
				}
				return nil // no vacation reply for blocked senders
			}
		}
	}

	// 2. Vacation auto-reply
	if p.Vacation != nil && p.Vacation.Enabled {
		h.maybeVacationReply(ctx, ev, p.Vacation)
	}

	return nil
}

func (h *Handler) moveToTrash(ctx context.Context, userID, messageID string) error {
	folders, err := h.service.ListFolders(ctx, userID)
	if err != nil {
		return err
	}
	for _, f := range folders {
		if f.SystemType == "trash" {
			return h.service.MoveMessage(ctx, userID, messageID, f.ID)
		}
	}
	return nil // no trash folder — do nothing
}

func (h *Handler) maybeVacationReply(ctx context.Context, ev storedEvent, vac *vacationSettings) {
	sender := strings.TrimSpace(ev.EnvelopeFrom)
	if sender == "" {
		return
	}
	// Don't reply to auto-generated addresses.
	lower := strings.ToLower(sender)
	for _, prefix := range []string{"mailer-daemon", "postmaster", "no-reply", "noreply", "do-not-reply", "donotreply"} {
		if strings.HasPrefix(lower, prefix+"@") || strings.HasPrefix(lower, prefix+"+") {
			return
		}
	}
	// Don't reply to yourself.
	if strings.EqualFold(sender, strings.TrimSpace(ev.Recipient)) {
		return
	}

	// Date range check.
	now := time.Now().UTC()
	if vac.StartDate != "" {
		if start, err := time.Parse("2006-01-02", vac.StartDate); err == nil && now.Before(start) {
			return
		}
	}
	if vac.EndDate != "" {
		if end, err := time.Parse("2006-01-02", vac.EndDate); err == nil && now.After(end.AddDate(0, 0, 1)) {
			return
		}
	}

	// Rate limiting: one reply per sender per vacationCooldown.
	key := ev.UserID + ":" + strings.ToLower(sender)
	if v, ok := h.vacationSent.Load(key); ok {
		if sentAt, ok := v.(time.Time); ok && time.Since(sentAt) < vacationCooldown {
			return
		}
	}
	h.vacationSent.Store(key, now)

	msg := vac.Message
	if msg == "" {
		msg = "자리를 비우고 있습니다."
	}
	subject := "Re: " + ev.Subject
	if ev.Subject == "" {
		subject = "부재중 자동 응답"
	}

	_, _ = h.service.SendText(ctx, mailservice.SendTextRequest{
		UserID:        ev.UserID,
		From:          ev.Recipient,
		To:            []outbound.Address{{Email: sender}},
		Subject:       subject,
		TextBody:      msg,
		Transactional: true,
	})
}

type preferences struct {
	BlockedSenders []string         `json:"blocked_senders"`
	Vacation       *vacationSettings `json:"vacation"`
}

type vacationSettings struct {
	Enabled   bool   `json:"enabled"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Message   string `json:"message"`
}
