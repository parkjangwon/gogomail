package smtpd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
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

type MessageRecorder interface {
	// Record persists the received message and returns the database message ID
	// (a UUID string) that can be used to correlate log entries across services.
	// Implementations that do not persist to a database (e.g. noopRecorder)
	// should return an empty string.
	Record(ctx context.Context, msg ReceivedMessage) (string, error)
}

type Deduplicator interface {
	CheckAndSet(ctx context.Context, key DedupKey) (bool, error)
}

type RateLimiter interface {
	Allow(ctx context.Context, key RateLimitKey) (bool, error)
}

type Backpressure interface {
	Accept(ctx context.Context) (bool, error)
}

type Authenticator interface {
	AuthenticatePlain(ctx context.Context, identity string, username string, password string) error
}

type AuthenticatorWithRole interface {
	AuthenticatePlainWithRole(ctx context.Context, identity string, username string, password string) (string, error)
}

type RelayAuthorizer interface {
	AllowRelay(ctx context.Context, remoteAddr string) (bool, error)
}

type RateLimitKey struct {
	Stage      Stage
	RemoteAddr string
	Recipient  string
}

type DedupKey struct {
	MessageID string
	Recipient string
}

type ReceivedMessage struct {
	EnvelopeFrom     string
	Mailbox          Mailbox
	DSN              DSNOptions
	StoragePath      string
	Parsed           message.ParsedMessage
	Authentication   AuthenticationResults
	ReceivedAt       time.Time
	Size             int64
	FolderSystemType string
	SpamScore        *float64
}

type DSNOptions struct {
	Return     string
	EnvelopeID string
	Recipients []DSNRecipientOptions
}

type DSNRecipientOptions struct {
	Address           string
	Notify            []string
	OriginalRecipient string
}

type ReceiverOptions struct {
	Store              storage.Store
	Resolver           RecipientResolver
	Recorder           MessageRecorder
	Deduplicator       Deduplicator
	RateLimiter        RateLimiter
	Backpressure       Backpressure
	AuthVerifier       AuthenticationVerifier
	Authenticator      Authenticator
	RelayAuthorizer    RelayAuthorizer
	DomainPolicyLookup DomainPolicyLookup
	Metrics            Metrics
	Logger             *slog.Logger // nil → slog.Default()
	RequireAuth        bool
	DMARCEnforce       bool
	SupportSMTPUTF8    bool
	SupportRequireTLS  bool
	SupportDSN         bool
	SupportBinaryMIME  bool
	AddReceivedHeader  bool
	ReceivedDomain     string
	Hooks              []Hook
	Policy             ReceivePolicy
	IDGenerator        IDGenerator
	Clock              func() time.Time
	MaxMessageBytes    int64
	BulkSenderLimiter  *BulkSenderLimiter // nil disables bulk sender rate limiting
	LatencyTracker     *LatencyTracker    // nil disables latency tracking
}

type Receiver struct {
	store              storage.Store
	resolver           RecipientResolver
	recorder           MessageRecorder
	deduplicator       Deduplicator
	rateLimiter        RateLimiter
	backpressure       Backpressure
	authVerifier       AuthenticationVerifier
	authenticator      Authenticator
	relayAuthorizer    RelayAuthorizer
	domainPolicyLookup DomainPolicyLookup
	metrics            Metrics
	logger             *slog.Logger
	requireAuth        bool
	dmarcEnforce       bool
	supportSMTPUTF8    bool
	supportRequireTLS  bool
	supportDSN         bool
	supportBinaryMIME  bool
	addReceivedHeader  bool
	receivedDomain     string
	hooks              []Hook
	policy             ReceivePolicy
	idGenerator        IDGenerator
	clock              func() time.Time
	bulkSenderLimiter  *BulkSenderLimiter
	latencyTracker     *LatencyTracker
	authFailures       *authFailureTracker
	// baseCtx is the parent context for all sessions created by NewSession.
	// When set via SetBaseContext (e.g. wired to the server's shutdown ctx),
	// in-flight session work is cancelled when the server starts shutting down.
	baseCtx context.Context
}

