package mailservice

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/gogomail/gogomail/internal/mail"
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
	ListMessagesPage(ctx context.Context, userID string, folderID string, limit int, cursor maildb.MessageListCursor, filter maildb.MessageListFilter) ([]maildb.MessageSummary, error)
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
	CancelAttachmentUpload(ctx context.Context, userID string, attachmentID string) (maildb.Attachment, error)
}

type AttachmentUploadSessionRepository interface {
	CreateAttachmentUploadSession(ctx context.Context, req maildb.CreateAttachmentUploadSessionRequest) (maildb.AttachmentUploadSession, error)
	CancelAttachmentUploadSession(ctx context.Context, req maildb.CancelAttachmentUploadSessionRequest) (maildb.AttachmentUploadSession, error)
	GetAttachmentUploadSession(ctx context.Context, req maildb.GetAttachmentUploadSessionRequest) (maildb.AttachmentUploadSession, error)
	StoreAttachmentUploadSessionBody(ctx context.Context, req maildb.StoreAttachmentUploadSessionBodyRequest) (maildb.AttachmentUploadSession, error)
	FinalizeAttachmentUploadSession(ctx context.Context, req maildb.FinalizeAttachmentUploadSessionRequest) (maildb.Attachment, error)
	ExpireAttachmentUploadSessions(ctx context.Context, req maildb.ExpireAttachmentUploadSessionsRequest) ([]maildb.AttachmentUploadSession, error)
}

type AttachmentUploadSessionCleanupRepository interface {
	CountStaleAttachmentUploadSessions(ctx context.Context, req maildb.ExpireAttachmentUploadSessionsRequest) (maildb.StaleAttachmentUploadSessionCount, error)
	ListStaleAttachmentUploadSessions(ctx context.Context, req maildb.ExpireAttachmentUploadSessionsRequest) ([]maildb.StaleAttachmentUploadSessionCandidate, error)
}

type AttachmentCleanupRepository interface {
	ExpireStaleAttachmentUploads(ctx context.Context, req maildb.ExpireStaleAttachmentUploadsRequest) ([]maildb.Attachment, error)
	CountStaleAttachmentUploads(ctx context.Context, req maildb.ExpireStaleAttachmentUploadsRequest) (maildb.StaleAttachmentUploadCount, error)
	ListStaleAttachmentUploads(ctx context.Context, req maildb.ExpireStaleAttachmentUploadsRequest) ([]maildb.StaleAttachmentUploadCandidate, error)
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
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return nil, err
	}
	return s.repository.ListFolders(ctx, userID)
}

func (s *Service) CreateFolder(ctx context.Context, req maildb.CreateFolderRequest) (maildb.Folder, error) {
	req.UserID = strings.TrimSpace(req.UserID)
	req.Name = strings.TrimSpace(req.Name)
	if err := validateServiceResourceID("user_id", req.UserID); err != nil {
		return maildb.Folder{}, err
	}
	if err := validateServiceResourceID("folder_name", req.Name); err != nil {
		return maildb.Folder{}, err
	}
	return s.repository.CreateFolder(ctx, req)
}

func (s *Service) RenameFolder(ctx context.Context, userID string, folderID string, name string) (maildb.Folder, error) {
	userID = strings.TrimSpace(userID)
	folderID = strings.TrimSpace(folderID)
	name = strings.TrimSpace(name)
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return maildb.Folder{}, err
	}
	if err := validateServiceResourceID("folder_id", folderID); err != nil {
		return maildb.Folder{}, err
	}
	if err := validateServiceResourceID("folder_name", name); err != nil {
		return maildb.Folder{}, err
	}
	return s.repository.RenameFolder(ctx, userID, folderID, name)
}

func (s *Service) DeleteFolder(ctx context.Context, userID string, folderID string) error {
	userID = strings.TrimSpace(userID)
	folderID = strings.TrimSpace(folderID)
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return err
	}
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

func (s *Service) ListThreads(ctx context.Context, userID string, limit int) ([]maildb.ThreadSummary, error) {
	return s.ListThreadsPage(ctx, userID, limit, maildb.ThreadListCursor{}, maildb.ThreadListFilter{})
}

func (s *Service) ListThreadsPage(ctx context.Context, userID string, limit int, cursor maildb.ThreadListCursor, filter maildb.ThreadListFilter) ([]maildb.ThreadSummary, error) {
	repo, ok := s.repository.(interface {
		ListThreadsPage(context.Context, string, int, maildb.ThreadListCursor, maildb.ThreadListFilter) ([]maildb.ThreadSummary, error)
	})
	if !ok {
		return nil, fmt.Errorf("thread repository is required")
	}
	userID = strings.TrimSpace(userID)
	filter.FolderID = strings.TrimSpace(filter.FolderID)
	if filter.FolderID != "" {
		if err := validateServiceResourceID("folder_id", filter.FolderID); err != nil {
			return nil, err
		}
	}
	sortMode, ok := maildb.NormalizeListSort(filter.Sort)
	if !ok {
		return nil, fmt.Errorf("sort must be newest or oldest")
	}
	filter.Sort = sortMode
	limit = maildb.NormalizeMessageListLimit(limit)
	return repo.ListThreadsPage(ctx, userID, limit, cursor, filter)
}

