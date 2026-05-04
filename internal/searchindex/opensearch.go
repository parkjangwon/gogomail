package searchindex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type OpenSearchOptions struct {
	Endpoint string
	Index    string
	Client   *http.Client
	Username string
	Password string
}

type OpenSearchIndexer struct {
	endpoint *url.URL
	index    string
	client   *http.Client
	username string
	password string
}

func NewOpenSearchIndexer(opts OpenSearchOptions) (OpenSearchIndexer, error) {
	endpoint, err := url.Parse(strings.TrimSpace(opts.Endpoint))
	if err != nil {
		return OpenSearchIndexer{}, fmt.Errorf("parse opensearch endpoint: %w", err)
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return OpenSearchIndexer{}, fmt.Errorf("opensearch endpoint must use http or https")
	}
	if endpoint.Host == "" {
		return OpenSearchIndexer{}, fmt.Errorf("opensearch endpoint host is required")
	}
	index, err := normalizeOpenSearchIndex(opts.Index)
	if err != nil {
		return OpenSearchIndexer{}, err
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return OpenSearchIndexer{
		endpoint: endpoint,
		index:    index,
		client:   client,
		username: strings.TrimSpace(opts.Username),
		password: opts.Password,
	}, nil
}

func (i OpenSearchIndexer) IndexMessage(ctx context.Context, doc Document) error {
	messageID := strings.TrimSpace(doc.MessageID)
	if messageID == "" {
		return fmt.Errorf("message_id is required")
	}
	payload, err := json.Marshal(openSearchDocument(doc))
	if err != nil {
		return fmt.Errorf("marshal opensearch document: %w", err)
	}
	target := *i.endpoint
	target.Path = path.Join(target.Path, i.index, "_doc", url.PathEscape(messageID))

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, target.String(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create opensearch index request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if i.username != "" || i.password != "" {
		req.SetBasicAuth(i.username, i.password)
	}

	resp, err := i.client.Do(req)
	if err != nil {
		return fmt.Errorf("index opensearch message %q: %w", messageID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("index opensearch message %q: status %d: %s", messageID, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (i OpenSearchIndexer) EnsureIndex(ctx context.Context) error {
	payload, err := json.Marshal(openSearchIndexDefinition())
	if err != nil {
		return fmt.Errorf("marshal opensearch index definition: %w", err)
	}
	target := *i.endpoint
	target.Path = path.Join(target.Path, i.index)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, target.String(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create opensearch index request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if i.username != "" || i.password != "" {
		req.SetBasicAuth(i.username, i.password)
	}

	resp, err := i.client.Do(req)
	if err != nil {
		return fmt.Errorf("ensure opensearch index %q: %w", i.index, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("ensure opensearch index %q: status %d: %s", i.index, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func normalizeOpenSearchIndex(index string) (string, error) {
	index = strings.TrimSpace(index)
	if index == "" {
		return "", fmt.Errorf("opensearch index is required")
	}
	if strings.ContainsAny(index, `/\?#*:,"<>| `) || strings.HasPrefix(index, ".") || strings.HasPrefix(index, "_") {
		return "", fmt.Errorf("opensearch index %q is invalid", index)
	}
	return index, nil
}

func openSearchIndexDefinition() map[string]any {
	return map[string]any{
		"settings": map[string]any{
			"index": map[string]any{
				"number_of_shards":   1,
				"number_of_replicas": 1,
			},
		},
		"mappings": map[string]any{
			"dynamic": "strict",
			"properties": map[string]any{
				"message_id":     map[string]any{"type": "keyword"},
				"rfc_message_id": map[string]any{"type": "keyword"},
				"in_reply_to":    map[string]any{"type": "keyword"},
				"references":     map[string]any{"type": "keyword"},
				"company_id":     map[string]any{"type": "keyword"},
				"domain_id":      map[string]any{"type": "keyword"},
				"user_id":        map[string]any{"type": "keyword"},
				"folder_id":      map[string]any{"type": "keyword"},
				"recipient":      map[string]any{"type": "keyword"},
				"subject":        map[string]any{"type": "text"},
				"from_addr":      map[string]any{"type": "keyword"},
				"from_name":      map[string]any{"type": "text"},
				"storage_path":   map[string]any{"type": "keyword", "index": false},
				"received_at":    map[string]any{"type": "date"},
				"size":           map[string]any{"type": "long"},
				"has_attachment": map[string]any{"type": "boolean"},
				"body_text":      map[string]any{"type": "text"},
				"body_truncated": map[string]any{"type": "boolean"},
				"body_max_bytes": map[string]any{"type": "long"},
			},
		},
	}
}

func openSearchDocument(doc Document) map[string]any {
	return map[string]any{
		"message_id":     strings.TrimSpace(doc.MessageID),
		"rfc_message_id": strings.TrimSpace(doc.RFCMessageID),
		"in_reply_to":    strings.TrimSpace(doc.InReplyTo),
		"references":     append([]string(nil), doc.References...),
		"company_id":     strings.TrimSpace(doc.CompanyID),
		"domain_id":      strings.TrimSpace(doc.DomainID),
		"user_id":        strings.TrimSpace(doc.UserID),
		"folder_id":      strings.TrimSpace(doc.FolderID),
		"recipient":      strings.TrimSpace(doc.Recipient),
		"subject":        strings.TrimSpace(doc.Subject),
		"from_addr":      strings.TrimSpace(doc.FromAddr),
		"from_name":      strings.TrimSpace(doc.FromName),
		"storage_path":   strings.TrimSpace(doc.StoragePath),
		"received_at":    strings.TrimSpace(doc.ReceivedAt),
		"size":           doc.Size,
		"has_attachment": doc.HasAttachment,
		"body_text":      doc.BodyText,
		"body_truncated": doc.BodyTruncated,
		"body_max_bytes": doc.BodyMaxBytes,
	}
}
