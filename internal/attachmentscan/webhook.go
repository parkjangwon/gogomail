package attachmentscan

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gogomail/gogomail/internal/webhook"
)

const (
	maxWebhookResponseBytes       = int64(64 << 10)
	maxWebhookIdentityBytes       = 200
	maxWebhookAddressBytes        = 320
	maxWebhookRemoteAddrBytes     = 200
	maxWebhookMessageIDBytes      = 500
	maxWebhookSubjectBytes        = 500
	maxWebhookTokenBytes          = 4096
	maxWebhookAttachmentNameBytes = 255
	maxWebhookRecipients          = 500
	maxWebhookAttachments         = 200
)

type WebhookOptions struct {
	Endpoint string
	Token    string
	Client   *http.Client
}

type WebhookScanner struct {
	endpoint string
	token    string
	client   *http.Client
}

func NewWebhookScanner(opts WebhookOptions) (*WebhookScanner, error) {
	endpoint := strings.TrimSpace(opts.Endpoint)
	if strings.ContainsAny(endpoint, "\r\n") {
		return nil, fmt.Errorf("attachment scanner webhook endpoint cannot contain line breaks")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("attachment scanner webhook endpoint must be a valid URL: %w", err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("attachment scanner webhook endpoint must be an http or https URL")
	}
	client := opts.Client
	if client == nil {
		client = http.DefaultClient
	}
	token, err := webhook.NormalizeWebhookToken(opts.Token, maxWebhookTokenBytes)
	if err != nil {
		return nil, fmt.Errorf("attachment scanner webhook token: %w", err)
	}
	return &WebhookScanner{endpoint: endpoint, token: token, client: client}, nil
}

func (s *WebhookScanner) ScanAttachments(ctx context.Context, req Request) (Result, error) {
	if s == nil || s.client == nil || strings.TrimSpace(s.endpoint) == "" {
		return Result{}, fmt.Errorf("attachment scanner webhook is not configured")
	}
	raw, err := json.Marshal(webhookRequestFromScan(req))
	if err != nil {
		return Result{}, fmt.Errorf("marshal attachment scanner webhook request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(raw))
	if err != nil {
		return Result{}, fmt.Errorf("build attachment scanner webhook request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if s.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return Result{}, fmt.Errorf("call attachment scanner webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if preview := webhook.ErrorBodyPreview(resp.Body, 512); preview != "" {
			return Result{}, fmt.Errorf("attachment scanner webhook returned HTTP %d: %s", resp.StatusCode, preview)
		}
		return Result{}, fmt.Errorf("attachment scanner webhook returned HTTP %d", resp.StatusCode)
	}
	result, err := decodeWebhookResponse(resp.Body)
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func decodeWebhookResponse(body io.Reader) (Result, error) {
	raw, err := io.ReadAll(io.LimitReader(body, maxWebhookResponseBytes+1))
	if err != nil {
		return Result{}, fmt.Errorf("read attachment scanner webhook response: %w", err)
	}
	if int64(len(raw)) > maxWebhookResponseBytes {
		return Result{}, fmt.Errorf("attachment scanner webhook response is too large")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var result Result
	if err := decoder.Decode(&result); err != nil {
		return Result{}, fmt.Errorf("decode attachment scanner webhook response: %w", err)
	}
	if decoder.More() {
		return Result{}, fmt.Errorf("attachment scanner webhook response has trailing JSON tokens")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Result{}, fmt.Errorf("attachment scanner webhook response has trailing JSON tokens")
	}
	result.Verdict = Verdict(strings.ToLower(strings.TrimSpace(string(result.Verdict))))
	result.Reason = strings.TrimSpace(result.Reason)
	return result, nil
}

type webhookRequest struct {
	RemoteAddr     string              `json:"remote_addr"`
	EnvelopeFrom   string              `json:"envelope_from"`
	Recipients     []string            `json:"recipients"`
	CompanyID      string              `json:"company_id,omitempty"`
	DomainID       string              `json:"domain_id,omitempty"`
	UserID         string              `json:"user_id,omitempty"`
	SubmissionUser string              `json:"submission_user,omitempty"`
	MessageID      string              `json:"message_id,omitempty"`
	Subject        string              `json:"subject,omitempty"`
	Size           int64               `json:"size"`
	Attachments    []webhookAttachment `json:"attachments"`
}

type webhookAttachment struct {
	Filename string `json:"filename"`
}

func webhookRequestFromScan(req Request) webhookRequest {
	attachments := make([]webhookAttachment, 0, min(len(req.Attachments), maxWebhookAttachments))
	for _, attachment := range req.Attachments {
		if len(attachments) >= maxWebhookAttachments {
			break
		}
		attachments = append(attachments, webhookAttachment{Filename: cleanWebhookText(attachment.Filename, maxWebhookAttachmentNameBytes)})
	}
	size := req.Size
	if size < 0 {
		size = 0
	}
	return webhookRequest{
		RemoteAddr:     cleanWebhookText(req.RemoteAddr, maxWebhookRemoteAddrBytes),
		EnvelopeFrom:   cleanWebhookText(req.EnvelopeFrom, maxWebhookAddressBytes),
		Recipients:     cleanWebhookRecipients(req.Recipients),
		CompanyID:      cleanWebhookText(req.CompanyID, maxWebhookIdentityBytes),
		DomainID:       cleanWebhookText(req.DomainID, maxWebhookIdentityBytes),
		UserID:         cleanWebhookText(req.UserID, maxWebhookIdentityBytes),
		SubmissionUser: cleanWebhookText(req.SubmissionUser, maxWebhookAddressBytes),
		MessageID:      cleanWebhookText(req.MessageID, maxWebhookMessageIDBytes),
		Subject:        cleanWebhookText(req.Subject, maxWebhookSubjectBytes),
		Size:           size,
		Attachments:    attachments,
	}
}

func cleanWebhookRecipients(recipients []string) []string {
	cleaned := make([]string, 0, min(len(recipients), maxWebhookRecipients))
	for _, recipient := range recipients {
		if len(cleaned) >= maxWebhookRecipients {
			break
		}
		value := cleanWebhookText(recipient, maxWebhookAddressBytes)
		if value == "" {
			continue
		}
		cleaned = append(cleaned, value)
	}
	return cleaned
}

func cleanWebhookText(value string, maxBytes int) string {
	value = strings.ToValidUTF8(strings.TrimSpace(value), "")
	value = strings.NewReplacer("\r", " ", "\n", " ").Replace(value)
	value = strings.Join(strings.Fields(value), " ")
	return truncateUTF8Bytes(value, maxBytes)
}