func (s *Service) ListThreadMessages(ctx context.Context, userID string, threadID string, limit int) ([]maildb.MessageSummary, error) {
	return s.ListThreadMessagesPage(ctx, userID, threadID, limit, maildb.MessageListCursor{})
}

func (s *Service) ListThreadMessagesPage(ctx context.Context, userID string, threadID string, limit int, cursor maildb.MessageListCursor) ([]maildb.MessageSummary, error) {
	repo, ok := s.repository.(interface {
		ListThreadMessagesPage(context.Context, string, string, int, maildb.MessageListCursor) ([]maildb.MessageSummary, error)
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
	return repo.ListThreadMessagesPage(ctx, userID, threadID, limit, cursor)
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

func (s *Service) SearchDrafts(ctx context.Context, query maildb.DraftSearchQuery) ([]maildb.MessageDetail, error) {
	query = normalizeDraftSearchQuery(query)
	if err := validateDraftSearchQuery(query); err != nil {
		return nil, err
	}
	repo, ok := s.repository.(interface {
		SearchDrafts(context.Context, maildb.DraftSearchQuery) ([]maildb.MessageDetail, error)
	})
	if !ok {
		return nil, fmt.Errorf("draft search repository is required")
	}
	return repo.SearchDrafts(ctx, query)
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

func normalizeDraftSearchQuery(query maildb.DraftSearchQuery) maildb.DraftSearchQuery {
	query.UserID = strings.TrimSpace(query.UserID)
	query.Query = strings.TrimSpace(query.Query)
	query.From = strings.TrimSpace(query.From)
	query.Subject = strings.TrimSpace(query.Subject)
	query.Limit = maildb.NormalizeMessageListLimit(query.Limit)
	return query
}

func validateDraftSearchQuery(query maildb.DraftSearchQuery) error {
	if query.UserID == "" {
		return fmt.Errorf("user_id is required")
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
		if err := s.repository.SetMessageFlag(ctx, userID, messageID, "read", true); err == nil {
			_ = s.publishIMAPMessageUIDEvents(ctx, imapgw.MailboxEventFlags, userID, []string{messageID})
		}
	}
	if s.store == nil || detail.StoragePath == "" {
		return detail, nil
	}
	attachments, err := s.repository.ListAttachments(ctx, userID, messageID)
	if err != nil {
		return maildb.MessageDetail{}, err
	}
	detail.Attachments = attachments

	storagePath, err := requireStoredObjectPath("message body", detail.StoragePath)
	if err != nil {
		return maildb.MessageDetail{}, err
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
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return imapgw.Message{}, err
	}
	if err := validateServiceResourceID("mailbox_id", mailboxID); err != nil {
		return imapgw.Message{}, err
	}
	if req.UID == 0 {
		return imapgw.Message{}, fmt.Errorf("uid must be positive")
	}
	stored, err := repo.GetIMAPMessage(ctx, userID, mailboxID, req.UID)
	if err != nil {
		return imapgw.Message{}, err
	}
	storagePath, err := requireStoredObjectPath("imap message body", stored.StoragePath)
	if err != nil {
		return imapgw.Message{}, err
	}

	body, err := s.store.Get(ctx, storagePath)
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
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return nil, err
	}
	return repo.ListIMAPMailboxes(ctx, userID)
}

func (s *Service) ListSubscribedIMAPMailboxes(ctx context.Context, req imapgw.ListMailboxesRequest) ([]imapgw.MailboxSubscription, error) {
	repo, ok := s.repository.(interface {
		ListSubscribedIMAPMailboxes(context.Context, string) ([]imapgw.MailboxSubscription, error)
	})
	if !ok {
		return nil, fmt.Errorf("imap mailbox subscription repository is required")
	}
	userID := strings.TrimSpace(string(req.UserID))
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return nil, err
	}
	return repo.ListSubscribedIMAPMailboxes(ctx, userID)
}

func (s *Service) GetIMAPMailbox(ctx context.Context, userID imapgw.UserID, mailboxID imapgw.MailboxID) (imapgw.Mailbox, error) {
	repo, ok := s.repository.(interface {
		GetIMAPMailbox(context.Context, string, string) (imapgw.Mailbox, error)
	})
	if !ok {
		return imapgw.Mailbox{}, fmt.Errorf("imap mailbox repository is required")
	}
	user := strings.TrimSpace(string(userID))
	mailbox := strings.TrimSpace(string(mailboxID))
	if err := validateServiceResourceID("user_id", user); err != nil {
		return imapgw.Mailbox{}, err
	}
	if err := validateServiceResourceID("mailbox_id", mailbox); err != nil {
		return imapgw.Mailbox{}, err
	}
	return repo.GetIMAPMailbox(ctx, user, mailbox)
}

func (s *Service) SubscribeIMAPMailboxName(ctx context.Context, userID imapgw.UserID, mailboxID imapgw.MailboxID) (imapgw.MailboxSubscription, error) {
	repo, ok := s.repository.(interface {
		SubscribeIMAPMailbox(context.Context, string, string) (imapgw.MailboxSubscription, error)
	})
	if !ok {
		return imapgw.MailboxSubscription{}, fmt.Errorf("imap mailbox subscription repository is required")
	}
	user := strings.TrimSpace(string(userID))
	mailbox := strings.TrimSpace(string(mailboxID))
	if err := validateServiceResourceID("user_id", user); err != nil {
		return imapgw.MailboxSubscription{}, err
	}
	if err := validateServiceResourceID("mailbox_id", mailbox); err != nil {
		return imapgw.MailboxSubscription{}, err
	}
	return repo.SubscribeIMAPMailbox(ctx, user, mailbox)
}

func (s *Service) UnsubscribeIMAPMailboxName(ctx context.Context, userID imapgw.UserID, mailboxID imapgw.MailboxID) error {
	repo, ok := s.repository.(interface {
		UnsubscribeIMAPMailbox(context.Context, string, string) error
	})
	if !ok {
		return fmt.Errorf("imap mailbox subscription repository is required")
	}
	user := strings.TrimSpace(string(userID))
	mailbox := strings.TrimSpace(string(mailboxID))
	if err := validateServiceResourceID("user_id", user); err != nil {
		return err
	}
	if err := validateServiceResourceID("mailbox_id", mailbox); err != nil {
		return err
	}
	return repo.UnsubscribeIMAPMailbox(ctx, user, mailbox)
}

func (s *Service) CreateIMAPMailbox(ctx context.Context, userID imapgw.UserID, mailboxID imapgw.MailboxID) (imapgw.Mailbox, error) {
	user := strings.TrimSpace(string(userID))
	name := strings.Trim(strings.TrimSpace(string(mailboxID)), "/")
	folder, err := s.CreateFolder(ctx, maildb.CreateFolderRequest{
		UserID: user,
		Name:   name,
	})
	if err != nil {
		return imapgw.Mailbox{}, err
	}
	return s.GetIMAPMailbox(ctx, imapgw.UserID(user), imapgw.MailboxID(folder.ID))
}

func (s *Service) DeleteIMAPMailbox(ctx context.Context, userID imapgw.UserID, mailboxID imapgw.MailboxID) error {
	user := strings.TrimSpace(string(userID))
	mailbox, err := s.GetIMAPMailbox(ctx, imapgw.UserID(user), mailboxID)
	if err != nil {
		return err
	}
	return s.DeleteFolder(ctx, user, string(mailbox.ID))
}

func (s *Service) RenameIMAPMailbox(ctx context.Context, userID imapgw.UserID, mailboxID imapgw.MailboxID, newMailboxID imapgw.MailboxID) (imapgw.Mailbox, error) {
	user := strings.TrimSpace(string(userID))
	mailbox, err := s.GetIMAPMailbox(ctx, imapgw.UserID(user), mailboxID)
	if err != nil {
		return imapgw.Mailbox{}, err
	}
	name := strings.Trim(strings.TrimSpace(string(newMailboxID)), "/")
	folder, err := s.RenameFolder(ctx, user, string(mailbox.ID), name)
	if err != nil {
		return imapgw.Mailbox{}, err
	}
	return s.GetIMAPMailbox(ctx, imapgw.UserID(user), imapgw.MailboxID(folder.ID))
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
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return nil, err
	}
	if err := validateServiceResourceID("mailbox_id", mailboxID); err != nil {
		return nil, err
	}
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
	user := strings.TrimSpace(string(userID))
	mailbox := strings.TrimSpace(string(mailboxID))
	if err := validateServiceResourceID("user_id", user); err != nil {
		return nil, nil, err
	}
	if err := validateServiceResourceID("mailbox_id", mailbox); err != nil {
		return nil, nil, err
	}
	return broker.Subscribe(ctx, imapgw.UserID(user), imapgw.MailboxID(mailbox))
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
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return nil, err
	}
	if err := validateServiceResourceID("mailbox_id", mailboxID); err != nil {
		return nil, err
	}
	limit = maildb.NormalizeMessageListLimit(limit)
	return repo.BackfillIMAPMailboxUIDs(ctx, userID, mailboxID, limit)
}

