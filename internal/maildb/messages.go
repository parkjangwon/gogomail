package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

type MessageSummary struct {
	ID               string                   `json:"id"`
	FolderID         string                   `json:"folder_id"`
	Subject          string                   `json:"subject"`
	Preview          string                   `json:"preview"`
	FromAddr         string                   `json:"from_addr"`
	FromName         string                   `json:"from_name"`
	ReceivedAt       time.Time                `json:"received_at"`
	Size             int64                    `json:"size"`
	HasAttachment    bool                     `json:"has_attachment"`
	Read             bool                     `json:"read"`
	Starred          bool                     `json:"starred"`
	SearchRank       *float64                 `json:"search_rank,omitempty"`
	SearchHighlights *MessageSearchHighlights `json:"search_highlights,omitempty"`
}

type MessageSearchHighlights struct {
	Subject []string `json:"subject,omitempty"`
	From    []string `json:"from,omitempty"`
	Body    []string `json:"body,omitempty"`
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
	HTMLBody      string          `json:"html_body,omitempty"`
	Attachments   []Attachment    `json:"attachments,omitempty"`
}

type Folder struct {
	ID             string `json:"id"`
	ParentID       string `json:"parent_id,omitempty"`
	Name           string `json:"name"`
	FullPath       string `json:"full_path"`
	Type           string `json:"type"`
	SystemType     string `json:"system_type,omitempty"`
	OrderIndex     int    `json:"order_index"`
	Total          int64  `json:"total"`
	Unread         int64  `json:"unread"`
	Starred        int64  `json:"starred"`
	TotalSize      int64  `json:"-"`
	IMAPUnassigned int64  `json:"-"`
}

type CreateFolderRequest struct {
	UserID string
	Name   string
}

type BulkMessageFlagRequest struct {
	UserID     string   `json:"user_id,omitempty"`
	MessageIDs []string `json:"message_ids"`
	Flag       string   `json:"flag"`
	Value      bool     `json:"value"`
}

type BulkThreadFlagRequest struct {
	UserID    string   `json:"user_id,omitempty"`
	ThreadIDs []string `json:"thread_ids"`
	Flag      string   `json:"flag"`
	Value     bool     `json:"value"`
}

type BulkThreadFlagResult struct {
	Updated    int64
	MessageIDs []string
}

type BulkThreadMoveRequest struct {
	UserID    string   `json:"user_id,omitempty"`
	ThreadIDs []string `json:"thread_ids"`
	FolderID  string   `json:"folder_id"`
}

type BulkThreadMoveResult struct {
	Updated    int64
	MessageIDs []string
}

type BulkThreadDeleteRequest struct {
	UserID    string   `json:"user_id,omitempty"`
	ThreadIDs []string `json:"thread_ids"`
}

type BulkThreadRestoreRequest struct {
	UserID    string   `json:"user_id,omitempty"`
	ThreadIDs []string `json:"thread_ids"`
}

type BulkThreadDeleteResult struct {
	Updated    int64
	MessageIDs []string
}

type BulkThreadRestoreResult struct {
	Updated    int64
	MessageIDs []string
}

type BulkMessageMoveRequest struct {
	UserID     string   `json:"user_id,omitempty"`
	MessageIDs []string `json:"message_ids"`
	FolderID   string   `json:"folder_id"`
}

type BulkMessageDeleteRequest struct {
	UserID     string   `json:"user_id,omitempty"`
	MessageIDs []string `json:"message_ids"`
}

type BulkMessageRestoreRequest struct {
	UserID     string   `json:"user_id,omitempty"`
	MessageIDs []string `json:"message_ids"`
}

type BulkMessageRestoreResult struct {
	Updated    int64
	MessageIDs []string
}

const BulkMessageMaxIDs = 500
const maxMailboxResourceIDBytes = 200
const maxFolderNameBytes = 200

func ValidateBulkMessageFlagRequest(req BulkMessageFlagRequest) error {
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if len(req.MessageIDs) == 0 {
		return fmt.Errorf("message_ids is required")
	}
	if len(req.MessageIDs) > BulkMessageMaxIDs {
		return fmt.Errorf("too many message_ids")
	}
	if err := validateBulkMessageIDs(req.MessageIDs); err != nil {
		return err
	}
	if !allowedMessageFlag(strings.TrimSpace(req.Flag)) {
		return fmt.Errorf("unsupported message flag %q", req.Flag)
	}
	return nil
}

func ValidateBulkThreadFlagRequest(req BulkThreadFlagRequest) error {
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if len(req.ThreadIDs) == 0 {
		return fmt.Errorf("thread_ids is required")
	}
	if len(req.ThreadIDs) > BulkMessageMaxIDs {
		return fmt.Errorf("too many thread_ids")
	}
	if err := validateBulkThreadIDs(req.ThreadIDs); err != nil {
		return err
	}
	if !allowedMessageFlag(strings.TrimSpace(req.Flag)) {
		return fmt.Errorf("unsupported message flag %q", req.Flag)
	}
	return nil
}

