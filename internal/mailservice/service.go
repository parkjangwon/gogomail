package mailservice

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/message"
	"github.com/gogomail/gogomail/internal/outbound"
	"github.com/gogomail/gogomail/internal/searchindex"
	"github.com/gogomail/gogomail/internal/storage"
)

type Repository interface {
	ListFolders(ctx context.Context, userID string) ([]maildb.Folder, error)
	CreateFolder(ctx context.Context, req maildb.CreateFolderRequest) (maildb.Folder, error)
	RenameFolder(ctx context.Context, userID string, folderID string, name string) (maildb.Folder, error)
	DeleteFolder(ctx context.Context, userID string, folderID string) error
	ListMessages(ctx context.Context, userID string, limit int) ([]maildb.MessageSummary, error)
	ListMessagesInFolder(ctx context.Context, userID string, folderID string, limit int) ([]maildb.MessageSummary, error)
	ListMessagesPage(ctx context.Context, userID string, folderID string, limit int, cursor maildb.MessageListCursor) ([]maildb.MessageSummary, error)
	GetMessage(ctx context.Context, userID string, messageID string) (maildb.MessageDetail, error)
	SetMessageFlag(ctx context.Context, userID string, messageID string, flag string, value bool) error
	BulkSetMessageFlag(ctx context.Context, req maildb.BulkMessageFlagRequest) (int64, error)
	MoveMessage(ctx context.Context, userID string, messageID string, folderID string) error
	BulkMoveMessages(ctx context.Context, req maildb.BulkMessageMoveRequest) (int64, error)
	DeleteMessage(ctx context.Context, userID string, messageID string) error
	BulkDeleteMessages(ctx context.Context, req maildb.BulkMessageDeleteRequest) (int64, error)
	ListPushDevices(ctx context.Context, userID string, limit int) ([]maildb.PushDevice, error)
	UpsertPushDevice(ctx context.Context, req maildb.UpsertPushDeviceRequest) (maildb.PushDevice, error)
	DeletePushDevice(ctx context.Context, userID string, id string) error
	ListAttachments(ctx context.Context, userID string, messageID string) ([]maildb.Attachment, error)
	GetAttachment(ctx context.Context, userID string, messageID string, attachmentID string) (maildb.Attachment, error)
	SenderForUser(ctx context.Context, userID string, fromAddress string) (maildb.Sender, error)
	SuppressedRecipients(ctx context.Context, domainID string, recipients []string) ([]string, error)
	RecordOutgoing(ctx context.Context, msg maildb.OutgoingMessage) (string, error)
}

type DraftRepository interface {
	SaveDraft(ctx context.Context, req maildb.SaveDraftRequest) (maildb.MessageDetail, error)
	DeleteDraft(ctx context.Context, userID string, draftID string) error
}

type DraftSendRepository interface {
	GetDraftForSend(ctx context.Context, userID string, draftID string) (maildb.DraftForSend, error)
	MarkDraftSent(ctx context.Context, userID string, draftID string, sentMessageID string) error
}

type AttachmentUploadRepository interface {
	CreateAttachmentUpload(ctx context.Context, req maildb.CreateAttachmentUploadRequest) (maildb.Attachment, error)
}

type AttachmentCleanupRepository interface {
	ExpireStaleAttachmentUploads(ctx context.Context, req maildb.ExpireStaleAttachmentUploadsRequest) ([]maildb.Attachment, error)
	CountStaleAttachmentUploads(ctx context.Context, req maildb.ExpireStaleAttachmentUploadsRequest) (maildb.StaleAttachmentUploadCount, error)
}

type DeliveryStatusRepository interface {
	MessageDeliveryStatus(ctx context.Context, userID string, messageID string) (maildb.MessageDeliveryStatusView, error)
}

type DomainPolicyRepository interface {
	DomainPolicy(ctx context.Context, domainID string) (maildb.DomainPolicyView, error)
}

type UserDomainPolicyRepository interface {
	DomainPolicyForUser(ctx context.Context, userID string) (maildb.DomainPolicyView, error)
}

type SourceThreadRepository interface {
	SourceThread(ctx context.Context, userID string, sourceMessageID string) (maildb.SourceThreadView, error)
}

type Service struct {
	repository     Repository
	store          storage.Store
	searchIDSource SearchIDSource
	imapEvents     IMAPMailboxEventPublisher
}

func New(repository Repository, store storage.Store) *Service {
	return &Service{repository: repository, store: store}
}

type SearchIDSource interface {
	SearchMessageIDs(ctx context.Context, query searchindex.OpenSearchSearchQuery) ([]searchindex.OpenSearchHit, error)
}

type IMAPMailboxEventPublisher interface {
	Publish(ctx context.Context, event imapgw.MailboxEvent) error
}

func (s *Service) WithSearchIDSource(source SearchIDSource) *Service {
	s.searchIDSource = source
	return s
}

func (s *Service) WithIMAPMailboxEvents(publisher IMAPMailboxEventPublisher) *Service {
	s.imapEvents = publisher
	return s
}

func (s *Service) ListFolders(ctx context.Context, userID string) ([]maildb.Folder, error) {
	userID = strings.TrimSpace(userID)
	return s.repository.ListFolders(ctx, userID)
}

func (s *Service) CreateFolder(ctx context.Context, req maildb.CreateFolderRequest) (maildb.Folder, error) {
	req.UserID = strings.TrimSpace(req.UserID)
	req.Name = strings.TrimSpace(req.Name)
	return s.repository.CreateFolder(ctx, req)
}

func (s *Service) RenameFolder(ctx context.Context, userID string, folderID string, name string) (maildb.Folder, error) {
	userID = strings.TrimSpace(userID)
	folderID = strings.TrimSpace(folderID)
	name = strings.TrimSpace(name)
	if err := validateServiceResourceID("folder_id", folderID); err != nil {
		return maildb.Folder{}, err
	}
	return s.repository.RenameFolder(ctx, userID, folderID, name)
}