func (s *Service) StoreIMAPFlags(ctx context.Context, req imapgw.StoreFlagsRequest) ([]imapgw.MessageSummary, error) {
	repo, ok := s.repository.(interface {
		StoreIMAPFlags(context.Context, string, string, []imapgw.UID, imapgw.MessageFlags, imapgw.StoreFlagsMode, uint64) ([]imapgw.MessageSummary, error)
	})
	if !ok {
		return nil, fmt.Errorf("imap flag repository is required")
	}
	userID := strings.TrimSpace(string(req.UserID))
	mailboxID := strings.TrimSpace(string(req.MailboxID))
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return nil, err
	}
	if err := validateServiceResourceID("mailbox_id", mailboxID); err != nil {
		return nil, err
	}
	if err := validateNonEmptyIMAPUIDs(req.UIDs); err != nil {
		return nil, err
	}
	summaries, err := repo.StoreIMAPFlags(ctx, userID, mailboxID, req.UIDs, req.Flags, req.Mode, req.UnchangedSince)
	if err != nil {
		var modified *imapgw.StoreModifiedError
		if errors.As(err, &modified) {
			_ = s.publishIMAPSummaryEvents(ctx, imapgw.MailboxEventFlags, userID, imapSuccessfulStoreSummaries(summaries, modified))
			return summaries, err
		}
		return nil, err
	}
	_ = s.publishIMAPSummaryEvents(ctx, imapgw.MailboxEventFlags, userID, summaries)
	return summaries, nil
}

