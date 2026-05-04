package mailservice

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/message"
	"github.com/gogomail/gogomail/internal/outbound"
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
}

type DeliveryStatusRepository interface {
	MessageDeliveryStatus(ctx context.Context, userID string, messageID string) (maildb.MessageDeliveryStatusView, error)
}

type DomainPolicyRepository interface {
	DomainPolicy(ctx context.Context, domainID string) (maildb.DomainPolicyView, error)
}

type Service struct {
	repository Repository
	store      storage.Store
}

func New(repository Repository, store storage.Store) *Service {
	return &Service{repository: repository, store: store}
}

func (s *Service) ListFolders(ctx context.Context, userID string) ([]maildb.Folder, error) {
	return s.repository.ListFolders(ctx, userID)
}

func (s *Service) CreateFolder(ctx context.Context, req maildb.CreateFolderRequest) (maildb.Folder, error) {
	return s.repository.CreateFolder(ctx, req)
}

func (s *Service) RenameFolder(ctx context.Context, userID string, folderID string, name string) (maildb.Folder, error) {
	return s.repository.RenameFolder(ctx, userID, folderID, name)
}

func (s *Service) DeleteFolder(ctx context.Context, userID string, folderID string) error {
	return s.repository.DeleteFolder(ctx, userID, folderID)
}

func (s *Service) ListMessages(ctx context.Context, userID string, limit int) ([]maildb.MessageSummary, error) {
	return s.repository.ListMessages(ctx, userID, limit)
}

func (s *Service) ListMessagesInFolder(ctx context.Context, userID string, folderID string, limit int) ([]maildb.MessageSummary, error) {
	return s.repository.ListMessagesInFolder(ctx, userID, folderID, limit)
}

func (s *Service) ListMessagesPage(ctx context.Context, userID string, folderID string, limit int, cursor maildb.MessageListCursor) ([]maildb.MessageSummary, error) {
	return s.repository.ListMessagesPage(ctx, userID, folderID, limit, cursor)
}

func (s *Service) GetMessage(ctx context.Context, userID string, messageID string) (maildb.MessageDetail, error) {
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

func (s *Service) MessageDeliveryStatus(ctx context.Context, userID string, messageID string) (maildb.MessageDeliveryStatusView, error) {
	repo, ok := s.repository.(DeliveryStatusRepository)
	if !ok {
		return maildb.MessageDeliveryStatusView{}, fmt.Errorf("delivery status repository is required")
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
	return s.repository.SetMessageFlag(ctx, userID, messageID, flag, value)
}

func (s *Service) BulkSetMessageFlag(ctx context.Context, req maildb.BulkMessageFlagRequest) (int64, error) {
	if err := maildb.ValidateBulkMessageFlagRequest(req); err != nil {
		return 0, err
	}
	return s.repository.BulkSetMessageFlag(ctx, req)
}

func (s *Service) MoveMessage(ctx context.Context, userID string, messageID string, folderID string) error {
	return s.repository.MoveMessage(ctx, userID, messageID, folderID)
}

func (s *Service) BulkMoveMessages(ctx context.Context, req maildb.BulkMessageMoveRequest) (int64, error) {
	if err := maildb.ValidateBulkMessageMoveRequest(req); err != nil {
		return 0, err
	}
	return s.repository.BulkMoveMessages(ctx, req)
}

func (s *Service) DeleteMessage(ctx context.Context, userID string, messageID string) error {
	return s.repository.DeleteMessage(ctx, userID, messageID)
}

func (s *Service) BulkDeleteMessages(ctx context.Context, req maildb.BulkMessageDeleteRequest) (int64, error) {
	if err := maildb.ValidateBulkMessageDeleteRequest(req); err != nil {
		return 0, err
	}
	return s.repository.BulkDeleteMessages(ctx, req)
}

func (s *Service) SaveDraft(ctx context.Context, req SaveDraftRequest) (maildb.MessageDetail, error) {
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

func (s *Service) DeleteDraft(ctx context.Context, userID string, draftID string) error {
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
	return s.repository.ListAttachments(ctx, userID, messageID)
}

func (s *Service) CreateAttachmentUpload(ctx context.Context, req CreateAttachmentUploadRequest) (maildb.Attachment, error) {
	if err := ValidateCreateAttachmentUploadRequest(req); err != nil {
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
	if err := ValidateUploadAttachmentRequest(req); err != nil {
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
	expired, err := repo.ExpireStaleAttachmentUploads(ctx, maildb.ExpireStaleAttachmentUploadsRequest{
		Before: before,
		Limit:  limit,
	})
	if err != nil {
		return nil, err
	}
	if s.store == nil {
		return expired, nil
	}
	for _, attachment := range expired {
		if strings.TrimSpace(attachment.StoragePath) != "" {
			_ = s.store.Delete(ctx, attachment.StoragePath)
		}
	}
	return expired, nil
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
	if draftID == "" {
		return SendTextResult{}, fmt.Errorf("draft_id is required")
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

	from := outbound.Address{Name: sender.DisplayName, Email: sender.Address}
	composed, err := outbound.ComposeText(outbound.TextMessage{
		From:     from,
		To:       req.To,
		Cc:       req.Cc,
		Bcc:      req.Bcc,
		Subject:  req.Subject,
		TextBody: req.TextBody,
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

func (s *Service) domainPolicy(ctx context.Context, domainID string) (maildb.DomainPolicyView, error) {
	repo, ok := s.repository.(DomainPolicyRepository)
	if !ok {
		return maildb.DomainPolicyView{DomainID: domainID, InboundMode: "inherit", OutboundMode: "inherit"}, nil
	}
	return repo.DomainPolicy(ctx, domainID)
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
