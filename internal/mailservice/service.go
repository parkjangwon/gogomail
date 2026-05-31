package mailservice

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/gogomail/gogomail/internal/maildb"
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
	BulkSetThreadFlag(ctx context.Context, req maildb.BulkThreadFlagRequest) (maildb.BulkThreadFlagResult, error)
	MoveMessage(ctx context.Context, userID string, messageID string, folderID string) error
	BulkMoveMessages(ctx context.Context, req maildb.BulkMessageMoveRequest) (int64, error)
	BulkMoveThreads(ctx context.Context, req maildb.BulkThreadMoveRequest) (maildb.BulkThreadMoveResult, error)
	DeleteMessage(ctx context.Context, userID string, messageID string) error
	BulkDeleteMessages(ctx context.Context, req maildb.BulkMessageDeleteRequest) (int64, error)
	BulkDeleteThreads(ctx context.Context, req maildb.BulkThreadDeleteRequest) (maildb.BulkThreadDeleteResult, error)
	RestoreMessage(ctx context.Context, userID string, messageID string) error
	BulkRestoreMessages(ctx context.Context, req maildb.BulkMessageRestoreRequest) (maildb.BulkMessageRestoreResult, error)
	BulkRestoreThreads(ctx context.Context, req maildb.BulkThreadRestoreRequest) (maildb.BulkThreadRestoreResult, error)
	EnsureIMAPMessageUIDsForMessages(ctx context.Context, userID string, messageIDs []string) ([]maildb.IMAPMessageUID, error)
	ListPushDevices(ctx context.Context, userID string, limit int) ([]maildb.PushDevice, error)
	UpsertPushDevice(ctx context.Context, req maildb.UpsertPushDeviceRequest) (maildb.PushDevice, error)
	DeletePushDevice(ctx context.Context, userID string, id string) error
	ListAttachments(ctx context.Context, userID string, messageID string) ([]maildb.Attachment, error)
	GetAttachment(ctx context.Context, userID string, messageID string, attachmentID string) (maildb.Attachment, error)
	AttachmentsByIDs(ctx context.Context, userID string, attachmentIDs []string) ([]maildb.Attachment, error)
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

// AtomicDraftSendRepository is an optional extension of DraftSendRepository
// whose RecordOutgoingFromDraft combines message insertion, outbox insertion,
// and draft marking into a single transaction. When the repository implements
// this interface, SendDraft uses it to eliminate the crash window between
// RecordOutgoing and MarkDraftSent.
type AtomicDraftSendRepository interface {
	RecordOutgoingFromDraft(ctx context.Context, msg maildb.OutgoingMessage, draftID string) (string, error)
}

type RecipientGroupRepository interface {
	ExpandOrgRecipients(ctx context.Context, userID string, orgID string, includeChildren bool) ([]outbound.Address, error)
	ExpandAddressBookRecipients(ctx context.Context, userID string, addressBookID string) ([]outbound.Address, error)
}

type AttachmentUploadRepository interface {
	CreateAttachmentUpload(ctx context.Context, req maildb.CreateAttachmentUploadRequest) (maildb.Attachment, error)
	CancelAttachmentUpload(ctx context.Context, userID string, attachmentID string) (maildb.Attachment, error)
}

type UserStorageScopeRepository interface {
	UserStorageScope(ctx context.Context, userID string) (maildb.UserStorageScope, error)
}

