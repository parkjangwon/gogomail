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
	ListMessages(ctx context.Context, userID string, limit int) ([]maildb.MessageSummary, error)
	GetMessage(ctx context.Context, userID string, messageID string) (maildb.MessageDetail, error)
	SenderForUser(ctx context.Context, userID string, fromAddress string) (maildb.Sender, error)
	RecordOutgoing(ctx context.Context, msg maildb.OutgoingMessage) (string, error)
}

type Service struct {
	repository Repository
	store      storage.Store
}

func New(repository Repository, store storage.Store) *Service {
	return &Service{repository: repository, store: store}
}

func (s *Service) ListMessages(ctx context.Context, userID string, limit int) ([]maildb.MessageSummary, error) {
	return s.repository.ListMessages(ctx, userID, limit)
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
