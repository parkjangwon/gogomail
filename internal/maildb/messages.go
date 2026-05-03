package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type MessageSummary struct {
	ID            string    `json:"id"`
	Subject       string    `json:"subject"`
	FromAddr      string    `json:"from_addr"`
	FromName      string    `json:"from_name"`
	ReceivedAt    time.Time `json:"received_at"`
	Size          int64     `json:"size"`
	HasAttachment bool      `json:"has_attachment"`
	Read          bool      `json:"read"`
	Starred       bool      `json:"starred"`
}

type MessageDetail struct {
	ID            string          `json:"id"`
	MessageID     string          `json:"message_id"`
	Subject       string          `json:"subject"`
	FromAddr      string          `json:"from_addr"`
	FromName      string          `json:"from_name"`
	ToAddrs       json.RawMessage `json:"to_addrs"`
	CcAddrs       json.RawMessage `json:"cc_addrs"`
	BccAddrs      json.RawMessage `json:"bcc_addrs"`
	ReceivedAt    time.Time       `json:"received_at"`
	Size          int64           `json:"size"`
	HasAttachment bool            `json:"has_attachment"`
	Flags         json.RawMessage `json:"flags"`
	StoragePath   string          `json:"storage_path"`
	TextBody      string          `json:"text_body"`
}

type Folder struct {
	ID         string `json:"id"`
	ParentID   string `json:"parent_id,omitempty"`
	Name       string `json:"name"`
	FullPath   string `json:"full_path"`
	Type       string `json:"type"`
	SystemType string `json:"system_type,omitempty"`
	OrderIndex int    `json:"order_index"`
}

func (r *Repository) ListFolders(ctx context.Context, userID string) ([]Folder, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	const query = `
SELECT
  id::text,
  COALESCE(parent_id::text, ''),
  name,
  full_path,
  type,
  COALESCE(system_type, ''),
  order_index
FROM folders
WHERE user_id = $1
ORDER BY type DESC, order_index ASC, full_path ASC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	defer rows.Close()

	folders := make([]Folder, 0)
	for rows.Next() {
		var folder Folder
		if err := rows.Scan(
			&folder.ID,
			&folder.ParentID,
			&folder.Name,
			&folder.FullPath,
			&folder.Type,
			&folder.SystemType,
			&folder.OrderIndex,
		); err != nil {
			return nil, fmt.Errorf("scan folder: %w", err)
		}
		folders = append(folders, folder)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate folders: %w", err)
	}
	return folders, nil
}

func (r *Repository) ListMessages(ctx context.Context, userID string, limit int) ([]MessageSummary, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	limit = normalizeLimit(limit)

	const query = `
SELECT
  id::text,
  subject,
  from_addr,
  from_name,
  COALESCE(received_at, created_at),
  size,
  has_attachment,
  COALESCE((flags->>'read')::boolean, false) AS read,
  COALESCE((flags->>'starred')::boolean, false) AS starred
FROM messages
WHERE user_id = $1
  AND status = 'active'
ORDER BY COALESCE(received_at, created_at) DESC
LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	messages := make([]MessageSummary, 0)
	for rows.Next() {
		var msg MessageSummary
		if err := rows.Scan(
			&msg.ID,
			&msg.Subject,
			&msg.FromAddr,
			&msg.FromName,
			&msg.ReceivedAt,
			&msg.Size,
			&msg.HasAttachment,
			&msg.Read,
			&msg.Starred,
		); err != nil {
			return nil, fmt.Errorf("scan message summary: %w", err)
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate message summaries: %w", err)
	}
	return messages, nil
}

func (r *Repository) GetMessage(ctx context.Context, userID string, messageID string) (MessageDetail, error) {
	if r.db == nil {
		return MessageDetail{}, fmt.Errorf("database handle is required")
	}

	const query = `
SELECT
  id::text,
  COALESCE(rfc_message_id, ''),
  subject,
  from_addr,
  from_name,
  to_addrs,
  cc_addrs,
  bcc_addrs,
  COALESCE(received_at, created_at),
  size,
  has_attachment,
  flags,
  storage_path
FROM messages
WHERE user_id = $1
  AND id = $2
  AND status = 'active'
LIMIT 1`

	var msg MessageDetail
	err := r.db.QueryRowContext(ctx, query, userID, messageID).Scan(
		&msg.ID,
		&msg.MessageID,
		&msg.Subject,
		&msg.FromAddr,
		&msg.FromName,
		&msg.ToAddrs,
		&msg.CcAddrs,
		&msg.BccAddrs,
		&msg.ReceivedAt,
		&msg.Size,
		&msg.HasAttachment,
		&msg.Flags,
		&msg.StoragePath,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return MessageDetail{}, fmt.Errorf("message %q not found", messageID)
		}
		return MessageDetail{}, fmt.Errorf("get message: %w", err)
	}
	return msg, nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}
