package smtpd

import (
	"context"
	"errors"
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
	CompanyID           string
	DomainID            string
	UserID              string
	Address             string
	Role                string
	DisplayName         string
	AuthorizedAddresses []string
	MustChangePassword  bool
}

type SubmissionAuthenticator interface {
	AuthenticatePlain(ctx context.Context, identity string, username string, password string) (SubmissionUser, error)
}

type SubmittedMessage struct {
	EnvelopeFrom  string
	User          SubmissionUser
	Recipients    []string
	DSN           DSNOptions
	StoragePath   string
	Parsed        message.ParsedMessage
	SubmittedAt   time.Time
	Size          int64
	RFCCompliance RFCCompliance
}

type SubmissionRecorder interface {
	RecordSubmitted(ctx context.Context, msg SubmittedMessage) (string, error)
}

type SubmissionOptions struct {
	Store              storage.Store
	Authenticator      SubmissionAuthenticator
	Recorder           SubmissionRecorder
	DomainPolicyLookup DomainPolicyLookup
	Metrics            Metrics
	Hooks              []Hook
	SupportSMTPUTF8    bool
	SupportRequireTLS  bool
	SupportDSN         bool
	SupportBinaryMIME  bool
	AddReceivedHeader  bool
	ReceivedDomain     string
	Policy             ReceivePolicy
	IDGenerator        IDGenerator
	Clock              func() time.Time
	MaxMessageBytes    int64
	BulkSenderLimiter  *BulkSenderLimiter // nil disables bulk sender rate limiting
}

type SubmissionReceiver struct {
	store              storage.Store
	authenticator      SubmissionAuthenticator
	recorder           SubmissionRecorder
	domainPolicyLookup DomainPolicyLookup
	metrics            Metrics
	hooks              []Hook
	supportSMTPUTF8    bool
	supportRequireTLS  bool
	supportDSN         bool
	supportBinaryMIME  bool
	addReceivedHeader  bool
	receivedDomain     string
	policy             ReceivePolicy
	idGenerator        IDGenerator
	clock              func() time.Time
	bulkSenderLimiter  *BulkSenderLimiter
}

