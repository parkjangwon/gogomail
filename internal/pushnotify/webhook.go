package pushnotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type WebhookOptions struct {
	Endpoint string
	Token    string
	Client   *http.Client
}

type WebhookSink struct {
	endpoint string
	token    string
	client   *http.Client
}

func NewWebhookSink(opts WebhookOptions) (*WebhookSink, error) {
	endpoint := strings.TrimSpace(opts.Endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("push notification webhook endpoint must be a valid URL: %w", err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("push notification webhook endpoint must be an http or https URL")
	}
	client := opts.Client
	if client == nil {
		client = http.DefaultClient
	}
	return &WebhookSink{endpoint: endpoint, token: strings.TrimSpace(opts.Token), client: client}, nil
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
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
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
		targets = append(targets, webhookTarget{
			AttemptID:   target.AttemptID,
			DeviceID:    target.DeviceID,
			Platform:    target.Platform,
			Token:       target.Token,
			TokenSuffix: target.TokenSuffix,
			Label:       target.Label,
		})
	}
	return webhookPayload{
		MessageID:    notification.MessageID,
		RFCMessageID: notification.RFCMessageID,
		CompanyID:    notification.CompanyID,
		DomainID:     notification.DomainID,
		UserID:       notification.UserID,
		Recipient:    notification.Recipient,
		Subject:      notification.Subject,
		ReceivedAt:   notification.ReceivedAt,
		Targets:      targets,
	}
}
