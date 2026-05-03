package mailservice

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
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
	GetMessage(ctx context.Context, userID string, messageID string) (maildb.MessageDetail, error)
	SetMessageFlag(ctx context.Context, userID string, messageID string, flag string, value bool) error
	MoveMessage(ctx context.Context, userID string, messageID string, folderID string) error
	DeleteMessage(ctx context.Context, userID string, messageID string) error
	ListAttachments(ctx context.Context, userID string, messageID string) ([]maildb.Attachment, error)
	SenderForUser(ctx context.Context, userID string, fromAddress string) (maildb.Sender, error)
	SuppressedRecipients(ctx context.Context, domainID string, recipients []string) ([]string, error)
	RecordOutgoing(ctx context.Context, msg maildb.OutgoingMessage) (string, error)
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

func (s *Service) GetMessage(ctx context.Context, userID string, messageID string) (maildb.MessageDetail, error) {
	detail, err := s.repository.GetMessage(ctx, userID, messageID)
	if err != nil {
		return maildb.MessageDetail{}, err
	}
	if s.store == nil || detail.StoragePath == "" {
		return detail, nil
	}

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

func (s *Service) SetMessageFlag(ctx context.Context, userID string, messageID string, flag string, value bool) error {
	return s.repository.SetMessageFlag(ctx, userID, messageID, flag, value)
}

func (s *Service) MoveMessage(ctx context.Context, userID string, messageID string, folderID string) error {
	return s.repository.MoveMessage(ctx, userID, messageID, folderID)
}

func (s *Service) DeleteMessage(ctx context.Context, userID string, messageID string) error {
	return s.repository.DeleteMessage(ctx, userID, messageID)
}

func (s *Service) ListAttachments(ctx context.Context, userID string, messageID string) ([]maildb.Attachment, error) {
	return s.repository.ListAttachments(ctx, userID, messageID)
}

type SendTextRequest struct {
	UserID        string             `json:"user_id"`
	From          string             `json:"from"`
	To            []outbound.Address `json:"to"`
	Cc            []outbound.Address `json:"cc"`
	Bcc           []outbound.Address `json:"bcc"`
	Subject       string             `json:"subject"`
	TextBody      string             `json:"text_body"`
	Transactional bool               `json:"transactional"`
	ScheduledAt   time.Time          `json:"scheduled_at"`
}

type SendTextResult struct {
	ID           string        `json:"id"`
	RFCMessageID string        `json:"message_id"`
	Farm         outbound.Farm `json:"farm"`
}

func (s *Service) SendText(ctx context.Context, req SendTextRequest) (SendTextResult, error) {
	if s.repository == nil {
		return SendTextResult{}, fmt.Errorf("mail repository is required")
	}
	if s.store == nil {
		return SendTextResult{}, fmt.Errorf("mail storage is required")
	}
	if strings.TrimSpace(req.UserID) == "" {
		return SendTextResult{}, fmt.Errorf("user_id is required")
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
		CompanyID:    sender.CompanyID,
		DomainID:     sender.DomainID,
		UserID:       sender.UserID,
		RFCMessageID: composed.MessageID,
		Subject:      req.Subject,
		From:         from,
		To:           req.To,
		Cc:           req.Cc,
		Bcc:          req.Bcc,
		SentAt:       now,
		Size:         composed.Size,
		StoragePath:  path,
		Farm:         farm,
	})
	if err != nil {
		return SendTextResult{}, err
	}

	return SendTextResult{ID: id, RFCMessageID: composed.MessageID, Farm: farm}, nil
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
	for _, addr := range req.To {
		recipients = append(recipients, addr.Email)
	}
	for _, addr := range req.Cc {
		recipients = append(recipients, addr.Email)
	}
	for _, addr := range req.Bcc {
		recipients = append(recipients, addr.Email)
	}
	return recipients
}
