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

const (
	MailFlowIndexName = "mail_flow"

	maxMailFlowMetadataBytes = 1000
)

type MailFlowDocument struct {
	MessageID      string    `json:"message_id"`
	RFCMessageID   string    `json:"rfc_message_id"`
	Direction      string    `json:"direction"`
	CompanyID      string    `json:"company_id"`
	DomainID       string    `json:"domain_id"`
	UserID         string    `json:"user_id"`
	FromAddr       string    `json:"from_addr"`
	ToAddr         string    `json:"to_addr"`
	FlowStatus     string    `json:"flow_status"`
	EnhancedStatus string    `json:"enhanced_status"`
	Size           int64     `json:"size"`
	CreatedAt      time.Time `json:"created_at"`
}

type MailFlowIndexer struct {
	indexer OpenSearchIndexer
}

func NewMailFlowIndexer(opts OpenSearchOptions) (MailFlowIndexer, error) {
	opts.Index = MailFlowIndexName
	indexer, err := NewOpenSearchIndexer(opts)
	if err != nil {
		return MailFlowIndexer{}, err
	}
	return MailFlowIndexer{indexer: indexer}, nil
}

func (i MailFlowIndexer) IndexMailFlow(ctx context.Context, doc MailFlowDocument) error {
	docID, err := cleanMailFlowDocumentID(doc.MessageID, doc.Direction, doc.CreatedAt)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(mailFlowDocument(doc))
	if err != nil {
		return fmt.Errorf("marshal mail flow document: %w", err)
	}
	target := *i.indexer.endpoint
	target.Path = path.Join(target.Path, i.indexer.index, "_doc", url.PathEscape(docID))

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, target.String(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create mail flow index request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if i.indexer.username != "" || i.indexer.password != "" {
		req.SetBasicAuth(i.indexer.username, i.indexer.password)
	}

	resp, err := i.indexer.client.Do(req)
	if err != nil {
		return fmt.Errorf("index mail flow %q: %w", docID, err)
	}
	defer func() { _ = webhook.DrainAndClose(resp.Body, webhook.DefaultDrainBytes) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("index mail flow %q: status %d: %s", docID, resp.StatusCode, webhook.ErrorBodyPreview(resp.Body, 512))
	}
	return nil
}

func (i MailFlowIndexer) EnsureIndex(ctx context.Context) error {
	payload, err := json.Marshal(mailFlowIndexDefinition())
	if err != nil {
		return fmt.Errorf("marshal mail flow index definition: %w", err)
	}
	target := *i.indexer.endpoint
	target.Path = path.Join(target.Path, i.indexer.index)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, target.String(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create mail flow index request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if i.indexer.username != "" || i.indexer.password != "" {
		req.SetBasicAuth(i.indexer.username, i.indexer.password)
	}

	resp, err := i.indexer.client.Do(req)
	if err != nil {
		return fmt.Errorf("ensure mail flow index %q: %w", i.indexer.index, err)
	}
	defer func() { _ = webhook.DrainAndClose(resp.Body, webhook.DefaultDrainBytes) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ensure mail flow index %q: status %d: %s", i.indexer.index, resp.StatusCode, webhook.ErrorBodyPreview(resp.Body, 512))
	}
	return nil
}

func cleanMailFlowDocumentID(messageID, direction string, createdAt time.Time) (string, error) {
	messageID = strings.ToValidUTF8(strings.TrimSpace(messageID), "")
	if messageID == "" {
		return "", fmt.Errorf("message_id is required")
	}
	if strings.ContainsAny(messageID, "\r\n") {
		return "", fmt.Errorf("message_id is invalid")
	}
	if len(messageID) > maxMailFlowMetadataBytes {
		return "", fmt.Errorf("message_id is oversized")
	}
	return fmt.Sprintf("%s_%s_%d", messageID, direction, createdAt.UnixNano()), nil
}

func mailFlowIndexDefinition() map[string]any {
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
				"message_id":       map[string]any{"type": "keyword"},
				"rfc_message_id":   map[string]any{"type": "keyword"},
				"direction":        map[string]any{"type": "keyword"},
				"company_id":       map[string]any{"type": "keyword"},
				"domain_id":        map[string]any{"type": "keyword"},
				"user_id":          map[string]any{"type": "keyword"},
				"from_addr":        map[string]any{"type": "keyword"},
				"to_addr":          map[string]any{"type": "keyword"},
				"flow_status":      map[string]any{"type": "keyword"},
				"enhanced_status":  map[string]any{"type": "keyword"},
				"size":             map[string]any{"type": "long"},
				"created_at":       map[string]any{"type": "date"},
			},
		},
	}
}

func mailFlowDocument(doc MailFlowDocument) map[string]any {
	messageID := cleanMailFlowMetadata(doc.MessageID)
	rfcMessageID := cleanMailFlowMetadata(doc.RFCMessageID)
	companyID := cleanMailFlowMetadata(doc.CompanyID)
	domainID := cleanMailFlowMetadata(doc.DomainID)
	userID := cleanMailFlowMetadata(doc.UserID)
	fromAddr := cleanMailFlowMetadata(doc.FromAddr)
	toAddr := cleanMailFlowMetadata(doc.ToAddr)
	createdAt := doc.CreatedAt.UTC().Format(time.RFC3339)

	return map[string]any{
		"message_id":     messageID,
		"rfc_message_id": rfcMessageID,
		"direction":      strings.ToLower(strings.TrimSpace(doc.Direction)),
		"company_id":     companyID,
		"domain_id":      domainID,
		"user_id":        userID,
		"from_addr":      fromAddr,
		"to_addr":        toAddr,
		"flow_status":    strings.ToLower(strings.TrimSpace(doc.FlowStatus)),
		"enhanced_status": cleanMailFlowMetadata(doc.EnhancedStatus),
		"size":           doc.Size,
		"created_at":     createdAt,
	}
}

func cleanMailFlowMetadata(value string) string {
	value = strings.ToValidUTF8(strings.TrimSpace(value), "")
	if len(value) <= maxMailFlowMetadataBytes {
		return value
	}
	cut := 0
	for i := range value {
		if i > maxMailFlowMetadataBytes {
			return value[:cut]
		}
		cut = i
	}
	return value[:cut]
}
