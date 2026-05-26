package dm

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"
	"time"
)

func (s *Service) SendAttachment(ctx context.Context, principal Principal, roomID string, upload AttachmentUpload) (Message, error) {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return Message{}, err
	}
	if s.attachments == nil {
		return Message{}, fmt.Errorf("%w: attachment storage is not configured", ErrInvalid)
	}
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return Message{}, fmt.Errorf("%w: room_id is required", ErrInvalid)
	}
	upload.Filename = sanitizeAttachmentName(upload.Filename)
	if upload.Filename == "" {
		return Message{}, fmt.Errorf("%w: attachment filename is required", ErrInvalid)
	}
	if upload.Size <= 0 || upload.Size > MaxAttachmentBytes || int64(len(upload.Body)) != upload.Size {
		return Message{}, fmt.Errorf("%w: attachment size is invalid", ErrInvalid)
	}
	contentType, err := safeMIMEType(upload.ContentType)
	if err != nil {
		return Message{}, err
	}
	roomKey, err := s.roomKey(ctx, principal, roomID)
	if err != nil {
		return Message{}, err
	}
	defer zeroBytes(roomKey)
	storagePath := fmt.Sprintf("dm/%s/%s/%d-%s", path.Clean(principal.DomainID), path.Clean(roomID), s.now().UTC().UnixNano(), upload.Filename)
	if err := s.attachments.Put(ctx, storagePath, bytes.NewReader(upload.Body)); err != nil {
		return Message{}, fmt.Errorf("store dm attachment: %w", err)
	}
	bodyCiphertext, err := s.crypto.EncryptBody(roomKey, []byte(upload.Filename))
	if err != nil {
		return Message{}, err
	}
	pathCiphertext, err := s.crypto.EncryptBody(roomKey, []byte(storagePath))
	if err != nil {
		return Message{}, err
	}
	record := MessageRecord{
		Message: Message{
			RoomID:             roomID,
			SenderID:           principal.UserID,
			MessageType:        MessageTypeFile,
			AttachmentName:     upload.Filename,
			AttachmentSize:     upload.Size,
			AttachmentMIMEType: contentType,
		},
		BodyCiphertext:                  bodyCiphertext,
		AttachmentStoragePathCiphertext: pathCiphertext,
	}
	stored, err := s.store.InsertMessage(ctx, principal, record, nil)
	if err != nil {
		return Message{}, err
	}
	stored.Body = upload.Filename
	return stored.Message, nil
}

func (s *Service) ListMedia(ctx context.Context, principal Principal, roomID string, query MediaQuery) ([]MediaItem, error) {
	principal = normalizePrincipal(principal)
	if err := validatePrincipal(principal); err != nil {
		return nil, err
	}
	// Normalize API-level type names to the store's internal tokens.
	// API uses: "file", "drive_link", "link"
	// Store uses: "file" (→ listFileMedia), "drive" (→ listDriveMedia), "links" (→ listLinkMedia)
	switch strings.ToLower(strings.TrimSpace(query.Type)) {
	case "drive_link", "drive":
		query.Type = "drive"
	case "link", "links":
		query.Type = "links"
	default:
		query.Type = "file"
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 30
	}
	return s.store.ListMedia(ctx, principal, strings.TrimSpace(roomID), query)
}

func (s *Service) SignAttachmentDownload(messageID string, expiresAt time.Time) (string, error) {
	if s == nil || s.crypto == nil {
		return "", fmt.Errorf("dm crypto is not configured")
	}
	return s.crypto.SignAttachmentToken(messageID, expiresAt)
}

func (s *Service) VerifyAttachmentDownload(token string) (string, error) {
	if s == nil || s.crypto == nil {
		return "", fmt.Errorf("dm crypto is not configured")
	}
	return s.crypto.VerifyAttachmentToken(token, s.now())
}

func (s *Service) OpenAttachment(ctx context.Context, token string) (AttachmentDownload, error) {
	if s == nil || s.crypto == nil {
		return AttachmentDownload{}, fmt.Errorf("dm crypto is not configured")
	}
	if s.attachments == nil {
		return AttachmentDownload{}, fmt.Errorf("%w: attachment storage is not configured", ErrInvalid)
	}
	messageID, err := s.VerifyAttachmentDownload(token)
	if err != nil {
		return AttachmentDownload{}, err
	}
	record, wrapped, err := s.store.AttachmentByMessageID(ctx, messageID)
	if err != nil {
		return AttachmentDownload{}, err
	}
	roomKey, err := s.crypto.UnwrapRoomKey(wrapped)
	if err != nil {
		return AttachmentDownload{}, fmt.Errorf("dm room key unavailable: %w", err)
	}
	defer zeroBytes(roomKey)
	plainPath, err := s.crypto.DecryptBody(roomKey, record.AttachmentStoragePathCiphertext)
	if err != nil {
		return AttachmentDownload{}, err
	}
	body, err := s.attachments.Get(ctx, string(plainPath))
	if err != nil {
		return AttachmentDownload{}, err
	}
	return AttachmentDownload{
		Filename:    record.AttachmentName,
		Size:        record.AttachmentSize,
		ContentType: record.AttachmentMIMEType,
		Body:        body,
	}, nil
}
