package mailservice

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/gogomail/gogomail/internal/mail"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/message"
)

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
	mailboxID := string(req.MailboxID)
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
	mailbox := string(mailboxID)
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
	mailbox := string(mailboxID)
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
	mailbox := string(mailboxID)
	if err := validateServiceResourceID("user_id", user); err != nil {
		return fmt.Errorf("unsubscribe IMAP mailbox: %w", err)
	}
	if err := validateServiceResourceID("mailbox_id", mailbox); err != nil {
		return fmt.Errorf("unsubscribe IMAP mailbox: %w", err)
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
		return fmt.Errorf("delete IMAP mailbox: %w", err)
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
	mailboxID := string(req.MailboxID)
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
	mailbox := string(mailboxID)
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
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return nil, err
	}
	if err := validateServiceResourceID("mailbox_id", mailboxID); err != nil {
		return nil, err
	}
	limit = maildb.NormalizeMessageListLimit(limit)
	return repo.BackfillIMAPMailboxUIDs(ctx, userID, mailboxID, limit)
}

func (s *Service) LookupIMAPMessageUIDs(ctx context.Context, userID, mailboxID string, messageIDs []string) (map[string]uint32, error) {
	repo, ok := s.repository.(interface {
		LookupIMAPMessageUIDs(context.Context, string, string, []string) (map[string]uint32, error)
	})
	if !ok {
		return nil, fmt.Errorf("imap uid lookup repository is required")
	}
	return repo.LookupIMAPMessageUIDs(ctx, userID, mailboxID, messageIDs)
}

func (s *Service) StoreIMAPFlags(ctx context.Context, req imapgw.StoreFlagsRequest) ([]imapgw.MessageSummary, error) {
	repo, ok := s.repository.(interface {
		StoreIMAPFlags(context.Context, string, string, []imapgw.UID, imapgw.MessageFlags, imapgw.StoreFlagsMode, uint64, bool) ([]imapgw.MessageSummary, error)
	})
	if !ok {
		return nil, fmt.Errorf("imap flag repository is required")
	}
	userID := strings.TrimSpace(string(req.UserID))
	mailboxID := string(req.MailboxID)
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return nil, err
	}
	if err := validateServiceResourceID("mailbox_id", mailboxID); err != nil {
		return nil, err
	}
	if err := validateNonEmptyIMAPUIDs(req.UIDs); err != nil {
		return nil, err
	}
	summaries, err := repo.StoreIMAPFlags(ctx, userID, mailboxID, req.UIDs, req.Flags, req.Mode, req.UnchangedSince, req.UnchangedSinceSet)
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
	s.emitQuotaWarningIfNeeded(ctx, string(req.UserID))
	return result, nil
}

