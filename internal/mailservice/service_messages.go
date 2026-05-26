package mailservice

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/message"
)

func (s *Service) ListMessages(ctx context.Context, userID string, limit int) ([]maildb.MessageSummary, error) {
	userID = strings.TrimSpace(userID)
	limit = maildb.NormalizeMessageListLimit(limit)
	return s.repository.ListMessages(ctx, userID, limit)
}

func (s *Service) ListMessagesInFolder(ctx context.Context, userID string, folderID string, limit int) ([]maildb.MessageSummary, error) {
	userID = strings.TrimSpace(userID)
	folderID = strings.TrimSpace(folderID)
	if err := validateServiceResourceID("folder_id", folderID); err != nil {
		return nil, err
	}
	limit = maildb.NormalizeMessageListLimit(limit)
	return s.repository.ListMessagesInFolder(ctx, userID, folderID, limit)
}

func (s *Service) ListMessagesPage(ctx context.Context, userID string, folderID string, limit int, cursor maildb.MessageListCursor, filter maildb.MessageListFilter) ([]maildb.MessageSummary, error) {
	userID = strings.TrimSpace(userID)
	folderID = strings.TrimSpace(folderID)
	if folderID != "" {
		if err := validateServiceResourceID("folder_id", folderID); err != nil {
			return nil, err
		}
	}
	sortMode, ok := maildb.NormalizeListSort(filter.Sort)
	if !ok {
		return nil, fmt.Errorf("sort must be newest or oldest")
	}
	filter.Sort = sortMode
	limit = maildb.NormalizeMessageListLimit(limit)
	return s.repository.ListMessagesPage(ctx, userID, folderID, limit, cursor, filter)
}