func imapSuccessfulStoreSummaries(summaries []imapgw.MessageSummary, modified *imapgw.StoreModifiedError) []imapgw.MessageSummary {
	if modified == nil {
		return summaries
	}
	source := modified.Summaries
	if len(source) == 0 {
		source = summaries
	}
	if len(source) == 0 || len(modified.UIDs) == 0 {
		return source
	}
	modifiedUIDs := make(map[imapgw.UID]struct{}, len(modified.UIDs))
	for _, uid := range modified.UIDs {
		modifiedUIDs[uid] = struct{}{}
	}
	successful := make([]imapgw.MessageSummary, 0, len(source))
	for _, summary := range source {
		if _, stale := modifiedUIDs[summary.UID]; stale {
			continue
		}
		successful = append(successful, summary)
	}
	return successful
}

func (s *Service) AppendIMAPMessage(ctx context.Context, req imapgw.AppendMessageRequest) (imapgw.AppendMessageResult, error) {
	repo, ok := s.repository.(interface {
		ResolveIMAPAppendTarget(context.Context, string, string) (maildb.IMAPAppendTarget, error)
		AppendStoredIMAPMessage(context.Context, maildb.AppendStoredIMAPMessageRequest) (imapgw.AppendMessageResult, error)
	})
	if !ok {
		return imapgw.AppendMessageResult{}, imapgw.ErrUnsupportedAppend
	}
	if s.store == nil {
		return imapgw.AppendMessageResult{}, fmt.Errorf("imap append storage is required")
	}
	req.UserID = imapgw.UserID(strings.TrimSpace(string(req.UserID)))
	req.MailboxID = imapgw.MailboxID(strings.TrimSpace(string(req.MailboxID)))
	if err := validateServiceResourceID("user_id", string(req.UserID)); err != nil {
		return imapgw.AppendMessageResult{}, err
	}
	if err := validateServiceResourceID("mailbox_id", string(req.MailboxID)); err != nil {
		return imapgw.AppendMessageResult{}, err
	}
	if req.Body == nil {
		return imapgw.AppendMessageResult{}, fmt.Errorf("append body is required")
	}
	if req.Size < 0 {
		return imapgw.AppendMessageResult{}, fmt.Errorf("append size must not be negative")
	}
	internalDate := req.InternalDate
	if internalDate.IsZero() {
		internalDate = time.Now().UTC()
	}
	target, err := repo.ResolveIMAPAppendTarget(ctx, string(req.UserID), string(req.MailboxID))
	if err != nil {
		return imapgw.AppendMessageResult{}, err
	}

	spooled, copied, err := spoolIMAPAppendBody(req.Body, req.Size)
	if err != nil {
		return imapgw.AppendMessageResult{}, err
	}
	defer os.Remove(spooled.Name())
	defer spooled.Close()
	if copied != req.Size {
		return imapgw.AppendMessageResult{}, fmt.Errorf("append literal size mismatch: got %d bytes, want %d", copied, req.Size)
	}
	if _, err := spooled.Seek(0, io.SeekStart); err != nil {
		return imapgw.AppendMessageResult{}, fmt.Errorf("rewind imap append body for parse: %w", err)
	}
	parsed, err := message.ParseEML(spooled)
	if err != nil {
		return imapgw.AppendMessageResult{}, fmt.Errorf("parse imap append message: %w", err)
	}
	path := buildIMAPAppendStoragePath(target, randomObjectID(), internalDate)
	if _, err := spooled.Seek(0, io.SeekStart); err != nil {
		return imapgw.AppendMessageResult{}, fmt.Errorf("rewind imap append body for store: %w", err)
	}
	if err := s.store.Put(ctx, path, spooled); err != nil {
		return imapgw.AppendMessageResult{}, fmt.Errorf("store imap append message: %w", err)
	}
	result, err := repo.AppendStoredIMAPMessage(ctx, maildb.AppendStoredIMAPMessageRequest{
		Target:       target,
		StoragePath:  path,
		Parsed:       parsed,
		Flags:        req.Flags,
		InternalDate: internalDate,
		Size:         copied,
	})
	if err != nil {
		_ = s.store.Delete(context.Background(), path)
		if errors.Is(err, mail.ErrMailboxFull) {
			return imapgw.AppendMessageResult{}, imapgw.ErrOverQuota
		}
		return imapgw.AppendMessageResult{}, err
	}
	_ = s.publishIMAPSummaryEvents(ctx, imapgw.MailboxEventExists, string(req.UserID), []imapgw.MessageSummary{result.Summary})
	return result, nil
}

func spoolIMAPAppendBody(body io.Reader, expectedSize int64) (*os.File, int64, error) {
	spooled, err := os.CreateTemp("", "gogomail-imap-append-*.eml")
	if err != nil {
		return nil, 0, fmt.Errorf("create imap append spool: %w", err)
	}
	copied, copyErr := io.Copy(spooled, io.LimitReader(body, expectedSize+1))
	if copyErr != nil {
		spooled.Close()
		os.Remove(spooled.Name())
		return nil, 0, fmt.Errorf("spool imap append body: %w", copyErr)
	}
	return spooled, copied, nil
}

func buildIMAPAppendStoragePath(target maildb.IMAPAppendTarget, objectID string, internalDate time.Time) string {
	return strings.Join([]string{
		"mailstore",
		sanitizeStoragePathSegment(target.CompanyID),
		sanitizeStoragePathSegment(target.DomainID),
		sanitizeStoragePathSegment(target.UserID),
		"imap-append",
		internalDate.Format("2006"),
		internalDate.Format("01"),
		sanitizeStoragePathSegment(objectID) + ".eml",
	}, "/")
}

func sanitizeStoragePathSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "_"
	}
	var b strings.Builder
	b.Grow(len(value))
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		return "_"
	}
	return out
}

func (s *Service) CopyIMAPMessages(ctx context.Context, req imapgw.CopyMessagesRequest) ([]imapgw.MessageSummary, error) {
	repo, ok := s.repository.(interface {
		CopyIMAPMessages(context.Context, string, string, string, []imapgw.UID) ([]imapgw.MessageSummary, error)
	})
	if !ok {
		return nil, fmt.Errorf("imap copy repository is required")
	}
	userID := strings.TrimSpace(string(req.UserID))
	sourceMailboxID := strings.TrimSpace(string(req.SourceMailboxID))
	destMailboxID := strings.TrimSpace(string(req.DestMailboxID))
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return nil, err
	}
	if err := validateServiceResourceID("source_mailbox_id", sourceMailboxID); err != nil {
		return nil, err
	}
	if err := validateServiceResourceID("dest_mailbox_id", destMailboxID); err != nil {
		return nil, err
	}
	if err := validateNonEmptyIMAPUIDs(req.UIDs); err != nil {
		return nil, err
	}
	summaries, err := repo.CopyIMAPMessages(ctx, userID, sourceMailboxID, destMailboxID, req.UIDs)
	if err != nil {
		return nil, err
	}
	_ = s.publishIMAPSummaryEvents(ctx, imapgw.MailboxEventExists, userID, summaries)
	return summaries, nil
}

func (s *Service) MoveIMAPMessages(ctx context.Context, req imapgw.MoveMessagesRequest) ([]imapgw.MoveMessageResult, error) {
	repo, ok := s.repository.(interface {
		MoveIMAPMessages(context.Context, string, string, string, []imapgw.UID) ([]imapgw.MoveMessageResult, error)
	})
	if !ok {
		return nil, fmt.Errorf("imap move repository is required")
	}
	userID := strings.TrimSpace(string(req.UserID))
	sourceMailboxID := strings.TrimSpace(string(req.SourceMailboxID))
	destMailboxID := strings.TrimSpace(string(req.DestMailboxID))
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return nil, err
	}
	if err := validateServiceResourceID("source_mailbox_id", sourceMailboxID); err != nil {
		return nil, err
	}
	if err := validateServiceResourceID("dest_mailbox_id", destMailboxID); err != nil {
		return nil, err
	}
	if err := validateNonEmptyIMAPUIDs(req.UIDs); err != nil {
		return nil, err
	}
	results, err := repo.MoveIMAPMessages(ctx, userID, sourceMailboxID, destMailboxID, req.UIDs)
	if err != nil {
		return nil, err
	}
	_ = s.publishIMAPSummaryEvents(ctx, imapgw.MailboxEventExpunge, userID, imapMoveSourceSummaries(results))
	return results, nil
}

func (s *Service) ExpungeIMAPMessages(ctx context.Context, req imapgw.ExpungeRequest) ([]imapgw.MessageSummary, error) {
	repo, ok := s.repository.(interface {
		ExpungeIMAPMessages(context.Context, string, string, []imapgw.UID) ([]imapgw.MessageSummary, error)
	})
	if !ok {
		return nil, fmt.Errorf("imap expunge repository is required")
	}
	userID := strings.TrimSpace(string(req.UserID))
	mailboxID := strings.TrimSpace(string(req.MailboxID))
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return nil, err
	}
	if err := validateServiceResourceID("mailbox_id", mailboxID); err != nil {
		return nil, err
	}
	if err := validateIMAPUIDs(req.UIDs); err != nil {
		return nil, err
	}
	summaries, err := repo.ExpungeIMAPMessages(ctx, userID, mailboxID, req.UIDs)
	if err != nil {
		return nil, err
	}
	_ = s.publishIMAPSummaryEvents(ctx, imapgw.MailboxEventExpunge, userID, summaries)
	return summaries, nil
}

func validateIMAPUIDs(uids []imapgw.UID) error {
	for _, uid := range uids {
		if uid == 0 {
			return fmt.Errorf("uids must contain only positive UIDs")
		}
	}
	return nil
}

func validateNonEmptyIMAPUIDs(uids []imapgw.UID) error {
	if len(uids) == 0 {
		return fmt.Errorf("uids are required")
	}
	return validateIMAPUIDs(uids)
}

func imapMoveSourceSummaries(results []imapgw.MoveMessageResult) []imapgw.MessageSummary {
	summaries := make([]imapgw.MessageSummary, 0, len(results))
	for _, result := range results {
		summaries = append(summaries, result.Source)
	}
	return summaries
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
			Type:           eventType,
			UserID:         imapgw.UserID(userID),
			MailboxID:      summary.MailboxID,
			UID:            summary.UID,
			SequenceNumber: imapEventSequenceNumber(eventType, summary),
			Messages:       imapEventMessageCount(eventType, summary),
		}); err != nil {
			return err
		}
	}
	return nil
}

func imapEventMessageCount(eventType imapgw.MailboxEventType, summary imapgw.MessageSummary) uint32 {
	if eventType == imapgw.MailboxEventExists {
		return summary.SequenceNumber
	}
	return 0
}

