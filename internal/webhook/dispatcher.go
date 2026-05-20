package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// WebhookEvent represents a single event to be dispatched to company webhooks.
type WebhookEvent struct {
	// Event is the event type, e.g. "mail.received", "mail.sent", "mail.bounced".
	Event string `json:"event"`
	// CompanyID is the ID of the company owning the event.
	CompanyID string `json:"company_id"`
	// OccurredAt is the RFC3339 timestamp when the event occurred.
	OccurredAt string `json:"occurred_at"`
	// Data holds event-specific payload fields.
	Data map[string]interface{} `json:"data"`
}

// WebhookEntry is a single webhook registration stored in company config.
type WebhookEntry struct {
	ID      string   `json:"id"`
	URL     string   `json:"url"`
	Secret  string   `json:"secret"`
	Events  []string `json:"events"`
	Enabled bool     `json:"enabled"`
}

// WebhookConfigLoader loads webhook registrations for a given company.
type WebhookConfigLoader interface {
	LoadWebhooks(ctx context.Context, companyID string) ([]WebhookEntry, error)
}

// WebhookDispatcherOptions configures a WebhookDispatcher.
type WebhookDispatcherOptions struct {
	Loader  WebhookConfigLoader
	Client  *http.Client
	Logger  *slog.Logger
	// URLGuardOpts controls SSRF protection applied to each webhook URL at dispatch time.
	URLGuardOpts OutboundURLGuardOptions
}

// WebhookDispatcher dispatches events to registered company webhooks (best-effort).
type WebhookDispatcher struct {
	loader  WebhookConfigLoader
	client  *http.Client
	logger  *slog.Logger
	guardOpts OutboundURLGuardOptions
}

// NewWebhookDispatcher creates a dispatcher that posts events to company webhook URLs.
func NewWebhookDispatcher(opts WebhookDispatcherOptions) *WebhookDispatcher {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	client := opts.Client
	if client == nil {
		client = GuardedHTTPClient(nil, opts.URLGuardOpts)
	}
	return &WebhookDispatcher{
		loader:    opts.Loader,
		client:    client,
		logger:    logger,
		guardOpts: opts.URLGuardOpts,
	}
}

// Dispatch posts event to all enabled webhooks for the company that subscribe to the event type.
// Failures are logged but not returned; delivery is best-effort.
func (d *WebhookDispatcher) Dispatch(ctx context.Context, event WebhookEvent) error {
	if d.loader == nil {
		return nil
	}
	webhooks, err := d.loader.LoadWebhooks(ctx, event.CompanyID)
	if err != nil {
		return fmt.Errorf("load webhooks for company %q: %w", event.CompanyID, err)
	}

	for _, wh := range webhooks {
		if !wh.Enabled {
			continue
		}
		if !webhookSubscribesToEvent(wh, event.Event) {
			continue
		}
		if err := d.deliver(ctx, wh, event); err != nil {
			d.logger.Warn("webhook delivery failed",
				"webhook_id", wh.ID,
				"event", event.Event,
				"company_id", event.CompanyID,
				"url", wh.URL,
				"error", err,
			)
		}
	}
	return nil
}

func webhookSubscribesToEvent(wh WebhookEntry, eventType string) bool {
	for _, e := range wh.Events {
		if e == eventType || e == "*" {
			return true
		}
	}
	return false
}

func (d *WebhookDispatcher) deliver(ctx context.Context, wh WebhookEntry, event WebhookEvent) error {
	// Validate URL with SSRF guard before each delivery.
	if _, err := ValidateOutboundHTTPURL(ctx, wh.URL, d.guardOpts); err != nil {
		return fmt.Errorf("webhook url rejected: %w", err)
	}

	payload := struct {
		Event      string                 `json:"event"`
		CompanyID  string                 `json:"company_id"`
		OccurredAt string                 `json:"occurred_at"`
		Data       map[string]interface{} `json:"data"`
	}{
		Event:      event.Event,
		CompanyID:  event.CompanyID,
		OccurredAt: event.OccurredAt,
		Data:       event.Data,
	}
	if payload.OccurredAt == "" {
		payload.OccurredAt = time.Now().UTC().Format(time.RFC3339)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	deliveryID := uuid.New().String()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gogomail-Event", event.Event)
	req.Header.Set("X-Gogomail-Delivery", deliveryID)

	// HMAC-SHA256 signature when secret is configured.
	if wh.Secret != "" {
		mac := hmac.New(sha256.New, []byte(wh.Secret))
		mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Gogomail-Signature", "sha256="+sig)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("post webhook: %w", err)
	}
	defer func() { _ = DrainAndClose(resp.Body, DefaultDrainBytes) }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		preview := ErrorBodyPreview(resp.Body, maxWebhookErrorBodyPreviewBytes)
		if preview != "" {
			return fmt.Errorf("webhook returned HTTP %d: %s", resp.StatusCode, preview)
		}
		return fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}

	d.logger.Info("webhook delivered",
		"webhook_id", wh.ID,
		"event", event.Event,
		"company_id", event.CompanyID,
		"delivery_id", deliveryID,
		"status", resp.StatusCode,
	)
	return nil
}