func (s *Service) FetchRawMessageBody(ctx context.Context, userID, messageID string) (string, error) {
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	if err := validateServiceResourceID("message_id", messageID); err != nil {
		return "", err
	}
	detail, err := s.repository.GetMessage(ctx, userID, messageID)
	if err != nil {
		return "", err
	}
	if s.store == nil || detail.StoragePath == "" {
		return "", fmt.Errorf("message body not available")
	}
	storagePath, err := requireStoredObjectPath("message body", detail.StoragePath)
	if err != nil {
		return "", err
	}
	rc, err := s.store.Get(ctx, storagePath)
	if err != nil {
		return "", fmt.Errorf("open message body: %w", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("read message body: %w", err)
	}
	return string(data), nil
}

func (s *Service) GetMessage(ctx context.Context, userID string, messageID string) (maildb.MessageDetail, error) {
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	if err := validateServiceResourceID("message_id", messageID); err != nil {
		return maildb.MessageDetail{}, err
	}
	detail, err := s.repository.GetMessage(ctx, userID, messageID)
	if err != nil {
		return maildb.MessageDetail{}, err
	}
	if !messageFlagRead(detail.Flags) {
		if err := s.repository.SetMessageFlag(ctx, userID, messageID, "read", true); err == nil {
			_ = s.publishIMAPMessageUIDEvents(ctx, imapgw.MailboxEventFlags, userID, []string{messageID})
		}
	}
	if s.store == nil || detail.StoragePath == "" {
		// When there is no object-store body, fall back to the inline html_body column
		// (populated by seed/test data and draft-converted messages).
		if detail.HTMLBody != "" && detail.TextBody == "" {
			detail.TextBody = stripHTMLTags(detail.HTMLBody)
		}
		return detail, nil
	}
	if detail.HasAttachment {
		attachments, err := s.repository.ListAttachments(ctx, userID, messageID)
		if err != nil {
			return maildb.MessageDetail{}, err
		}
		detail.Attachments = attachments
	}

	storagePath, err := requireStoredObjectPath("message body", detail.StoragePath)
	if err != nil {
		return maildb.MessageDetail{}, err
	}
	if body, ok := s.bodyCache.get(storagePath, time.Now()); ok {
		detail.TextBody = body.text
		detail.HTMLBody = body.html
		return detail, nil
	}
	body, err := s.store.Get(ctx, storagePath)
	if err != nil {
		return maildb.MessageDetail{}, fmt.Errorf("open message body: %w", err)
	}
	defer body.Close()

	parsed, err := message.ParseEML(body)
	if err != nil {
		return maildb.MessageDetail{}, fmt.Errorf("parse message body: %w", err)
	}
	detail.TextBody = parsed.TextBody
	detail.HTMLBody = parsed.HTMLBody
	s.bodyCache.put(storagePath, parsedMessageBody{text: parsed.TextBody, html: parsed.HTMLBody}, time.Now())
	return detail, nil
}
func messageFlagRead(flags json.RawMessage) bool {
	if len(flags) == 0 {
		return false
	}
	var values map[string]bool
	if err := json.Unmarshal(flags, &values); err != nil {
		return false
	}
	return values["read"]
}

func (s *Service) SetMessageFlag(ctx context.Context, userID string, messageID string, flag string, value bool) error {
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	flag = strings.TrimSpace(flag)
	if err := validateServiceResourceID("message_id", messageID); err != nil {
		return fmt.Errorf("set message flag: %w", err)
	}
	if err := s.repository.SetMessageFlag(ctx, userID, messageID, flag, value); err != nil {
		return fmt.Errorf("set message flag: %w", err)
	}
	_ = s.publishIMAPMessageUIDEvents(ctx, imapgw.MailboxEventFlags, userID, []string{messageID})
	return nil
}

func (s *Service) BulkSetMessageFlag(ctx context.Context, req maildb.BulkMessageFlagRequest) (int64, error) {
	req = normalizeBulkMessageFlagRequest(req)
	if err := maildb.ValidateBulkMessageFlagRequest(req); err != nil {
		return 0, err
	}
	updated, err := s.repository.BulkSetMessageFlag(ctx, req)
	if err != nil {
		return 0, err
	}
	_ = s.publishIMAPMessageUIDEvents(ctx, imapgw.MailboxEventFlags, req.UserID, req.MessageIDs)
	return updated, nil
}

func (s *Service) BulkSetThreadFlag(ctx context.Context, req maildb.BulkThreadFlagRequest) (int64, error) {
	req = normalizeBulkThreadFlagRequest(req)
	if err := maildb.ValidateBulkThreadFlagRequest(req); err != nil {
		return 0, err
	}
	repo, ok := s.repository.(interface {
		BulkSetThreadFlag(context.Context, maildb.BulkThreadFlagRequest) (maildb.BulkThreadFlagResult, error)
	})
	if !ok {
		return 0, fmt.Errorf("thread flag repository is required")
	}
	result, err := repo.BulkSetThreadFlag(ctx, req)
	if err != nil {
		return 0, err
	}
	_ = s.publishIMAPMessageUIDEvents(ctx, imapgw.MailboxEventFlags, req.UserID, result.MessageIDs)
	return result.Updated, nil
}

func (s *Service) MoveMessage(ctx context.Context, userID string, messageID string, folderID string) error {
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	folderID = strings.TrimSpace(folderID)
	if err := validateServiceResourceID("message_id", messageID); err != nil {
		return fmt.Errorf("move message: %w", err)
	}
	if err := validateServiceResourceID("folder_id", folderID); err != nil {
		return fmt.Errorf("move message: %w", err)
	}
	uids, err := s.lookupExistingIMAPMessageUIDs(ctx, userID, []string{messageID})
	if err != nil {
		return fmt.Errorf("move message: lookup IMAP UIDs: %w", err)
	}
	if err := s.repository.MoveMessage(ctx, userID, messageID, folderID); err != nil {
		return fmt.Errorf("move message: %w", err)
	}
	_ = s.publishIMAPUIDEvents(ctx, imapgw.MailboxEventExpunge, userID, uids)
	return nil
}

func (s *Service) BulkMoveMessages(ctx context.Context, req maildb.BulkMessageMoveRequest) (int64, error) {
	req = normalizeBulkMessageMoveRequest(req)
	if err := maildb.ValidateBulkMessageMoveRequest(req); err != nil {
		return 0, err
	}
	uids, err := s.lookupExistingIMAPMessageUIDs(ctx, req.UserID, req.MessageIDs)
	if err != nil {
		return 0, err
	}
	updated, err := s.repository.BulkMoveMessages(ctx, req)
	if err != nil {
		return 0, err
	}
	_ = s.publishIMAPUIDEvents(ctx, imapgw.MailboxEventExpunge, req.UserID, uids)
	return updated, nil
}

func (s *Service) BulkMoveThreads(ctx context.Context, req maildb.BulkThreadMoveRequest) (int64, error) {
	req = normalizeBulkThreadMoveRequest(req)
	if err := maildb.ValidateBulkThreadMoveRequest(req); err != nil {
		return 0, err
	}
	repo, ok := s.repository.(interface {
		ListMessageIDsForThreads(context.Context, string, []string) ([]string, error)
		BulkMoveThreads(context.Context, maildb.BulkThreadMoveRequest) (maildb.BulkThreadMoveResult, error)
	})
	if !ok {
		return 0, fmt.Errorf("thread move repository is required")
	}
	messageIDs, err := repo.ListMessageIDsForThreads(ctx, req.UserID, req.ThreadIDs)
	if err != nil {
		return 0, err
	}
	uids, err := s.lookupExistingIMAPMessageUIDs(ctx, req.UserID, messageIDs)
	if err != nil {
		return 0, err
	}
	result, err := repo.BulkMoveThreads(ctx, req)
	if err != nil {
		return 0, err
	}
	_ = s.publishIMAPUIDEvents(ctx, imapgw.MailboxEventExpunge, req.UserID, uids)
	return result.Updated, nil
}

func (s *Service) DeleteMessage(ctx context.Context, userID string, messageID string) error {
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	if err := validateServiceResourceID("message_id", messageID); err != nil {
		return fmt.Errorf("delete message: %w", err)
	}
	uids, err := s.lookupExistingIMAPMessageUIDs(ctx, userID, []string{messageID})
	if err != nil {
		return fmt.Errorf("delete message: lookup IMAP UIDs: %w", err)
	}
	// Resolve deleteable storage paths BEFORE the DB record is removed so that
	// the reference-count check in LookupDeleteableStoragePaths is accurate.
	storagePaths := s.lookupGCStoragePaths(ctx, userID, []string{messageID})
	if err := s.repository.DeleteMessage(ctx, userID, messageID); err != nil {
		return fmt.Errorf("delete message: %w", err)
	}
	s.deleteStorageObjects(ctx, storagePaths)
	_ = s.publishIMAPUIDEvents(ctx, imapgw.MailboxEventExpunge, userID, uids)
	return nil
}

func (s *Service) BulkDeleteMessages(ctx context.Context, req maildb.BulkMessageDeleteRequest) (int64, error) {
	req = normalizeBulkMessageDeleteRequest(req)
	if err := maildb.ValidateBulkMessageDeleteRequest(req); err != nil {
		return 0, err
	}
	uids, err := s.lookupExistingIMAPMessageUIDs(ctx, req.UserID, req.MessageIDs)
	if err != nil {
		return 0, err
	}
	storagePaths := s.lookupGCStoragePaths(ctx, req.UserID, req.MessageIDs)
	updated, err := s.repository.BulkDeleteMessages(ctx, req)
	if err != nil {
		return 0, err
	}
	s.deleteStorageObjects(ctx, storagePaths)
	_ = s.publishIMAPUIDEvents(ctx, imapgw.MailboxEventExpunge, req.UserID, uids)
	return updated, nil
}

func (s *Service) BulkDeleteThreads(ctx context.Context, req maildb.BulkThreadDeleteRequest) (int64, error) {
	req = normalizeBulkThreadDeleteRequest(req)
	if err := maildb.ValidateBulkThreadDeleteRequest(req); err != nil {
		return 0, err
	}
	repo, ok := s.repository.(interface {
		ListMessageIDsForThreads(context.Context, string, []string) ([]string, error)
		BulkDeleteThreads(context.Context, maildb.BulkThreadDeleteRequest) (maildb.BulkThreadDeleteResult, error)
	})
	if !ok {
		return 0, fmt.Errorf("thread delete repository is required")
	}
	messageIDs, err := repo.ListMessageIDsForThreads(ctx, req.UserID, req.ThreadIDs)
	if err != nil {
		return 0, err
	}
	uids, err := s.lookupExistingIMAPMessageUIDs(ctx, req.UserID, messageIDs)
	if err != nil {
		return 0, err
	}
	storagePaths := s.lookupGCStoragePaths(ctx, req.UserID, messageIDs)
	result, err := repo.BulkDeleteThreads(ctx, req)
	if err != nil {
		return 0, err
	}
	s.deleteStorageObjects(ctx, storagePaths)
	_ = s.publishIMAPUIDEvents(ctx, imapgw.MailboxEventExpunge, req.UserID, uids)
	return result.Updated, nil
}

func (s *Service) RestoreMessage(ctx context.Context, userID string, messageID string) error {
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	if err := validateServiceResourceID("message_id", messageID); err != nil {
		return fmt.Errorf("restore message: %w", err)
	}
	if err := s.repository.RestoreMessage(ctx, userID, messageID); err != nil {
		return fmt.Errorf("restore message: %w", err)
	}
	_ = s.publishIMAPRestoredMessageEvents(ctx, userID, []string{messageID})
	return nil
}

func (s *Service) BulkRestoreMessages(ctx context.Context, req maildb.BulkMessageRestoreRequest) (int64, error) {
	req = normalizeBulkMessageRestoreRequest(req)
	if err := maildb.ValidateBulkMessageRestoreRequest(req); err != nil {
		return 0, err
	}
	result, err := s.repository.BulkRestoreMessages(ctx, req)
	if err != nil {
		return 0, err
	}
	_ = s.publishIMAPRestoredMessageEvents(ctx, req.UserID, result.MessageIDs)
	return result.Updated, nil
}

func (s *Service) BulkRestoreThreads(ctx context.Context, req maildb.BulkThreadRestoreRequest) (int64, error) {
	req = normalizeBulkThreadRestoreRequest(req)
	if err := maildb.ValidateBulkThreadRestoreRequest(req); err != nil {
		return 0, err
	}
	result, err := s.repository.BulkRestoreThreads(ctx, req)
	if err != nil {
		return 0, err
	}
	_ = s.publishIMAPRestoredMessageEvents(ctx, req.UserID, result.MessageIDs)
	return result.Updated, nil
}
func normalizeBulkMessageFlagRequest(req maildb.BulkMessageFlagRequest) maildb.BulkMessageFlagRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.MessageIDs = normalizeStringList(req.MessageIDs)
	req.Flag = strings.TrimSpace(req.Flag)
	return req
}

