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
)

const maxWebhookResponseBytes = int64(64 << 10)

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
	return &WebhookScanner{endpoint: endpoint, token: strings.TrimSpace(opts.Token), client: client}, nil
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
	attachments := make([]webhookAttachment, 0, len(req.Attachments))
	for _, attachment := range req.Attachments {
		attachments = append(attachments, webhookAttachment{Filename: attachment.Filename})
	}
	return webhookRequest{
		RemoteAddr:     req.RemoteAddr,
		EnvelopeFrom:   req.EnvelopeFrom,
		Recipients:     append([]string(nil), req.Recipients...),
		CompanyID:      req.CompanyID,
		DomainID:       req.DomainID,
		UserID:         req.UserID,
		SubmissionUser: req.SubmissionUser,
		MessageID:      req.MessageID,
		Subject:        req.Subject,
		Size:           req.Size,
		Attachments:    attachments,
	}
}
