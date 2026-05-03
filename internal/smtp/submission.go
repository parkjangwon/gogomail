package smtpd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-sasl"
	gosmtp "github.com/emersion/go-smtp"

	"github.com/gogomail/gogomail/internal/mail"
	"github.com/gogomail/gogomail/internal/message"
	"github.com/gogomail/gogomail/internal/storage"
)

type SubmissionUser struct {
	CompanyID   string
	DomainID    string
	UserID      string
	Address     string
	DisplayName string
}

type SubmissionAuthenticator interface {
	AuthenticatePlain(ctx context.Context, identity string, username string, password string) (SubmissionUser, error)
}

type SubmittedMessage struct {
	EnvelopeFrom string
	User         SubmissionUser
	Recipients   []string
	StoragePath  string
	Parsed       message.ParsedMessage
	SubmittedAt  time.Time
	Size         int64
}

type SubmissionRecorder interface {
	RecordSubmitted(ctx context.Context, msg SubmittedMessage) (string, error)
}

type SubmissionOptions struct {
	Store           storage.Store
	Authenticator   SubmissionAuthenticator
	Recorder        SubmissionRecorder
	Hooks           []Hook
	SupportSMTPUTF8 bool
	IDGenerator     IDGenerator
	Clock           func() time.Time
	MaxMessageBytes int64
}

type SubmissionReceiver struct {
	store           storage.Store
	authenticator   SubmissionAuthenticator
	recorder        SubmissionRecorder
	hooks           []Hook
	supportSMTPUTF8 bool
	idGenerator     IDGenerator
	clock           func() time.Time
	maxMessageBytes int64
}

func NewSubmissionReceiver(opts SubmissionOptions) *SubmissionReceiver {
	idGenerator := opts.IDGenerator
	if idGenerator == nil {
		idGenerator = randomMessageID
	}
	maxBytes := opts.MaxMessageBytes
	if maxBytes <= 0 {
		maxBytes = 25 * 1024 * 1024
	}
	return &SubmissionReceiver{
		store:           opts.Store,
		authenticator:   opts.Authenticator,
		recorder:        opts.Recorder,
		hooks:           append([]Hook(nil), opts.Hooks...),
		supportSMTPUTF8: opts.SupportSMTPUTF8,
		idGenerator:     idGenerator,
		clock:           clockOrDefault(opts.Clock),
		maxMessageBytes: maxBytes,
	}
}

func (r *SubmissionReceiver) NewSession(conn *gosmtp.Conn) (gosmtp.Session, error) {
	if r.store == nil {
		return nil, fmt.Errorf("submission store is required")
	}
	if r.authenticator == nil {
		return nil, fmt.Errorf("submission authenticator is required")
	}
	if r.recorder == nil {
		return nil, fmt.Errorf("submission recorder is required")
	}
	return &submissionSession{receiver: r, remoteAddr: remoteAddrFromConn(conn)}, nil
}

type submissionSession struct {
	receiver   *SubmissionReceiver
	user       SubmissionUser
	from       string
	recipients []string
	remoteAddr string
}

func (s *submissionSession) AuthMechanisms() []string {
	return []string{sasl.Plain}
}

func (s *submissionSession) Auth(mech string) (sasl.Server, error) {
	if !strings.EqualFold(mech, sasl.Plain) {
		return nil, gosmtp.ErrAuthUnsupported
	}
	return sasl.NewPlainServer(func(identity, username, password string) error {
		user, err := s.receiver.authenticator.AuthenticatePlain(context.Background(), identity, username, password)
		if err != nil {
			return gosmtp.ErrAuthFailed
		}
		s.user = user
		if err := s.emit(context.Background(), Event{
			Stage:          StageAuthenticated,
			SubmissionUser: user,
		}); err != nil {
			return err
		}
		return nil
	}), nil
}

func (s *submissionSession) Mail(from string, opts *gosmtp.MailOptions) error {
	if s.user.UserID == "" {
		return gosmtp.ErrAuthRequired
	}
	if opts != nil && opts.UTF8 && !s.receiver.supportSMTPUTF8 {
		return fmt.Errorf("SMTPUTF8 is not supported")
	}
	normalized, err := mail.NormalizeAddress(from)
	if err != nil {
		return err
	}
	if !strings.EqualFold(normalized, s.user.Address) {
		return fmt.Errorf("mail from %q is not allowed for authenticated user", normalized)
	}
	s.from = normalized
	if err := s.emit(context.Background(), Event{
		Stage:          StageMailFrom,
		EnvelopeFrom:   s.from,
		SubmissionUser: s.user,
	}); err != nil {
		return err
	}
	return nil
}