func (s *Service) DeleteFolder(ctx context.Context, userID string, folderID string) error {
	userID = strings.TrimSpace(userID)
	folderID = strings.TrimSpace(folderID)
	if err := validateServiceResourceID("folder_id", folderID); err != nil {
		return err
	}
	return s.repository.DeleteFolder(ctx, userID, folderID)
}

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

func (s *Service) ListMessagesPage(ctx context.Context, userID string, folderID string, limit int, cursor maildb.MessageListCursor) ([]maildb.MessageSummary, error) {
	userID = strings.TrimSpace(userID)
	folderID = strings.TrimSpace(folderID)
	if folderID != "" {
		if err := validateServiceResourceID("folder_id", folderID); err != nil {
			return nil, err
		}
	}
	limit = maildb.NormalizeMessageListLimit(limit)
	return s.repository.ListMessagesPage(ctx, userID, folderID, limit, cursor)
}

func (s *Service) ListThreads(ctx context.Context, userID string, limit int) ([]maildb.ThreadSummary, error) {
	repo, ok := s.repository.(interface {
		ListThreads(context.Context, string, int) ([]maildb.ThreadSummary, error)
	})
	if !ok {
		return nil, fmt.Errorf("thread repository is required")
	}
	userID = strings.TrimSpace(userID)
	limit = maildb.NormalizeMessageListLimit(limit)
	return repo.ListThreads(ctx, userID, limit)
}

func (s *Service) ListThreadMessages(ctx context.Context, userID string, threadID string, limit int) ([]maildb.MessageSummary, error) {
	repo, ok := s.repository.(interface {
		ListThreadMessages(context.Context, string, string, int) ([]maildb.MessageSummary, error)
	})
	if !ok {
		return nil, fmt.Errorf("thread repository is required")
	}
	userID = strings.TrimSpace(userID)
	threadID = strings.TrimSpace(threadID)
	if err := validateServiceResourceID("thread_id", threadID); err != nil {
		return nil, err
	}
	limit = maildb.NormalizeMessageListLimit(limit)
	return repo.ListThreadMessages(ctx, userID, threadID, limit)
}

func (s *Service) SearchMessages(ctx context.Context, query maildb.MessageSearchQuery) ([]maildb.MessageSummary, error) {
	query = normalizeMessageSearchQuery(query)
	if err := validateMessageSearchQuery(query); err != nil {
		return nil, err
	}
	if s.searchIDSource != nil && canUseSearchIDSource(query) {
		return s.searchMessagesByExternalIDs(ctx, query)
	}
	repo, ok := s.repository.(interface {
		SearchMessages(context.Context, maildb.MessageSearchQuery) ([]maildb.MessageSummary, error)
	})
	if !ok {
		return nil, fmt.Errorf("search repository is required")
	}
	return repo.SearchMessages(ctx, query)
}

func (s *Service) searchMessagesByExternalIDs(ctx context.Context, query maildb.MessageSearchQuery) ([]maildb.MessageSummary, error) {
	hydrator, ok := s.repository.(interface {
		ListMessagesByIDs(context.Context, string, []string) ([]maildb.MessageSummary, error)
	})
	if !ok {
		return nil, fmt.Errorf("search hydration repository is required")
	}
	hits, err := s.searchIDSource.SearchMessageIDs(ctx, searchindex.OpenSearchSearchQuery{
		UserID:            query.UserID,
		FolderID:          query.FolderID,
		Query:             query.Query,
		From:              query.From,
		Subject:           query.Subject,
		HasAttachment:     query.HasAttachment,
		IncludeHighlights: query.IncludeHighlights,
		Limit:             query.Limit,
	})
	if err != nil {
		return nil, err
	}
	messageIDs := make([]string, 0, len(hits))
	ranks := make(map[string]float64, len(hits))
	highlights := make(map[string]searchindex.OpenSearchHighlights, len(hits))
	for _, hit := range hits {
		id := strings.TrimSpace(hit.MessageID)
		if id == "" {
			continue
		}
		if _, ok := ranks[id]; ok {
			continue
		}
		messageIDs = append(messageIDs, id)
		ranks[id] = hit.Score
		highlights[id] = hit.Highlights
	}
	messages, err := hydrator.ListMessagesByIDs(ctx, query.UserID, messageIDs)
	if err != nil {
		return nil, err
	}
	if query.IncludeRank {
		for i := range messages {
			if rank, ok := ranks[messages[i].ID]; ok {
				messages[i].SearchRank = &rank
			}
		}
	}
	if query.IncludeHighlights {
		for i := range messages {
			if highlight, ok := highlights[messages[i].ID]; ok {
				messages[i].SearchHighlights = &maildb.MessageSearchHighlights{
					Subject: append([]string(nil), highlight.Subject...),
					From:    append([]string(nil), highlight.From...),
					Body:    append([]string(nil), highlight.Body...),
				}
			}
		}
	}
	return messages, nil
}

func canUseSearchIDSource(query maildb.MessageSearchQuery) bool {
	return strings.TrimSpace(query.Query) != "" &&
		normalizedSearchSort(query.Sort) == maildb.MessageSearchSortRelevance
}

func normalizedSearchSort(sort string) string {
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "", maildb.MessageSearchSortDate:
		return maildb.MessageSearchSortDate
	case maildb.MessageSearchSortRelevance:
		return maildb.MessageSearchSortRelevance
	default:
		return ""
	}
}

func normalizeMessageSearchQuery(query maildb.MessageSearchQuery) maildb.MessageSearchQuery {
	query.UserID = strings.TrimSpace(query.UserID)
	query.Query = strings.TrimSpace(query.Query)
	query.FolderID = strings.TrimSpace(query.FolderID)
	query.From = strings.TrimSpace(query.From)
	query.Subject = strings.TrimSpace(query.Subject)
	if sort := normalizedSearchSort(query.Sort); sort != "" {
		query.Sort = sort
	} else {
		query.Sort = strings.TrimSpace(query.Sort)
	}
	return query
}