func NewSubmissionReceiver(opts SubmissionOptions) *SubmissionReceiver {
	idGenerator := opts.IDGenerator
	if idGenerator == nil {
		idGenerator = randomMessageID
	}
	return &SubmissionReceiver{
		store:              opts.Store,
		authenticator:      opts.Authenticator,
		recorder:           opts.Recorder,
		domainPolicyLookup: opts.DomainPolicyLookup,
		metrics:            metricsOrDefault(opts.Metrics),
		hooks:              append([]Hook(nil), opts.Hooks...),
		supportSMTPUTF8:    opts.SupportSMTPUTF8,
		supportRequireTLS:  opts.SupportRequireTLS,
		supportDSN:         opts.SupportDSN,
		supportBinaryMIME:  opts.SupportBinaryMIME,
		addReceivedHeader:  opts.AddReceivedHeader,
		receivedDomain:     opts.ReceivedDomain,
		policy:             normalizePolicy(opts.Policy, opts.MaxMessageBytes),
		idGenerator:        idGenerator,
		clock:              clockOrDefault(opts.Clock),
		bulkSenderLimiter:  opts.BulkSenderLimiter,
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
	receiver           *SubmissionReceiver
	user               SubmissionUser
	from               string
	recipients         []string
	dsn                DSNOptions
	smtpUTF8           bool
	remoteAddr         string
	domainPolicy       *InboundDomainPolicy
	domainPolicyLoaded bool
}

func (s *submissionSession) AuthMechanisms() []string {
	return []string{sasl.Plain}
}

func (s *submissionSession) Auth(mech string) (sasl.Server, error) {
	if !strings.EqualFold(mech, sasl.Plain) {
		return nil, gosmtp.ErrAuthUnsupported
	}
	if s.user.UserID != "" {
		return nil, smtpAlreadyAuthenticated()
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
		if user.MustChangePassword {
			authErr = gosmtp.ErrAuthFailed
			return gosmtp.ErrAuthFailed
		}
		if err := s.emit(context.Background(), Event{
			Stage:          StageAuthenticated,
			SubmissionUser: user,
		}); err != nil {
			authErr = err
			return err
		}
		s.user = user
		s.clearDomainPolicy()
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
	s.clearEnvelope()
	if err := validateMailOptions(opts, extensionSupport{
		SMTPUTF8:   s.receiver.supportSMTPUTF8,
		RequireTLS: s.receiver.supportRequireTLS,
		DSN:        s.receiver.supportDSN,
		BinaryMIME: s.receiver.supportBinaryMIME,
	}); err != nil {
		return err
	}
	if err := validateAnnouncedMessageSize(opts, s.receiver.policy.MaxMessageBytes); err != nil {
		return err
	}
	normalized, err := mail.NormalizeAddress(from)
	if err != nil {
		return err
	}
	if err := validateSMTPUTF8Address(from, normalized, mailOptionsUTF8(opts), s.receiver.supportSMTPUTF8); err != nil {
		return err
	}
	if !submissionSenderAllowed(normalized, s.user) {
		return smtpPolicyReject("mail from %q is not allowed for authenticated user", normalized)
	}
	if s.receiver.bulkSenderLimiter != nil && s.user.UserID != "" {
		if !s.receiver.bulkSenderLimiter.AllowSubmission(s.user.UserID, s.user.Role) {
			return smtpRateLimited(from)
		}
	}
	s.from = normalized
	s.smtpUTF8 = mailOptionsUTF8(opts)
	s.dsn.Return = normalizeDSNReturn(opts)
	s.dsn.EnvelopeID = normalizeDSNEnvelopeID(opts)
	if err := s.emit(context.Background(), Event{
		Stage:          StageMailFrom,
		EnvelopeFrom:   s.from,
		SubmissionUser: s.user,
		DSN:            s.currentDSNOptions(),
	}); err != nil {
		return err
	}
	return nil
}

func submissionSenderAllowed(address string, user SubmissionUser) bool {
	if strings.EqualFold(address, user.Address) {
		return true
	}
	for _, allowed := range user.AuthorizedAddresses {
		if strings.EqualFold(address, allowed) {
			return true
		}
	}
	return false
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
		return smtpBadSequence("RCPT")
	}
	if err := validateRcptOptions(opts, extensionSupport{DSN: s.receiver.supportDSN}); err != nil {
		return err
	}
	maxRecipients := effectiveMaxRecipients(s.receiver.policy.MaxRecipientsPerMessage, s.currentDomainPolicy(context.Background()))
	if len(s.recipients) >= maxRecipients {
		return smtpTooManyRecipients(maxRecipients)
	}
	normalized, err := mail.NormalizeAddress(to)
	if err != nil {
		return err
	}
	if err := validateSMTPUTF8Address(to, normalized, s.smtpUTF8, s.receiver.supportSMTPUTF8); err != nil {
		return err
	}
	s.recipients = append(s.recipients, normalized)
	s.dsn.Recipients = upsertDSNRecipientOption(s.dsn.Recipients, normalizeDSNRecipientOptions(normalized, opts))
	if err := s.emit(context.Background(), Event{
		Stage:          StageRcpt,
		EnvelopeFrom:   s.from,
		SubmissionUser: s.user,
		Recipients:     append([]string(nil), s.recipients...),
		DSN:            s.currentDSNOptions(),
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
		return smtpBadSequence("DATA")
	}
	if len(s.recipients) == 0 {
		return smtpBadSequence("DATA")
	}
	envelopeFrom = s.from
	recipients = append([]string(nil), s.recipients...)
	defer s.Reset()

	// Apply per-domain recipient cap against what was already collected.
	maxRecipients := effectiveMaxRecipients(s.receiver.policy.MaxRecipientsPerMessage, s.currentDomainPolicy(context.Background()))
	if len(s.recipients) > maxRecipients {
		return smtpTooManyRecipients(maxRecipients)
	}

	spooled, size, err := spoolMessage(r, effectiveMaxBytes(s.receiver.policy.MaxMessageBytes, s.currentDomainPolicy(context.Background())))
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
		DSN:            s.currentDSNOptions(),
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

	// Validate RFC compliance - check envelope and basic message format
	rfcValidator := NewRFCCompliant()
	var rfcCompliance RFCCompliance
	if _, err := spooled.Seek(0, io.SeekStart); err == nil {
		rawData := make([]byte, observedSize)
		if n, err := spooled.Read(rawData); err == nil && int64(n) == observedSize {
			rfcCompliance = rfcValidator.ValidateMessage(
				string(rawData),
				s.from,
				s.recipients,
				s.currentDSNOptions().Return,
				[]string{}, // DSN notify options per recipient
				"",         // DKIM signature (if present in message)
			)
			// Log violations but don't reject (yet) - in Phase 8, we could make this stricter
			if !rfcCompliance.IsCompliant() && s.receiver.metrics != nil {
				s.receiver.metrics.ObserveRFCNonCompliance(rfcCompliance)
			}
		}
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

	// Inject List-Unsubscribe for bulk sends (Google 2024 bulk sender policy).
	// Only added when sending to multiple recipients and not already present.
	if len(s.recipients) >= bulkRecipientThreshold {
		if !messageHasListUnsubscribe(spooled, observedSize) {
			_, fromDomain, _ := strings.Cut(s.from, "@")
			unsubHeader := "List-Unsubscribe: <mailto:unsubscribe@" + fromDomain + "?subject=unsubscribe>\r\n" +
				"List-Unsubscribe-Post: List-Unsubscribe=One-Click\r\n"
			prefixed, prefixedSize, err := insertHeaderAfterTraceHeaders(spooled, unsubHeader)
			cleanupSpool(spooled)
			if err != nil {
				return err
			}
			spooled = prefixed
			size = prefixedSize
			observedSize = size
		}
	}

	if err := s.emit(context.Background(), Event{
		Stage:          StageParsed,
		EnvelopeFrom:   s.from,
		SubmissionUser: s.user,
		Recipients:     append([]string(nil), s.recipients...),
		DSN:            s.currentDSNOptions(),
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
		DSN:            s.currentDSNOptions(),
		StoragePath:    path,
		Parsed:         parsed,
		SubmittedAt:    submittedAt,
		Size:           size,
	}); err != nil {
		s.deleteStoredMessage(path)
		return err
	}
	_, err = s.receiver.recorder.RecordSubmitted(context.Background(), SubmittedMessage{
		EnvelopeFrom:  s.from,
		User:          s.user,
		Recipients:    append([]string(nil), s.recipients...),
		DSN:           s.currentDSNOptions(),
		StoragePath:   path,
		Parsed:        parsed,
		SubmittedAt:   submittedAt,
		Size:          size,
		RFCCompliance: rfcCompliance,
	})
	if err != nil {
		s.deleteStoredMessage(path)
		if errors.Is(err, mail.ErrMailboxFull) {
			return smtpMailboxFull(s.user.Address)
		}
		return fmt.Errorf("record submitted message: %w", err)
	}
	if err := s.emit(context.Background(), Event{
		Stage:          StageRecorded,
		EnvelopeFrom:   s.from,
		SubmissionUser: s.user,
		Recipients:     append([]string(nil), s.recipients...),
		DSN:            s.currentDSNOptions(),
		StoragePath:    path,
		Parsed:         parsed,
		SubmittedAt:    submittedAt,
		Size:           size,
	}); err != nil {
		return err
	}
	return nil
}

func (s *submissionSession) deleteStoredMessage(path string) {
	if strings.TrimSpace(path) == "" || s.receiver.store == nil {
		return
	}
	_ = s.receiver.store.Delete(context.Background(), path)
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

// bulkRecipientThreshold is the minimum recipient count that triggers
// List-Unsubscribe header injection (Google 2024 bulk sender policy).
const bulkRecipientThreshold = 5

// messageHasListUnsubscribe reports whether the spooled message already
// contains a List-Unsubscribe header so we do not duplicate it.
func messageHasListUnsubscribe(spooled *os.File, size int64) bool {
	if _, err := spooled.Seek(0, io.SeekStart); err != nil {
		return false
	}
	limit := size
	if limit > 16*1024 { // only scan headers; 16 KiB is more than enough
		limit = 16 * 1024
	}
	buf := make([]byte, limit)
	n, _ := spooled.Read(buf)
	lower := strings.ToLower(string(buf[:n]))
	return strings.Contains(lower, "\nlist-unsubscribe:") ||
		strings.HasPrefix(lower, "list-unsubscribe:")
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

func (s *submissionSession) currentDSNOptions() DSNOptions {
	return cloneDSNOptions(s.dsn)
}

func (s *submissionSession) currentDomainPolicy(ctx context.Context) *InboundDomainPolicy {
	if s.domainPolicyLoaded {
		return s.domainPolicy
	}
	s.domainPolicyLoaded = true
	if s.receiver.domainPolicyLookup == nil || s.user.DomainID == "" {
		return nil
	}
	dp, err := s.receiver.domainPolicyLookup.InboundDomainPolicy(ctx, s.user.DomainID)
	if err != nil {
		return nil
	}
	s.domainPolicy = &dp
	return s.domainPolicy
}

func (s *submissionSession) Reset() {
	s.clearEnvelope()
}

func (s *submissionSession) clearEnvelope() {
	s.from = ""
	s.recipients = nil
	s.dsn = DSNOptions{}
	s.smtpUTF8 = false
}

func (s *submissionSession) clearDomainPolicy() {
	s.domainPolicy = nil
	s.domainPolicyLoaded = false
}

func (s *submissionSession) Logout() error {
	s.Reset()
	s.user = SubmissionUser{}
	s.clearDomainPolicy()
	return nil
}
