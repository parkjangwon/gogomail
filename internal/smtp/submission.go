package smtpd

import (
	"context"
	"fmt"
	"io"
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
	Store             storage.Store
	Authenticator     SubmissionAuthenticator
	Recorder          SubmissionRecorder
	Metrics           Metrics
	Hooks             []Hook
	SupportSMTPUTF8   bool
	SupportRequireTLS bool
	SupportDSN        bool
	SupportBinaryMIME bool
	AddReceivedHeader bool
	ReceivedDomain    string
	Policy            ReceivePolicy
	IDGenerator       IDGenerator
	Clock             func() time.Time
	MaxMessageBytes   int64
}

type SubmissionReceiver struct {
	store             storage.Store
	authenticator     SubmissionAuthenticator
	recorder          SubmissionRecorder
	metrics           Metrics
	hooks             []Hook
	supportSMTPUTF8   bool
	supportRequireTLS bool
	supportDSN        bool
	supportBinaryMIME bool
	addReceivedHeader bool
	receivedDomain    string
	policy            ReceivePolicy
	idGenerator       IDGenerator
	clock             func() time.Time
}

func NewSubmissionReceiver(opts SubmissionOptions) *SubmissionReceiver {
	idGenerator := opts.IDGenerator
	if idGenerator == nil {
		idGenerator = randomMessageID
	}
	return &SubmissionReceiver{
		store:             opts.Store,
		authenticator:     opts.Authenticator,
		recorder:          opts.Recorder,
		metrics:           metricsOrDefault(opts.Metrics),
		hooks:             append([]Hook(nil), opts.Hooks...),
		supportSMTPUTF8:   opts.SupportSMTPUTF8,
		supportRequireTLS: opts.SupportRequireTLS,
		supportDSN:        opts.SupportDSN,
		supportBinaryMIME: opts.SupportBinaryMIME,
		addReceivedHeader: opts.AddReceivedHeader,
		receivedDomain:    opts.ReceivedDomain,
		policy:            normalizePolicy(opts.Policy, opts.MaxMessageBytes),
		idGenerator:       idGenerator,
		clock:             clockOrDefault(opts.Clock),
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
		var authErr error
		defer func() {
			s.observe(context.Background(), MetricEvent{
				Stage:  StageAuthenticated,
				Result: metricResult(authErr),
				Error:  metricError(authErr),
			})
		}()
		user, err := s.receiver.authenticator.AuthenticatePlain(context.Background(), identity, username, password)
		if err != nil {
			authErr = gosmtp.ErrAuthFailed
			return gosmtp.ErrAuthFailed
		}
		s.user = user
		if err := s.emit(context.Background(), Event{
			Stage:          StageAuthenticated,
			SubmissionUser: user,
		}); err != nil {
			authErr = err
			return err
		}
		return nil
	}), nil
}

