package searchindex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/webhook"
)

type OpenSearchOptions struct {
	Endpoint string
	Index    string
	Client   *http.Client
	Username string
	Password string
	// KoreanAnalyzer enables the Nori Korean text analyzer for subject and
	// body_text fields when the index is created via EnsureIndex.
	// Requires the analysis-nori plugin to be installed on OpenSearch.
	// Controlled by GOGOMAIL_OPENSEARCH_KOREAN_ANALYZER=true.
	KoreanAnalyzer bool
}

type OpenSearchIndexer struct {
	endpoint       *url.URL
	index          string
	client         *http.Client
	username       string
	password       string
	koreanAnalyzer bool
}

const (
	maxOpenSearchMetadataBytes   = 1000
	maxOpenSearchCredentialBytes = 4096
)

func NewOpenSearchIndexer(opts OpenSearchOptions) (OpenSearchIndexer, error) {
	rawEndpoint := strings.TrimSpace(opts.Endpoint)
	if strings.ContainsAny(rawEndpoint, "\r\n") {
		return OpenSearchIndexer{}, fmt.Errorf("opensearch endpoint cannot contain line breaks")
	}
	endpoint, err := url.Parse(rawEndpoint)
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
	username, password, err := normalizeOpenSearchCredentials(opts.Username, opts.Password)
	if err != nil {
		return OpenSearchIndexer{}, err
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return OpenSearchIndexer{
		endpoint:       endpoint,
		index:          index,
		client:         client,
		username:       username,
		password:       password,
		koreanAnalyzer: opts.KoreanAnalyzer,
	}, nil
}

func normalizeOpenSearchCredentials(username string, password string) (string, string, error) {
	username = strings.TrimSpace(username)
	if strings.ContainsAny(username, "\r\n") {
		return "", "", fmt.Errorf("opensearch username cannot contain line breaks")
	}
	if len(username) > maxOpenSearchCredentialBytes {
		return "", "", fmt.Errorf("opensearch username is too long")
	}
	if strings.ContainsAny(password, "\r\n") {
		return "", "", fmt.Errorf("opensearch password cannot contain line breaks")
	}
	if len(password) > maxOpenSearchCredentialBytes {
		return "", "", fmt.Errorf("opensearch password is too long")
	}
	return username, password, nil
}

func (i OpenSearchIndexer) IndexMessage(ctx context.Context, doc Document) error {
	messageID, err := cleanOpenSearchDocumentID(doc.MessageID)
	if err != nil {
		return err
	}
	doc.MessageID = messageID
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
	defer func() { _ = webhook.DrainAndClose(resp.Body, webhook.DefaultDrainBytes) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("index opensearch message %q: status %d: %s", messageID, resp.StatusCode, webhook.ErrorBodyPreview(resp.Body, 512))
	}
	return nil
}

func cleanOpenSearchDocumentID(value string) (string, error) {
	value = strings.ToValidUTF8(strings.TrimSpace(value), "")
	if value == "" {
		return "", fmt.Errorf("message_id is required")
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("message_id is invalid")
	}
	if len(value) > maxOpenSearchMetadataBytes {
		return "", fmt.Errorf("message_id is oversized")
	}
	return value, nil
}

func (i OpenSearchIndexer) EnsureIndex(ctx context.Context) error {
	payload, err := json.Marshal(openSearchIndexDefinition(i.koreanAnalyzer))
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
	defer func() { _ = webhook.DrainAndClose(resp.Body, webhook.DefaultDrainBytes) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview := webhook.ErrorBodyPreview(resp.Body, 512)
		if i.koreanAnalyzer && strings.Contains(strings.ToLower(preview), "nori") {
			return fmt.Errorf("ensure opensearch index %q: status %d: %s (GOGOMAIL_OPENSEARCH_KOREAN_ANALYZER requires the OpenSearch analysis-nori plugin)", i.index, resp.StatusCode, preview)
		}
		return fmt.Errorf("ensure opensearch index %q: status %d: %s", i.index, resp.StatusCode, preview)
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

// openSearchIndexDefinition returns the index settings and mappings.
// When koreanAnalyzer is true the "korean" (Nori) analyzer is registered and
// applied to subject and body_text. This requires the analysis-nori plugin.
func openSearchIndexDefinition(koreanAnalyzer bool) map[string]any {
	subjectMapping := map[string]any{"type": "text"}
	bodyTextMapping := map[string]any{"type": "text"}

	settings := map[string]any{
		"index": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 1,
		},
	}

	if koreanAnalyzer {
		settings["analysis"] = map[string]any{
			"analyzer": map[string]any{
				"korean": map[string]any{
					"type":            "nori",
					"decompound_mode": "mixed",
				},
			},
		}
		subjectMapping["analyzer"] = "korean"
		bodyTextMapping["analyzer"] = "korean"
	}

	return map[string]any{
		"settings": settings,
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
				"subject":        subjectMapping,
				"subject_lc":     map[string]any{"type": "keyword"},
				"from_addr":      map[string]any{"type": "keyword"},
				"from_addr_lc":   map[string]any{"type": "keyword"},
				"from_name":      map[string]any{"type": "text"},
				"to_addrs_lc":    map[string]any{"type": "keyword"},
				"cc_addrs_lc":    map[string]any{"type": "keyword"},
				"bcc_addrs_lc":   map[string]any{"type": "keyword"},
				"storage_path":   map[string]any{"type": "keyword", "index": false},
				"received_at":    map[string]any{"type": "date"},
				"size":           map[string]any{"type": "long"},
				"has_attachment": map[string]any{"type": "boolean"},
				"body_text":      bodyTextMapping,
				"body_truncated": map[string]any{"type": "boolean"},
				"body_max_bytes": map[string]any{"type": "long"},
			},
		},
	}
}