type AttachmentUploadSessionRepository interface {
	CreateAttachmentUploadSession(ctx context.Context, req maildb.CreateAttachmentUploadSessionRequest) (maildb.AttachmentUploadSession, error)
	CancelAttachmentUploadSession(ctx context.Context, req maildb.CancelAttachmentUploadSessionRequest) (maildb.AttachmentUploadSession, error)
	GetAttachmentUploadSession(ctx context.Context, req maildb.GetAttachmentUploadSessionRequest) (maildb.AttachmentUploadSession, error)
	StoreAttachmentUploadSessionBody(ctx context.Context, req maildb.StoreAttachmentUploadSessionBodyRequest) (maildb.AttachmentUploadSession, error)
	StoreAttachmentUploadSessionChunk(ctx context.Context, req maildb.StoreAttachmentUploadSessionChunkRequest) (maildb.AttachmentUploadSession, error)
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

type PreferencesRepository interface {
	GetWebmailPreferences(ctx context.Context, userID string) (json.RawMessage, error)
	SetWebmailPreferences(ctx context.Context, userID string, prefs json.RawMessage) error
}

type MeRepository interface {
	GetUserProfile(ctx context.Context, userID string) (maildb.UserProfile, error)
	GetUserProfileByEmail(ctx context.Context, email string) (maildb.UserProfile, error)
	ListUserAddresses(ctx context.Context, userID string) ([]maildb.UserAddress, error)
	UpdateUserDisplayName(ctx context.Context, userID, displayName string) error
	UpdateOwnRecoveryEmail(ctx context.Context, userID, recoveryEmail string) error
	UpdateUserAvatarURL(ctx context.Context, userID, avatarURL string) error
	ChangeUserPassword(ctx context.Context, userID, currentPassword, newPassword string) error
}

// TrackingRepository is the minimal interface required for open-tracking.
type TrackingRepository interface {
	CreateTrackingPixels(ctx context.Context, pixels []maildb.TrackingPixel) error
}

// StoragePathRepository is an optional repository extension that lets the
// service look up which EML storage paths are safe to delete before removing
// the database records.
type StoragePathRepository interface {
	LookupDeleteableStoragePaths(ctx context.Context, userID string, messageIDs []string) ([]string, error)
}

type Service struct {
	repository        Repository
	store             storage.Store
	searchIDSource    SearchIDSource
	imapEvents        IMAPMailboxEventPublisher
	quotaAlertEmitter maildb.QuotaWarningEmitterInterface
	trackingRepo      TrackingRepository
	publicBaseURL     string
	bodyCache         *messageBodyCache
	logger            *slog.Logger
}

func New(repository Repository, store storage.Store) *Service {
	return &Service{repository: repository, store: store, bodyCache: newMessageBodyCache(defaultMessageBodyCacheEntries, defaultMessageBodyCacheTTL)}
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

func (s *Service) WithLogger(logger *slog.Logger) *Service {
	if s == nil {
		return nil
	}
	s.logger = logger
	return s
}

func (s *Service) loggerOrDefault() *slog.Logger {
	if s != nil && s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

func (s *Service) WithMessageBodyCache(capacity int, ttl time.Duration) *Service {
	s.bodyCache = newMessageBodyCache(capacity, ttl)
	return s
}

func (s *Service) MessageBodyCacheSnapshot() MessageBodyCacheSnapshot {
	return s.bodyCache.snapshot()
}

func (s *Service) WithQuotaAlertEmitter(emitter maildb.QuotaWarningEmitterInterface) *Service {
	s.quotaAlertEmitter = emitter
	return s
}

// WithTrackingRepo enables open-tracking pixel injection on outgoing mail.
// publicBaseURL is the externally reachable base URL of this server
// (e.g. "https://mail.example.com"). If empty, open-tracking pixels are skipped.
func (s *Service) WithTrackingRepo(repo TrackingRepository, publicBaseURL string) *Service {
	s.trackingRepo = repo
	s.publicBaseURL = strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	return s
}

// lookupGCStoragePaths returns EML storage paths that are safe to delete for
// the given message IDs.  It must be called BEFORE the database records are
// removed so the reference-count check in LookupDeleteableStoragePaths is
// accurate.  Returns nil (not an error) when the repository does not support
// the StoragePathRepository interface.
func (s *Service) lookupGCStoragePaths(ctx context.Context, userID string, messageIDs []string) []string {
	if s.store == nil || len(messageIDs) == 0 {
		return nil
	}
	repo, ok := s.repository.(StoragePathRepository)
	if !ok {
		return nil
	}
	paths, err := repo.LookupDeleteableStoragePaths(ctx, userID, messageIDs)
	if err != nil {
		s.loggerOrDefault().Warn("failed to look up message storage paths for gc", "user_id", userID, "error", err)
		return nil
	}
	return paths
}

// deleteStorageObjects performs a best-effort deletion of EML objects from the
// backing store.  It should be called AFTER a successful database deletion.
// Failures are logged but never returned as errors.
func (s *Service) deleteStorageObjects(ctx context.Context, paths []string) {
	for _, p := range paths {
		if err := s.store.Delete(ctx, p); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.loggerOrDefault().Warn("failed to delete message eml object", "path", p, "error", err)
		}
	}
}

func (s *Service) deleteMessageObjectBestEffort(ctx context.Context, storagePath string, operation string, attrs ...any) {
	if s == nil || s.store == nil || strings.TrimSpace(storagePath) == "" {
		return
	}
	if err := s.store.Delete(ctx, storagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		args := []any{"operation", operation, "storage_path", storagePath, "error", err}
		args = append(args, attrs...)
		s.loggerOrDefault().Warn("failed to delete message storage object", args...)
	}
}

func (s *Service) emitQuotaWarningIfNeeded(ctx context.Context, userID string) {
	if s.quotaAlertEmitter == nil {
		return
	}
	if err := s.quotaAlertEmitter.EmitIfNeeded(ctx, userID); err != nil {
		slog.Warn("failed to emit quota warning", "user_id", userID, "error", err)
	}
}
