package inboundfilter

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
	"github.com/gogomail/gogomail/internal/outbound"
)

// fakeService implements Service for testing.
type fakeService struct {
	prefs       json.RawMessage
	folders     []maildb.Folder
	movedTo     string // last folder ID passed to MoveMessage
	sentTo      []string
	sendTextErr error
}

func (f *fakeService) GetWebmailPreferences(_ context.Context, _ string) (json.RawMessage, error) {
	return f.prefs, nil
}

func (f *fakeService) ListFolders(_ context.Context, _ string) ([]maildb.Folder, error) {
	return f.folders, nil
}

func (f *fakeService) MoveMessage(_ context.Context, _, _, folderID string) error {
	f.movedTo = folderID
	return nil
}

func (f *fakeService) SendText(_ context.Context, req mailservice.SendTextRequest) (mailservice.SendTextResult, error) {
	for _, t := range req.To {
		f.sentTo = append(f.sentTo, t.Email)
	}
	return mailservice.SendTextResult{}, f.sendTextErr
}

func mustEvent(t *testing.T, ev storedEvent) eventstream.Message {
	t.Helper()
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	return eventstream.Message{Payload: b}
}

func mustPrefs(t *testing.T, p preferences) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal prefs: %v", err)
	}
	return b
}

func TestBlockedSender_MovesToTrash(t *testing.T) {
	svc := &fakeService{
		prefs: mustPrefs(t, preferences{
			BlockedSenders: []string{"spammer@evil.com"},
		}),
		folders: []maildb.Folder{
			{ID: "trash-id", SystemType: "trash"},
			{ID: "inbox-id", SystemType: "inbox"},
		},
	}
	h := NewHandler(svc)

	ev := storedEvent{
		Event:        EventMailStored,
		MessageID:    "msg-1",
		UserID:       "user-1",
		FolderID:     "inbox-id",
		EnvelopeFrom: "spammer@evil.com",
		Recipient:    "alice@example.com",
		Subject:      "Buy now!",
	}
	if err := h.HandleEvent(context.Background(), mustEvent(t, ev)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if svc.movedTo != "trash-id" {
		t.Errorf("expected message moved to trash-id, got %q", svc.movedTo)
	}
	if len(svc.sentTo) != 0 {
		t.Errorf("expected no vacation reply sent to blocked sender, got %v", svc.sentTo)
	}
}

func TestBlockedSender_CaseInsensitive(t *testing.T) {
	svc := &fakeService{
		prefs: mustPrefs(t, preferences{
			BlockedSenders: []string{"SPAMMER@EVIL.COM"},
		}),
		folders: []maildb.Folder{{ID: "trash-id", SystemType: "trash"}},
	}
	h := NewHandler(svc)

	ev := storedEvent{
		Event: EventMailStored, MessageID: "msg-2", UserID: "user-1",
		EnvelopeFrom: "spammer@evil.com", Recipient: "alice@example.com",
	}
	if err := h.HandleEvent(context.Background(), mustEvent(t, ev)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if svc.movedTo != "trash-id" {
		t.Errorf("expected move to trash, got %q", svc.movedTo)
	}
}

func TestBlockedSender_NotBlocked_NoMove(t *testing.T) {
	svc := &fakeService{
		prefs: mustPrefs(t, preferences{
			BlockedSenders: []string{"other@evil.com"},
		}),
		folders: []maildb.Folder{{ID: "trash-id", SystemType: "trash"}},
	}
	h := NewHandler(svc)

	ev := storedEvent{
		Event: EventMailStored, MessageID: "msg-3", UserID: "user-1",
		EnvelopeFrom: "friend@good.com", Recipient: "alice@example.com",
	}
	if err := h.HandleEvent(context.Background(), mustEvent(t, ev)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if svc.movedTo != "" {
		t.Errorf("expected no move, got %q", svc.movedTo)
	}
}

func TestVacation_ActiveDateRange_SendsReply(t *testing.T) {
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	tomorrow := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")

	svc := &fakeService{
		prefs: mustPrefs(t, preferences{
			Vacation: &vacationSettings{
				Enabled:   true,
				StartDate: yesterday,
				EndDate:   tomorrow,
				Message:   "I'm away.",
			},
		}),
		folders: []maildb.Folder{{ID: "inbox-id", SystemType: "inbox"}},
	}
	h := NewHandler(svc)

	ev := storedEvent{
		Event: EventMailStored, MessageID: "msg-4", UserID: "user-1",
		EnvelopeFrom: "sender@example.com", Recipient: "alice@example.com",
		Subject: "Hello",
	}
	if err := h.HandleEvent(context.Background(), mustEvent(t, ev)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(svc.sentTo) == 0 || svc.sentTo[0] != "sender@example.com" {
		t.Errorf("expected vacation reply to sender@example.com, got %v", svc.sentTo)
	}
}

func TestVacation_Disabled_NoReply(t *testing.T) {
	svc := &fakeService{
		prefs: mustPrefs(t, preferences{
			Vacation: &vacationSettings{Enabled: false, Message: "away"},
		}),
		folders: []maildb.Folder{},
	}
	h := NewHandler(svc)

	ev := storedEvent{
		Event: EventMailStored, MessageID: "msg-5", UserID: "user-1",
		EnvelopeFrom: "sender@example.com", Recipient: "alice@example.com",
	}
	if err := h.HandleEvent(context.Background(), mustEvent(t, ev)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(svc.sentTo) != 0 {
		t.Errorf("expected no reply when vacation disabled, got %v", svc.sentTo)
	}
}

func TestVacation_BeforeStartDate_NoReply(t *testing.T) {
	tomorrow := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	nextWeek := time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02")

	svc := &fakeService{
		prefs: mustPrefs(t, preferences{
			Vacation: &vacationSettings{
				Enabled: true, StartDate: tomorrow, EndDate: nextWeek, Message: "away",
			},
		}),
	}
	h := NewHandler(svc)

	ev := storedEvent{
		Event: EventMailStored, MessageID: "msg-6", UserID: "user-1",
		EnvelopeFrom: "sender@example.com", Recipient: "alice@example.com",
	}
	if err := h.HandleEvent(context.Background(), mustEvent(t, ev)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(svc.sentTo) != 0 {
		t.Errorf("expected no reply before vacation start, got %v", svc.sentTo)
	}
}

func TestVacation_MailerDaemon_NoReply(t *testing.T) {
	svc := &fakeService{
		prefs: mustPrefs(t, preferences{
			Vacation: &vacationSettings{Enabled: true, Message: "away"},
		}),
	}
	h := NewHandler(svc)

	for _, from := range []string{
		"mailer-daemon@example.com",
		"postmaster@example.com",
		"noreply@example.com",
		"no-reply@example.com",
	} {
		svc.sentTo = nil
		ev := storedEvent{
			Event: EventMailStored, MessageID: "msg-loop", UserID: "user-1",
			EnvelopeFrom: from, Recipient: "alice@example.com",
		}
		if err := h.HandleEvent(context.Background(), mustEvent(t, ev)); err != nil {
			t.Fatalf("HandleEvent(%s): %v", from, err)
		}
		if len(svc.sentTo) != 0 {
			t.Errorf("expected no reply to %s, got %v", from, svc.sentTo)
		}
	}
}

func TestVacation_RateLimit_OncePerCooldown(t *testing.T) {
	svc := &fakeService{
		prefs: mustPrefs(t, preferences{
			Vacation: &vacationSettings{Enabled: true, Message: "away"},
		}),
	}
	h := NewHandler(svc)

	ev := storedEvent{
		Event: EventMailStored, MessageID: "msg-7", UserID: "user-1",
		EnvelopeFrom: "sender@example.com", Recipient: "alice@example.com",
	}
	if err := h.HandleEvent(context.Background(), mustEvent(t, ev)); err != nil {
		t.Fatalf("first HandleEvent: %v", err)
	}
	first := len(svc.sentTo)

	// Second event from same sender within cooldown window
	ev.MessageID = "msg-8"
	if err := h.HandleEvent(context.Background(), mustEvent(t, ev)); err != nil {
		t.Fatalf("second HandleEvent: %v", err)
	}
	if len(svc.sentTo) != first {
		t.Errorf("expected no second reply within cooldown, sentTo=%v", svc.sentTo)
	}
}

// Verify outbound.Address import is used (compilation check).
var _ = outbound.Address{}