func (s *submissionSession) Rcpt(to string, _ *gosmtp.RcptOptions) error {
	if s.user.UserID == "" {
		return gosmtp.ErrAuthRequired
	}
	normalized, err := mail.NormalizeAddress(to)
	if err != nil {
		return err
	}
	s.recipients = append(s.recipients, normalized)
	if err := s.emit(context.Background(), Event{
		Stage:          StageRcpt,
		EnvelopeFrom:   s.from,
		SubmissionUser: s.user,
		Recipients:     append([]string(nil), s.recipients...),
	}); err != nil {
		return err
	}
	return nil
}

func (s *submissionSession) Data(r io.Reader) error {
	if s.user.UserID == "" {
		return gosmtp.ErrAuthRequired
	}
	if s.from == "" {
		return fmt.Errorf("mail command is required before data")
	}
	if len(s.recipients) == 0 {
		return fmt.Errorf("at least one recipient is required before data")
	}

	spooled, size, err := spoolMessage(r, s.receiver.maxMessageBytes)
	if err != nil {
		return err
	}
	defer func() {
		_ = spooled.Close()
		_ = os.Remove(spooled.Name())
	}()
	if err := s.emit(context.Background(), Event{
		Stage:          StageSpooled,
		EnvelopeFrom:   s.from,
		SubmissionUser: s.user,
		Recipients:     append([]string(nil), s.recipients...),
		Size:           size,
	}); err != nil {
		return err
	}

	if _, err := spooled.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind submitted message for parse: %w", err)
	}
	parsed, err := message.ParseEML(spooled)
	if err != nil {
		return fmt.Errorf("parse submitted message: %w", err)
	}
	if err := s.emit(context.Background(), Event{
		Stage:          StageParsed,
		EnvelopeFrom:   s.from,
		SubmissionUser: s.user,
		Recipients:     append([]string(nil), s.recipients...),
		Parsed:         parsed,
		Size:           size,
	}); err != nil {
		return err
	}

	submittedAt := s.receiver.clock()
	path := BuildStoragePath(Mailbox{
		CompanyID: s.user.CompanyID,
		DomainID:  s.user.DomainID,
		UserID:    s.user.UserID,
		Address:   s.user.Address,
	}, s.receiver.idGenerator(), submittedAt)

	if _, err := spooled.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind submitted message for store: %w", err)
	}
	if err := s.receiver.store.Put(context.Background(), path, spooled); err != nil {
		return fmt.Errorf("store submitted message: %w", err)
	}
	if err := s.emit(context.Background(), Event{
		Stage:          StageStored,
		EnvelopeFrom:   s.from,
		SubmissionUser: s.user,
		Recipients:     append([]string(nil), s.recipients...),
		StoragePath:    path,
		Parsed:         parsed,
		SubmittedAt:    submittedAt,
		Size:           size,
	}); err != nil {
		return err
	}
	_, err = s.receiver.recorder.RecordSubmitted(context.Background(), SubmittedMessage{
		EnvelopeFrom: s.from,
		User:         s.user,
		Recipients:   append([]string(nil), s.recipients...),
		StoragePath:  path,
		Parsed:       parsed,
		SubmittedAt:  submittedAt,
		Size:         size,
	})
	if err != nil {
		return fmt.Errorf("record submitted message: %w", err)
	}
	if err := s.emit(context.Background(), Event{
		Stage:          StageRecorded,
		EnvelopeFrom:   s.from,
		SubmissionUser: s.user,
		Recipients:     append([]string(nil), s.recipients...),
		StoragePath:    path,
		Parsed:         parsed,
		SubmittedAt:    submittedAt,
		Size:           size,
	}); err != nil {
		return err
	}
	return nil
}

func (s *submissionSession) emit(ctx context.Context, event Event) error {
	for _, hook := range s.receiver.hooks {
		if err := hook(ctx, event); err != nil {
			return fmt.Errorf("submission hook %s: %w", event.Stage, err)
		}
	}
	return nil
}

func (s *submissionSession) Reset() {
	s.from = ""
	s.recipients = nil
}

func (s *submissionSession) Logout() error {
	s.Reset()
	s.user = SubmissionUser{}
	return nil
}