func ValidateBulkMessageMoveRequest(req BulkMessageMoveRequest) error {
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(req.FolderID) == "" {
		return fmt.Errorf("folder_id is required")
	}
	if err := validateMailboxResourceID("folder_id", req.FolderID); err != nil {
		return err
	}
	if len(req.MessageIDs) == 0 {
		return fmt.Errorf("message_ids is required")
	}
	if len(req.MessageIDs) > BulkMessageMaxIDs {
		return fmt.Errorf("too many message_ids")
	}
	return validateBulkMessageIDs(req.MessageIDs)
}

func ValidateBulkThreadMoveRequest(req BulkThreadMoveRequest) error {
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(req.FolderID) == "" {
		return fmt.Errorf("folder_id is required")
	}
	if err := validateMailboxResourceID("folder_id", req.FolderID); err != nil {
		return err
	}
	if len(req.ThreadIDs) == 0 {
		return fmt.Errorf("thread_ids is required")
	}
	if len(req.ThreadIDs) > BulkMessageMaxIDs {
		return fmt.Errorf("too many thread_ids")
	}
	return validateBulkThreadIDs(req.ThreadIDs)
}

func ValidateBulkMessageDeleteRequest(req BulkMessageDeleteRequest) error {
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if len(req.MessageIDs) == 0 {
		return fmt.Errorf("message_ids is required")
	}
	if len(req.MessageIDs) > BulkMessageMaxIDs {
		return fmt.Errorf("too many message_ids")
	}
	return validateBulkMessageIDs(req.MessageIDs)
}

func ValidateBulkMessageRestoreRequest(req BulkMessageRestoreRequest) error {
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if len(req.MessageIDs) == 0 {
		return fmt.Errorf("message_ids is required")
	}
	if len(req.MessageIDs) > BulkMessageMaxIDs {
		return fmt.Errorf("too many message_ids")
	}
	return validateBulkMessageIDs(req.MessageIDs)
}

func ValidateBulkThreadDeleteRequest(req BulkThreadDeleteRequest) error {
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if len(req.ThreadIDs) == 0 {
		return fmt.Errorf("thread_ids is required")
	}
	if len(req.ThreadIDs) > BulkMessageMaxIDs {
		return fmt.Errorf("too many thread_ids")
	}
	return validateBulkThreadIDs(req.ThreadIDs)
}

func ValidateBulkThreadRestoreRequest(req BulkThreadRestoreRequest) error {
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if len(req.ThreadIDs) == 0 {
		return fmt.Errorf("thread_ids is required")
	}
	if len(req.ThreadIDs) > BulkMessageMaxIDs {
		return fmt.Errorf("too many thread_ids")
	}
	return validateBulkThreadIDs(req.ThreadIDs)
}

