package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
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

func (r *Repository) ListMessagesInFolder(ctx context.Context, userID string, folderID string, limit int) ([]MessageSummary, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if strings.TrimSpace(folderID) == "" {
		return nil, fmt.Errorf("folder_id is required")
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
  AND folder_id = $2
  AND status = 'active'
ORDER BY COALESCE(received_at, created_at) DESC
LIMIT $3`

	rows, err := r.db.QueryContext(ctx, query, userID, folderID, limit)
	if err != nil {
		return nil, fmt.Errorf("list folder messages: %w", err)
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
			return nil, fmt.Errorf("scan folder message summary: %w", err)
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate folder message summaries: %w", err)
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

func (r *Repository) SetMessageFlag(ctx context.Context, userID string, messageID string, flag string, value bool) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	flag = strings.TrimSpace(flag)
	if !allowedMessageFlag(flag) {
		return fmt.Errorf("unsupported message flag %q", flag)
	}

	const query = `
UPDATE messages
SET flags = jsonb_set(flags, $4::text[], to_jsonb($5::boolean), true),
    updated_at = now()
WHERE user_id = $1
  AND id = $2
  AND status = 'active'`

	result, err := r.db.ExecContext(ctx, query, userID, messageID, "{"+flag+"}", value)
	if err != nil {
		return fmt.Errorf("set message flag: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect message flag update: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("message %q not found", messageID)
	}
	return nil
}

func (r *Repository) MoveMessage(ctx context.Context, userID string, messageID string, folderID string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if strings.TrimSpace(folderID) == "" {
		return fmt.Errorf("folder_id is required")
	}

	const query = `
UPDATE messages
SET folder_id = $3,
    updated_at = now()
WHERE user_id = $1
  AND id = $2
  AND status = 'active'
  AND EXISTS (
    SELECT 1
    FROM folders
    WHERE folders.id = $3
      AND folders.user_id = $1
  )`

	result, err := r.db.ExecContext(ctx, query, userID, messageID, folderID)
	if err != nil {
		return fmt.Errorf("move message: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect message move: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("message %q or folder %q not found", messageID, folderID)
	}
	return nil
}

func (r *Repository) DeleteMessage(ctx context.Context, userID string, messageID string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	const query = `
UPDATE messages
SET status = 'deleted',
    deleted_at = now(),
    updated_at = now()
WHERE user_id = $1
  AND id = $2
  AND status = 'active'`

	result, err := r.db.ExecContext(ctx, query, userID, messageID)
	if err != nil {
		return fmt.Errorf("delete message: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect message delete: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("message %q not found", messageID)
	}
	return nil
}

func allowedMessageFlag(flag string) bool {
	switch flag {
	case "read", "starred", "answered", "forwarded":
		return true
	default:
		return false
	}
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
