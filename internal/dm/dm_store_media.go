package dm

import (
	"context"
	"strings"
)

func (s *PostgresStore) ListMedia(ctx context.Context, principal Principal, roomID string, query MediaQuery) ([]MediaItem, error) {
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	mediaType := strings.ToLower(strings.TrimSpace(query.Type))
	if mediaType == "links" {
		return s.listLinkMedia(ctx, principal, roomID, query)
	}
	if mediaType == "drive" {
		return s.listDriveMedia(ctx, principal, roomID, query)
	}
	return s.listFileMedia(ctx, principal, roomID, query)
}

func (s *PostgresStore) listLinkMedia(ctx context.Context, principal Principal, roomID string, query MediaQuery) ([]MediaItem, error) {
	const sqlQuery = `
SELECT u.message_id::text, m.sender_id::text, u.url, u.created_at
FROM dm_message_urls u
JOIN dm_messages m ON m.id = u.message_id
JOIN dm_rooms r ON r.id = u.room_id
JOIN dm_participants p ON p.room_id = r.id AND p.user_id = $1
WHERE r.id = $2 AND r.company_id = $3 AND r.domain_id = $4
ORDER BY u.created_at DESC
LIMIT $5`
	rows, err := s.db.QueryContext(ctx, sqlQuery, principal.UserID, roomID, principal.CompanyID, principal.DomainID, query.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []MediaItem
	for rows.Next() {
		var item MediaItem
		item.MessageType = "link"
		if err := rows.Scan(&item.MessageID, &item.SenderID, &item.URL, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) listDriveMedia(ctx context.Context, principal Principal, roomID string, query MediaQuery) ([]MediaItem, error) {
	const sqlQuery = `
SELECT m.id::text, COALESCE(m.sender_id::text, ''), m.drive_file_id::text, COALESCE(n.name, ''), m.created_at
FROM dm_messages m
JOIN dm_rooms r ON r.id = m.room_id
JOIN dm_participants p ON p.room_id = r.id AND p.user_id = $1
LEFT JOIN drive_nodes n ON n.id = m.drive_file_id
WHERE r.id = $2 AND r.company_id = $3 AND r.domain_id = $4 AND m.drive_file_id IS NOT NULL AND m.deleted_at IS NULL
ORDER BY m.created_at DESC
LIMIT $5`
	rows, err := s.db.QueryContext(ctx, sqlQuery, principal.UserID, roomID, principal.CompanyID, principal.DomainID, query.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []MediaItem
	for rows.Next() {
		var item MediaItem
		item.MessageType = MessageTypeDriveLink
		if err := rows.Scan(&item.MessageID, &item.SenderID, &item.DriveFileID, &item.DriveName, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) listFileMedia(ctx context.Context, principal Principal, roomID string, query MediaQuery) ([]MediaItem, error) {
	filter := ""
	switch strings.ToLower(strings.TrimSpace(query.Type)) {
	case "image":
		filter = " AND m.attachment_mime_type LIKE 'image/%'"
	case "video":
		filter = " AND m.attachment_mime_type LIKE 'video/%'"
	case "file":
		filter = " AND (m.attachment_mime_type IS NULL OR (m.attachment_mime_type NOT LIKE 'image/%' AND m.attachment_mime_type NOT LIKE 'video/%'))"
	}
	sqlQuery := `
SELECT m.id::text, COALESCE(m.sender_id::text, ''), COALESCE(m.attachment_name, ''), COALESCE(m.attachment_size, 0),
  COALESCE(m.attachment_mime_type, ''), m.created_at
FROM dm_messages m
JOIN dm_rooms r ON r.id = m.room_id
JOIN dm_participants p ON p.room_id = r.id AND p.user_id = $1
WHERE r.id = $2 AND r.company_id = $3 AND r.domain_id = $4 AND m.message_type = 'file' AND m.deleted_at IS NULL` + filter + `
ORDER BY m.created_at DESC
LIMIT $5`
	rows, err := s.db.QueryContext(ctx, sqlQuery, principal.UserID, roomID, principal.CompanyID, principal.DomainID, query.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []MediaItem
	for rows.Next() {
		var item MediaItem
		item.MessageType = MessageTypeFile
		if err := rows.Scan(&item.MessageID, &item.SenderID, &item.AttachmentName, &item.AttachmentSize, &item.AttachmentMIMEType, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
