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
	// trashCache caches system trash folder IDs per user: key = userID, value = string (folderID)
	trashCache sync.Map
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

	// 1. Personal allowlist — takes precedence over blocked-sender check.
	// If the sender is explicitly allowed by the user, skip the blocklist.
	allowed := ev.EnvelopeFrom != "" &&
		len(p.AllowedSenders) > 0 &&
		matchesSender(ev.EnvelopeFrom, p.AllowedSenders)

	// 2. Blocked senders (skipped when sender is personally allowed).
	if !allowed && ev.EnvelopeFrom != "" && len(p.BlockedSenders) > 0 {
		if matchesSender(ev.EnvelopeFrom, p.BlockedSenders) {
			if err := h.moveToTrash(ctx, ev.UserID, ev.MessageID); err != nil {
				return fmt.Errorf("inboundfilter: move blocked message: %w", err)
			}
			return nil // no vacation reply for blocked senders
		}
	}

	// 2. Vacation auto-reply
	if p.Vacation != nil && p.Vacation.Enabled {
		h.maybeVacationReply(ctx, ev, p.Vacation)
	}

	return nil
}

func (h *Handler) moveToTrash(ctx context.Context, userID, messageID string) error {
	trashID, err := h.resolveTrashFolderID(ctx, userID)
	if err != nil {
		return err
	}
	if trashID == "" {
		return nil // no trash folder — do nothing
	}
	return h.service.MoveMessage(ctx, userID, messageID, trashID)
}

// resolveTrashFolderID returns the system trash folder ID for userID, caching
// the result to avoid a ListFolders call on every blocked-sender event.
func (h *Handler) resolveTrashFolderID(ctx context.Context, userID string) (string, error) {
	if v, ok := h.trashCache.Load(userID); ok {
		if s, ok := v.(string); ok {
			return s, nil
		}
	}
	folders, err := h.service.ListFolders(ctx, userID)
	if err != nil {
		return "", err
	}
	id := ""
	for _, f := range folders {
		if f.SystemType == "trash" {
			id = f.ID
			break
		}
	}
	h.trashCache.Store(userID, id)
	return id, nil
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
	h.pruneVacationSent(now)

	// Prefer Body/Subject (current webmail format); fall back to legacy Message field.
	msg := vac.Body
	if msg == "" {
		msg = vac.Message
	}
	if msg == "" {
		msg = "자리를 비우고 있습니다."
	}
	vacSubject := vac.Subject
	subject := ""
	if vacSubject != "" {
		subject = vacSubject
	} else if ev.Subject != "" {
		subject = "Re: " + ev.Subject
	} else {
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

// pruneVacationSent removes expired entries from vacationSent to prevent unbounded growth.
func (h *Handler) pruneVacationSent(now time.Time) {
	h.vacationSent.Range(func(k, v any) bool {
		if sentAt, ok := v.(time.Time); ok && now.Sub(sentAt) >= vacationCooldown {
			h.vacationSent.Delete(k)
		}
		return true
	})
}

type preferences struct {
	BlockedSenders []string          `json:"blocked_senders"`
	AllowedSenders []string          `json:"allowed_senders"`
	Vacation       *vacationSettings `json:"vacation"`
}

// matchesSender returns true if addr matches an entry in list.
// Entries are either exact email addresses (case-insensitive) or "@domain"
// prefix patterns that match any address at that domain.
func matchesSender(addr string, list []string) bool {
	addrNorm := strings.ToLower(strings.TrimSpace(addr))
	if addrNorm == "" {
		return false
	}
	atIdx := strings.Index(addrNorm, "@")
	addrDomain := ""
	if atIdx >= 0 {
		addrDomain = addrNorm[atIdx:] // e.g. "@example.com"
	}
	for _, entry := range list {
		entryNorm := strings.ToLower(strings.TrimSpace(entry))
		if entryNorm == "" {
			continue
		}
		if strings.HasPrefix(entryNorm, "@") {
			// Domain pattern: match any address at this domain.
			if addrDomain != "" && addrDomain == entryNorm {
				return true
			}
		} else {
			if entryNorm == addrNorm {
				return true
			}
		}
	}
	return false
}

type vacationSettings struct {
	Enabled   bool   `json:"enabled"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Body      string `json:"body"`
	Subject   string `json:"subject"`
	// Message is kept for backward-compatibility with older stored prefs.
	Message string `json:"message"`
}