func imapEventSequenceNumber(eventType imapgw.MailboxEventType, summary imapgw.MessageSummary) uint32 {
	if eventType == imapgw.MailboxEventExpunge {
		return summary.SequenceNumber
	}
	return 0
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
			Type:           eventType,
			UserID:         imapgw.UserID(userID),
			MailboxID:      uid.MailboxID,
			UID:            uid.UID,
			SequenceNumber: imapUIDEventSequenceNumber(eventType, uid),
		}); err != nil {
			return err
		}
	}
	return nil
}

func imapUIDEventSequenceNumber(eventType imapgw.MailboxEventType, uid maildb.IMAPMessageUID) uint32 {
	if eventType == imapgw.MailboxEventExpunge {
		return uid.SequenceNumber
	}
	return 0
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

func (s *Service) CancelAttachmentUpload(ctx context.Context, userID string, attachmentID string) (maildb.Attachment, error) {
	userID = strings.TrimSpace(userID)
	attachmentID = strings.TrimSpace(attachmentID)
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return maildb.Attachment{}, err
	}
	if err := validateServiceResourceID("attachment_id", attachmentID); err != nil {
		return maildb.Attachment{}, err
	}
	repo, ok := s.repository.(AttachmentUploadRepository)
	if !ok {
		return maildb.Attachment{}, fmt.Errorf("attachment upload repository is required")
	}
	attachment, err := repo.CancelAttachmentUpload(ctx, userID, attachmentID)
	if err != nil {
		return maildb.Attachment{}, err
	}
	if s.store != nil && strings.TrimSpace(attachment.StoragePath) != "" {
		storagePath, err := requireStoredObjectPath("attachment body", attachment.StoragePath)
		if err != nil {
			return attachment, err
		}
		if err := s.store.Delete(ctx, storagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return attachment, fmt.Errorf("delete canceled attachment object: %w", err)
		}
	}
	return attachment, nil
}

func (s *Service) CreateAttachmentUploadSession(ctx context.Context, req CreateAttachmentUploadSessionRequest) (maildb.AttachmentUploadSession, error) {
	req = normalizeCreateAttachmentUploadSessionRequest(req)
	if err := ValidateCreateAttachmentUploadSessionRequest(req); err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	now := time.Now().UTC()
	if !req.ExpiresAt.After(now) {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("expires_at must be in the future")
	}
	if req.ExpiresAt.After(now.Add(MaxAttachmentUploadSessionTTL)) {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("expires_at must be within %s", MaxAttachmentUploadSessionTTL)
	}
	if err := s.enforceAttachmentPolicy(ctx, req.UserID, req.DeclaredSize); err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	repo, ok := s.repository.(AttachmentUploadSessionRepository)
	if !ok {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("attachment upload session repository is required")
	}
	return repo.CreateAttachmentUploadSession(ctx, maildb.CreateAttachmentUploadSessionRequest{
		UserID:       req.UserID,
		DraftID:      req.DraftID,
		Filename:     req.Filename,
		DeclaredSize: req.DeclaredSize,
		MIMEType:     req.MIMEType,
		ExpiresAt:    req.ExpiresAt,
	})
}