func spoolIMAPAppendBody(body io.Reader, expectedSize int64) (*os.File, int64, error) {
	spooled, err := os.CreateTemp("", "gogomail-imap-append-*.eml")
	if err != nil {
		return nil, 0, fmt.Errorf("create imap append spool: %w", err)
	}
	copied, copyErr := io.Copy(spooled, io.LimitReader(body, expectedSize+1))
	if copyErr != nil {
		if closeErr := spooled.Close(); closeErr != nil {
			fmt.Fprintln(os.Stderr, "close imap append spool after copy error:", closeErr)
		}
		if rmErr := os.Remove(spooled.Name()); rmErr != nil {
			fmt.Fprintln(os.Stderr, "remove imap append spool after copy error:", rmErr)
		}
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

func (s *Service) CopyIMAPMessages(ctx context.Context, req imapgw.CopyMessagesRequest) ([]imapgw.CopyMessageResult, error) {
	repo, ok := s.repository.(interface {
		CopyIMAPMessages(context.Context, string, string, string, []imapgw.UID) ([]imapgw.CopyMessageResult, error)
	})
	if !ok {
		return nil, fmt.Errorf("imap copy repository is required")
	}
	userID := strings.TrimSpace(string(req.UserID))
	sourceMailboxID := string(req.SourceMailboxID)
	destMailboxID := string(req.DestMailboxID)
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
	_ = s.publishIMAPSummaryEvents(ctx, imapgw.MailboxEventExists, userID, imapCopyDestinationSummaries(summaries))
	s.emitQuotaWarningIfNeeded(ctx, userID)
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
	sourceMailboxID := string(req.SourceMailboxID)
	destMailboxID := string(req.DestMailboxID)
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
	s.emitQuotaWarningIfNeeded(ctx, userID)
	return results, nil
}

// expungeStoragePathRepository is the optional repository extension used to
// look up EML paths that are safe to GC before an IMAP EXPUNGE.
type expungeStoragePathRepository interface {
	LookupExpungeStoragePaths(ctx context.Context, userID string, mailboxID string, uids []imapgw.UID) ([]string, error)
}

func (s *Service) ExpungeIMAPMessages(ctx context.Context, req imapgw.ExpungeRequest) ([]imapgw.MessageSummary, error) {
	repo, ok := s.repository.(interface {
		ExpungeIMAPMessages(context.Context, string, string, []imapgw.UID) ([]imapgw.MessageSummary, error)
	})
	if !ok {
		return nil, fmt.Errorf("imap expunge repository is required")
	}
	userID := strings.TrimSpace(string(req.UserID))
	mailboxID := string(req.MailboxID)
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return nil, err
	}
	if err := validateServiceResourceID("mailbox_id", mailboxID); err != nil {
		return nil, err
	}
	if err := validateIMAPUIDs(req.UIDs); err != nil {
		return nil, err
	}
	// Resolve deleteable storage paths BEFORE the DB records are removed.
	var storagePaths []string
	if s.store != nil {
		if spRepo, ok := s.repository.(expungeStoragePathRepository); ok {
			paths, err := spRepo.LookupExpungeStoragePaths(ctx, userID, mailboxID, req.UIDs)
			if err != nil {
				slog.Warn("failed to look up expunge storage paths", "user_id", userID, "error", err)
			} else {
				storagePaths = paths
			}
		}
	}
	summaries, err := repo.ExpungeIMAPMessages(ctx, userID, mailboxID, req.UIDs)
	if err != nil {
		return nil, err
	}
	s.deleteStorageObjects(ctx, storagePaths)
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

func imapCopyDestinationSummaries(results []imapgw.CopyMessageResult) []imapgw.MessageSummary {
	summaries := make([]imapgw.MessageSummary, 0, len(results))
	for _, result := range results {
		summaries = append(summaries, result.Destination)
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
	if eventType == imapgw.MailboxEventExists {
		uids = coalesceIMAPUIDExistsEvents(uids)
	}
	userID = strings.TrimSpace(userID)
	for _, uid := range uids {
		if uid.MailboxID == "" {
			continue
		}
		if err := s.imapEvents.Publish(ctx, imapgw.MailboxEvent{
			Type:           eventType,
			UserID:         imapgw.UserID(userID),
			MailboxID:      uid.MailboxID,
			UID:            uid.UID,
			SequenceNumber: imapUIDEventSequenceNumber(eventType, uid),
			Messages:       imapUIDEventMessageCount(eventType, uid),
		}); err != nil {
			return err
		}
	}
	return nil
}

func coalesceIMAPUIDExistsEvents(uids []maildb.IMAPMessageUID) []maildb.IMAPMessageUID {
	if len(uids) < 2 {
		return uids
	}
	byMailbox := make(map[imapgw.MailboxID]int, len(uids))
	coalesced := make([]maildb.IMAPMessageUID, 0, len(uids))
	for _, uid := range uids {
		if idx, ok := byMailbox[uid.MailboxID]; ok {
			if uid.SequenceNumber > coalesced[idx].SequenceNumber {
				coalesced[idx] = uid
			}
			continue
		}
		byMailbox[uid.MailboxID] = len(coalesced)
		coalesced = append(coalesced, uid)
	}
	return coalesced
}

func (s *Service) publishIMAPRestoredMessageEvents(ctx context.Context, userID string, messageIDs []string) error {
	if s.imapEvents == nil || len(messageIDs) == 0 {
		return nil
	}
	uids, err := s.repository.EnsureIMAPMessageUIDsForMessages(ctx, userID, messageIDs)
	if err != nil {
		return err
	}
	return s.publishIMAPUIDEvents(ctx, imapgw.MailboxEventExists, userID, uids)
}

func imapUIDEventSequenceNumber(eventType imapgw.MailboxEventType, uid maildb.IMAPMessageUID) uint32 {
	if eventType == imapgw.MailboxEventExpunge {
		return uid.SequenceNumber
	}
	return 0
}

func imapUIDEventMessageCount(eventType imapgw.MailboxEventType, uid maildb.IMAPMessageUID) uint32 {
	if eventType == imapgw.MailboxEventExists {
		return uid.SequenceNumber
	}
	return 0
}