func (s *submissionSession) Mail(from string, opts *gosmtp.MailOptions) (err error) {
	defer func() {
		s.observe(context.Background(), MetricEvent{
			Stage:        StageMailFrom,
			Result:       metricResult(err),
			EnvelopeFrom: from,
			Error:        metricError(err),
		})
	}()
	if s.user.UserID == "" {
		return gosmtp.ErrAuthRequired
	}
	if err := validateMailOptions(opts, extensionSupport{
		SMTPUTF8:   s.receiver.supportSMTPUTF8,
		RequireTLS: s.receiver.supportRequireTLS,
		DSN:        s.receiver.supportDSN,
		BinaryMIME: s.receiver.supportBinaryMIME,
	}); err != nil {
		return err
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

func (s *submissionSession) Rcpt(to string, opts *gosmtp.RcptOptions) (err error) {
	defer func() {
		s.observe(context.Background(), MetricEvent{
			Stage:        StageRcpt,
			Result:       metricResult(err),
			EnvelopeFrom: s.from,
			Recipient:    to,
			Error:        metricError(err),
		})
	}()
	if s.user.UserID == "" {
		return gosmtp.ErrAuthRequired
	}
	if s.from == "" {
		return fmt.Errorf("mail command is required before rcpt")
	}
	if err := validateRcptOptions(opts, extensionSupport{DSN: s.receiver.supportDSN}); err != nil {
		return err
	}
	if len(s.recipients) >= s.receiver.policy.MaxRecipientsPerMessage {
		return fmt.Errorf("too many recipients; max %d", s.receiver.policy.MaxRecipientsPerMessage)
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

func (s *submissionSession) Data(r io.Reader) (err error) {
	envelopeFrom := s.from
	recipients := append([]string(nil), s.recipients...)
	var observedSize int64
	defer func() {
		s.observe(context.Background(), MetricEvent{
			Stage:        StageRecorded,
			Result:       metricResult(err),
			EnvelopeFrom: envelopeFrom,
			Recipients:   recipients,
			Size:         observedSize,
			Error:        metricError(err),
		})
	}()
	if s.user.UserID == "" {
		return gosmtp.ErrAuthRequired
	}
	if s.from == "" {
		return fmt.Errorf("mail command is required before data")
	}
	if len(s.recipients) == 0 {
		return fmt.Errorf("at least one recipient is required before data")
	}
	envelopeFrom = s.from
	recipients = append([]string(nil), s.recipients...)
	defer s.Reset()

	spooled, size, err := spoolMessage(r, s.receiver.policy.MaxMessageBytes)
	if err != nil {
		return err
	}
	observedSize = size
	messageID := s.receiver.idGenerator()
	submittedAt := s.receiver.clock()
	if s.receiver.addReceivedHeader {
		prefixed, prefixedSize, err := prependHeaderToSpool(spooled, BuildReceivedHeaderWithProtocol(s.remoteAddr, s.receiver.receivedDomain, "ESMTPA", messageID, submittedAt))
		cleanupSpool(spooled)
		if err != nil {
			return err
		}
		spooled = prefixed
		size = prefixedSize
		observedSize = size
	}
	defer cleanupSpool(spooled)
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
	parsed, err := message.ParseEMLWithOptions(spooled, message.ParseOptions{SkipTextBody: true})
	if err != nil {
		return fmt.Errorf("parse submitted message: %w", err)
	}
	if parsed.MessageID == "" {
		parsed.MessageID = submittedMessageID(messageID, s.user.Address)
		prefixed, prefixedSize, err := insertHeaderAfterTraceHeaders(spooled, "Message-ID: "+parsed.MessageID+"\r\n")
		cleanupSpool(spooled)
		if err != nil {
			return err
		}
		spooled = prefixed
		size = prefixedSize
		observedSize = size
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

	path := BuildStoragePath(Mailbox{
		CompanyID: s.user.CompanyID,
		DomainID:  s.user.DomainID,
		UserID:    s.user.UserID,
		Address:   s.user.Address,
	}, messageID, submittedAt)

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

func submittedMessageID(id string, fromAddress string) string {
	_, domain, ok := strings.Cut(strings.TrimSpace(fromAddress), "@")
	if !ok || strings.TrimSpace(domain) == "" {
		domain = "localhost"
	}
	id = sanitizeReceivedToken(id)
	if id == "" {
		id = randomMessageID()
	}
	return "<" + id + "@" + strings.ToLower(strings.TrimSpace(domain)) + ">"
}

func (s *submissionSession) emit(ctx context.Context, event Event) error {
	if event.RemoteAddr == "" {
		event.RemoteAddr = s.remoteAddr
	}
	for _, hook := range s.receiver.hooks {
		if err := hook(ctx, event); err != nil {
			return fmt.Errorf("submission hook %s: %w", event.Stage, err)
		}
	}
	return nil
}

func (s *submissionSession) observe(ctx context.Context, event MetricEvent) {
	event.RemoteAddr = s.remoteAddr
	s.receiver.metrics.ObserveSMTP(ctx, event)
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