func (s *Service) CancelAttachmentUploadSession(ctx context.Context, userID string, sessionID string) (maildb.AttachmentUploadSession, error) {
	userID = strings.TrimSpace(userID)
	sessionID = strings.TrimSpace(sessionID)
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	if err := validateServiceResourceID("session_id", sessionID); err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	repo, ok := s.repository.(AttachmentUploadSessionRepository)
	if !ok {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("attachment upload session repository is required")
	}
	session, err := repo.CancelAttachmentUploadSession(ctx, maildb.CancelAttachmentUploadSessionRequest{
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	if s.store != nil && strings.TrimSpace(session.StoragePath) != "" {
		storagePath, err := validateUploadSessionObjectPath(session.StoragePath)
		if err != nil {
			return session, err
		}
		if err := s.store.Delete(ctx, storagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return session, fmt.Errorf("delete canceled upload session object: %w", err)
		}
	}
	return session, nil
}

func (s *Service) GetAttachmentUploadSession(ctx context.Context, userID string, sessionID string) (maildb.AttachmentUploadSession, error) {
	userID = strings.TrimSpace(userID)
	sessionID = strings.TrimSpace(sessionID)
	if err := maildb.ValidateGetAttachmentUploadSessionRequest(maildb.GetAttachmentUploadSessionRequest{
		UserID:    userID,
		SessionID: sessionID,
	}); err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	repo, ok := s.repository.(AttachmentUploadSessionRepository)
	if !ok {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("attachment upload session repository is required")
	}
	return repo.GetAttachmentUploadSession(ctx, maildb.GetAttachmentUploadSessionRequest{
		UserID:    userID,
		SessionID: sessionID,
	})
}

func (s *Service) StoreAttachmentUploadSessionBody(ctx context.Context, req StoreAttachmentUploadSessionBodyRequest) (maildb.AttachmentUploadSession, error) {
	req = normalizeStoreAttachmentUploadSessionBodyRequest(req)
	if err := ValidateStoreAttachmentUploadSessionBodyRequest(req); err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	if s.store == nil {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("mail storage is required")
	}
	repo, ok := s.repository.(AttachmentUploadSessionRepository)
	if !ok {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("attachment upload session repository is required")
	}
	session, err := repo.GetAttachmentUploadSession(ctx, maildb.GetAttachmentUploadSessionRequest{
		UserID:    req.UserID,
		SessionID: req.SessionID,
	})
	if err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	if session.Status != "pending" && session.Status != "uploading" && session.Status != "failed" {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("attachment upload session %q is not writable", req.SessionID)
	}
	if !session.ExpiresAt.After(time.Now().UTC()) {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("attachment upload session %q is expired", req.SessionID)
	}

	path := strings.Join([]string{
		"upload-sessions",
		safeObjectPathSegment(req.UserID),
		safeObjectPathSegment(req.SessionID),
		"bodies",
		randomObjectID(),
	}, "/")
	counter := &countingReader{reader: req.Body}
	limitedBody := &io.LimitedReader{R: counter, N: session.DeclaredSize + 1}
	hash := sha256.New()
	if err := s.store.Put(ctx, path, io.TeeReader(limitedBody, hash)); err != nil {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("store attachment upload session body: %w", err)
	}
	if limitedBody.N == 0 {
		_ = s.store.Delete(ctx, path)
		return maildb.AttachmentUploadSession{}, fmt.Errorf("attachment upload session body exceeds declared size %d", session.DeclaredSize)
	}
	if counter.n != session.DeclaredSize {
		_ = s.store.Delete(ctx, path)
		return maildb.AttachmentUploadSession{}, fmt.Errorf("attachment upload session body size %d does not match declared size %d", counter.n, session.DeclaredSize)
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	if req.ExpectedChecksumSHA256 != "" && checksum != req.ExpectedChecksumSHA256 {
		_ = s.store.Delete(ctx, path)
		return maildb.AttachmentUploadSession{}, fmt.Errorf("attachment upload session checksum %s does not match expected %s", checksum, req.ExpectedChecksumSHA256)
	}
	stored, err := repo.StoreAttachmentUploadSessionBody(ctx, maildb.StoreAttachmentUploadSessionBodyRequest{
		UserID:         req.UserID,
		SessionID:      req.SessionID,
		ReceivedSize:   counter.n,
		StoragePath:    path,
		ChecksumSHA256: checksum,
	})
	if err != nil {
		_ = s.store.Delete(ctx, path)
		return maildb.AttachmentUploadSession{}, err
	}
	if previousPath := strings.TrimSpace(session.StoragePath); previousPath != "" && previousPath != path {
		if previousPath, err := validateUploadSessionObjectPath(previousPath); err == nil {
			_ = s.store.Delete(ctx, previousPath)
		}
	}
	return stored, nil
}

func (s *Service) FinalizeAttachmentUploadSession(ctx context.Context, userID string, sessionID string) (maildb.Attachment, error) {
	userID = strings.TrimSpace(userID)
	sessionID = strings.TrimSpace(sessionID)
	if err := maildb.ValidateFinalizeAttachmentUploadSessionRequest(maildb.FinalizeAttachmentUploadSessionRequest{
		UserID:    userID,
		SessionID: sessionID,
	}); err != nil {
		return maildb.Attachment{}, err
	}
	repo, ok := s.repository.(AttachmentUploadSessionRepository)
	if !ok {
		return maildb.Attachment{}, fmt.Errorf("attachment upload session repository is required")
	}
	if s.store == nil {
		return maildb.Attachment{}, fmt.Errorf("mail storage is required")
	}
	session, err := repo.GetAttachmentUploadSession(ctx, maildb.GetAttachmentUploadSessionRequest{
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		return maildb.Attachment{}, err
	}
	if err := s.verifyUploadSessionBody(ctx, session); err != nil {
		return maildb.Attachment{}, err
	}
	return repo.FinalizeAttachmentUploadSession(ctx, maildb.FinalizeAttachmentUploadSessionRequest{
		UserID:    userID,
		SessionID: sessionID,
	})
}

func (s *Service) verifyUploadSessionBody(ctx context.Context, session maildb.AttachmentUploadSession) error {
	if session.Status != "uploading" {
		return fmt.Errorf("attachment upload session %q is not ready for finalization", session.ID)
	}
	if session.ReceivedSize != session.DeclaredSize || session.DeclaredSize < 0 {
		return fmt.Errorf("attachment upload session %q has incomplete body", session.ID)
	}
	if strings.TrimSpace(session.StoragePath) == "" {
		return fmt.Errorf("attachment upload session %q storage path is required", session.ID)
	}
	storagePath, err := validateUploadSessionObjectPath(session.StoragePath)
	if err != nil {
		return err
	}
	if !isLowerSHA256Hex(strings.TrimSpace(session.ChecksumSHA256)) {
		return fmt.Errorf("attachment upload session %q checksum is required", session.ID)
	}
	if !session.ExpiresAt.After(time.Now().UTC()) {
		return fmt.Errorf("attachment upload session %q is expired", session.ID)
	}
	body, err := s.store.Get(ctx, storagePath)
	if err != nil {
		return fmt.Errorf("open attachment upload session body: %w", err)
	}
	defer body.Close()

	counter := &countingReader{reader: body}
	limitedBody := &io.LimitedReader{R: counter, N: session.DeclaredSize + 1}
	hash := sha256.New()
	if _, err := io.Copy(hash, limitedBody); err != nil {
		return fmt.Errorf("read attachment upload session body: %w", err)
	}
	if limitedBody.N == 0 || counter.n != session.DeclaredSize {
		return fmt.Errorf("attachment upload session body size %d does not match declared size %d", counter.n, session.DeclaredSize)
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	if checksum != session.ChecksumSHA256 {
		return fmt.Errorf("attachment upload session checksum %s does not match stored %s", checksum, session.ChecksumSHA256)
	}
	return nil
}

func (s *Service) ExpireAttachmentUploadSessions(ctx context.Context, before time.Time, limit int) ([]maildb.AttachmentUploadSession, error) {
	repo, ok := s.repository.(AttachmentUploadSessionRepository)
	if !ok {
		return nil, fmt.Errorf("attachment upload session repository is required")
	}
	req := maildb.ExpireAttachmentUploadSessionsRequest{
		Before: before,
		Limit:  limit,
	}
	if err := maildb.ValidateExpireAttachmentUploadSessionsRequest(req); err != nil {
		return nil, err
	}
	expired, err := repo.ExpireAttachmentUploadSessions(ctx, req)
	if err != nil {
		return nil, err
	}
	if s.store != nil {
		for _, session := range expired {
			if strings.TrimSpace(session.StoragePath) == "" {
				continue
			}
			storagePath, err := validateUploadSessionObjectPath(session.StoragePath)
			if err != nil {
				return expired, err
			}
			if err := s.store.Delete(ctx, storagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
				return expired, fmt.Errorf("delete expired upload session object: %w", err)
			}
		}
	}
	return expired, nil
}

func (s *Service) CountStaleAttachmentUploadSessions(ctx context.Context, before time.Time, limit int) (maildb.StaleAttachmentUploadSessionCount, error) {
	repo, ok := s.repository.(AttachmentUploadSessionCleanupRepository)
	if !ok {
		return maildb.StaleAttachmentUploadSessionCount{}, fmt.Errorf("attachment upload session repository is required")
	}
	req := maildb.ExpireAttachmentUploadSessionsRequest{
		Before: before,
		Limit:  limit,
	}
	if err := maildb.ValidateExpireAttachmentUploadSessionsRequest(req); err != nil {
		return maildb.StaleAttachmentUploadSessionCount{}, err
	}
	return repo.CountStaleAttachmentUploadSessions(ctx, req)
}

func (s *Service) ListStaleAttachmentUploadSessions(ctx context.Context, before time.Time, limit int) ([]maildb.StaleAttachmentUploadSessionCandidate, error) {
	repo, ok := s.repository.(AttachmentUploadSessionCleanupRepository)
	if !ok {
		return nil, fmt.Errorf("attachment upload session repository is required")
	}
	req := maildb.ExpireAttachmentUploadSessionsRequest{
		Before: before,
		Limit:  limit,
	}
	if err := maildb.ValidateExpireAttachmentUploadSessionsRequest(req); err != nil {
		return nil, err
	}
	return repo.ListStaleAttachmentUploadSessions(ctx, req)
}

func normalizeCreateAttachmentUploadRequest(req CreateAttachmentUploadRequest) CreateAttachmentUploadRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.DraftID = strings.TrimSpace(req.DraftID)
	req.Filename = strings.TrimSpace(req.Filename)
	req.MIMEType = strings.TrimSpace(req.MIMEType)
	req.StoragePath = strings.TrimSpace(req.StoragePath)
	return req
}

func normalizeCreateAttachmentUploadSessionRequest(req CreateAttachmentUploadSessionRequest) CreateAttachmentUploadSessionRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.DraftID = strings.TrimSpace(req.DraftID)
	req.Filename = strings.TrimSpace(req.Filename)
	req.MIMEType = strings.TrimSpace(req.MIMEType)
	return req
}

func normalizeStoreAttachmentUploadSessionBodyRequest(req StoreAttachmentUploadSessionBodyRequest) StoreAttachmentUploadSessionBodyRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.ExpectedChecksumSHA256 = strings.TrimSpace(req.ExpectedChecksumSHA256)
	return req
}

func validateUploadSessionObjectPath(storagePath string) (string, error) {
	storagePath = strings.TrimSpace(storagePath)
	if err := validateAttachmentStoragePath(storagePath); err != nil {
		return "", err
	}
	if storagePath == "" {
		return "", fmt.Errorf("storage_path is required")
	}
	if !strings.HasPrefix(storagePath, "upload-sessions/") {
		return "", fmt.Errorf("storage_path must use upload-sessions prefix")
	}
	return storagePath, nil
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
	storagePath, err := requireStoredObjectPath("attachment body", attachment.StoragePath)
	if err != nil {
		return AttachmentDownload{}, err
	}
	body, err := s.store.Get(ctx, storagePath)
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
			storagePath, err := requireStoredObjectPath("attachment body", attachment.StoragePath)
			if err != nil {
				deleteErrors = append(deleteErrors, fmt.Errorf("%s: %w", attachment.ID, err))
				continue
			}
			if err := s.store.Delete(ctx, storagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
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

func (s *Service) ListStaleAttachmentUploads(ctx context.Context, before time.Time, limit int) ([]maildb.StaleAttachmentUploadCandidate, error) {
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
	return repo.ListStaleAttachmentUploads(ctx, req)
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
