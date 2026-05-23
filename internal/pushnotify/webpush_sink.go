package pushnotify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// WebPushSubData holds keys needed to send an encrypted push notification.
type WebPushSubData struct {
	ID       string
	Endpoint string
	P256DH   string
	Auth     string
}

// WebPushSubReader reads web push subscriptions from storage.
type WebPushSubReader interface {
	ListActiveWebPushSubscriptions(ctx context.Context, userID string) ([]WebPushSubData, error)
	SoftDeleteWebPushSubscriptionByEndpoint(ctx context.Context, endpoint string) error
}

// WebPushSinkOptions configures a WebPushSink.
type WebPushSinkOptions struct {
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	ContactEmail    string
	DB              WebPushSubReader
	HTTPClient      *http.Client
	Logger          *slog.Logger
}

// WebPushSink implements Sink by sending encrypted Web Push notifications.
type WebPushSink struct {
	vapidPublicKey  string
	vapidPrivateKey string
	contactEmail    string
	db              WebPushSubReader
	client          webpush.HTTPClient
	logger          *slog.Logger
}

// NewWebPushSink creates a WebPushSink.
func NewWebPushSink(opts WebPushSinkOptions) (*WebPushSink, error) {
	opts.VAPIDPublicKey = strings.TrimSpace(opts.VAPIDPublicKey)
	opts.VAPIDPrivateKey = strings.TrimSpace(opts.VAPIDPrivateKey)
	if opts.VAPIDPublicKey == "" {
		return nil, fmt.Errorf("webpush: VAPID public key is required")
	}
	if opts.VAPIDPrivateKey == "" {
		return nil, fmt.Errorf("webpush: VAPID private key is required")
	}
	if opts.DB == nil {
		return nil, fmt.Errorf("webpush: DB reader is required")
	}
	var client webpush.HTTPClient
	if opts.HTTPClient != nil {
		client = opts.HTTPClient
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	sub := opts.ContactEmail
	if sub != "" && !strings.HasPrefix(sub, "mailto:") {
		sub = "mailto:" + sub
	}
	return &WebPushSink{
		vapidPublicKey:  opts.VAPIDPublicKey,
		vapidPrivateKey: opts.VAPIDPrivateKey,
		contactEmail:    sub,
		db:              opts.DB,
		client:          client,
		logger:          logger,
	}, nil
}

// EnqueuePush sends an encrypted push notification to all active subscriptions for the user.
func (s *WebPushSink) EnqueuePush(ctx context.Context, n Notification) error {
	subs, err := s.db.ListActiveWebPushSubscriptions(ctx, n.UserID)
	if err != nil {
		return fmt.Errorf("webpush: list subscriptions: %w", err)
	}
	if len(subs) == 0 {
		return nil
	}
	b, err := json.Marshal(map[string]string{
		"title": webPushTitle(n),
		"body":  n.Subject,
		"tag":   webPushTag(n),
		"url":   "/mail",
	})
	if err != nil {
		return fmt.Errorf("webpush: marshal payload: %w", err)
	}
	for _, sub := range subs {
		s.sendToSubscription(ctx, sub, b)
	}
	return nil
}

func (s *WebPushSink) sendToSubscription(ctx context.Context, sub WebPushSubData, body []byte) {
	opts := &webpush.Options{
		VAPIDPublicKey:  s.vapidPublicKey,
		VAPIDPrivateKey: s.vapidPrivateKey,
		Subscriber:      s.contactEmail,
		TTL:             86400,
		Urgency:         webpush.UrgencyNormal,
	}
	if s.client != nil {
		opts.HTTPClient = s.client
	}
	resp, err := webpush.SendNotificationWithContext(ctx, body, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			Auth:   sub.Auth,
			P256dh: sub.P256DH,
		},
	}, opts)
	if err != nil {
		s.logger.Warn("webpush send error",
			"endpoint_suffix", endpointSuffix(sub.Endpoint),
			"error", err,
		)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusGone {
		if delErr := s.db.SoftDeleteWebPushSubscriptionByEndpoint(ctx, sub.Endpoint); delErr != nil {
			s.logger.Warn("webpush soft-delete gone subscription", "error", delErr)
		}
	}
}

func webPushTitle(n Notification) string {
	from := strings.TrimSpace(n.EnvelopeFrom)
	if from != "" {
		return from
	}
	if n.Recipient != "" {
		return n.Recipient
	}
	return "새 메일"
}

func webPushTag(n Notification) string {
	if n.MessageID != "" {
		tag := "mail-" + n.MessageID
		if len(tag) > 128 {
			return tag[:128]
		}
		return tag
	}
	return "mail-received"
}

func endpointSuffix(endpoint string) string {
	if len(endpoint) <= 16 {
		return endpoint
	}
	return "..." + endpoint[len(endpoint)-16:]
}