const maxSearchFilterBytes = 1024

func validateMessageSearchQuery(query maildb.MessageSearchQuery) error {
	if query.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if query.FolderID != "" {
		if err := validateServiceResourceID("folder_id", query.FolderID); err != nil {
			return err
		}
	}
	for field, value := range map[string]string{
		"q":       query.Query,
		"from":    query.From,
		"subject": query.Subject,
	} {
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%s must not contain CR or LF", field)
		}
		if len(value) > maxSearchFilterBytes {
			return fmt.Errorf("%s is too long", field)
		}
	}
	return nil
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
		_ = s.repository.SetMessageFlag(ctx, userID, messageID, "read", true)
	}
	if s.store == nil || detail.StoragePath == "" {
		return detail, nil
	}
	attachments, err := s.repository.ListAttachments(ctx, userID, messageID)
	if err != nil {
		return maildb.MessageDetail{}, err
	}
	detail.Attachments = attachments

	body, err := s.store.Get(ctx, detail.StoragePath)
	if err != nil {
		return maildb.MessageDetail{}, fmt.Errorf("open message body: %w", err)
	}
	defer body.Close()

	parsed, err := message.ParseEML(body)
	if err != nil {
		return maildb.MessageDetail{}, fmt.Errorf("parse message body: %w", err)
	}
	detail.TextBody = parsed.TextBody
	return detail, nil
}

func (s *Service) FetchIMAPMessage(ctx context.Context, req imapgw.FetchMessageRequest) (imapgw.Message, error) {
	repo, ok := s.repository.(interface {
		GetIMAPMessage(context.Context, string, string, imapgw.UID) (maildb.IMAPStoredMessage, error)
	})
	if !ok {
		return imapgw.Message{}, fmt.Errorf("imap message repository is required")
	}
	if s.store == nil {
		return imapgw.Message{}, fmt.Errorf("message storage is required")
	}

	userID := strings.TrimSpace(string(req.UserID))
	mailboxID := strings.TrimSpace(string(req.MailboxID))
	stored, err := repo.GetIMAPMessage(ctx, userID, mailboxID, req.UID)
	if err != nil {
		return imapgw.Message{}, err
	}
	if strings.TrimSpace(stored.StoragePath) == "" {
		return imapgw.Message{}, fmt.Errorf("imap message storage path is required")
	}

	body, err := s.store.Get(ctx, stored.StoragePath)
	if err != nil {
		return imapgw.Message{}, fmt.Errorf("open imap message body: %w", err)
	}
	return imapgw.Message{Summary: stored.Summary, Body: body}, nil
}

func (s *Service) ListIMAPMailboxes(ctx context.Context, req imapgw.ListMailboxesRequest) ([]imapgw.Mailbox, error) {
	repo, ok := s.repository.(interface {
		ListIMAPMailboxes(context.Context, string) ([]imapgw.Mailbox, error)
	})
	if !ok {
		return nil, fmt.Errorf("imap mailbox repository is required")
	}
	userID := strings.TrimSpace(string(req.UserID))
	return repo.ListIMAPMailboxes(ctx, userID)
}

func (s *Service) GetIMAPMailbox(ctx context.Context, userID imapgw.UserID, mailboxID imapgw.MailboxID) (imapgw.Mailbox, error) {
	repo, ok := s.repository.(interface {
		GetIMAPMailbox(context.Context, string, string) (imapgw.Mailbox, error)
	})
	if !ok {
		return imapgw.Mailbox{}, fmt.Errorf("imap mailbox repository is required")
	}
	return repo.GetIMAPMailbox(ctx, strings.TrimSpace(string(userID)), strings.TrimSpace(string(mailboxID)))
}

func (s *Service) ListIMAPMessages(ctx context.Context, req imapgw.ListMessagesRequest) ([]imapgw.MessageSummary, error) {
	repo, ok := s.repository.(interface {
		ListIMAPMessages(context.Context, string, string, int, imapgw.UID) ([]imapgw.MessageSummary, error)
	})
	if !ok {
		return nil, fmt.Errorf("imap message repository is required")
	}
	userID := strings.TrimSpace(string(req.UserID))
	mailboxID := strings.TrimSpace(string(req.MailboxID))
	limit := maildb.NormalizeMessageListLimit(req.Limit)
	return repo.ListIMAPMessages(ctx, userID, mailboxID, limit, req.AfterUID)
}

func (s *Service) SubscribeIMAPMailbox(ctx context.Context, userID imapgw.UserID, mailboxID imapgw.MailboxID) (<-chan imapgw.MailboxEvent, func(), error) {
	broker, ok := s.imapEvents.(interface {
		Subscribe(context.Context, imapgw.UserID, imapgw.MailboxID) (<-chan imapgw.MailboxEvent, func(), error)
	})
	if !ok {
		return nil, nil, fmt.Errorf("imap mailbox event broker is required")
	}
	return broker.Subscribe(ctx, imapgw.UserID(strings.TrimSpace(string(userID))), imapgw.MailboxID(strings.TrimSpace(string(mailboxID))))
}

func (s *Service) BackfillIMAPMailboxUIDs(ctx context.Context, userID string, mailboxID string, limit int) ([]maildb.IMAPMessageUID, error) {
	repo, ok := s.repository.(interface {
		BackfillIMAPMailboxUIDs(context.Context, string, string, int) ([]maildb.IMAPMessageUID, error)
	})
	if !ok {
		return nil, fmt.Errorf("imap uid backfill repository is required")
	}
	userID = strings.TrimSpace(userID)
	mailboxID = strings.TrimSpace(mailboxID)
	limit = maildb.NormalizeMessageListLimit(limit)
	return repo.BackfillIMAPMailboxUIDs(ctx, userID, mailboxID, limit)
}

