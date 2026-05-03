package smtpd

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	gosmtp "github.com/emersion/go-smtp"

	"github.com/gogomail/gogomail/internal/mail"
	"github.com/gogomail/gogomail/internal/message"
	"github.com/gogomail/gogomail/internal/storage"
)

type Mailbox struct {
	CompanyID string
	DomainID  string
	UserID    string
	Address   string
}

type RecipientResolver interface {
	ResolveRecipient(ctx context.Context, address string) (Mailbox, error)
}

type StaticResolver map[string]Mailbox

func (r StaticResolver) ResolveRecipient(_ context.Context, address string) (Mailbox, error) {
	normalized, err := mail.NormalizeAddress(address)
	if err != nil {
		return Mailbox{}, err
	}
	mailbox, ok := r[normalized]
	if !ok {
		return Mailbox{}, fmt.Errorf("recipient %q not found", normalized)
	}
	return mailbox, nil
}

type IDGenerator func() string

type ReceiverOptions struct {
	Store       storage.Store
	Resolver    RecipientResolver
	IDGenerator IDGenerator
	Clock       func() time.Time
}

type Receiver struct {
	store       storage.Store
	resolver    RecipientResolver
	idGenerator IDGenerator
	clock       func() time.Time
}

func NewReceiver(opts ReceiverOptions) *Receiver {
	idGenerator := opts.IDGenerator
	if idGenerator == nil {
		idGenerator = randomMessageID
	}
	return &Receiver{
		store:       opts.Store,
		resolver:    opts.Resolver,
		idGenerator: idGenerator,
		clock:       clockOrDefault(opts.Clock),
	}
}

func (r *Receiver) NewSession(_ *gosmtp.Conn) (gosmtp.Session, error) {
	if r.store == nil {
		return nil, fmt.Errorf("smtp receiver store is required")
	}
	if r.resolver == nil {
		return nil, fmt.Errorf("smtp receiver resolver is required")
	}
	return &session{receiver: r}, nil
}

type session struct {
	receiver   *Receiver
	from       string
	recipients []Mailbox
}

func (s *session) Mail(from string, _ *gosmtp.MailOptions) error {
	normalized, err := mail.NormalizeAddress(from)
	if err != nil {
		return err
	}
	s.from = normalized
	return nil
}

func (s *session) Rcpt(to string, _ *gosmtp.RcptOptions) error {
	mailbox, err := s.receiver.resolver.ResolveRecipient(context.Background(), to)
	if err != nil {
		return err
	}
	s.recipients = append(s.recipients, mailbox)
	return nil
}

func (s *session) Data(r io.Reader) error {
	if s.from == "" {
		return fmt.Errorf("mail command is required before data")
	}
	if len(s.recipients) == 0 {
		return fmt.Errorf("at least one recipient is required before data")
	}

	raw, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read smtp data: %w", err)
	}
	if _, err := message.ParseEML(bytes.NewReader(raw)); err != nil {
		return fmt.Errorf("parse smtp message: %w", err)
	}

	messageID := s.receiver.idGenerator()
	for _, recipient := range s.recipients {
		path := BuildStoragePath(recipient, messageID, s.receiver.clock())
		if err := s.receiver.store.Put(context.Background(), path, bytes.NewReader(raw)); err != nil {
			return fmt.Errorf("store message for %s: %w", recipient.Address, err)
		}
	}
	return nil
}

func (s *session) Reset() {
	s.from = ""
	s.recipients = nil
}

func (s *session) Logout() error {
	s.Reset()
	return nil
}

func BuildStoragePath(mailbox Mailbox, messageID string, receivedAt time.Time) string {
	return strings.Join([]string{
		"mailstore",
		mailbox.CompanyID,
		mailbox.DomainID,
		mailbox.UserID,
		"maildir",
		receivedAt.Format("2006"),
		receivedAt.Format("01"),
		messageID + ".eml",
	}, "/")
}

func clockOrDefault(clock func() time.Time) func() time.Time {
	if clock != nil {
		return clock
	}
	return time.Now
}

func randomMessageID() string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixMilli(), hex.EncodeToString(random[:]))
}
