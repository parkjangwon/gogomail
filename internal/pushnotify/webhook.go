package pushnotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gogomail/gogomail/internal/webhook"
)

const (
	maxWebhookIdentityBytes  = 200
	maxWebhookAddressBytes   = 320
	maxWebhookMessageIDBytes = 500
	maxWebhookSubjectBytes   = 500
	maxWebhookTokenBytes     = 4096
	maxWebhookLabelBytes     = 200
	maxWebhookTimestampBytes = 100
)

type WebhookOptions struct {
	Endpoint            string
	Token               string
	Client              *http.Client
	AllowPrivateNetwork bool
}

type WebhookSink struct {
	endpoint string
	token    string
	client   *http.Client
}

func NewWebhookSink(opts WebhookOptions) (*WebhookSink, error) {
	endpoint := strings.TrimSpace(opts.Endpoint)
	if err := webhook.ValidateOutboundHTTPURLFormat(opts.Endpoint); err != nil {
		return nil, fmt.Errorf("push notification webhook endpoint: %w", err)
	}
	client := opts.Client
	if client == nil {
		client = http.DefaultClient
	}
	token, err := webhook.NormalizeWebhookToken(opts.Token, maxWebhookTokenBytes)
	if err != nil {
		return nil, fmt.Errorf("push notification webhook token: %w", err)
	}
	return &WebhookSink{endpoint: endpoint, token: token, client: client}, nil
}

func (s *WebhookSink) EnqueuePush(ctx context.Context, notification Notification) error {
	if s == nil || s.client == nil || strings.TrimSpace(s.endpoint) == "" {
		return fmt.Errorf("push notification webhook is not configured")
	}
	raw, err := json.Marshal(webhookPayloadFromNotification(notification))
	if err != nil {
		return fmt.Errorf("marshal push notification webhook payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("build push notification webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("call push notification webhook: %w", err)
	}
	defer func() { _ = webhook.DrainAndClose(resp.Body, webhook.DefaultDrainBytes) }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if preview := webhook.ErrorBodyPreview(resp.Body, 512); preview != "" {
			return fmt.Errorf("push notification webhook returned HTTP %d: %s", resp.StatusCode, preview)
		}
		return fmt.Errorf("push notification webhook returned HTTP %d", resp.StatusCode)
	}
	return nil
}

type webhookPayload struct {
	MessageID    string          `json:"message_id"`
	RFCMessageID string          `json:"rfc_message_id,omitempty"`
	CompanyID    string          `json:"company_id,omitempty"`
	DomainID     string          `json:"domain_id,omitempty"`
	UserID       string          `json:"user_id"`
	Recipient    string          `json:"recipient,omitempty"`
	Subject      string          `json:"subject,omitempty"`
	ReceivedAt   string          `json:"received_at,omitempty"`
	Targets      []webhookTarget `json:"targets"`
}

type webhookTarget struct {
	AttemptID   string `json:"attempt_id,omitempty"`
	DeviceID    string `json:"device_id"`
	Platform    string `json:"platform"`
	Token       string `json:"token"`
	TokenSuffix string `json:"token_suffix,omitempty"`
	Label       string `json:"label,omitempty"`
}

func webhookPayloadFromNotification(notification Notification) webhookPayload {
	targets := make([]webhookTarget, 0, len(notification.Targets))
	for _, target := range notification.Targets {
		cleanTarget, ok := webhookTargetFromTarget(target)
		if !ok {
			continue
		}
		targets = append(targets, cleanTarget)
	}
	return webhookPayload{
		MessageID:    cleanWebhookText(notification.MessageID, maxWebhookIdentityBytes),
		RFCMessageID: cleanWebhookText(notification.RFCMessageID, maxWebhookMessageIDBytes),
		CompanyID:    cleanWebhookText(notification.CompanyID, maxWebhookIdentityBytes),
		DomainID:     cleanWebhookText(notification.DomainID, maxWebhookIdentityBytes),
		UserID:       cleanWebhookText(notification.UserID, maxWebhookIdentityBytes),
		Recipient:    cleanWebhookText(notification.Recipient, maxWebhookAddressBytes),
		Subject:      cleanWebhookText(notification.Subject, maxWebhookSubjectBytes),
		ReceivedAt:   cleanWebhookText(notification.ReceivedAt, maxWebhookTimestampBytes),
		Targets:      targets,
	}
}

func webhookTargetFromTarget(target Target) (webhookTarget, bool) {
	deviceID := cleanWebhookText(target.DeviceID, maxWebhookIdentityBytes)
	platform := strings.ToLower(cleanWebhookText(target.Platform, maxWebhookIdentityBytes))
	token := cleanWebhookText(target.Token, maxWebhookTokenBytes)
	if deviceID == "" || platform == "" || token == "" || !maildbAllowedPushPlatform(platform) {
		return webhookTarget{}, false
	}
	if strings.TrimSpace(target.Token) != token {
		return webhookTarget{}, false
	}
	return webhookTarget{
		AttemptID:   cleanWebhookText(target.AttemptID, maxWebhookIdentityBytes),
		DeviceID:    deviceID,
		Platform:    platform,
		Token:       token,
		TokenSuffix: cleanWebhookText(target.TokenSuffix, maxWebhookLabelBytes),
		Label:       cleanWebhookText(target.Label, maxWebhookLabelBytes),
	}, true
}

func cleanWebhookText(value string, maxBytes int) string {
	value = strings.ToValidUTF8(strings.TrimSpace(value), "")
	value = strings.NewReplacer("\r", " ", "\n", " ").Replace(value)
	value = strings.Join(strings.Fields(value), " ")
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	cut := 0
	for i := range value {
		if i > maxBytes {
			return value[:cut]
		}
		cut = i
	}
	return value[:cut]
}