func openSearchDocument(doc Document) map[string]any {
	messageID := cleanOpenSearchMetadata(doc.MessageID)
	rfcMessageID := cleanOpenSearchMetadata(doc.RFCMessageID)
	inReplyTo := cleanOpenSearchMetadata(doc.InReplyTo)
	companyID := cleanOpenSearchMetadata(doc.CompanyID)
	domainID := cleanOpenSearchMetadata(doc.DomainID)
	userID := cleanOpenSearchMetadata(doc.UserID)
	folderID := cleanOpenSearchMetadata(doc.FolderID)
	recipient := cleanOpenSearchMetadata(doc.Recipient)
	subject := cleanOpenSearchMetadata(doc.Subject)
	fromAddr := cleanOpenSearchMetadata(doc.FromAddr)
	fromName := cleanOpenSearchMetadata(doc.FromName)
	storagePath := cleanOpenSearchMetadata(doc.StoragePath)
	receivedAt := cleanOpenSearchMetadata(doc.ReceivedAt)
	return map[string]any{
		"message_id":     messageID,
		"rfc_message_id": rfcMessageID,
		"in_reply_to":    inReplyTo,
		"references":     cleanOpenSearchReferences(doc.References),
		"company_id":     companyID,
		"domain_id":      domainID,
		"user_id":        userID,
		"folder_id":      folderID,
		"recipient":      recipient,
		"subject":        subject,
		"subject_lc":     strings.ToLower(subject),
		"from_addr":      fromAddr,
		"from_addr_lc":   strings.ToLower(fromAddr),
		"from_name":      fromName,
		"to_addrs_lc":    lowercaseAddrs(doc.ToAddrs),
		"cc_addrs_lc":    lowercaseAddrs(doc.CcAddrs),
		"bcc_addrs_lc":   lowercaseAddrs(doc.BccAddrs),
		"storage_path":   storagePath,
		"received_at":    receivedAt,
		"size":           doc.Size,
		"has_attachment": doc.HasAttachment,
		"body_text":      doc.BodyText,
		"body_truncated": doc.BodyTruncated,
		"body_max_bytes": doc.BodyMaxBytes,
	}
}

func cleanOpenSearchReferences(values []string) []string {
	out := make([]string, 0, min(len(values), maxEventReferences))
	for _, value := range values {
		if len(out) >= maxEventReferences {
			break
		}
		if strings.ContainsAny(value, "\r\n") {
			continue
		}
		value = cleanOpenSearchMetadata(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func lowercaseAddrs(addrs []string) []string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		a = strings.ToValidUTF8(strings.TrimSpace(a), "")
		if a != "" {
			out = append(out, strings.ToLower(a))
		}
	}
	return out
}

func cleanOpenSearchMetadata(value string) string {
	value = strings.ToValidUTF8(strings.TrimSpace(value), "")
	if len(value) <= maxOpenSearchMetadataBytes {
		return value
	}
	cut := 0
	for i := range value {
		if i > maxOpenSearchMetadataBytes {
			return value[:cut]
		}
		cut = i
	}
	return value[:cut]
}
