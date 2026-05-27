package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

// EmailDispatcher sends alert notifications via SMTP.
type EmailDispatcher struct {
	SMTPAddr string // host:port, e.g. "localhost:25"
	From     string // GOGOMAIL_ALERT_EMAIL_FROM
	To       string // GOGOMAIL_ALERT_EMAIL_TO
}

// NewEmailDispatcher creates an EmailDispatcher from environment-style parameters.
func NewEmailDispatcher(smtpAddr, from, to string) *EmailDispatcher {
	return &EmailDispatcher{SMTPAddr: smtpAddr, From: from, To: to}
}

// DispatchNotification implements Dispatcher for email channels.
func (d *EmailDispatcher) DispatchNotification(ctx context.Context, notification *Notification, channels []Channel) error {
	var lastErr error
	for _, ch := range channels {
		if !ch.IsEnabled {
			continue
		}
		switch ch.ChannelType {
		case ChannelTypeEmail:
			to := d.To
			if v, ok := ch.Config["email"]; ok {
				if s, ok := v.(string); ok && s != "" {
					to = s
				}
			}
			from := d.From
			if v, ok := ch.Config["from"]; ok {
				if s, ok := v.(string); ok && s != "" {
					from = s
				}
			}
			if err := d.sendEmail(from, to, notification); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

// sanitizeEmailHeader strips CR and LF from an RFC 2822 header value to prevent
// header injection via admin-configured alert channel fields.
func sanitizeEmailHeader(v string) string {
	v = strings.ReplaceAll(v, "\r", "")
	v = strings.ReplaceAll(v, "\n", "")
	return v
}

func (d *EmailDispatcher) sendEmail(from, to string, notification *Notification) error {
	subject := fmt.Sprintf("[ALERT] %s threshold exceeded (%.1f%%)", sanitizeEmailHeader(string(notification.AlertType)), notification.Threshold)
	body := fmt.Sprintf(
		"Alert: %s\nCurrent value: %.2f\nThreshold: %.2f\nTime: %s\n",
		notification.AlertType,
		notification.CurrentValue,
		notification.Threshold,
		time.Now().UTC().Format(time.RFC3339),
	)
	msg := []byte("From: " + sanitizeEmailHeader(from) + "\r\n" +
		"To: " + sanitizeEmailHeader(to) + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		body)

	host := d.SMTPAddr
	if idx := strings.LastIndex(host, ":"); idx >= 0 {
		host = host[:idx]
	}
	return smtp.SendMail(d.SMTPAddr, nil, from, []string{to}, msg)
}

// WebhookDispatcher sends alert notifications to a Slack-compatible incoming webhook.
type WebhookDispatcher struct {
	WebhookURL string // GOGOMAIL_ALERT_WEBHOOK_URL
	client     *http.Client
}

// NewWebhookDispatcher creates a WebhookDispatcher.
func NewWebhookDispatcher(webhookURL string) *WebhookDispatcher {
	return &WebhookDispatcher{
		WebhookURL: webhookURL,
		client:     &http.Client{Timeout: 5 * time.Second},
	}
}

// DispatchNotification implements Dispatcher for webhook channels.
func (d *WebhookDispatcher) DispatchNotification(ctx context.Context, notification *Notification, channels []Channel) error {
	var lastErr error
	for _, ch := range channels {
		if !ch.IsEnabled {
			continue
		}
		switch ch.ChannelType {
		case ChannelTypeWebhook:
			url := d.WebhookURL
			if v, ok := ch.Config["url"]; ok {
				if s, ok := v.(string); ok && s != "" {
					url = s
				}
			}
			if err := d.post(ctx, url, notification); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

func (d *WebhookDispatcher) post(ctx context.Context, url string, notification *Notification) error {
	text := fmt.Sprintf("[ALERT] %s threshold exceeded\nCurrent: %.2f / Threshold: %.2f",
		notification.AlertType, notification.CurrentValue, notification.Threshold)
	payload := map[string]string{"text": text}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// MultiDispatcher fans out to multiple Dispatcher implementations.
type MultiDispatcher struct {
	dispatchers []Dispatcher
}

// NewMultiDispatcher wraps multiple dispatchers into one.
func NewMultiDispatcher(dispatchers ...Dispatcher) *MultiDispatcher {
	return &MultiDispatcher{dispatchers: dispatchers}
}

// DispatchNotification calls all underlying dispatchers and returns the last error.
func (m *MultiDispatcher) DispatchNotification(ctx context.Context, notification *Notification, channels []Channel) error {
	var lastErr error
	for _, d := range m.dispatchers {
		if err := d.DispatchNotification(ctx, notification, channels); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