func validateBulkMessageIDs(messageIDs []string) error {
	seen := make(map[string]struct{}, len(messageIDs))
	for _, id := range messageIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			return fmt.Errorf("message id must not be blank")
		}
		if err := validateMailboxResourceID("message id", id); err != nil {
			return err
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("message id %q is duplicated", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func validateBulkThreadIDs(threadIDs []string) error {
	seen := make(map[string]struct{}, len(threadIDs))
	for _, id := range threadIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			return fmt.Errorf("thread id must not be blank")
		}
		if err := validateMailboxResourceID("thread id", id); err != nil {
			return err
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("thread id %q is duplicated", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func validateMailboxResourceID(field string, id string) error {
	id = strings.TrimSpace(id)
	if strings.ContainsAny(id, "\r\n") {
		return fmt.Errorf("%s must not contain CR or LF", field)
	}
	if len(id) > maxMailboxResourceIDBytes {
		return fmt.Errorf("%s is too long", field)
	}
	return nil
}

func (r *Repository) CreateFolder(ctx context.Context, req CreateFolderRequest) (Folder, error) {
	if r.db == nil {
		return Folder{}, fmt.Errorf("database handle is required")
	}
	name := strings.TrimSpace(req.Name)
	if err := validateFolderName(name); err != nil {
		return Folder{}, err
	}

	const query = `
INSERT INTO folders (user_id, name, full_path, type)
VALUES ($1, $2, $2, 'user')
RETURNING id::text, COALESCE(parent_id::text, ''), name, full_path, type, COALESCE(system_type, ''), order_index`

	var folder Folder
	if err := r.db.QueryRowContext(ctx, query, req.UserID, name).Scan(
		&folder.ID,
		&folder.ParentID,
		&folder.Name,
		&folder.FullPath,
		&folder.Type,
		&folder.SystemType,
		&folder.OrderIndex,
	); err != nil {
		return Folder{}, fmt.Errorf("create folder: %w", err)
	}
	return folder, nil
}

func (r *Repository) RenameFolder(ctx context.Context, userID string, folderID string, name string) (Folder, error) {
	if r.db == nil {
		return Folder{}, fmt.Errorf("database handle is required")
	}
	name = strings.TrimSpace(name)
	if err := validateFolderName(name); err != nil {
		return Folder{}, err
	}

	const query = `
UPDATE folders
SET name = $3,
    full_path = $3
WHERE user_id = $1
  AND id = $2
  AND type = 'user'
RETURNING id::text, COALESCE(parent_id::text, ''), name, full_path, type, COALESCE(system_type, ''), order_index`

	var folder Folder
	if err := r.db.QueryRowContext(ctx, query, userID, folderID, name).Scan(
		&folder.ID,
		&folder.ParentID,
		&folder.Name,
		&folder.FullPath,
		&folder.Type,
		&folder.SystemType,
		&folder.OrderIndex,
	); err != nil {
		if err == sql.ErrNoRows {
			return Folder{}, fmt.Errorf("user folder %q not found", folderID)
		}
		return Folder{}, fmt.Errorf("rename folder: %w", err)
	}
	return folder, nil
}

func validateFolderName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("folder name is required")
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("folder name must not contain path separators")
	}
	if strings.ContainsAny(name, "\r\n") {
		return fmt.Errorf("folder name must not contain CR or LF")
	}
	if len(name) > maxFolderNameBytes {
		return fmt.Errorf("folder name is too long")
	}
	return nil
}

func (r *Repository) DeleteFolder(ctx context.Context, userID string, folderID string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	const query = `
DELETE FROM folders
WHERE user_id = $1
  AND id = $2
  AND type = 'user'
  AND NOT EXISTS (
    SELECT 1
    FROM messages
    WHERE messages.folder_id = folders.id
  )`

	result, err := r.db.ExecContext(ctx, query, userID, folderID)
	if err != nil {
		return fmt.Errorf("delete folder: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect folder delete: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("user folder %q not found or not empty", folderID)
	}
	return nil
}

func (r *Repository) ListFolders(ctx context.Context, userID string) ([]Folder, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	const query = `
SELECT
  f.id::text,
  COALESCE(f.parent_id::text, ''),
  f.name,
  f.full_path,
  f.type,
  COALESCE(f.system_type, ''),
  f.order_index,
  COALESCE(c.total, 0) AS total,
  COALESCE(c.unread, 0) AS unread,
  COALESCE(c.starred, 0) AS starred,
  COALESCE(c.total_size, 0) AS total_size,
  COALESCE(c.imap_unassigned, 0) AS imap_unassigned
FROM folders f
LEFT JOIN (
  SELECT
    m.folder_id,
    COUNT(*) AS total,
    COUNT(*) FILTER (WHERE COALESCE((flags->>'read')::boolean, false) = false) AS unread,
    COUNT(*) FILTER (WHERE COALESCE((flags->>'starred')::boolean, false) = true) AS starred,
    SUM(size) AS total_size,
    COUNT(*) FILTER (WHERE i.message_id IS NULL) AS imap_unassigned
  FROM messages m
  LEFT JOIN imap_message_uid i
    ON i.message_id = m.id
   AND i.user_id = m.user_id
   AND i.mailbox_id = m.folder_id
  WHERE m.user_id = $1
    AND m.status = 'active'
  GROUP BY m.folder_id
) c ON c.folder_id = f.id
WHERE f.user_id = $1
ORDER BY type DESC, order_index ASC, full_path ASC`

	// Ensure all system folders exist (idempotent, ON CONFLICT DO NOTHING)
	_ = createSystemFolders(ctx, r.db, userID)

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
			&folder.Total,
			&folder.Unread,
			&folder.Starred,
			&folder.TotalSize,
			&folder.IMAPUnassigned,
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
  m.id::text,
  m.folder_id::text,
  m.subject,
  left(btrim(regexp_replace(left(coalesce(msd.body_text, ''), 2000), '[[:space:]]+', ' ', 'g')), 280) AS preview,
  m.from_addr,
  m.from_name,
  COALESCE(m.received_at, m.created_at),
  m.size,
  m.has_attachment,
  COALESCE((m.flags->>'read')::boolean, false) AS read,
  COALESCE((m.flags->>'starred')::boolean, false) AS starred
FROM messages m
LEFT JOIN message_search_documents msd
  ON msd.message_id = m.id
 AND msd.user_id = m.user_id
WHERE m.user_id = $1
  AND m.status = 'active'
ORDER BY COALESCE(m.received_at, m.created_at) DESC
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
			&msg.FolderID,
			&msg.Subject,
			&msg.Preview,
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
  m.id::text,
  m.folder_id::text,
  m.subject,
  left(btrim(regexp_replace(left(coalesce(msd.body_text, ''), 2000), '[[:space:]]+', ' ', 'g')), 280) AS preview,
  m.from_addr,
  m.from_name,
  COALESCE(m.received_at, m.created_at),
  m.size,
  m.has_attachment,
  COALESCE((m.flags->>'read')::boolean, false) AS read,
  COALESCE((m.flags->>'starred')::boolean, false) AS starred
FROM messages m
LEFT JOIN message_search_documents msd
  ON msd.message_id = m.id
 AND msd.user_id = m.user_id
WHERE m.user_id = $1
  AND m.folder_id = $2
  AND m.status = 'active'
ORDER BY COALESCE(m.received_at, m.created_at) DESC
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
			&msg.FolderID,
			&msg.Subject,
			&msg.Preview,
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

func (r *Repository) ListMessagesPage(ctx context.Context, userID string, folderID string, limit int, cursor MessageListCursor, filter MessageListFilter) ([]MessageSummary, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	limit = NormalizeMessageListLimit(limit) + 1
	sortMode, ok := NormalizeListSort(filter.Sort)
	if !ok {
		return nil, fmt.Errorf("unsupported list sort %q", filter.Sort)
	}

	query := messageListPageNewestSQL
	if sortMode == ListSortOldest {
		query = messageListPageOldestSQL
	}

	rows, err := r.db.QueryContext(
		ctx,
		query,
		strings.TrimSpace(userID),
		strings.TrimSpace(folderID),
		cursor.At,
		strings.TrimSpace(cursor.ID),
		limit,
		filter.Read,
		filter.Starred,
		filter.HasAttachment,
	)
	if err != nil {
		return nil, fmt.Errorf("list message page: %w", err)
	}
	defer rows.Close()

	messages := make([]MessageSummary, 0, limit)
	for rows.Next() {
		var msg MessageSummary
		if err := rows.Scan(
			&msg.ID,
			&msg.FolderID,
			&msg.Subject,
			&msg.Preview,
			&msg.FromAddr,
			&msg.FromName,
			&msg.ReceivedAt,
			&msg.Size,
			&msg.HasAttachment,
			&msg.Read,
			&msg.Starred,
		); err != nil {
			return nil, fmt.Errorf("scan message page summary: %w", err)
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate message page summaries: %w", err)
	}
	return messages, nil
}

const messageListPageNewestSQL = `
SELECT
  messages.id::text,
  messages.folder_id::text,
  messages.subject,
  left(btrim(regexp_replace(left(coalesce(msd.body_text, ''), 2000), '[[:space:]]+', ' ', 'g')), 280) AS preview,
  messages.from_addr,
  messages.from_name,
  COALESCE(messages.received_at, messages.sent_at, messages.draft_updated_at, messages.created_at) AS message_at,
  messages.size,
  messages.has_attachment,
  COALESCE((messages.flags->>'read')::boolean, false) AS read,
  COALESCE((messages.flags->>'starred')::boolean, false) AS starred
FROM messages
LEFT JOIN message_search_documents msd
  ON msd.message_id = messages.id
 AND msd.user_id = messages.user_id
WHERE messages.user_id = $1
  AND messages.status = 'active'
  AND ($2 = '' OR messages.folder_id::text = $2)
  AND ($6::boolean IS NULL OR COALESCE((messages.flags->>'read')::boolean, false) = $6::boolean)
  AND ($7::boolean IS NULL OR COALESCE((messages.flags->>'starred')::boolean, false) = $7::boolean)
  AND ($8::boolean IS NULL OR messages.has_attachment = $8::boolean)
  AND (
    $4 = ''
    OR (COALESCE(messages.received_at, messages.sent_at, messages.draft_updated_at, messages.created_at), messages.id)
       < ($3::timestamptz, $4::uuid)
  )
ORDER BY message_at DESC, id DESC
LIMIT $5`

const messageListPageOldestSQL = `
SELECT
  messages.id::text,
  messages.folder_id::text,
  messages.subject,
  left(btrim(regexp_replace(left(coalesce(msd.body_text, ''), 2000), '[[:space:]]+', ' ', 'g')), 280) AS preview,
  messages.from_addr,
  messages.from_name,
  COALESCE(messages.received_at, messages.sent_at, messages.draft_updated_at, messages.created_at) AS message_at,
  messages.size,
  messages.has_attachment,
  COALESCE((messages.flags->>'read')::boolean, false) AS read,
  COALESCE((messages.flags->>'starred')::boolean, false) AS starred
FROM messages
LEFT JOIN message_search_documents msd
  ON msd.message_id = messages.id
 AND msd.user_id = messages.user_id
WHERE messages.user_id = $1
  AND messages.status = 'active'
  AND ($2 = '' OR messages.folder_id::text = $2)
  AND ($6::boolean IS NULL OR COALESCE((messages.flags->>'read')::boolean, false) = $6::boolean)
  AND ($7::boolean IS NULL OR COALESCE((messages.flags->>'starred')::boolean, false) = $7::boolean)
  AND ($8::boolean IS NULL OR messages.has_attachment = $8::boolean)
  AND (
    $4 = ''
    OR (COALESCE(messages.received_at, messages.sent_at, messages.draft_updated_at, messages.created_at), messages.id)
       > ($3::timestamptz, $4::uuid)
  )
ORDER BY message_at ASC, id ASC
LIMIT $5`

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
SET flags = jsonb_set(COALESCE(flags, '{}'::jsonb), $3::text[], to_jsonb($4::boolean), true),
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

func (r *Repository) BulkSetMessageFlag(ctx context.Context, req BulkMessageFlagRequest) (int64, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database handle is required")
	}
	if err := ValidateBulkMessageFlagRequest(req); err != nil {
		return 0, err
	}
	rawIDs, err := json.Marshal(req.MessageIDs)
	if err != nil {
		return 0, fmt.Errorf("encode message ids: %w", err)
	}
	flag := strings.TrimSpace(req.Flag)

	const query = `
UPDATE messages
SET flags = jsonb_set(COALESCE(flags, '{}'::jsonb), $3::text[], to_jsonb($4::boolean), true),
    updated_at = now()
WHERE user_id = $1
  AND id IN (SELECT value::uuid FROM jsonb_array_elements_text($2::jsonb))
  AND status = 'active'`

	result, err := r.db.ExecContext(ctx, query, strings.TrimSpace(req.UserID), string(rawIDs), "{"+flag+"}", req.Value)
	if err != nil {
		return 0, fmt.Errorf("bulk set message flag: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("inspect bulk message flag update: %w", err)
	}
	return affected, nil
}

func (r *Repository) BulkSetThreadFlag(ctx context.Context, req BulkThreadFlagRequest) (BulkThreadFlagResult, error) {
	if r.db == nil {
		return BulkThreadFlagResult{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateBulkThreadFlagRequest(req); err != nil {
		return BulkThreadFlagResult{}, err
	}
	flag := strings.TrimSpace(req.Flag)

	rows, err := r.db.QueryContext(ctx, bulkSetThreadFlagSQL, strings.TrimSpace(req.UserID), pq.Array(req.ThreadIDs), "{"+flag+"}", req.Value)
	if err != nil {
		return BulkThreadFlagResult{}, fmt.Errorf("bulk set thread flag: %w", err)
	}
	defer rows.Close()

	var messageIDs []string
	for rows.Next() {
		var messageID string
		if err := rows.Scan(&messageID); err != nil {
			return BulkThreadFlagResult{}, fmt.Errorf("scan bulk thread flag message: %w", err)
		}
		messageIDs = append(messageIDs, messageID)
	}
	if err := rows.Err(); err != nil {
		return BulkThreadFlagResult{}, fmt.Errorf("iterate bulk thread flag messages: %w", err)
	}
	return BulkThreadFlagResult{Updated: int64(len(messageIDs)), MessageIDs: messageIDs}, nil
}

const bulkSetThreadFlagSQL = `
WITH requested AS (
  SELECT value AS id
  FROM unnest($2::uuid[]) AS requested(value)
),
target_messages AS (
  SELECT messages.id
  FROM messages
  JOIN requested ON messages.thread_id = requested.id
  WHERE user_id = $1
    AND status = 'active'
  UNION
  SELECT messages.id
  FROM messages
  JOIN requested ON messages.id = requested.id
  WHERE user_id = $1
    AND status = 'active'
),
updated_messages AS (
  UPDATE messages
  SET flags = jsonb_set(COALESCE(flags, '{}'::jsonb), $3::text[], to_jsonb($4::boolean), true),
      updated_at = now()
  WHERE id IN (SELECT id FROM target_messages)
  RETURNING id::text
)
SELECT id
FROM updated_messages
ORDER BY id`

func (r *Repository) ListMessageIDsForThreads(ctx context.Context, userID string, threadIDs []string) ([]string, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if err := validateBulkThreadIDs(threadIDs); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, listMessageIDsForThreadsSQL, userID, pq.Array(threadIDs))
	if err != nil {
		return nil, fmt.Errorf("list message ids for threads: %w", err)
	}
	defer rows.Close()

	var messageIDs []string
	for rows.Next() {
		var messageID string
		if err := rows.Scan(&messageID); err != nil {
			return nil, fmt.Errorf("scan thread message id: %w", err)
		}
		messageIDs = append(messageIDs, messageID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate thread message ids: %w", err)
	}
	return messageIDs, nil
}

const listMessageIDsForThreadsSQL = `
WITH requested AS (
  SELECT value AS id
  FROM unnest($2::uuid[]) AS requested(value)
)
SELECT id::text
FROM messages
JOIN requested ON messages.thread_id = requested.id
WHERE user_id = $1
  AND status = 'active'
UNION
SELECT id::text
FROM messages
JOIN requested ON messages.id = requested.id
WHERE user_id = $1
  AND status = 'active'
ORDER BY id`

func (r *Repository) MoveMessage(ctx context.Context, userID string, messageID string, folderID string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	folderID = strings.TrimSpace(folderID)
	if strings.TrimSpace(folderID) == "" {
		return fmt.Errorf("folder_id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin move message transaction: %w", err)
	}
	defer tx.Rollback()

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
  )
RETURNING id::text`

	var movedID string
	if err := tx.QueryRowContext(ctx, query, userID, messageID, folderID).Scan(&movedID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("message %q or folder %q not found", messageID, folderID)
		}
		return fmt.Errorf("move message: %w", err)
	}
	if err := deleteIMAPUIDRowsForMessages(ctx, tx, userID, []string{movedID}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit move message transaction: %w", err)
	}
	return nil
}

func (r *Repository) BulkMoveMessages(ctx context.Context, req BulkMessageMoveRequest) (int64, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database handle is required")
	}
	if err := ValidateBulkMessageMoveRequest(req); err != nil {
		return 0, err
	}
	userID := strings.TrimSpace(req.UserID)
	folderID := strings.TrimSpace(req.FolderID)
	rawIDs, err := json.Marshal(req.MessageIDs)
	if err != nil {
		return 0, fmt.Errorf("encode message ids: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin bulk move transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
UPDATE messages
SET folder_id = $3,
    updated_at = now()
WHERE user_id = $1
  AND id IN (SELECT value::uuid FROM jsonb_array_elements_text($2::jsonb))
  AND status = 'active'
  AND EXISTS (
    SELECT 1
    FROM folders
    WHERE folders.id = $3
      AND folders.user_id = $1
  )
RETURNING id::text`

	rows, err := tx.QueryContext(ctx, query, userID, string(rawIDs), folderID)
	if err != nil {
		return 0, fmt.Errorf("bulk move messages: %w", err)
	}
	var movedIDs []string
	for rows.Next() {
		var movedID string
		if err := rows.Scan(&movedID); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan bulk moved message: %w", err)
		}
		movedIDs = append(movedIDs, movedID)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close bulk moved message rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate bulk moved messages: %w", err)
	}
	if err := deleteIMAPUIDRowsForMessages(ctx, tx, userID, movedIDs); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit bulk move transaction: %w", err)
	}
	return int64(len(movedIDs)), nil
}

func (r *Repository) BulkMoveThreads(ctx context.Context, req BulkThreadMoveRequest) (BulkThreadMoveResult, error) {
	if r.db == nil {
		return BulkThreadMoveResult{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateBulkThreadMoveRequest(req); err != nil {
		return BulkThreadMoveResult{}, err
	}
	userID := strings.TrimSpace(req.UserID)
	folderID := strings.TrimSpace(req.FolderID)
	rawIDs, err := json.Marshal(req.ThreadIDs)
	if err != nil {
		return BulkThreadMoveResult{}, fmt.Errorf("encode thread ids: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return BulkThreadMoveResult{}, fmt.Errorf("begin bulk thread move transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, bulkMoveThreadsSQL, userID, string(rawIDs), folderID)
	if err != nil {
		return BulkThreadMoveResult{}, fmt.Errorf("bulk move threads: %w", err)
	}
	var movedIDs []string
	for rows.Next() {
		var movedID string
		if err := rows.Scan(&movedID); err != nil {
			rows.Close()
			return BulkThreadMoveResult{}, fmt.Errorf("scan bulk moved thread message: %w", err)
		}
		movedIDs = append(movedIDs, movedID)
	}
	if err := rows.Close(); err != nil {
		return BulkThreadMoveResult{}, fmt.Errorf("close bulk moved thread rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return BulkThreadMoveResult{}, fmt.Errorf("iterate bulk moved thread messages: %w", err)
	}
	if err := deleteIMAPUIDRowsForMessages(ctx, tx, userID, movedIDs); err != nil {
		return BulkThreadMoveResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return BulkThreadMoveResult{}, fmt.Errorf("commit bulk thread move transaction: %w", err)
	}
	return BulkThreadMoveResult{Updated: int64(len(movedIDs)), MessageIDs: movedIDs}, nil
}

const bulkMoveThreadsSQL = `
UPDATE messages
SET folder_id = $3,
    updated_at = now()
WHERE user_id = $1
  AND COALESCE(thread_id, id)::text IN (
    SELECT value FROM jsonb_array_elements_text($2::jsonb)
  )
  AND status = 'active'
  AND EXISTS (
    SELECT 1
    FROM folders
    WHERE folders.id = $3
      AND folders.user_id = $1
  )
RETURNING id::text`

func (r *Repository) DeleteMessage(ctx context.Context, userID string, messageID string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete message transaction: %w", err)
	}
	defer tx.Rollback()

	var size int64
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(size, 0) FROM messages WHERE user_id = $1 AND id = $2 AND status = 'active'`,
		userID, messageID,
	).Scan(&size); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("message %q not found", messageID)
		}
		return fmt.Errorf("read message size for delete: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE messages
SET status = 'deleted',
    deleted_at = now(),
    updated_at = now()
WHERE user_id = $1
  AND id = $2
  AND status = 'active'`, userID, messageID); err != nil {
		return fmt.Errorf("delete message: %w", err)
	}

	if err := deleteIMAPUIDRowsForMessages(ctx, tx, userID, []string{messageID}); err != nil {
		return err
	}
	if err := decrementUserQuota(ctx, tx, userID, size); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete message transaction: %w", err)
	}
	return nil
}

func (r *Repository) RestoreMessage(ctx context.Context, userID string, messageID string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin restore message transaction: %w", err)
	}
	defer tx.Rollback()

	var size int64
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(size, 0) FROM messages WHERE user_id = $1 AND id = $2 AND status = 'deleted' FOR UPDATE`,
		userID, messageID,
	).Scan(&size); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("message %q not found", messageID)
		}
		return fmt.Errorf("read message size for restore: %w", err)
	}
	if err := checkAndIncrementUserQuota(ctx, tx, userID, size); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE messages
SET status = 'active',
    deleted_at = NULL,
    updated_at = now()
WHERE user_id = $1
  AND id = $2
  AND status = 'deleted'`, userID, messageID); err != nil {
		return fmt.Errorf("restore message: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit restore message transaction: %w", err)
	}
	return nil
}

func (r *Repository) BulkDeleteMessages(ctx context.Context, req BulkMessageDeleteRequest) (int64, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database handle is required")
	}
	if err := ValidateBulkMessageDeleteRequest(req); err != nil {
		return 0, err
	}
	rawIDs, err := json.Marshal(req.MessageIDs)
	if err != nil {
		return 0, fmt.Errorf("encode message ids: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin bulk delete transaction: %w", err)
	}
	defer tx.Rollback()

	var totalSize int64
	if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(SUM(size), 0)
FROM messages
WHERE user_id = $1
  AND id IN (SELECT value::uuid FROM jsonb_array_elements_text($2::jsonb))
  AND status = 'active'`, strings.TrimSpace(req.UserID), string(rawIDs),
	).Scan(&totalSize); err != nil {
		return 0, fmt.Errorf("sum message sizes for bulk delete: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
UPDATE messages
SET status = 'deleted',
    deleted_at = now(),
    updated_at = now()
WHERE user_id = $1
  AND id IN (SELECT value::uuid FROM jsonb_array_elements_text($2::jsonb))
  AND status = 'active'`, strings.TrimSpace(req.UserID), string(rawIDs))
	if err != nil {
		return 0, fmt.Errorf("bulk delete messages: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("inspect bulk message delete: %w", err)
	}

	if err := deleteIMAPUIDRowsForMessages(ctx, tx, strings.TrimSpace(req.UserID), req.MessageIDs); err != nil {
		return 0, err
	}
	if err := decrementUserQuota(ctx, tx, strings.TrimSpace(req.UserID), totalSize); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit bulk delete transaction: %w", err)
	}
	return affected, nil
}

func (r *Repository) BulkRestoreMessages(ctx context.Context, req BulkMessageRestoreRequest) (BulkMessageRestoreResult, error) {
	if r.db == nil {
		return BulkMessageRestoreResult{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateBulkMessageRestoreRequest(req); err != nil {
		return BulkMessageRestoreResult{}, err
	}
	rawIDs, err := json.Marshal(req.MessageIDs)
	if err != nil {
		return BulkMessageRestoreResult{}, fmt.Errorf("encode message ids: %w", err)
	}
	userID := strings.TrimSpace(req.UserID)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return BulkMessageRestoreResult{}, fmt.Errorf("begin bulk restore transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
SELECT id::text, COALESCE(size, 0)
FROM messages
WHERE user_id = $1
  AND id IN (SELECT value::uuid FROM jsonb_array_elements_text($2::jsonb))
  AND status = 'deleted'
FOR UPDATE`, userID, string(rawIDs))
	if err != nil {
		return BulkMessageRestoreResult{}, fmt.Errorf("list messages for bulk restore: %w", err)
	}
	var restoredIDs []string
	var totalSize int64
	for rows.Next() {
		var id string
		var size int64
		if err := rows.Scan(&id, &size); err != nil {
			rows.Close()
			return BulkMessageRestoreResult{}, fmt.Errorf("scan message for bulk restore: %w", err)
		}
		restoredIDs = append(restoredIDs, id)
		totalSize += size
	}
	if err := rows.Close(); err != nil {
		return BulkMessageRestoreResult{}, fmt.Errorf("close bulk restore rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return BulkMessageRestoreResult{}, fmt.Errorf("iterate bulk restore rows: %w", err)
	}
	if err := checkAndIncrementUserQuota(ctx, tx, userID, totalSize); err != nil {
		return BulkMessageRestoreResult{}, err
	}

	if len(restoredIDs) > 0 {
		rawRestoredIDs, err := json.Marshal(restoredIDs)
		if err != nil {
			return BulkMessageRestoreResult{}, fmt.Errorf("encode restored message ids: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE messages
SET status = 'active',
    deleted_at = NULL,
    updated_at = now()
WHERE user_id = $1
  AND id IN (SELECT value::uuid FROM jsonb_array_elements_text($2::jsonb))
  AND status = 'deleted'`, userID, string(rawRestoredIDs)); err != nil {
			return BulkMessageRestoreResult{}, fmt.Errorf("bulk restore messages: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return BulkMessageRestoreResult{}, fmt.Errorf("commit bulk restore transaction: %w", err)
	}
	return BulkMessageRestoreResult{Updated: int64(len(restoredIDs)), MessageIDs: restoredIDs}, nil
}

func (r *Repository) BulkRestoreThreads(ctx context.Context, req BulkThreadRestoreRequest) (BulkThreadRestoreResult, error) {
	if r.db == nil {
		return BulkThreadRestoreResult{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateBulkThreadRestoreRequest(req); err != nil {
		return BulkThreadRestoreResult{}, err
	}
	userID := strings.TrimSpace(req.UserID)
	rawIDs, err := json.Marshal(req.ThreadIDs)
	if err != nil {
		return BulkThreadRestoreResult{}, fmt.Errorf("encode thread ids: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return BulkThreadRestoreResult{}, fmt.Errorf("begin bulk thread restore transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, bulkRestoreThreadsSQL, userID, string(rawIDs))
	if err != nil {
		return BulkThreadRestoreResult{}, fmt.Errorf("bulk restore threads: %w", err)
	}
	var restoredIDs []string
	var totalSize int64
	for rows.Next() {
		var restoredID string
		var size int64
		if err := rows.Scan(&restoredID, &size); err != nil {
			rows.Close()
			return BulkThreadRestoreResult{}, fmt.Errorf("scan bulk restored thread message: %w", err)
		}
		restoredIDs = append(restoredIDs, restoredID)
		totalSize += size
	}
	if err := rows.Close(); err != nil {
		return BulkThreadRestoreResult{}, fmt.Errorf("close bulk restored thread rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return BulkThreadRestoreResult{}, fmt.Errorf("iterate bulk restored thread messages: %w", err)
	}
	if err := checkAndIncrementUserQuota(ctx, tx, userID, totalSize); err != nil {
		return BulkThreadRestoreResult{}, err
	}
	if len(restoredIDs) > 0 {
		rawMessageIDs, err := json.Marshal(restoredIDs)
		if err != nil {
			return BulkThreadRestoreResult{}, fmt.Errorf("encode restored message ids: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE messages
SET status = 'active',
    deleted_at = NULL,
    updated_at = now()
WHERE user_id = $1
  AND id IN (SELECT value::uuid FROM jsonb_array_elements_text($2::jsonb))
  AND status = 'deleted'`, userID, string(rawMessageIDs)); err != nil {
			return BulkThreadRestoreResult{}, fmt.Errorf("activate bulk restored thread messages: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return BulkThreadRestoreResult{}, fmt.Errorf("commit bulk thread restore transaction: %w", err)
	}
	return BulkThreadRestoreResult{Updated: int64(len(restoredIDs)), MessageIDs: restoredIDs}, nil
}

const bulkRestoreThreadsSQL = `
SELECT id::text, COALESCE(size, 0)
FROM messages
WHERE user_id = $1
  AND COALESCE(thread_id, id)::text IN (
    SELECT value FROM jsonb_array_elements_text($2::jsonb)
  )
  AND status = 'deleted'
FOR UPDATE`

func (r *Repository) BulkDeleteThreads(ctx context.Context, req BulkThreadDeleteRequest) (BulkThreadDeleteResult, error) {
	if r.db == nil {
		return BulkThreadDeleteResult{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateBulkThreadDeleteRequest(req); err != nil {
		return BulkThreadDeleteResult{}, err
	}
	userID := strings.TrimSpace(req.UserID)
	rawIDs, err := json.Marshal(req.ThreadIDs)
	if err != nil {
		return BulkThreadDeleteResult{}, fmt.Errorf("encode thread ids: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return BulkThreadDeleteResult{}, fmt.Errorf("begin bulk thread delete transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, bulkDeleteThreadsSQL, userID, string(rawIDs))
	if err != nil {
		return BulkThreadDeleteResult{}, fmt.Errorf("bulk delete threads: %w", err)
	}
	var deletedIDs []string
	var totalSize int64
	for rows.Next() {
		var deletedID string
		var size int64
		if err := rows.Scan(&deletedID, &size); err != nil {
			rows.Close()
			return BulkThreadDeleteResult{}, fmt.Errorf("scan bulk deleted thread message: %w", err)
		}
		deletedIDs = append(deletedIDs, deletedID)
		totalSize += size
	}
	if err := rows.Close(); err != nil {
		return BulkThreadDeleteResult{}, fmt.Errorf("close bulk deleted thread rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return BulkThreadDeleteResult{}, fmt.Errorf("iterate bulk deleted thread messages: %w", err)
	}
	if err := deleteIMAPUIDRowsForMessages(ctx, tx, userID, deletedIDs); err != nil {
		return BulkThreadDeleteResult{}, err
	}
	if err := decrementUserQuota(ctx, tx, userID, totalSize); err != nil {
		return BulkThreadDeleteResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return BulkThreadDeleteResult{}, fmt.Errorf("commit bulk thread delete transaction: %w", err)
	}
	return BulkThreadDeleteResult{Updated: int64(len(deletedIDs)), MessageIDs: deletedIDs}, nil
}

const bulkDeleteThreadsSQL = `
UPDATE messages
SET status = 'deleted',
    deleted_at = now(),
    updated_at = now()
WHERE user_id = $1
  AND COALESCE(thread_id, id)::text IN (
    SELECT value FROM jsonb_array_elements_text($2::jsonb)
  )
  AND status = 'active'
RETURNING id::text, COALESCE(size, 0)`

func allowedMessageFlag(flag string) bool {
	switch flag {
	case "read", "starred", "answered", "forwarded":
		return true
	default:
		return false
	}
}

func deleteIMAPUIDRowsForMessages(ctx context.Context, tx *sql.Tx, userID string, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	rawIDs, err := json.Marshal(messageIDs)
	if err != nil {
		return fmt.Errorf("encode imap uid message ids: %w", err)
	}
	const query = `
DELETE FROM imap_message_uid
WHERE user_id = $1::uuid
  AND message_id IN (SELECT value::uuid FROM jsonb_array_elements_text($2::jsonb))`
	if _, err := tx.ExecContext(ctx, query, strings.TrimSpace(userID), string(rawIDs)); err != nil {
		return fmt.Errorf("delete imap message uid rows: %w", err)
	}
	return nil
}

func normalizeLimit(limit int) int {
	return NormalizeMessageListLimit(limit)
}