func (s *Service) StoreIMAPFlags(ctx context.Context, req imapgw.StoreFlagsRequest) ([]imapgw.MessageSummary, error) {
	repo, ok := s.repository.(interface {
		StoreIMAPFlags(context.Context, string, string, []imapgw.UID, imapgw.MessageFlags, imapgw.StoreFlagsMode) ([]imapgw.MessageSummary, error)
	})
	if !ok {
		return nil, fmt.Errorf("imap flag repository is required")
	}
	userID := strings.TrimSpace(string(req.UserID))
	mailboxID := strings.TrimSpace(string(req.MailboxID))
	summaries, err := repo.StoreIMAPFlags(ctx, userID, mailboxID, req.UIDs, req.Flags, req.Mode)
	if err != nil {
		return nil, err
	}
	_ = s.publishIMAPSummaryEvents(ctx, imapgw.MailboxEventFlags, string(req.UserID), summaries)
	return summaries, nil
}

func (s *Service) publishIMAPSummaryEvents(ctx context.Context, eventType imapgw.MailboxEventType, userID string, summaries []imapgw.MessageSummary) error {
	if s.imapEvents == nil || len(summaries) == 0 {
		return nil
	}
	userID = strings.TrimSpace(userID)
	for _, summary := range summaries {
		if summary.MailboxID == "" {
			continue
		}
		if err := s.imapEvents.Publish(ctx, imapgw.MailboxEvent{
			Type:      eventType,
			UserID:    imapgw.UserID(userID),
			MailboxID: summary.MailboxID,
			UID:       summary.UID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) publishIMAPMessageUIDEvents(ctx context.Context, eventType imapgw.MailboxEventType, userID string, messageIDs []string) error {
	if s.imapEvents == nil || len(messageIDs) == 0 {
		return nil
	}
	uids, err := s.lookupExistingIMAPMessageUIDs(ctx, userID, messageIDs)
	if err != nil {
		return err
	}
	return s.publishIMAPUIDEvents(ctx, eventType, userID, uids)
}

func (s *Service) lookupExistingIMAPMessageUIDs(ctx context.Context, userID string, messageIDs []string) ([]maildb.IMAPMessageUID, error) {
	if s.imapEvents == nil || len(messageIDs) == 0 {
		return nil, nil
	}
	repo, ok := s.repository.(interface {
		ExistingIMAPMessageUIDs(context.Context, string, []string) ([]maildb.IMAPMessageUID, error)
	})
	if !ok {
		return nil, nil
	}
	return repo.ExistingIMAPMessageUIDs(ctx, userID, messageIDs)
}

func (s *Service) publishIMAPUIDEvents(ctx context.Context, eventType imapgw.MailboxEventType, userID string, uids []maildb.IMAPMessageUID) error {
	if s.imapEvents == nil || len(uids) == 0 {
		return nil
	}
	userID = strings.TrimSpace(userID)
	for _, uid := range uids {
		if err := s.imapEvents.Publish(ctx, imapgw.MailboxEvent{
			Type:      eventType,
			UserID:    imapgw.UserID(userID),
			MailboxID: uid.MailboxID,
			UID:       uid.UID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) MessageDeliveryStatus(ctx context.Context, userID string, messageID string) (maildb.MessageDeliveryStatusView, error) {
	repo, ok := s.repository.(DeliveryStatusRepository)
	if !ok {
		return maildb.MessageDeliveryStatusView{}, fmt.Errorf("delivery status repository is required")
	}
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	if err := validateServiceResourceID("message_id", messageID); err != nil {
		return maildb.MessageDeliveryStatusView{}, err
	}
	return repo.MessageDeliveryStatus(ctx, userID, messageID)
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
		return err
	}
	if err := s.repository.SetMessageFlag(ctx, userID, messageID, flag, value); err != nil {
		return err
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

func (s *Service) MoveMessage(ctx context.Context, userID string, messageID string, folderID string) error {
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	folderID = strings.TrimSpace(folderID)
	if err := validateServiceResourceID("message_id", messageID); err != nil {
		return err
	}
	if err := validateServiceResourceID("folder_id", folderID); err != nil {
		return err
	}
	uids, err := s.lookupExistingIMAPMessageUIDs(ctx, userID, []string{messageID})
	if err != nil {
		return err
	}
	if err := s.repository.MoveMessage(ctx, userID, messageID, folderID); err != nil {
		return err
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

func (s *Service) DeleteMessage(ctx context.Context, userID string, messageID string) error {
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	if err := validateServiceResourceID("message_id", messageID); err != nil {
		return err
	}
	uids, err := s.lookupExistingIMAPMessageUIDs(ctx, userID, []string{messageID})
	if err != nil {
		return err
	}
	if err := s.repository.DeleteMessage(ctx, userID, messageID); err != nil {
		return err
	}
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
	updated, err := s.repository.BulkDeleteMessages(ctx, req)
	if err != nil {
		return 0, err
	}
	_ = s.publishIMAPUIDEvents(ctx, imapgw.MailboxEventExpunge, req.UserID, uids)
	return updated, nil
}

func (s *Service) ListPushDevices(ctx context.Context, userID string, limit int) ([]maildb.PushDevice, error) {
	repo, ok := s.repository.(interface {
		ListPushDevices(context.Context, string, int) ([]maildb.PushDevice, error)
	})
	if !ok {
		return nil, fmt.Errorf("push device repository is required")
	}
	userID = strings.TrimSpace(userID)
	limit = maildb.NormalizeMessageListLimit(limit)
	return repo.ListPushDevices(ctx, userID, limit)
}

func (s *Service) UpsertPushDevice(ctx context.Context, req maildb.UpsertPushDeviceRequest) (maildb.PushDevice, error) {
	req.UserID = strings.TrimSpace(req.UserID)
	req.Platform = strings.ToLower(strings.TrimSpace(req.Platform))
	req.Token = strings.TrimSpace(req.Token)
	req.Label = strings.TrimSpace(req.Label)
	if err := maildb.ValidateUpsertPushDeviceRequest(req); err != nil {
		return maildb.PushDevice{}, err
	}
	repo, ok := s.repository.(interface {
		UpsertPushDevice(context.Context, maildb.UpsertPushDeviceRequest) (maildb.PushDevice, error)
	})
	if !ok {
		return maildb.PushDevice{}, fmt.Errorf("push device repository is required")
	}
	return repo.UpsertPushDevice(ctx, req)
}

func (s *Service) DeletePushDevice(ctx context.Context, userID string, id string) error {
	repo, ok := s.repository.(interface {
		DeletePushDevice(context.Context, string, string) error
	})
	if !ok {
		return fmt.Errorf("push device repository is required")
	}
	userID = strings.TrimSpace(userID)
	id = strings.TrimSpace(id)
	if err := validateServiceResourceID("device_id", id); err != nil {
		return err
	}
	return repo.DeletePushDevice(ctx, userID, id)
}

func (s *Service) SaveDraft(ctx context.Context, req SaveDraftRequest) (maildb.MessageDetail, error) {
	req = normalizeSaveDraftRequest(req)
	if err := ValidateSaveDraftRequest(req); err != nil {
		return maildb.MessageDetail{}, err
	}
	repo, ok := s.repository.(DraftRepository)
	if !ok {
		return maildb.MessageDetail{}, fmt.Errorf("draft repository is required")
	}
	return repo.SaveDraft(ctx, maildb.SaveDraftRequest{
		UserID:          req.UserID,
		DraftID:         req.DraftID,
		Intent:          string(req.Intent),
		SourceMessageID: req.SourceMessageID,
		From:            req.From,
		To:              req.To,
		Cc:              req.Cc,
		Bcc:             req.Bcc,
		Subject:         req.Subject,
		TextBody:        req.TextBody,
		AttachmentIDs:   req.AttachmentIDs,
	})
}

func normalizeSaveDraftRequest(req SaveDraftRequest) SaveDraftRequest {
	intent, err := NormalizeComposeIntent(string(req.Intent))
	if err == nil {
		req.Intent = intent
	}
	req.UserID = strings.TrimSpace(req.UserID)
	req.DraftID = strings.TrimSpace(req.DraftID)
	req.SourceMessageID = strings.TrimSpace(req.SourceMessageID)
	req.From = strings.TrimSpace(req.From)
	req.To = normalizeComposeAddresses(req.To)
	req.Cc = normalizeComposeAddresses(req.Cc)
	req.Bcc = normalizeComposeAddresses(req.Bcc)
	req.AttachmentIDs = normalizeStringList(req.AttachmentIDs)
	return req
}

func (s *Service) DeleteDraft(ctx context.Context, userID string, draftID string) error {
	userID = strings.TrimSpace(userID)
	draftID = strings.TrimSpace(draftID)
	if err := ValidateDeleteDraftRequest(userID, draftID); err != nil {
		return err
	}
	repo, ok := s.repository.(DraftRepository)
	if !ok {
		return fmt.Errorf("draft repository is required")
	}
	return repo.DeleteDraft(ctx, userID, draftID)
}

func (s *Service) ListAttachments(ctx context.Context, userID string, messageID string) ([]maildb.Attachment, error) {
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	if err := validateServiceResourceID("message_id", messageID); err != nil {
		return nil, err
	}
	return s.repository.ListAttachments(ctx, userID, messageID)
}

func (s *Service) CreateAttachmentUpload(ctx context.Context, req CreateAttachmentUploadRequest) (maildb.Attachment, error) {
	req = normalizeCreateAttachmentUploadRequest(req)
	if err := ValidateCreateAttachmentUploadRequest(req); err != nil {
		return maildb.Attachment{}, err
	}
	if err := s.enforceAttachmentPolicy(ctx, req.UserID, req.Size); err != nil {
		return maildb.Attachment{}, err
	}
	repo, ok := s.repository.(AttachmentUploadRepository)
	if !ok {
		return maildb.Attachment{}, fmt.Errorf("attachment upload repository is required")
	}
	return repo.CreateAttachmentUpload(ctx, maildb.CreateAttachmentUploadRequest{
		UserID:      req.UserID,
		DraftID:     req.DraftID,
		Filename:    req.Filename,
		Size:        req.Size,
		MIMEType:    req.MIMEType,
		StoragePath: req.StoragePath,
	})
}

func (s *Service) UploadAttachment(ctx context.Context, req UploadAttachmentRequest) (maildb.Attachment, error) {
	req = normalizeUploadAttachmentRequest(req)
	if err := ValidateUploadAttachmentRequest(req); err != nil {
		return maildb.Attachment{}, err
	}
	if err := s.enforceAttachmentPolicy(ctx, req.UserID, req.Size); err != nil {
		return maildb.Attachment{}, err
	}
	if s.store == nil {
		return maildb.Attachment{}, fmt.Errorf("mail storage is required")
	}
	repo, ok := s.repository.(AttachmentUploadRepository)
	if !ok {
		return maildb.Attachment{}, fmt.Errorf("attachment upload repository is required")
	}

	path := strings.Join([]string{
		"uploads",
		safeObjectPathSegment(req.UserID),
		randomObjectID(),
		safeObjectFilename(req.Filename),
	}, "/")
	counter := &countingReader{reader: req.Body}
	limitedBody := &io.LimitedReader{R: counter, N: MaxAttachmentUploadBytes + 1}
	if err := s.store.Put(ctx, path, limitedBody); err != nil {
		return maildb.Attachment{}, fmt.Errorf("store attachment upload: %w", err)
	}
	if limitedBody.N == 0 {
		_ = s.store.Delete(ctx, path)
		return maildb.Attachment{}, fmt.Errorf("attachment body exceeds %d bytes", MaxAttachmentUploadBytes)
	}
	if counter.n != req.Size {
		_ = s.store.Delete(ctx, path)
		return maildb.Attachment{}, fmt.Errorf("attachment body size %d does not match declared size %d", counter.n, req.Size)
	}

	attachment, err := repo.CreateAttachmentUpload(ctx, maildb.CreateAttachmentUploadRequest{
		UserID:      req.UserID,
		DraftID:     req.DraftID,
		Filename:    req.Filename,
		Size:        req.Size,
		MIMEType:    req.MIMEType,
		StoragePath: path,
	})
	if err != nil {
		_ = s.store.Delete(ctx, path)
		return maildb.Attachment{}, err
	}
	return attachment, nil
}

func normalizeCreateAttachmentUploadRequest(req CreateAttachmentUploadRequest) CreateAttachmentUploadRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.DraftID = strings.TrimSpace(req.DraftID)
	req.Filename = strings.TrimSpace(req.Filename)
	req.MIMEType = strings.TrimSpace(req.MIMEType)
	req.StoragePath = strings.TrimSpace(req.StoragePath)
	return req
}

func normalizeUploadAttachmentRequest(req UploadAttachmentRequest) UploadAttachmentRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.DraftID = strings.TrimSpace(req.DraftID)
	req.Filename = strings.TrimSpace(req.Filename)
	req.MIMEType = strings.TrimSpace(req.MIMEType)
	return req
}

type countingReader struct {
	reader io.Reader
	n      int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.n += int64(n)
	return n, err
}

func safeObjectFilename(filename string) string {
	filename = strings.ReplaceAll(strings.TrimSpace(filename), "/", "_")
	filename = strings.ReplaceAll(filename, `\`, "_")
	if filename == "" {
		return "attachment"
	}
	return filename
}

func safeObjectPathSegment(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	b.Grow(len(value))
	lastUnderscore := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_.")
	if out == "" {
		return "unknown"
	}
	return out
}

type AttachmentDownload struct {
	Attachment maildb.Attachment
	Body       io.ReadCloser
}

func (s *Service) OpenAttachment(ctx context.Context, userID string, messageID string, attachmentID string) (AttachmentDownload, error) {
	if s.store == nil {
		return AttachmentDownload{}, fmt.Errorf("mail storage is required")
	}
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	attachmentID = strings.TrimSpace(attachmentID)
	if err := validateServiceResourceID("message_id", messageID); err != nil {
		return AttachmentDownload{}, err
	}
	if err := validateServiceResourceID("attachment_id", attachmentID); err != nil {
		return AttachmentDownload{}, err
	}
	attachment, err := s.repository.GetAttachment(ctx, userID, messageID, attachmentID)
	if err != nil {
		return AttachmentDownload{}, err
	}
	body, err := s.store.Get(ctx, attachment.StoragePath)
	if err != nil {
		return AttachmentDownload{}, fmt.Errorf("open attachment body: %w", err)
	}
	return AttachmentDownload{Attachment: attachment, Body: body}, nil
}

func (s *Service) ExpireStaleAttachmentUploads(ctx context.Context, before time.Time, limit int) ([]maildb.Attachment, error) {
	repo, ok := s.repository.(AttachmentCleanupRepository)
	if !ok {
		return nil, fmt.Errorf("attachment cleanup repository is required")
	}
	req := maildb.ExpireStaleAttachmentUploadsRequest{
		Before: before,
		Limit:  limit,
	}
	if err := maildb.ValidateExpireStaleAttachmentUploadsRequest(req); err != nil {
		return nil, err
	}
	expired, err := repo.ExpireStaleAttachmentUploads(ctx, req)
	if err != nil {
		return nil, err
	}
	if s.store == nil {
		return expired, nil
	}
	var deleteErrors []error
	for _, attachment := range expired {
		if strings.TrimSpace(attachment.StoragePath) != "" {
			if err := s.store.Delete(ctx, attachment.StoragePath); err != nil && !errors.Is(err, os.ErrNotExist) {
				deleteErrors = append(deleteErrors, fmt.Errorf("%s: %w", attachment.ID, err))
			}
		}
	}
	if len(deleteErrors) > 0 {
		return expired, fmt.Errorf("delete expired attachment objects: %w", errors.Join(deleteErrors...))
	}
	return expired, nil
}

func (s *Service) CountStaleAttachmentUploads(ctx context.Context, before time.Time, limit int) (maildb.StaleAttachmentUploadCount, error) {
	repo, ok := s.repository.(AttachmentCleanupRepository)
	if !ok {
		return maildb.StaleAttachmentUploadCount{}, fmt.Errorf("attachment cleanup repository is required")
	}
	req := maildb.ExpireStaleAttachmentUploadsRequest{
		Before: before,
		Limit:  limit,
	}
	if err := maildb.ValidateExpireStaleAttachmentUploadsRequest(req); err != nil {
		return maildb.StaleAttachmentUploadCount{}, err
	}
	return repo.CountStaleAttachmentUploads(ctx, req)
}

type SendTextRequest struct {
	UserID          string             `json:"user_id"`
	Intent          ComposeIntent      `json:"intent"`
	SourceMessageID string             `json:"source_message_id"`
	From            string             `json:"from"`
	To              []outbound.Address `json:"to"`
	Cc              []outbound.Address `json:"cc"`
	Bcc             []outbound.Address `json:"bcc"`
	Subject         string             `json:"subject"`
	TextBody        string             `json:"text_body"`
	AttachmentIDs   []string           `json:"attachment_ids,omitempty"`
	Transactional   bool               `json:"transactional"`
	ScheduledAt     time.Time          `json:"scheduled_at"`
}

type SendTextResult struct {
	ID             string        `json:"id"`
	RFCMessageID   string        `json:"message_id"`
	Farm           outbound.Farm `json:"farm"`
	SendStatus     string        `json:"send_status"`
	DeliveryStatus string        `json:"delivery_status"`
	BounceStatus   string        `json:"bounce_status"`
}

func NormalizeSendTextResult(result SendTextResult) SendTextResult {
	if strings.TrimSpace(result.SendStatus) == "" {
		result.SendStatus = "queued"
	}
	if strings.TrimSpace(result.DeliveryStatus) == "" {
		result.DeliveryStatus = "pending"
	}
	if strings.TrimSpace(result.BounceStatus) == "" {
		result.BounceStatus = "none"
	}
	return result
}

func (s *Service) SendDraft(ctx context.Context, userID string, draftID string) (SendTextResult, error) {
	userID = strings.TrimSpace(userID)
	draftID = strings.TrimSpace(draftID)
	if userID == "" {
		return SendTextResult{}, fmt.Errorf("user_id is required")
	}
	if err := validateServiceResourceID("draft_id", draftID); err != nil {
		return SendTextResult{}, err
	}
	repo, ok := s.repository.(DraftSendRepository)
	if !ok {
		return SendTextResult{}, fmt.Errorf("draft send repository is required")
	}
	draft, err := repo.GetDraftForSend(ctx, userID, draftID)
	if err != nil {
		return SendTextResult{}, err
	}
	result, err := s.SendText(ctx, SendTextRequest{
		UserID:          userID,
		Intent:          ComposeIntent(draft.Intent),
		SourceMessageID: draft.SourceMessageID,
		From:            draft.From,
		To:              draft.To,
		Cc:              draft.Cc,
		Bcc:             draft.Bcc,
		Subject:         draft.Subject,
		TextBody:        draft.TextBody,
		AttachmentIDs:   draft.AttachmentIDs,
	})
	if err != nil {
		return SendTextResult{}, err
	}
	if err := repo.MarkDraftSent(ctx, userID, draftID, result.ID); err != nil {
		return SendTextResult{}, err
	}
	return result, nil
}

func (s *Service) SendText(ctx context.Context, req SendTextRequest) (SendTextResult, error) {
	if s.repository == nil {
		return SendTextResult{}, fmt.Errorf("mail repository is required")
	}
	if s.store == nil {
		return SendTextResult{}, fmt.Errorf("mail storage is required")
	}
	req = normalizeSendTextRequest(req)
	if err := ValidateSendTextRequest(req); err != nil {
		return SendTextResult{}, err
	}

	sender, err := s.repository.SenderForUser(ctx, req.UserID, req.From)
	if err != nil {
		return SendTextResult{}, err
	}

	recipients := recipientEmails(req)
	suppressed, err := s.repository.SuppressedRecipients(ctx, sender.DomainID, recipients)
	if err != nil {
		return SendTextResult{}, err
	}
	if len(suppressed) > 0 {
		return SendTextResult{}, fmt.Errorf("suppressed recipients: %s", strings.Join(suppressed, ", "))
	}
	policy, err := s.domainPolicy(ctx, sender.DomainID)
	if err != nil {
		return SendTextResult{}, err
	}
	if err := enforceOutboundRecipientPolicy(req, policy); err != nil {
		return SendTextResult{}, err
	}
	sourceThread, err := s.sourceThread(ctx, req)
	if err != nil {
		return SendTextResult{}, err
	}

	from := outbound.Address{Name: sender.DisplayName, Email: sender.Address}
	composed, err := outbound.ComposeText(outbound.TextMessage{
		From:       from,
		To:         req.To,
		Cc:         req.Cc,
		Bcc:        req.Bcc,
		Subject:    req.Subject,
		TextBody:   req.TextBody,
		InReplyTo:  sourceThread.MessageID,
		References: sourceThread.References(),
	})
	if err != nil {
		return SendTextResult{}, err
	}
	if err := enforceOutboundSizePolicy(composed.Size, policy); err != nil {
		return SendTextResult{}, err
	}

	now := time.Now().UTC()
	objectID := randomObjectID()
	path := strings.Join([]string{
		"mailstore",
		sender.CompanyID,
		sender.DomainID,
		sender.UserID,
		"maildir",
		now.Format("2006"),
		now.Format("01"),
		objectID + ".eml",
	}, "/")

	if err := s.store.Put(ctx, path, bytes.NewReader(composed.Raw)); err != nil {
		return SendTextResult{}, fmt.Errorf("store outgoing message: %w", err)
	}

	farm := outbound.Classify(outbound.ClassificationInput{
		Transactional:  req.Transactional,
		RecipientCount: len(req.To) + len(req.Cc) + len(req.Bcc),
		ScheduledAt:    req.ScheduledAt,
	})
	id, err := s.repository.RecordOutgoing(ctx, maildb.OutgoingMessage{
		CompanyID:       sender.CompanyID,
		DomainID:        sender.DomainID,
		UserID:          sender.UserID,
		ComposeIntent:   string(req.Intent),
		SourceMessageID: req.SourceMessageID,
		RFCMessageID:    composed.MessageID,
		Subject:         req.Subject,
		From:            from,
		To:              req.To,
		Cc:              req.Cc,
		Bcc:             req.Bcc,
		SentAt:          now,
		Size:            composed.Size,
		HasAttachment:   len(req.AttachmentIDs) > 0,
		StoragePath:     path,
		Farm:            farm,
	})
	if err != nil {
		_ = s.store.Delete(ctx, path)
		return SendTextResult{}, err
	}
	if err := s.markSourceMessageAfterSend(ctx, req); err != nil {
		return SendTextResult{}, err
	}

	return NormalizeSendTextResult(SendTextResult{
		ID:             id,
		RFCMessageID:   composed.MessageID,
		Farm:           farm,
		SendStatus:     "queued",
		DeliveryStatus: "pending",
		BounceStatus:   "none",
	}), nil
}

func normalizeSendTextRequest(req SendTextRequest) SendTextRequest {
	intent, err := NormalizeComposeIntent(string(req.Intent))
	if err == nil {
		req.Intent = intent
	}
	req.UserID = strings.TrimSpace(req.UserID)
	req.SourceMessageID = strings.TrimSpace(req.SourceMessageID)
	req.From = strings.TrimSpace(req.From)
	req.To = normalizeComposeAddresses(req.To)
	req.Cc = normalizeComposeAddresses(req.Cc)
	req.Bcc = normalizeComposeAddresses(req.Bcc)
	req.AttachmentIDs = normalizeStringList(req.AttachmentIDs)
	return req
}

func normalizeComposeAddresses(addresses []outbound.Address) []outbound.Address {
	for i := range addresses {
		addresses[i].Name = strings.TrimSpace(addresses[i].Name)
		addresses[i].Email = strings.TrimSpace(addresses[i].Email)
	}
	return addresses
}

func normalizeStringList(values []string) []string {
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
	}
	return values
}

const maxServiceResourceIDBytes = 200

func validateServiceResourceID(field string, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%s is required", field)
	}
	if strings.ContainsAny(id, "\r\n") {
		return fmt.Errorf("%s must not contain CR or LF", field)
	}
	if len(id) > maxServiceResourceIDBytes {
		return fmt.Errorf("%s is too long", field)
	}
	return nil
}

func normalizeBulkMessageFlagRequest(req maildb.BulkMessageFlagRequest) maildb.BulkMessageFlagRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.MessageIDs = normalizeStringList(req.MessageIDs)
	req.Flag = strings.TrimSpace(req.Flag)
	return req
}

func normalizeBulkMessageMoveRequest(req maildb.BulkMessageMoveRequest) maildb.BulkMessageMoveRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.MessageIDs = normalizeStringList(req.MessageIDs)
	req.FolderID = strings.TrimSpace(req.FolderID)
	return req
}

func normalizeBulkMessageDeleteRequest(req maildb.BulkMessageDeleteRequest) maildb.BulkMessageDeleteRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.MessageIDs = normalizeStringList(req.MessageIDs)
	return req
}

func (s *Service) sourceThread(ctx context.Context, req SendTextRequest) (maildb.SourceThreadView, error) {
	req = normalizeSendTextRequest(req)
	intent, err := NormalizeComposeIntent(string(req.Intent))
	if err != nil {
		return maildb.SourceThreadView{}, err
	}
	if intent != ComposeIntentReply || req.SourceMessageID == "" {
		return maildb.SourceThreadView{}, nil
	}
	repo, ok := s.repository.(SourceThreadRepository)
	if !ok {
		return maildb.SourceThreadView{}, fmt.Errorf("source thread repository is required")
	}
	return repo.SourceThread(ctx, req.UserID, req.SourceMessageID)
}

func (s *Service) domainPolicy(ctx context.Context, domainID string) (maildb.DomainPolicyView, error) {
	domainID = strings.TrimSpace(domainID)
	repo, ok := s.repository.(DomainPolicyRepository)
	if !ok {
		return maildb.DomainPolicyView{DomainID: domainID, InboundMode: "inherit", OutboundMode: "inherit"}, nil
	}
	return repo.DomainPolicy(ctx, domainID)
}

func (s *Service) enforceAttachmentPolicy(ctx context.Context, userID string, size int64) error {
	repo, ok := s.repository.(UserDomainPolicyRepository)
	if !ok {
		return nil
	}
	userID = strings.TrimSpace(userID)
	policy, err := repo.DomainPolicyForUser(ctx, userID)
	if err != nil {
		return err
	}
	return enforceAttachmentSizePolicy(size, policy)
}

func enforceOutboundRecipientPolicy(req SendTextRequest, policy maildb.DomainPolicyView) error {
	if policy.OutboundMode != "enforce" || policy.MaxRecipientsPerMessage <= 0 {
		return nil
	}
	recipientCount := len(recipientEmails(req))
	if recipientCount > policy.MaxRecipientsPerMessage {
		return fmt.Errorf("domain outbound policy max_recipients_per_message exceeded: %d > %d", recipientCount, policy.MaxRecipientsPerMessage)
	}
	return nil
}

func enforceOutboundSizePolicy(size int64, policy maildb.DomainPolicyView) error {
	if policy.OutboundMode != "enforce" || policy.MaxMessageBytes <= 0 {
		return nil
	}
	if size > policy.MaxMessageBytes {
		return fmt.Errorf("domain outbound policy max_message_bytes exceeded: %d > %d", size, policy.MaxMessageBytes)
	}
	return nil
}

func enforceAttachmentSizePolicy(size int64, policy maildb.DomainPolicyView) error {
	if policy.OutboundMode != "enforce" || policy.MaxAttachmentBytes <= 0 {
		return nil
	}
	if size > policy.MaxAttachmentBytes {
		return fmt.Errorf("domain outbound policy max_attachment_bytes exceeded: %d > %d", size, policy.MaxAttachmentBytes)
	}
	return nil
}

func (s *Service) markSourceMessageAfterSend(ctx context.Context, req SendTextRequest) error {
	intent, err := NormalizeComposeIntent(string(req.Intent))
	if err != nil {
		return err
	}
	switch intent {
	case ComposeIntentReply:
		return s.repository.SetMessageFlag(ctx, req.UserID, req.SourceMessageID, "answered", true)
	case ComposeIntentForward:
		return s.repository.SetMessageFlag(ctx, req.UserID, req.SourceMessageID, "forwarded", true)
	default:
		return nil
	}
}

func randomObjectID() string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixMilli(), hex.EncodeToString(random[:]))
}

func recipientEmails(req SendTextRequest) []string {
	recipients := make([]string, 0, len(req.To)+len(req.Cc)+len(req.Bcc))
	seen := make(map[string]struct{}, len(req.To)+len(req.Cc)+len(req.Bcc))
	appendRecipient := func(email string) {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			return
		}
		if _, ok := seen[email]; ok {
			return
		}
		seen[email] = struct{}{}
		recipients = append(recipients, email)
	}
	for _, addr := range req.To {
		appendRecipient(addr.Email)
	}
	for _, addr := range req.Cc {
		appendRecipient(addr.Email)
	}
	for _, addr := range req.Bcc {
		appendRecipient(addr.Email)
	}
	return recipients
}