func normalizeBulkThreadFlagRequest(req maildb.BulkThreadFlagRequest) maildb.BulkThreadFlagRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.ThreadIDs = normalizeStringList(req.ThreadIDs)
	req.Flag = strings.TrimSpace(req.Flag)
	return req
}

func normalizeBulkMessageMoveRequest(req maildb.BulkMessageMoveRequest) maildb.BulkMessageMoveRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.MessageIDs = normalizeStringList(req.MessageIDs)
	req.FolderID = strings.TrimSpace(req.FolderID)
	return req
}

func normalizeBulkThreadMoveRequest(req maildb.BulkThreadMoveRequest) maildb.BulkThreadMoveRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.ThreadIDs = normalizeStringList(req.ThreadIDs)
	req.FolderID = strings.TrimSpace(req.FolderID)
	return req
}

func normalizeBulkMessageDeleteRequest(req maildb.BulkMessageDeleteRequest) maildb.BulkMessageDeleteRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.MessageIDs = normalizeStringList(req.MessageIDs)
	return req
}

func normalizeBulkMessageRestoreRequest(req maildb.BulkMessageRestoreRequest) maildb.BulkMessageRestoreRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.MessageIDs = normalizeStringList(req.MessageIDs)
	return req
}

func normalizeBulkThreadRestoreRequest(req maildb.BulkThreadRestoreRequest) maildb.BulkThreadRestoreRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.ThreadIDs = normalizeStringList(req.ThreadIDs)
	return req
}

func normalizeBulkThreadDeleteRequest(req maildb.BulkThreadDeleteRequest) maildb.BulkThreadDeleteRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.ThreadIDs = normalizeStringList(req.ThreadIDs)
	return req
}

func stripHTMLTags(html string) string {
	var b strings.Builder
	inTag := false
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteRune(' ')
		case !inTag:
			b.WriteRune(r)
		}
	}
	// Collapse runs of whitespace
	out := strings.Join(strings.Fields(b.String()), " ")
	return out
}