// SetBaseContext sets the parent context for all subsequently-created sessions.
// Pass the server's lifecycle/shutdown context so in-flight sessions are
// notified when the server begins shutting down.
func (r *Receiver) SetBaseContext(ctx context.Context) {
	if ctx == nil {
		return
	}
	r.baseCtx = ctx
}

func NewReceiver(opts ReceiverOptions) *Receiver {
	idGenerator := opts.IDGenerator
	if idGenerator == nil {
		idGenerator = randomMessageID
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Receiver{
		store:              opts.Store,
		resolver:           opts.Resolver,
		recorder:           recorderOrDefault(opts.Recorder),
		deduplicator:       deduplicatorOrDefault(opts.Deduplicator),
		rateLimiter:        rateLimiterOrDefault(opts.RateLimiter),
		backpressure:       backpressureOrDefault(opts.Backpressure),
		authVerifier:       opts.AuthVerifier,
		authenticator:      opts.Authenticator,
		relayAuthorizer:    opts.RelayAuthorizer,
		domainPolicyLookup: opts.DomainPolicyLookup,
		metrics:            metricsOrDefault(opts.Metrics),
		logger:             logger,
		requireAuth:        opts.RequireAuth,
		dmarcEnforce:       opts.DMARCEnforce,
		supportSMTPUTF8:    opts.SupportSMTPUTF8,
		supportRequireTLS:  opts.SupportRequireTLS,
		supportDSN:         opts.SupportDSN,
		supportBinaryMIME:  opts.SupportBinaryMIME,
		addReceivedHeader:  opts.AddReceivedHeader,
		receivedDomain:     opts.ReceivedDomain,
		hooks:              append([]Hook(nil), opts.Hooks...),
		policy:             normalizePolicy(opts.Policy, opts.MaxMessageBytes),
		idGenerator:        idGenerator,
		clock:              clockOrDefault(opts.Clock),
		bulkSenderLimiter:  opts.BulkSenderLimiter,
		latencyTracker:     opts.LatencyTracker,
		authFailures:       newAuthFailureTracker(),
	}
}

func (r *Receiver) NewSession(conn *gosmtp.Conn) (gosmtp.Session, error) {
	if r.store == nil {
		return nil, fmt.Errorf("smtp receiver store is required")
	}
	if r.resolver == nil {
		return nil, fmt.Errorf("smtp receiver resolver is required")
	}
	parent := r.baseCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	return &session{receiver: r, remoteAddr: remoteAddrFromConn(conn), ctx: ctx, cancel: cancel, startedAt: time.Now()}, nil
}

type session struct {
	receiver              *Receiver
	ctx                   context.Context
	cancel                context.CancelFunc
	from                  string
	mailStarted           bool
	recipients            []Mailbox
	dsn                   DSNOptions
	smtpUTF8              bool
	remoteAddr            string
	authenticated         bool
	authenticatedUser     string // set after successful PLAIN auth
	authenticatedUserRole string
	domainPolicy          *InboundDomainPolicy
	startedAt             time.Time
}

func (s *session) Mail(from string, opts *gosmtp.MailOptions) (err error) {
	defer func() {
		s.observe(s.ctx, MetricEvent{
			Stage:        StageMailFrom,
			Result:       metricResult(err),
			EnvelopeFrom: from,
			Error:        metricError(err),
		})
	}()
	if s.receiver.requireAuth && !s.authenticated {
		return gosmtp.ErrAuthRequired
	}
	if err := s.authorizeRelay(); err != nil {
		return err
	}
	if s.receiver.bulkSenderLimiter != nil && s.authenticatedUser != "" {
		if !s.receiver.bulkSenderLimiter.AllowSubmission(s.authenticatedUser, s.authenticatedUserRole) {
			return smtpRateLimited(from)
		}
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
	normalized, err := normalizeReversePath(from)
	if err != nil {
		return err
	}
	if err := validateSMTPUTF8Address(from, normalized, mailOptionsUTF8(opts), s.receiver.supportSMTPUTF8); err != nil {
		return err
	}
	s.from = normalized
	s.mailStarted = true
	s.smtpUTF8 = mailOptionsUTF8(opts)
	s.dsn.Return = normalizeDSNReturn(opts)
	s.dsn.EnvelopeID = normalizeDSNEnvelopeID(opts)
	return nil
}

func (s *session) Rcpt(to string, opts *gosmtp.RcptOptions) (err error) {
	defer func() {
		s.observe(s.ctx, MetricEvent{
			Stage:     StageRcpt,
			Result:    metricResult(err),
			Recipient: to,
			Error:     metricError(err),
		})
	}()
	if s.receiver.requireAuth && !s.authenticated {
		return gosmtp.ErrAuthRequired
	}
	if !s.mailStarted {
		return smtpBadSequence("RCPT")
	}
	if err := validateRcptOptions(opts, extensionSupport{DSN: s.receiver.supportDSN}); err != nil {
		return err
	}
	allowed, err := s.receiver.rateLimiter.Allow(s.ctx, RateLimitKey{
		Stage:      StageRcpt,
		RemoteAddr: s.remoteAddr,
		Recipient:  to,
	})
	if err != nil {
		return fmt.Errorf("check rcpt rate limit: %w", err)
	}
	if !allowed {
		return smtpRateLimited(to)
	}

	mailbox, err := s.receiver.resolver.ResolveRecipient(s.ctx, to)
	if err != nil {
		return smtpMailboxUnavailable("recipient %q not found", to)
	}
	if err := validateSMTPUTF8Address(to, mailbox.Address, s.smtpUTF8, s.receiver.supportSMTPUTF8); err != nil {
		return err
	}

	nextDomainPolicy := s.domainPolicy
	if s.receiver.domainPolicyLookup != nil {
		dp, lookupErr := s.receiver.domainPolicyLookup.InboundDomainPolicy(s.ctx, mailbox.DomainID)
		if lookupErr != nil {
			return smtpPolicyTempfail("domain policy lookup failed for recipient %q", mailbox.Address)
		}
		nextDomainPolicy = mergeInboundDomainPolicy(nextDomainPolicy, dp)
	}
	maxRecipients := effectiveMaxRecipients(s.receiver.policy.MaxRecipientsPerMessage, nextDomainPolicy)
	if len(s.recipients) >= maxRecipients {
		return smtpTooManyRecipients(maxRecipients)
	}

	s.domainPolicy = nextDomainPolicy
	s.recipients = append(s.recipients, mailbox)
	s.dsn.Recipients = upsertDSNRecipientOption(s.dsn.Recipients, normalizeDSNRecipientOptions(mailbox.Address, opts))
	return nil
}

func (s *session) Data(r io.Reader) (err error) {
	envelopeFrom := s.from
	recipients := mailboxAddresses(s.recipients)
	var observedSize int64
	defer func() {
		s.observe(s.ctx, MetricEvent{
			Stage:        StageRecorded,
			Result:       metricResult(err),
			EnvelopeFrom: envelopeFrom,
			Recipients:   recipients,
			Size:         observedSize,
			Error:        metricError(err),
		})
	}()
	if s.receiver.requireAuth && !s.authenticated {
		return gosmtp.ErrAuthRequired
	}
	if !s.mailStarted {
		return smtpBadSequence("DATA")
	}
	if len(s.recipients) == 0 {
		return smtpBadSequence("DATA")
	}
	envelopeFrom = s.from
	recipients = mailboxAddresses(s.recipients)
	defer s.Reset()

	// Start latency tracing if enabled (messageID not yet known, use placeholder)
	dataStart := time.Now()
	_ = dataStart

	accepted, err := s.receiver.backpressure.Accept(s.ctx)
	if err != nil {
		return fmt.Errorf("check backpressure: %w", err)
	}
	if !accepted {
		return smtpBackpressure()
	}
	if err := s.emit(s.ctx, Event{
		Stage:        StageBackpressureChecked,
		RemoteAddr:   s.remoteAddr,
		EnvelopeFrom: s.from,
		Recipients:   mailboxAddresses(s.recipients),
		DSN:          s.currentDSNOptions(),
	}); err != nil {
		return err
	}

	spooled, size, err := spoolMessage(r, effectiveMaxBytes(s.receiver.policy.MaxMessageBytes, s.domainPolicy))
	if err != nil {
		return err
	}
	observedSize = size
	spooledAt := time.Now()
	messageID := s.receiver.idGenerator()
	receivedAt := s.receiver.clock()
	headerBuffer := NewHeaderBuffer()

	// Start latency tracing after spooling.
	var trace *MessageTracing
	if s.receiver.latencyTracker != nil {
		trace = s.receiver.latencyTracker.StartTracingMessage(messageID, s.from, mailboxAddresses(s.recipients))
		trace.RecordPhaseLatency("spooled", spooledAt.Sub(dataStart))
		defer func() {
			if trace != nil {
				trace.RecordCompletion()
				s.receiver.latencyTracker.StoreTrace(trace)
			}
		}()
	}

	if s.receiver.addReceivedHeader {
		headerBuffer.AddPrepend(BuildReceivedHeader(s.remoteAddr, s.receiver.receivedDomain, messageID, receivedAt))
	}
	defer cleanupSpool(spooled)
	if err := s.emit(s.ctx, Event{
		Stage:        StageSpooled,
		RemoteAddr:   s.remoteAddr,
		EnvelopeFrom: s.from,
		Recipients:   mailboxAddresses(s.recipients),
		DSN:          s.currentDSNOptions(),
		SpoolPath:    spooled.Name(),
		Size:         size,
	}); err != nil {
		return err
	}

	if _, err := spooled.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind spooled message for parse: %w", err)
	}
	parsed, err := message.ParseEMLWithOptions(spooled, message.ParseOptions{MaxTextBodyBytes: 64 << 10})
	if err != nil {
		return fmt.Errorf("parse smtp message: %w", err)
	}
	if parsed.MessageID == "" {
		parsed.MessageID = message.FallbackMessageID(s.from, mailboxAddresses(s.recipients), parsed.Date, parsed.Subject)
		headerBuffer.AddAfterTrace("Message-ID: " + parsed.MessageID + "\r\n")
	}
	if trace != nil {
		trace.RecordPhaseLatency("parsed", time.Since(spooledAt))
	}
	if err := s.emit(s.ctx, Event{
		Stage:        StageParsed,
		RemoteAddr:   s.remoteAddr,
		EnvelopeFrom: s.from,
		Recipients:   mailboxAddresses(s.recipients),
		DSN:          s.currentDSNOptions(),
		SpoolPath:    spooled.Name(),
		Parsed:       parsed,
		Size:         size,
	}); err != nil {
		return err
	}

	authCtx := context.WithValue(s.ctx, authenticationRawMessageKey{}, spooled)
	authResults, err := s.verifyAuthentication(authCtx, parsed, size)
	if err != nil {
		return err
	}
	dmarcQuarantine, err := enforceDMARCPolicy(s.receiver.dmarcEnforce, authResults)
	if err != nil {
		return err
	}
	if s.receiver.authVerifier != nil {
		if err := s.emit(s.ctx, Event{
			Stage:          StageAuthenticationChecked,
			RemoteAddr:     s.remoteAddr,
			EnvelopeFrom:   s.from,
			Recipients:     mailboxAddresses(s.recipients),
			DSN:            s.currentDSNOptions(),
			Parsed:         parsed,
			Authentication: authResults,
			ReceivedAt:     receivedAt,
			Size:           size,
		}); err != nil {
			return err
		}
		headerBuffer.AddAfterTrace(FormatAuthenticationResults(authResults))
	}

	// Apply all buffered headers in a single pass to avoid multiple file rewrites
	updated, updatedSize, err := headerBuffer.ApplyToFile(spooled)
	if err != nil {
		return fmt.Errorf("apply headers: %w", err)
	}
	cleanupSpool(spooled)
	spooled = updated
	size = updatedSize
	observedSize = size

	for _, recipient := range s.recipients {
		shouldProcess, err := s.receiver.deduplicator.CheckAndSet(s.ctx, DedupKey{
			MessageID: parsed.MessageID,
			Recipient: recipient.Address,
		})
		if err != nil {
			return fmt.Errorf("check duplicate message for %s: %w", recipient.Address, err)
		}
		if err := s.emit(s.ctx, Event{
			Stage:          StageDedupChecked,
			RemoteAddr:     s.remoteAddr,
			EnvelopeFrom:   s.from,
			Mailbox:        recipient,
			Recipients:     mailboxAddresses(s.recipients),
			DSN:            s.currentDSNOptions(),
			Parsed:         parsed,
			Authentication: authResults,
			ReceivedAt:     receivedAt,
			Size:           size,
			Duplicate:      !shouldProcess,
		}); err != nil {
			return err
		}
		if !shouldProcess {
			continue
		}

		path := BuildStoragePath(recipient, messageID, receivedAt)
		if _, err := spooled.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("rewind spooled message for store: %w", err)
		}
		storeStart := time.Now()
		if err := s.receiver.store.Put(s.ctx, path, spooled); err != nil {
			return fmt.Errorf("store message for %s: %w", recipient.Address, err)
		}
		if trace != nil {
			trace.RecordPhaseLatency("stored", time.Since(storeStart))
		}
		if err := s.emit(s.ctx, Event{
			Stage:          StageStored,
			RemoteAddr:     s.remoteAddr,
			EnvelopeFrom:   s.from,
			Mailbox:        recipient,
			Recipients:     mailboxAddresses(s.recipients),
			DSN:            s.currentDSNOptions(),
			StoragePath:    path,
			Parsed:         parsed,
			Authentication: authResults,
			ReceivedAt:     receivedAt,
			Size:           size,
		}); err != nil {
			s.deleteStoredMessage(path)
			return err
		}
		folderSystemType := "inbox"
		if dmarcQuarantine {
			folderSystemType = "spam"
		}
		dbMessageID, recordErr := s.receiver.recorder.Record(s.ctx, ReceivedMessage{
			EnvelopeFrom:     s.from,
			Mailbox:          recipient,
			DSN:              s.currentDSNOptions(),
			StoragePath:      path,
			Parsed:           parsed,
			Authentication:   authResults,
			ReceivedAt:       receivedAt,
			Size:             size,
			FolderSystemType: folderSystemType,
		})
		if recordErr != nil {
			s.deleteStoredMessage(path)
			if errors.Is(recordErr, mail.ErrMailboxFull) {
				return smtpMailboxFull(recipient.Address)
			}
			return fmt.Errorf("record message for %s: %w", recipient.Address, recordErr)
		}
		s.receiver.logger.Info("smtp message accepted",
			"smtp_id", messageID,
			"message_id", dbMessageID,
			"envelope_from", s.from,
			"recipient", recipient.Address,
			"rfc_message_id", parsed.MessageID,
			"remote_addr", s.remoteAddr,
			"size_bytes", size,
		)
		if trace != nil {
			trace.RecordPhaseLatency("recorded", time.Since(storeStart))
		}
		if err := s.emit(s.ctx, Event{
			Stage:          StageRecorded,
			RemoteAddr:     s.remoteAddr,
			EnvelopeFrom:   s.from,
			Mailbox:        recipient,
			Recipients:     mailboxAddresses(s.recipients),
			DSN:            s.currentDSNOptions(),
			StoragePath:    path,
			Parsed:         parsed,
			Authentication: authResults,
			ReceivedAt:     receivedAt,
			Size:           size,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) deleteStoredMessage(path string) {
	if strings.TrimSpace(path) == "" || s.receiver.store == nil {
		return
	}
	if err := s.receiver.store.Delete(s.ctx, path); err != nil {
		logger := s.receiver.logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("failed to delete stored smtp message after rollback", "storage_path", path, "remote_addr", s.remoteAddr, "error", err)
	}
}

func (s *session) verifyAuthentication(ctx context.Context, parsed message.ParsedMessage, size int64) (AuthenticationResults, error) {
	if s.receiver.authVerifier == nil {
		return AuthenticationResults{}, nil
	}
	if spooled, ok := ctx.Value(authenticationRawMessageKey{}).(*os.File); ok {
		if _, err := spooled.Seek(0, io.SeekStart); err != nil {
			return AuthenticationResults{}, fmt.Errorf("rewind spooled message for authentication: %w", err)
		}
	}
	results, err := s.receiver.authVerifier.VerifyAuthentication(ctx, AuthenticationRequest{
		RemoteAddr:   s.remoteAddr,
		EnvelopeFrom: s.from,
		Recipients:   mailboxAddresses(s.recipients),
		Parsed:       parsed,
		RawMessage:   rawMessageFromContext(ctx),
		Size:         size,
	})
	if err != nil {
		return AuthenticationResults{}, fmt.Errorf("verify smtp authentication results: %w", err)
	}
	return results, nil
}

type authenticationRawMessageKey struct{}

func rawMessageFromContext(ctx context.Context) io.Reader {
	raw, _ := ctx.Value(authenticationRawMessageKey{}).(io.Reader)
	return raw
}

func mailboxAddresses(mailboxes []Mailbox) []string {
	addresses := make([]string, 0, len(mailboxes))
	for _, mailbox := range mailboxes {
		addresses = append(addresses, mailbox.Address)
	}
	return addresses
}

func normalizeReversePath(from string) (string, error) {
	if strings.TrimSpace(strings.Trim(from, "<>")) == "" {
		return "", nil
	}
	return mail.NormalizeAddress(from)
}

func (s *session) currentDSNOptions() DSNOptions {
	return cloneDSNOptions(s.dsn)
}

func (s *session) emit(ctx context.Context, event Event) error {
	if event.RemoteAddr == "" {
		event.RemoteAddr = s.remoteAddr
	}
	for _, hook := range s.receiver.hooks {
		if err := hook(ctx, event); err != nil {
			return fmt.Errorf("smtp hook %s: %w", event.Stage, err)
		}
	}
	return nil
}

func (s *session) observe(ctx context.Context, event MetricEvent) {
	event.RemoteAddr = s.remoteAddr
	s.receiver.metrics.ObserveSMTP(ctx, event)
}

func metricResult(err error) MetricResult {
	if err != nil {
		return MetricRejected
	}
	return MetricAccepted
}

func metricError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (s *session) Reset() {
	s.clearEnvelope()
}

func (s *session) clearEnvelope() {
	s.from = ""
	s.mailStarted = false
	s.recipients = nil
	s.dsn = DSNOptions{}
	s.smtpUTF8 = false
	s.domainPolicy = nil
}

func (s *session) Logout() error {
	s.Reset()
	s.authenticated = false
	s.authenticatedUserRole = ""
	dur := time.Since(s.startedAt).Seconds()
	s.observe(s.ctx, MetricEvent{
		Stage:    StageLogout,
		Result:   MetricAccepted,
		Duration: dur,
	})
	// Cancel the current context (signals any in-flight work to stop),
	// then replace it so the session remains usable if reused. Parent on the
	// receiver's base context so shutdown still propagates to the replacement.
	s.cancel()
	parent := s.receiver.baseCtx
	if parent == nil {
		parent = context.Background()
	}
	s.ctx, s.cancel = context.WithCancel(parent)
	return nil
}
